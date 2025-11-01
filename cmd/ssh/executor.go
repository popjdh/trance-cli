package ssh

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"trance-cli/internal/logging"

	"github.com/spf13/cobra"
)

type Executor struct {
	logger logging.Logger
	DryRun bool
	Proxy  bool
}

func (executor *Executor) Run(cmd *cobra.Command, args []string) {
	executor.logger = logging.Logger{
		OutWriter: cmd.OutOrStdout(),
		ErrWriter: cmd.ErrOrStderr(),
		State:     logging.LoggerStateNewLine,
	}

	var host string
	var wrapperArgs []string
	var sshOptions []string
	var remoteCommandArgs []string

	var separatorIndexes []int
	for i, arg := range args {
		if arg == "--" {
			separatorIndexes = append(separatorIndexes, i)
		}
	}
	switch len(separatorIndexes) {
	case 0:
		wrapperArgs = args
	case 1:
		wrapperArgs = args[:separatorIndexes[0]]
		sshOptions = args[separatorIndexes[0]+1:]
	default:
		wrapperArgs = args[:separatorIndexes[0]]
		sshOptions = args[separatorIndexes[0]+1 : separatorIndexes[1]]
		remoteCommandArgs = args[separatorIndexes[1]+1:]
	}

	for _, arg := range wrapperArgs {
		if arg == "-n" || arg == "--dry-run" {
			executor.DryRun = true
		} else if arg == "-p" || arg == "--proxy" {
			executor.Proxy = true
		} else if arg == "-h" || arg == "--help" {
			err := cmd.Help()
			if err != nil {
				executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
			}
			os.Exit(0)
		} else if len(arg) > 0 && arg[0] == '-' {
			executor.logger.PrintfErr(logging.LogModeAppend, true, "未知参数: %s", arg)
		} else {
			if host != "" {
				executor.logger.PrintfErr(logging.LogModeAppend, true, "多个主机, 保留最后一个: %s", arg)
			}
			host = arg
		}
	}
	if host == "" {
		hosts, err := executor.collectHosts()
		if err != nil {
			executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
			os.Exit(1)
		}
		if len(hosts) == 0 {
			executor.logger.PrintfErr(logging.LogModeAppend, true, "未找到可供选择的主机")
			os.Exit(1)
		}
		selectedHost, err := RunSelector(hosts)
		if err != nil {
			executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
			os.Exit(1)
		}
		host = selectedHost
	}
	if executor.Proxy {
		err := executor.executeSSHProxy(host, sshOptions)
		if err != nil {
			executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
		}
	}
	err := executor.executeSSH(host, sshOptions, remoteCommandArgs)
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
	}
}

func (executor *Executor) collectHosts() ([]string, error) {
	hostSet := make(map[string]struct{})
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("获取当前用户失败\n%w", err)
	}
	homeDir := usr.HomeDir
	configPaths := []string{
		filepath.Join(homeDir, ".ssh", "config"),
		"/etc/ssh/ssh_config",
	}
	configDPath := filepath.Join(homeDir, ".ssh", "config.d")
	if entries, err := os.ReadDir(configDPath); err == nil {
		for _, entry := range entries {
			configPaths = append(configPaths, filepath.Join(configDPath, entry.Name()))
		}
	}
	for _, path := range configPaths {
		err := parseSSHConfig(path, hostSet)
		if err != nil {
			return nil, err
		}
	}
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	err = parseKnownHosts(knownHostsPath, hostSet)
	if err != nil {
		return nil, err
	}
	err = parseEtcHosts(hostSet)
	if err != nil {
		return nil, err
	}
	hosts := make([]string, 0, len(hostSet))
	for host := range hostSet {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts, nil
}

var hostPattern = regexp.MustCompile(`(?i)^\s*host(name)?\s+`)

func parseSSHConfig(path string, hostSet map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取 SSH 配置文件失败: %s\n%w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if hostPattern.MatchString(line) {
			fieldsStr := hostPattern.ReplaceAllString(line, "")
			fields := strings.Fields(fieldsStr)
			for _, host := range fields {
				if !strings.ContainsAny(host, "*?%") {
					hostSet[host] = struct{}{}
				}
			}
		}
	}
	return nil
}

func parseKnownHosts(path string, hostSet map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取已知主机文件失败: %s\n%w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), " ", 2)
		if len(fields) > 0 {
			hosts := strings.Split(fields[0], ",")
			for _, host := range hosts {
				host = strings.TrimPrefix(host, "[")
				host = strings.SplitN(host, "]:", 2)[0]
				hostSet[host] = struct{}{}
			}
		}
	}
	return nil
}

func parseEtcHosts(hostSet map[string]struct{}) error {
	file, err := os.Open("/etc/hosts")
	if err != nil {
		return fmt.Errorf("读取 /etc/hosts 文件失败\n%w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if commentIdx := strings.Index(line, "#"); commentIdx != -1 {
			line = line[:commentIdx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 1 {
			for _, host := range fields[1:] {
				hostSet[host] = struct{}{}
			}
		}
	}
	return nil
}

func (executor *Executor) executeSSH(host string, sshOptions []string, remoteCommandArgs []string) error {
	finalSSHArgs := []string{host}
	finalSSHArgs = append(sshOptions, finalSSHArgs...)
	finalSSHArgs = append(finalSSHArgs, remoteCommandArgs...)
	return executor.runSSH(finalSSHArgs...)
}

func (executor *Executor) executeSSHProxy(host string, sshOptions []string) error {
	finalSSHArgs := []string{"-CqTNn", "-D", "0.0.0.0:1080"}
	finalSSHArgs = append(sshOptions, finalSSHArgs...)
	finalSSHArgs = append(finalSSHArgs, host)
	return executor.runSSH(finalSSHArgs...)
}

func (executor *Executor) runSSH(args ...string) error {
	if executor.DryRun {
		executor.logger.PrintfOut(logging.LogModeAppend, true, "ssh %s", strings.Join(args, " "))
		return nil
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("SSH 命令执行失败: ssh %s\n%w", strings.Join(args, " "), err)
	}
	return nil
}

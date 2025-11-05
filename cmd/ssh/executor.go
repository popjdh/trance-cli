package ssh

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"trance-cli/internal/logging"

	"github.com/kevinburke/ssh_config"
	"github.com/spf13/cobra"
)

type Host struct {
	Alias     string
	Hostname  string
	Port      string
	User      string
	ProxyJump string
	Source    string
}

func (host Host) FilterValue() string {
	return host.Alias
}

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

	var hostArg string
	var wrapperArgs []string
	var sshOptions []string
	var remoteCommandArgs []string

	// 解析参数
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
			if hostArg != "" {
				executor.logger.PrintfErr(logging.LogModeAppend, true, "多个主机, 保留最后一个: %s", arg)
			}
			hostArg = arg
		}
	}
	// 收集可用主机
	hosts, err := executor.collectSSHHosts()
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
		os.Exit(1)
	}
	if hostArg == "" && len(hosts) == 0 {
		executor.logger.PrintfErr(logging.LogModeAppend, true, "未找到可供选择的主机, 请检查配置")
		os.Exit(1)
	}
	// 代理模式下忽略用户传入的 sshOptions remoteCommandArgs
	if executor.Proxy {
		sshOptions = []string{}
		sshOptions = append(sshOptions, "-CqTNn", "-D", "0.0.0.0:1080")
		remoteCommandArgs = []string{}
	}
	// 初始参数传递给 TUI
	tuiResult, err := RunSelector(hosts, hostArg, sshOptions, remoteCommandArgs)
	if err != nil {
		// 用户可能按 ESC 退出, 此时不应视为错误
		if err.Error() != "未选择主机" {
			executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
		}
		os.Exit(1)
	}

	finalUsr, finalServer, finalPort := parseHostString(tuiResult.HostStr)
	finalProxyJump := tuiResult.ProxyJump
	finalSshOptions := tuiResult.SshOptions
	finalRemoteCommandArgs := tuiResult.RemoteCommandArgs

	err = executor.runSSH(finalSshOptions, finalUsr, finalServer, finalPort, finalProxyJump, finalRemoteCommandArgs)
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
	}
}

func (executor *Executor) collectSSHHosts() ([]Host, error) {
	// Alias 去重
	hostMap := make(map[string]Host)
	// 获取当前用户主目录
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("获取当前用户失败\n%w", err)
	}
	homeDir := usr.HomeDir
	// 解析 ssh_config
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
		err := parseSSHConfig(path, hostMap)
		if err != nil {
			executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
		}
	}
	// 解析 known_hosts
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	err = parseKnownHosts(knownHostsPath, hostMap)
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
	}
	// 解析 etc_hosts
	err = parseEtcHosts(hostMap)
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, err.Error())
	}
	// 转换 map 为 slice
	hosts := make([]Host, 0, len(hostMap))
	for _, host := range hostMap {
		hosts = append(hosts, host)
	}

	// 按照 Source 分组排序, 组内按 Alias 排序
	sourceOrder := map[string]int{
		"ssh_config":  0,
		"etc_hosts":   1,
		"known_hosts": 2,
	}
	sort.Slice(hosts, func(i, j int) bool {
		orderI, okI := sourceOrder[hosts[i].Source]
		if !okI {
			orderI = 99 // 其他未知来源排在最后
		}
		orderJ, okJ := sourceOrder[hosts[j].Source]
		if !okJ {
			orderJ = 99
		}

		if orderI != orderJ {
			return orderI < orderJ
		}
		return hosts[i].Alias < hosts[j].Alias
	})

	return hosts, nil
}

func parseSSHConfig(path string, hostMap map[string]Host) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取 SSH 配置文件失败: %s\n%w", path, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(f)

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return fmt.Errorf("解析 SSH 配置文件失败: %s\n%w", path, err)
	}
	for _, host := range cfg.Hosts {
		for _, p := range host.Patterns {
			alias := p.String()
			// 忽略通配符
			if strings.ContainsAny(alias, "*?") {
				continue
			}
			// 已存在更高优先级的配置, 跳过
			if _, ok := hostMap[alias]; ok {
				continue
			}

			hostname, _ := cfg.Get(alias, "HostName")
			if hostname == "" {
				hostname = alias
			}

			port, _ := cfg.Get(alias, "Port")
			usr, _ := cfg.Get(alias, "User")
			proxy, _ := cfg.Get(alias, "ProxyJump")

			hostMap[alias] = Host{
				Alias:     alias,
				Hostname:  hostname,
				Port:      port,
				User:      usr,
				ProxyJump: proxy,
				Source:    "ssh_config",
			}
		}
	}
	return nil
}

func parseKnownHosts(path string, hostMap map[string]Host) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取 known_hosts 文件失败: %s\n%w", path, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), " ", 2)
		if len(fields) > 0 {
			hostsStr := strings.Split(fields[0], ",")
			for _, hostStr := range hostsStr {
				// 跳过 Hashed Hostnames
				if strings.HasPrefix(hostStr, "|") {
					continue
				}
				hostname := hostStr
				port := ""
				// known_hosts 中 IPv6 地址格式为 [hostname]:port
				if strings.HasPrefix(hostname, "[") && strings.Contains(hostname, "]:") {
					parts := strings.SplitN(strings.TrimPrefix(hostname, "["), "]:", 2)
					if len(parts) == 2 {
						hostname = parts[0]
						port = parts[1]
					}
				}
				if _, ok := hostMap[hostname]; !ok {
					hostMap[hostname] = Host{
						Alias:    hostname,
						Hostname: hostname,
						Port:     port,
						Source:   "known_hosts",
					}
				}
			}
		}
	}
	return nil
}

func parseEtcHosts(hostMap map[string]Host) error {
	file, err := os.Open("/etc/hosts")
	if err != nil {
		return fmt.Errorf("读取 /etc/hosts 文件失败\n%w", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if commentIdx := strings.Index(line, "#"); commentIdx != -1 {
			line = line[:commentIdx]
		}
		fields := strings.Fields(line)
		if len(fields) > 1 {
			for _, hostname := range fields[1:] {
				if _, ok := hostMap[hostname]; !ok {
					hostMap[hostname] = Host{
						Alias:    hostname,
						Hostname: hostname,
						Source:   "etc_hosts",
					}
				}
			}
		}
	}
	return nil
}

// 按 <user>@<server>:<port> 进行解析
func parseHostString(hostStr string) (usr, server, port string) {
	if i := strings.LastIndex(hostStr, "@"); i != -1 {
		usr = hostStr[:i]
		hostStr = hostStr[i+1:]
	}
	// IPv6 按照 [::1]:22 形式处理
	if i := strings.LastIndex(hostStr, ":"); i != -1 {
		// 确保冒号不是IPv6地址的一部分
		if !strings.Contains(hostStr, "]") || i > strings.LastIndex(hostStr, "]") {
			server = hostStr[:i]
			port = hostStr[i+1:]
			return
		}
	}
	server = hostStr
	server = strings.Trim(server, "[]")
	return
}

func (executor *Executor) runSSH(sshOptions []string, user string, server string, port string, proxyJump string, remoteCommandArgs []string) error {
	finalArgs := []string{}
	finalArgs = append(finalArgs, sshOptions...)
	if user != "" {
		finalArgs = append(finalArgs, "-o", fmt.Sprintf("User=%s", user))
	}
	if port != "" {
		finalArgs = append(finalArgs, "-o", fmt.Sprintf("Port=%s", port))
	}
	if proxyJump != "" {
		finalArgs = append(finalArgs, "-o", fmt.Sprintf("ProxyJump=%s", proxyJump))
	}
	finalArgs = append(finalArgs, server)
	finalArgs = append(finalArgs, remoteCommandArgs...)
	executor.logger.PrintfOut(logging.LogModeAppend, true, "ssh %s", strings.Join(finalArgs, " "))
	if executor.DryRun {
		return nil
	}
	cmd := exec.Command("ssh", finalArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

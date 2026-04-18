package unpack

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"trance-cli/internal/logging"
	"unicode/utf8"

	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type Executor struct {
	logger       logging.Logger
	Verbose      bool
	Recursive    bool
	Passwords    []string
	PasswordFile string
}

type ArchiveType int

const (
	Unknown ArchiveType = iota
	SevenZ
	Rar
	Zip
	Tar
	TarBz2
	TarZ
	TarGz
	TarLz4
	TarLz
	TarLzma
	TarXz
	TarZst
)

func (aType ArchiveType) String() string {
	switch aType {
	case Unknown:
		return "Unknown"
	case SevenZ:
		return "7z"
	case Rar:
		return "rar"
	case Zip:
		return "zip"
	case Tar:
		return "tar"
	case TarBz2:
		return "tar.bz2"
	case TarZ:
		return "tar.z"
	case TarGz:
		return "tar.gz"
	case TarLz4:
		return "tar.lz4"
	case TarLz:
		return "tar.lz"
	case TarLzma:
		return "tar.lzma"
	case TarXz:
		return "tar.xz"
	case TarZst:
		return "tar.zst"
	default:
		return fmt.Sprintf("ArchiveType(%d)", aType)
	}
}

var (
	regex7z      = regexp.MustCompile(`(?i)\.7z$`)             // .7z
	regex7zVol   = regexp.MustCompile(`(?i)\.7z\.(\d{3,})$`)   // .7z.001, .7z.002, ...
	regexRar     = regexp.MustCompile(`(?i)\.rar$`)            // .rar
	regexRarVol  = regexp.MustCompile(`(?i)\.part(\d+)\.rar$`) // .part1.rar, .part2.rar, ...
	regexRVol    = regexp.MustCompile(`(?i)\.r\d{2,}$`)        // .r00, .r01, ...
	regexZip     = regexp.MustCompile(`(?i)\.zip$`)            // .zip
	regexZipVol  = regexp.MustCompile(`(?i)\.zip\.(\d{3,})$`)  // .zip.001, .zip.002, ...
	regexZVol    = regexp.MustCompile(`(?i)\.z\d{2,}$`)        // .z01, .z02, ...
	regexTar     = regexp.MustCompile(`(?i)\.tar$`)            // .tar
	regexTarBz2  = regexp.MustCompile(`(?i)\.tar\.bz2$`)       // .tar.bz2
	regexTarZ    = regexp.MustCompile(`(?i)\.tar\.z$`)         // .tar.z
	regexTarGz   = regexp.MustCompile(`(?i)\.tar\.gz$`)        // .tar.gz
	regexTarLz4  = regexp.MustCompile(`(?i)\.tar\.lz4$`)       // .tar.lz4
	regexTarLz   = regexp.MustCompile(`(?i)\.tar\.lz$`)        // .tar.lz
	regexTarLzma = regexp.MustCompile(`(?i)\.tar\.lzma$`)      // .tar.lzma
	regexTarXz   = regexp.MustCompile(`(?i)\.tar\.xz$`)        // .tar.xz
	regexTarZst  = regexp.MustCompile(`(?i)\.tar\.zst$`)       // .tar.zst
)

func detectArchiveType(filePath string) (ArchiveType, bool, string) {
	fileName := filepath.Base(filePath)
	switch {
	case regex7zVol.MatchString(fileName):
		if m := regex7zVol.FindStringSubmatch(fileName); m != nil {
			return SevenZ, (m[1] == "001"), fileName[:len(fileName)-len(m[0])]
		}
	case regex7z.MatchString(fileName):
		return SevenZ, true, strings.TrimSuffix(fileName, ".7z")
	case regexRarVol.MatchString(fileName):
		if m := regexRarVol.FindStringSubmatch(fileName); m != nil {
			volNum := strings.TrimLeft(m[1], "0")
			isMain := (volNum == "1" || volNum == "")
			return Rar, isMain, fileName[:len(fileName)-len(m[0])]
		}
	case regexRar.MatchString(fileName):
		return Rar, true, strings.TrimSuffix(fileName, ".rar")
	case regexRVol.MatchString(fileName):
		return Rar, false, strings.TrimSuffix(fileName, filepath.Ext(fileName))
	case regexZipVol.MatchString(fileName):
		if m := regexZipVol.FindStringSubmatch(fileName); m != nil {
			return Zip, (m[1] == "001"), fileName[:len(fileName)-len(m[0])]
		}
	case regexZip.MatchString(fileName):
		return Zip, true, strings.TrimSuffix(fileName, ".zip")
	case regexZVol.MatchString(fileName):
		return Zip, false, strings.TrimSuffix(fileName, filepath.Ext(fileName))
	case regexTar.MatchString(fileName):
		return Tar, true, strings.TrimSuffix(fileName, ".tar")
	case regexTarBz2.MatchString(fileName):
		return TarBz2, true, strings.TrimSuffix(fileName, ".tar.bz2")
	case regexTarZ.MatchString(fileName):
		return TarZ, true, strings.TrimSuffix(fileName, ".tar.z")
	case regexTarGz.MatchString(fileName):
		return TarGz, true, strings.TrimSuffix(fileName, ".tar.gz")
	case regexTarLz4.MatchString(fileName):
		return TarLz4, true, strings.TrimSuffix(fileName, ".tar.lz4")
	case regexTarLz.MatchString(fileName):
		return TarLz, true, strings.TrimSuffix(fileName, ".tar.lz")
	case regexTarLzma.MatchString(fileName):
		return TarLzma, true, strings.TrimSuffix(fileName, ".tar.lzma")
	case regexTarXz.MatchString(fileName):
		return TarXz, true, strings.TrimSuffix(fileName, ".tar.xz")
	case regexTarZst.MatchString(fileName):
		return TarZst, true, strings.TrimSuffix(fileName, ".tar.zst")
	}
	return Unknown, false, ""
}

type ExtractJob struct {
	ArchiveType ArchiveType
	SrcPath     string
	DestPath    string
}

func (executor *Executor) Run(cmd *cobra.Command, rawPaths []string) {
	executor.logger = logging.Logger{
		OutWriter: cmd.OutOrStdout(),
		ErrWriter: cmd.ErrOrStderr(),
		State:     logging.LoggerStateNewLine,
	}
	passwords, err := executor.collectPasswords()
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, "解析密码出错: %v", err)
		os.Exit(1)
	}
	jobs, _ := executor.collectExtractJobs(rawPaths)
	if len(jobs) == 0 {
		return
	}
	var hadError bool
	if executor.Verbose {
		for _, job := range jobs {
			err := executor.processExtractJob(job, passwords)
			if err != nil {
				executor.logError(job.SrcPath, err.Error())
				hadError = true
			}
		}
	} else {
		bar := progressbar.NewOptions(len(jobs),
			progressbar.OptionSetWriter(cmd.OutOrStdout()),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
		executor.logger.SetState(logging.LoggerStateOutOldLine)
		for _, job := range jobs {
			err := executor.processExtractJob(job, passwords)
			if err != nil {
				executor.logError(job.SrcPath, err.Error())
				hadError = true
			} else {
				_ = bar.Add(1)
				executor.logger.SetState(logging.LoggerStateOutOldLine)
			}
		}
		executor.logger.PrintfOut(logging.LogModeAppend, false, "")
	}
	if hadError {
		os.Exit(1)
	}
}

func (executor *Executor) collectPasswords() ([]string, error) {
	var rawPasswords []string
	rawPasswords = append(rawPasswords, "")
	rawPasswords = append(rawPasswords, executor.Passwords...)
	if executor.PasswordFile != "" {
		content, err := os.ReadFile(executor.PasswordFile)
		if err != nil {
			return nil, fmt.Errorf("无法读取密码文件'%s'\n%w", executor.PasswordFile, err)
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if line != "" {
				rawPasswords = append(rawPasswords, line)
			}
		}
	}
	// 去重
	var uniquePasswords []string
	passwordSet := make(map[string]bool)
	for _, password := range rawPasswords {
		if !passwordSet[password] {
			passwordSet[password] = true
			uniquePasswords = append(uniquePasswords, password)
		}
	}
	return uniquePasswords, nil
}

func (executor *Executor) collectExtractJobs(rawPaths []string) ([]ExtractJob, error) {
	var jobs []ExtractJob

	for _, rawPath := range rawPaths {
		executor.logInProgressVerbose(rawPath, "检查路径")
		absPath, err := filepath.Abs(rawPath)
		if err != nil {
			executor.logError(rawPath, fmt.Sprintf("获取绝对路径失败\n%v", err))
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				executor.logError(rawPath, "路径不存在")
				continue
			} else {
				executor.logError(rawPath, fmt.Sprintf("无法获取路径状态\n%v", err))
				continue
			}
		}
		if info.IsDir() {
			if executor.Recursive {
				executor.logInProgressVerbose(rawPath, "递归搜索目录")
				resultDir := filepath.Join(absPath, "result")
				err := filepath.WalkDir(absPath, func(currentPath string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						// 跳过解压结果目录
						if currentPath == resultDir {
							return filepath.SkipDir
						}
						return nil
					}
					archiveType, isMainArchive, baseName := detectArchiveType(currentPath)
					if archiveType == Unknown || !isMainArchive {
						return nil
					}
					jobs = append(jobs, ExtractJob{
						ArchiveType: archiveType,
						SrcPath:     currentPath,
						DestPath:    filepath.Join(resultDir, baseName),
					})
					return nil
				})
				if err != nil {
					executor.logError(rawPath, fmt.Sprintf("遍历目录失败\n%v", err))
				}
				executor.logSuccessVerbose(rawPath, "递归搜索目录完成")
			} else {
				executor.logError(rawPath, "跳过目录")
			}
		} else {
			archiveType, isMainArchive, baseName := detectArchiveType(absPath)
			if archiveType == Unknown || !isMainArchive {
				executor.logErrorVerbose(rawPath, "跳过不支持的文件类型")
				continue
			}
			jobs = append(jobs, ExtractJob{
				ArchiveType: archiveType,
				SrcPath:     absPath,
				DestPath:    filepath.Join(filepath.Dir(absPath), baseName),
			})
		}
	}
	return jobs, nil
}

func (executor *Executor) processExtractJob(job ExtractJob, passwords []string) error {
	executor.logInProgressVerbose(job.SrcPath, "检查解压目录")
	if _, err := os.Stat(job.DestPath); err == nil {
		return fmt.Errorf("解压目录'%s'已存在", job.DestPath)
	}
	// Zip GBK 探测
	isZipGBK := false
	if job.ArchiveType == Zip {
		executor.logInProgressVerbose(job.SrcPath, "探测 Zip 文件编码")
		cmdOut, err := exec.Command("7z", "l", "-ba", "-p", job.SrcPath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("探测 Zip 文件编码失败\n%w", err)
		}
		isZipGBK = !utf8.Valid(cmdOut)
	}
	executor.logInProgressVerbose(job.SrcPath, "开始解压")
	success := false
	for _, password := range passwords {
		executor.logInProgressVerbose(job.SrcPath, fmt.Sprintf("尝试密码'%s'", password))
		var cmd *exec.Cmd
		if isZipGBK {
			cmd = exec.Command("unzip", "-O", "cp936", "-q", "-o", "-P", password, "-d", job.DestPath, job.SrcPath)
		} else {
			cmd = exec.Command("7z", "x", "-bso0", "-bse2", "-bsp0", "-spe", "-y", "-p"+password, "-o"+job.DestPath, job.SrcPath)
		}
		var cmdErr bytes.Buffer
		cmd.Stdout = io.Discard
		cmd.Stderr = &cmdErr
		err := cmd.Run()
		if err == nil {
			executor.logSuccessVerbose(job.SrcPath, "解压成功")
			success = true
			break
		}
		if executor.Verbose {
			cmdErrMsg := strings.TrimRight(cmdErr.String(), "\r\n")
			executor.logger.PrintfErr(logging.LogModeAppend, true, cmdErrMsg)
		}
		_ = os.RemoveAll(job.DestPath)
	}
	if !success {
		return fmt.Errorf("解压失败")
	}
	return nil
}

func (executor *Executor) logInProgress(path string, message string) {
	inProgressColor := color.New(color.FgCyan, color.Bold)
	executor.logger.PrintfOut(logging.LogModeInPlace, false, "%s %s: %s", inProgressColor.Sprintf("[>]"), path, message)
}

func (executor *Executor) logSuccess(path string, message string) {
	successColor := color.New(color.FgGreen, color.Bold)
	executor.logger.PrintfOut(logging.LogModeInPlace, true, "%s %s: %s", successColor.Sprintf("[O]"), path, message)
}

func (executor *Executor) logError(path string, message string) {
	errorColor := color.New(color.FgRed, color.Bold)
	executor.logger.PrintfErr(logging.LogModeAppend, true, "%s %s: %s", errorColor.Sprintf("[X]"), path, message)
}

func (executor *Executor) logInProgressVerbose(path string, message string) {
	if !executor.Verbose {
		return
	}
	executor.logInProgress(path, message)
}

func (executor *Executor) logSuccessVerbose(path string, message string) {
	if !executor.Verbose {
		return
	}
	executor.logSuccess(path, message)
}

func (executor *Executor) logErrorVerbose(path string, message string) {
	if !executor.Verbose {
		return
	}
	executor.logError(path, message)
}

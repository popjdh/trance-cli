package cjxl

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"trance-cli/internal/logging"

	"github.com/gookit/color"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type Executor struct {
	logger    logging.Logger
	Verbose   bool
	Recursive bool
}

func (executor *Executor) Run(cmd *cobra.Command, rawPaths []string) {
	executor.logger = logging.Logger{
		OutWriter: cmd.OutOrStdout(),
		ErrWriter: cmd.ErrOrStderr(),
		State:     logging.LoggerStateNewLine,
	}

	srcFilePaths, _ := executor.collectFiles(rawPaths)
	if len(srcFilePaths) == 0 {
		return
	}
	var hadError bool
	if executor.Verbose {
		for _, srcFilePath := range srcFilePaths {
			err := executor.processFile(srcFilePath)
			if err != nil {
				executor.logError(srcFilePath, err.Error())
				hadError = true
			}
		}
	} else {
		bar := progressbar.NewOptions(len(srcFilePaths),
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
		for _, srcFilePath := range srcFilePaths {
			err := executor.processFile(srcFilePath)
			if err != nil {
				executor.logError(srcFilePath, err.Error())
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

func (executor *Executor) collectFiles(rawPaths []string) ([]string, error) {
	var srcFilePaths []string
	for _, rawPath := range rawPaths {
		info, err := os.Stat(rawPath)
		if err != nil {
			if os.IsNotExist(err) {
				executor.logError(rawPath, "文件或目录不存在")
				continue
			} else {
				executor.logError(rawPath, fmt.Sprintf("无法获取文件或目录状态\n%v", err))
				continue
			}
		}
		if info.IsDir() {
			if executor.Recursive {
				if executor.Verbose {
					executor.logInProgress(rawPath, "递归搜索目录")
				}
				err := filepath.WalkDir(rawPath, func(currentPath string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !entry.IsDir() {
						ext := strings.ToLower(filepath.Ext(currentPath))
						switch ext {
						case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".gif", ".webp":
							srcFilePaths = append(srcFilePaths, currentPath)
						}
					}
					return nil
				})
				if err != nil {
					executor.logError(rawPath, fmt.Sprintf("遍历目录失败\n%v", err))
				}
				if executor.Verbose {
					executor.logSuccess(rawPath, "递归搜索目录完成")
				}
			} else {
				executor.logError(rawPath, "跳过目录")
			}
		} else {
			ext := strings.ToLower(filepath.Ext(rawPath))
			switch ext {
			case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".gif", ".webp":
				srcFilePaths = append(srcFilePaths, rawPath)
				break
			default:
				if executor.Verbose {
					executor.logError(rawPath, "跳过不支持的文件类型")
				}
			}
		}
	}
	return srcFilePaths, nil
}

func (executor *Executor) processFile(srcFilePath string) error {
	if executor.Verbose {
		executor.logInProgress(srcFilePath, "确认文件状态")
	}
	info, err := os.Lstat(srcFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("源文件不存在")
		}
		return fmt.Errorf("无法获取源文件状态\n%w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if executor.Verbose {
			executor.logSuccess(srcFilePath, "跳过符号链接")
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("源文件为非常规文件")
	}
	srcFileDir := filepath.Dir(srcFilePath)
	srcFileBaseName := filepath.Base(srcFilePath)
	ext := filepath.Ext(srcFileBaseName)
	srcFileBaseNameNoExt := strings.TrimSuffix(srcFileBaseName, ext)
	destFilePath := filepath.Join(srcFileDir, srcFileBaseNameNoExt+".jxl")
	if _, err := os.Stat(destFilePath); err == nil {
		return fmt.Errorf("目标文件已存在")
	}
	if executor.Verbose {
		executor.logInProgress(srcFilePath, "创建临时文件")
	}
	tmpFile, err := os.CreateTemp(srcFileDir, "to_jxl-*.jxl")
	if err != nil {
		return fmt.Errorf("无法创建临时文件\n%w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()
	tmpFilePath := tmpFile.Name()
	_ = tmpFile.Close()
	if executor.Verbose {
		executor.logInProgress(srcFilePath, "执行 cjxl 命令")
	}
	cmd := exec.Command("cjxl", "-d", "0", srcFilePath, tmpFilePath)
	var cmdErr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		cmdErrMsg := cmdErr.String()
		if !strings.HasSuffix(cmdErrMsg, "\n") {
			cmdErrMsg += "\n"
		}
		executor.logger.PrintfErr(logging.LogModeAppend, false, cmdErrMsg)
		return fmt.Errorf("执行 cjxl 命令失败\n%w", err)
	}
	if err := os.Rename(tmpFilePath, destFilePath); err != nil {
		return fmt.Errorf("无法写入目标文件\n%w", err)
	}
	if err := os.Remove(srcFilePath); err != nil {
		return fmt.Errorf("无法删除源文件\n%w", err)
	}
	if executor.Verbose {
		executor.logSuccess(srcFilePath, "转换完成")
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

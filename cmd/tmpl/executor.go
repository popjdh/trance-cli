package tmpl

import (
	"fmt"
	"os"
	"trance-cli/cmd/tmpl/core"
	"trance-cli/cmd/tmpl/tui"
	"trance-cli/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type Executor struct {
	logger              logging.Logger
	templateManager     *core.TemplateManager
	TemplateRegistryDir string
}

func (executor *Executor) Run(cmd *cobra.Command, args []string) {
	executor.logger = logging.Logger{
		OutWriter: cmd.OutOrStdout(),
		ErrWriter: cmd.ErrOrStderr(),
		State:     logging.LoggerStateNewLine,
	}

	templateManager, err := core.NewTemplateManager(executor.TemplateRegistryDir)
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, "%s", err.Error())
		os.Exit(1)
	}
	executor.templateManager = templateManager

	rootSlotInstance := core.NewRootSlotInstance()

	rootModel := tui.NewRootModel(templateManager, rootSlotInstance)

	program := tea.NewProgram(rootModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := program.Run()
	if err != nil {
		executor.logger.PrintfErr(logging.LogModeAppend, true, "%s", err.Error())
		os.Exit(1)
	}

	if rootModel, ok := finalModel.(tui.RootModel); ok {
		fmt.Println(rootModel.Result)
	}
}

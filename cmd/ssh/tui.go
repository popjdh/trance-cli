package ssh

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tuiAppStyle              = lipgloss.NewStyle().Padding(1, 1)
	tuiTitleStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	tuiHelpStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingTop(1)
	tuiTextInputLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(16) // 调整宽度以对齐
	tuiTextInputFocusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
)

type TuiModel struct {
	// Host 预览表格
	hostTable table.Model
	// HostStr 输入框
	hostStrInput textinput.Model
	// ProxyJump 输入框
	proxyJumpInput textinput.Model
	// sshOptions 输入框
	sshOptionsInput textinput.Model
	// remoteCommandArgs 输入框
	remoteCommandArgsInput textinput.Model
	// 传入的 Host 列表
	originalHosts []Host
	// 当前聚焦的输入框索引, 0:hostStrInput, 1:proxyJumpInput, 2:sshOptionsInput, 3:remoteCommandArgsInput
	focusIndex              int
	quitting                bool
	resultHostStr           string
	resultProxyJump         string
	resultSshOptions        []string
	resultRemoteCommandArgs []string
}

func newTuiModel(hosts []Host, defaultPresetHostStr string, defaultSshOptions []string, defaultRemoteCommandArgs []string) TuiModel {
	// HostStr 输入框
	hostStrInput := textinput.New()
	hostStrInput.Placeholder = "user@host:port or alias"
	hostStrInput.Focus()
	hostStrInput.CharLimit = 128
	hostStrInput.PromptStyle = tuiTextInputFocusedStyle
	hostStrInput.TextStyle = tuiTextInputFocusedStyle
	hostStrInput.SetValue(defaultPresetHostStr)
	// ProxyJump 输入框
	proxyJumpInput := textinput.New()
	proxyJumpInput.Placeholder = "user@host:port or alias"
	proxyJumpInput.CharLimit = 128
	// sshOptions 输入框
	sshOptionsInput := textinput.New()
	sshOptionsInput.Placeholder = "-o StrictHostKeyChecking=no -o ConnectTimeout=5"
	sshOptionsInput.CharLimit = 256
	sshOptionsInput.SetValue(strings.Join(defaultSshOptions, " "))
	// remoteCommandArgs 输入框
	remoteCommandArgsInput := textinput.New()
	remoteCommandArgsInput.Placeholder = "ls -l /"
	remoteCommandArgsInput.CharLimit = 256
	remoteCommandArgsInput.SetValue(strings.Join(defaultRemoteCommandArgs, " "))
	// HostStr 预览表格
	hostTableColumns := []table.Column{
		{Title: "Alias", Width: 15},
		{Title: "User", Width: 10},
		{Title: "Hostname", Width: 15},
		{Title: "Port", Width: 6},
		{Title: "ProxyJump", Width: 15},
		{Title: "Source", Width: 15},
	}
	hostTableRows := make([]table.Row, len(hosts))
	for i, host := range hosts {
		hostTableRows[i] = table.Row{
			host.Alias,
			host.User,
			host.Hostname,
			host.Port,
			host.ProxyJump,
			host.Source,
		}
	}
	hostTable := table.New(
		table.WithColumns(hostTableColumns),
		table.WithRows(hostTableRows),
		table.WithFocused(true),
		table.WithHeight(3),
	)
	hostTableStyle := table.DefaultStyles()
	hostTableStyle.Header = hostTableStyle.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	hostTableStyle.Selected = hostTableStyle.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	hostTable.SetStyles(hostTableStyle)

	model := TuiModel{
		hostTable:              hostTable,
		hostStrInput:           hostStrInput,
		proxyJumpInput:         proxyJumpInput,
		sshOptionsInput:        sshOptionsInput,
		remoteCommandArgsInput: remoteCommandArgsInput,
		originalHosts:          hosts,
		focusIndex:             0,
	}
	model.filterHost()
	return model
}

func (model *TuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (model *TuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	// 快捷键处理
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// 退出应用
		case "ctrl+c", "esc":
			model.quitting = true
			return model, tea.Quit
		// 确认选择并退出
		case "enter":
			model.resultHostStr = model.hostStrInput.Value()
			model.resultProxyJump = model.proxyJumpInput.Value()
			model.resultSshOptions = strings.Fields(model.sshOptionsInput.Value())
			model.resultRemoteCommandArgs = strings.Fields(model.remoteCommandArgsInput.Value())
			model.quitting = true
			return model, tea.Quit
		// 切换输入框焦点
		case "tab":
			model.focusIndex = (model.focusIndex + 1) % 4
			// 先将所有输入框设为失焦状态
			model.hostStrInput.Blur()
			model.proxyJumpInput.Blur()
			model.sshOptionsInput.Blur()
			model.remoteCommandArgsInput.Blur()
			model.hostStrInput.PromptStyle, model.hostStrInput.TextStyle = lipgloss.NewStyle(), lipgloss.NewStyle()
			model.proxyJumpInput.PromptStyle, model.proxyJumpInput.TextStyle = lipgloss.NewStyle(), lipgloss.NewStyle()
			model.sshOptionsInput.PromptStyle, model.sshOptionsInput.TextStyle = lipgloss.NewStyle(), lipgloss.NewStyle()
			model.remoteCommandArgsInput.PromptStyle, model.remoteCommandArgsInput.TextStyle = lipgloss.NewStyle(), lipgloss.NewStyle()
			// 根据 focusIndex 设置当前聚焦的输入框
			switch model.focusIndex {
			case 0:
				model.hostStrInput.Focus()
				model.hostStrInput.PromptStyle = tuiTextInputFocusedStyle
				model.hostStrInput.TextStyle = tuiTextInputFocusedStyle
			case 1:
				model.proxyJumpInput.Focus()
				model.proxyJumpInput.PromptStyle = tuiTextInputFocusedStyle
				model.proxyJumpInput.TextStyle = tuiTextInputFocusedStyle
			case 2:
				model.sshOptionsInput.Focus()
				model.sshOptionsInput.PromptStyle = tuiTextInputFocusedStyle
				model.sshOptionsInput.TextStyle = tuiTextInputFocusedStyle
			case 3:
				model.remoteCommandArgsInput.Focus()
				model.remoteCommandArgsInput.PromptStyle = tuiTextInputFocusedStyle
				model.remoteCommandArgsInput.TextStyle = tuiTextInputFocusedStyle
			}
			model.filterHost()
			return model, nil
		// 空格选择 HostStr
		case " ":
			if (model.focusIndex == 0 || model.focusIndex == 1) && len(model.hostTable.Rows()) > 0 && len(model.hostTable.SelectedRow()) > 0 {
				selectedAlias := model.hostTable.SelectedRow()[0]
				var selectedHost Host
				found := false
				for _, host := range model.originalHosts {
					if host.Alias == selectedAlias {
						selectedHost = host
						found = true
						break
					}
				}
				if found {
					var currentInputValue string
					if model.focusIndex == 0 {
						currentInputValue = model.hostStrInput.Value()
					} else {
						currentInputValue = model.proxyJumpInput.Value()
					}
					var valueToSet string
					// 对 ssh_config 来源的 HostStr, 首次选择填充 Alias, 再次选择进行展开
					if currentInputValue == selectedHost.Alias && selectedHost.Source == "ssh_config" {
						hostPart := selectedHost.Hostname
						if strings.Contains(hostPart, ":") {
							hostPart = "[" + hostPart + "]"
						}
						valueToSet = hostPart
						if selectedHost.User != "" {
							valueToSet = selectedHost.User + "@" + hostPart
						}
						if selectedHost.Port != "" {
							valueToSet = valueToSet + ":" + selectedHost.Port
						}
						// 对 HostStr 输入框, 展开同时填充 ProxyJump 输入框
						if model.focusIndex == 0 && selectedHost.ProxyJump != "" {
							model.proxyJumpInput.SetValue(selectedHost.ProxyJump)
						}
					} else {
						if selectedHost.Source == "ssh_config" {
							valueToSet = selectedHost.Alias
						} else {
							hostPart := selectedHost.Hostname
							if strings.Contains(hostPart, ":") {
								hostPart = "[" + hostPart + "]"
							}
							valueToSet = hostPart
							if selectedHost.User != "" {
								valueToSet = selectedHost.User + "@" + hostPart
							}
							if selectedHost.Port != "" {
								valueToSet = valueToSet + ":" + selectedHost.Port
							}
						}
					}
					if model.focusIndex == 0 {
						model.hostStrInput.SetValue(valueToSet)
						model.hostStrInput.SetCursor(len(valueToSet))
					} else {
						model.proxyJumpInput.SetValue(valueToSet)
						model.proxyJumpInput.SetCursor(len(valueToSet))
					}
					model.filterHost()
				}
				return model, nil
			}
		}
	}
	// 记录更新前的输入框值
	var oldVal string
	if model.focusIndex == 0 {
		oldVal = model.hostStrInput.Value()
	} else if model.focusIndex == 1 {
		oldVal = model.proxyJumpInput.Value()
	}
	// 将输入消息分派给当前聚焦的输入框
	switch model.focusIndex {
	case 0:
		model.hostStrInput, cmd = model.hostStrInput.Update(msg)
	case 1:
		model.proxyJumpInput, cmd = model.proxyJumpInput.Update(msg)
	case 2:
		model.sshOptionsInput, cmd = model.sshOptionsInput.Update(msg)
	case 3:
		model.remoteCommandArgsInput, cmd = model.remoteCommandArgsInput.Update(msg)
	}
	cmds = append(cmds, cmd)
	// 仅当 HostStr 或 ProxyJump 输入框文本变化时才过滤列表
	if (model.focusIndex == 0 && oldVal != model.hostStrInput.Value()) || (model.focusIndex == 1 && oldVal != model.proxyJumpInput.Value()) {
		model.filterHost()
	}
	// 表格接收方向键控制
	model.hostTable, cmd = model.hostTable.Update(msg)
	cmds = append(cmds, cmd)
	// 处理窗口大小调整
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		// 整体内容宽度
		contentWidth := size.Width - 2
		// 输入框宽度
		inputWidth := contentWidth - 16
		model.hostStrInput.Width = inputWidth
		model.proxyJumpInput.Width = inputWidth
		model.sshOptionsInput.Width = inputWidth
		model.remoteCommandArgsInput.Width = inputWidth
		// 表格尺寸
		model.hostTable.SetHeight(size.Height - 9)
		flexColumnWidth := (contentWidth - (10 + 6 + 15) - 10) / 3
		model.hostTable.SetColumns([]table.Column{
			{Title: "Alias", Width: flexColumnWidth},
			{Title: "User", Width: 10},
			{Title: "Hostname", Width: flexColumnWidth},
			{Title: "Port", Width: 6},
			{Title: "ProxyJump", Width: flexColumnWidth},
			{Title: "Source", Width: 15},
		})
	}
	return model, tea.Batch(cmds...)
}

func (model *TuiModel) filterHost() {
	var filterValue string
	// 只根据 HostStr 或 ProxyJump 输入框的值进行过滤
	switch model.focusIndex {
	case 0:
		filterValue = model.hostStrInput.Value()
	case 1:
		filterValue = model.proxyJumpInput.Value()
	default:
		// 当焦点在其他输入框时, 不进行过滤, 显示所有主机
		filterValue = ""
	}
	// 根据 Alias 忽略大小写过滤
	filterValue = strings.ToLower(filterValue)
	var hostTableRows []table.Row
	for _, host := range model.originalHosts {
		if strings.Contains(strings.ToLower(host.Alias), filterValue) {
			hostTableRows = append(hostTableRows, table.Row{
				host.Alias,
				host.User,
				host.Hostname,
				host.Port,
				host.ProxyJump,
				host.Source,
			})
		}
	}
	model.hostTable.SetRows(hostTableRows)
	if len(model.hostTable.Rows()) > 0 {
		model.hostTable.SetCursor(0)
	}
}

func (model *TuiModel) View() string {
	if model.quitting {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, tuiTextInputLabelStyle.Render("Host:"), model.hostStrInput.View()))
	builder.WriteString("\n")
	builder.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, tuiTextInputLabelStyle.Render("ProxyJump:"), model.proxyJumpInput.View()))
	builder.WriteString("\n")
	builder.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, tuiTextInputLabelStyle.Render("SSH Options:"), model.sshOptionsInput.View()))
	builder.WriteString("\n")
	builder.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, tuiTextInputLabelStyle.Render("Remote Command:"), model.remoteCommandArgsInput.View()))
	builder.WriteString("\n\n")
	builder.WriteString(tuiTitleStyle.Render("Select an SSH Host"))
	builder.WriteString("\n")
	builder.WriteString(model.hostTable.View())
	builder.WriteString(tuiHelpStyle.Render("up/down: navigate | space: select | tab: switch input | enter: connect | esc: quit"))

	return tuiAppStyle.Render(builder.String())
}

type TuiResult struct {
	HostStr           string
	ProxyJump         string
	SshOptions        []string
	RemoteCommandArgs []string
}

func RunSelector(hosts []Host, defaultHostStr string, defaultSshOptions []string, defaultRemoteCommandArgs []string) (TuiResult, error) {
	model := newTuiModel(hosts, defaultHostStr, defaultSshOptions, defaultRemoteCommandArgs)
	teaProgram := tea.NewProgram(&model, tea.WithAltScreen())
	teaModel, err := teaProgram.Run()
	if err != nil {
		return TuiResult{}, fmt.Errorf("TUI 运行错误\n%w", err)
	}

	result := teaModel.(*TuiModel)
	if result.resultHostStr == "" {
		return TuiResult{}, fmt.Errorf("未选择主机")
	}

	return TuiResult{
		HostStr:           result.resultHostStr,
		ProxyJump:         result.resultProxyJump,
		SshOptions:        result.resultSshOptions,
		RemoteCommandArgs: result.resultRemoteCommandArgs,
	}, nil
}

package tui

import (
	"fmt"
	"strings"

	"trance-cli/cmd/tmpl/core"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// region 样式

var literalsEditorStyle = struct {
	Global struct {
		Box      lipgloss.Style
		BoxTitle lipgloss.Style
		Tooltip  lipgloss.Style
	}
	LiteralSelector struct {
		StatusNull   lipgloss.Style
		StatusEmpty  lipgloss.Style
		StatusNormal lipgloss.Style
	}
	LiteralOptionSelector struct {
		Selected   lipgloss.Style
		Unselected lipgloss.Style
	}
}{}

func init() {
	literalsEditorStyle.Global.Box = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("2"))
	literalsEditorStyle.Global.BoxTitle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	literalsEditorStyle.Global.Tooltip = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))

	literalsEditorStyle.LiteralSelector.StatusNull = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	literalsEditorStyle.LiteralSelector.StatusEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	literalsEditorStyle.LiteralSelector.StatusNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	literalsEditorStyle.LiteralOptionSelector.Selected = lipgloss.NewStyle()
	literalsEditorStyle.LiteralOptionSelector.Unselected = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
}

// endregion

// region LiteralsEditorScreenLayout

type LiteralsEditorScreenLayout struct {
	width         int
	height        int
	contentHeight int
}

func (l *LiteralsEditorScreenLayout) update(width, height int) {
	l.width = width
	l.height = height
	l.contentHeight = height - 4
}

// endregion

// region LiteralsEditorScreen

type LiteralsEditorScreen struct {
	templateInstance *core.TemplateInstance
	literals         []core.LiteralInstance
	titledBox        TitledBox
	literalInput     textinput.Model
	cursor           int
	editing          bool
	optionCursor     int
	layout           LiteralsEditorScreenLayout
}

func NewLiteralEditorScreen(instance *core.TemplateInstance, width, height int) LiteralsEditorScreen {
	literals := make([]core.LiteralInstance, len(instance.Literals))
	copy(literals, instance.Literals)

	titledBox := NewTitledBox(
		literalsEditorStyle.Global.Box,
		literalsEditorStyle.Global.BoxTitle,
	)

	literalInput := textinput.New()
	literalInput.Prompt = ""
	literalInput.CharLimit = 256

	s := LiteralsEditorScreen{
		templateInstance: instance,
		literals:         literals,
		titledBox:        titledBox,
		literalInput:     literalInput,
		cursor:           0,
		editing:          false,
		optionCursor:     -1,
	}
	s.UpdateSizes(width, height)
	return s
}

func (model *LiteralsEditorScreen) Init() tea.Cmd {
	return nil
}

func (model *LiteralsEditorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.UpdateSizes(msg.Width, msg.Height)

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonLeft:
				lineIndex := msg.Y - 1
				if lineIndex >= 0 {
					// 计算滚动偏移
					lineIndex += model.scrollOffset()

					// 解析点击目标
					itemIndex := -1
					clickType := ""
					optionIndex := -1
					currentLine := 0
					for i := range model.literals {
						if i == model.cursor && model.editing {
							editLines := model.literalEditLines(i)
							for j := range editLines {
								if currentLine == lineIndex {
									itemIndex = i
									if j == 0 {
										clickType = "input"
									} else {
										clickType = "option"
										optionIndex = j - 1
									}
									break
								}
								currentLine++
							}
							if itemIndex >= 0 {
								break
							}
						} else {
							if currentLine == lineIndex {
								itemIndex = i
								clickType = "summary"
								break
							}
							currentLine++
						}
					}

					// 处理点击
					if itemIndex >= 0 {
						if model.editing {
							if itemIndex == model.cursor {
								switch clickType {
								case "input":
									model.optionCursor = -1
									model.literalInput.Focus()
								case "option":
									if model.optionCursor == optionIndex {
										model.confirmEdit()
									} else {
										model.optionCursor = optionIndex
										model.literalInput.Blur()
									}
								}
							} else {
								model.confirmEdit()
								model.cursor = itemIndex
							}
						} else {
							if itemIndex == model.cursor {
								model.enterEdit()
							} else {
								model.cursor = itemIndex
							}
						}
					} else if model.editing {
						model.confirmEdit()
					}
				}
			case tea.MouseButtonWheelUp:
				model.cursor = max(model.cursor-1, 0)
			case tea.MouseButtonWheelDown:
				model.cursor = min(model.cursor+1, len(model.literals)-1)
			}
		}

	case tea.KeyMsg:
		if model.editing {
			switch msg.String() {
			case "esc":
				model.exitEdit()
				return model, nil
			case "enter":
				model.confirmEdit()
				return model, nil
			}
			lit := &model.literals[model.cursor]
			if model.optionCursor < 0 {
				switch msg.String() {
				case "down":
					model.optionCursor = max(0, len(lit.Definition.Options)-1)
					if model.optionCursor >= 0 {
						model.literalInput.Blur()
						return model, nil
					}
				}
				var cmd tea.Cmd
				model.literalInput, cmd = model.literalInput.Update(msg)
				return model, cmd
			} else if model.optionCursor >= 0 {
				switch msg.String() {
				case "up":
					model.optionCursor--
					if model.optionCursor < 0 {
						model.literalInput.Focus()
						return model, textinput.Blink
					}
					return model, nil
				case "down":
					model.optionCursor = min(model.optionCursor+1, len(lit.Definition.Options)-1)
					return model, nil
				}
			}
		} else {
			switch msg.String() {
			case "ctrl+c":
				return model, tea.Quit
			case "esc":
				return model, func() tea.Msg { return RouteBackMsg{} }
			case "up":
				model.cursor = max(model.cursor-1, 0)
			case "down":
				model.cursor = min(model.cursor+1, len(model.literals)-1)
			case "tab":
				model.literals[model.cursor].Value = nil // tab 单向设 nil
				return model, nil
			case "enter":
				model.enterEdit()
				return model, textinput.Blink
			case "ctrl+s":
				model.templateInstance.Literals = model.literals
				return model, func() tea.Msg { return RouteBackMsg{} }
			}
			return model, nil
		}
	}

	return model, nil
}

func (model *LiteralsEditorScreen) View() string {
	body := model.titledBox.Render(
		fmt.Sprintf("Edit Literals: %s [%d/%d]", model.templateInstance.Definition.Name, model.cursor+1, len(model.literals)),
		strings.Join(model.visibleBodyLines(model.layout.contentHeight), "\n"),
		model.layout.width,
		model.layout.height-2,
	)

	var tooltip string
	if model.editing {
		if model.optionCursor >= 0 {
			tooltip = literalsEditorStyle.Global.Tooltip.Render("↑↓: select | ↑@top: input | Enter: confirm | Esc: cancel")
		} else {
			tooltip = literalsEditorStyle.Global.Tooltip.Render("Enter: confirm | ↓: options | Esc: cancel")
		}
	} else {
		tooltip = literalsEditorStyle.Global.Tooltip.Render("↑↓: navigate | Enter: edit | Tab: null | Ctrl+S: save | Esc: back")
	}

	return body + "\n" + tooltip
}

func (model *LiteralsEditorScreen) UpdateSizes(width, height int) {
	model.layout.update(width, height)
}

// endregion

// region 状态切换

func (model *LiteralsEditorScreen) enterEdit() {
	lit := &model.literals[model.cursor]
	if lit.Value == nil {
		model.literalInput.SetValue("")
	} else {
		model.literalInput.SetValue(*lit.Value)
	}
	model.literalInput.Focus()
	model.editing = true
	model.optionCursor = -1
}

func (model *LiteralsEditorScreen) confirmEdit() {
	lit := &model.literals[model.cursor]
	if model.optionCursor >= 0 {
		v := lit.Definition.Options[model.optionCursor]
		lit.Value = &v
	} else {
		v := model.literalInput.Value()
		lit.Value = &v
	}
	model.exitEdit()
}

func (model *LiteralsEditorScreen) exitEdit() {
	model.literalInput.Blur()
	model.editing = false
	model.optionCursor = -1
}

// endregion

// region 渲染计算

func (model *LiteralsEditorScreen) scrollOffset() int {
	lines := model.bodyLines()
	visibleHeight := model.layout.contentHeight

	if len(lines) <= visibleHeight {
		return 0
	}

	focusLine := model.cursor
	if model.editing && model.optionCursor >= 0 {
		focusLine += 1 + model.optionCursor
	}

	start := 0
	if focusLine >= visibleHeight {
		start = focusLine - visibleHeight + 1
	}

	maxStart := len(lines) - visibleHeight
	if start > maxStart {
		start = maxStart
	}

	return start
}

func (model *LiteralsEditorScreen) visibleBodyLines(visibleHeight int) []string {
	if visibleHeight <= 0 {
		return nil
	}

	lines := model.bodyLines()
	if len(lines) <= visibleHeight {
		return lines
	}

	start := model.scrollOffset()
	return lines[start : start+visibleHeight]
}

func (model *LiteralsEditorScreen) bodyLines() []string {
	lines := make([]string, 0, len(model.literals))

	for i := range model.literals {
		if i == model.cursor && model.editing {
			lines = append(lines, model.literalEditLines(i)...)
			continue
		}

		lines = append(lines, model.literalSummaryLine(i))
	}

	return lines
}

func (model *LiteralsEditorScreen) literalSummaryLine(index int) string {
	lit := model.literals[index]

	prefix := "  "
	if index == model.cursor {
		prefix = "> "
	}

	var display string
	var style lipgloss.Style

	if lit.Value == nil {
		display = "<NULL>"
		style = literalsEditorStyle.LiteralSelector.StatusNull
	} else if *lit.Value == "" {
		display = "<EMPTY>"
		style = literalsEditorStyle.LiteralSelector.StatusEmpty
	} else {
		display = *lit.Value
		style = literalsEditorStyle.LiteralSelector.StatusNormal
	}

	return style.Render(
		fmt.Sprintf("%s%s (%s): %s",
			prefix,
			lit.Definition.Label,
			lit.Definition.Key,
			display,
		),
	)
}

func (model *LiteralsEditorScreen) literalEditLines(index int) []string {
	lit := model.literals[index]
	contentPrefix := fmt.Sprintf("  %s (%s): ", lit.Definition.Label, lit.Definition.Key)
	lines := []string{
		contentPrefix + model.literalInput.View(),
	}
	if len(lit.Definition.Options) == 0 {
		return lines
	}
	alignWidth := lipgloss.Width(contentPrefix)
	for i, opt := range lit.Definition.Options {
		if i == model.optionCursor {
			lines = append(lines, literalsEditorStyle.LiteralOptionSelector.Selected.Render(strings.Repeat(" ", alignWidth-2)+"> "+opt))
		} else {
			lines = append(lines, literalsEditorStyle.LiteralOptionSelector.Unselected.Render(strings.Repeat(" ", alignWidth)+opt))
		}
	}
	return lines
}

// endregion

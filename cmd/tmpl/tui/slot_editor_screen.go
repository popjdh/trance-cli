package tui

import (
	"fmt"
	"io"

	"trance-cli/cmd/tmpl/core"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// region 样式

var slotEditorStyle = struct {
	Global struct {
		FocusedBox lipgloss.Style
		BlurredBox lipgloss.Style
		BoxTitle   lipgloss.Style
		Tooltip    lipgloss.Style
	}
	TemplateSelector struct {
		Selected   lipgloss.Style
		Unselected lipgloss.Style
	}
}{}

func init() {
	slotEditorStyle.Global.FocusedBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("2"))
	slotEditorStyle.Global.BlurredBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	slotEditorStyle.Global.BoxTitle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	slotEditorStyle.Global.Tooltip = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))

	slotEditorStyle.TemplateSelector.Selected = lipgloss.NewStyle()
	slotEditorStyle.TemplateSelector.Unselected = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
}

// endregion

// region TemplateSelector

type templateSelectorItem struct {
	template *core.Template // nil = [Empty]
}

func (i templateSelectorItem) DisplayName() string {
	if i.template == nil {
		return "[Empty]"
	}
	return i.template.Name
}

func (i templateSelectorItem) FilterValue() string { return "" }

type templateSelectorDelegate struct{}

func (d templateSelectorDelegate) Height() int                             { return 1 }
func (d templateSelectorDelegate) Spacing() int                            { return 0 }
func (d templateSelectorDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d templateSelectorDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item := listItem.(templateSelectorItem)
	prefix := "  "
	style := slotEditorStyle.TemplateSelector.Unselected
	if index == m.Index() {
		prefix = "> "
		style = slotEditorStyle.TemplateSelector.Selected
	}
	fmt.Fprint(w, style.Render(prefix+item.DisplayName()))
}

// endregion

// region SlotEditorScreenLayout

type SlotEditorScreenLayout struct {
	width           int
	height          int
	leftPanelWidth  int
	rightPanelWidth int
	contentHeight   int
}

func (l *SlotEditorScreenLayout) update(width, height int) {
	l.width = width
	l.height = height
	l.leftPanelWidth = (width-4)/3 + 2
	l.rightPanelWidth = width - l.leftPanelWidth
	l.contentHeight = height - 4
}

// endregion

// region SlotEditorScreen

type SlotEditorScreen struct {
	templateManager  *core.TemplateManager
	slotInstance     *core.SlotInstance
	focusedTitledBox TitledBox
	blurredTitledBox TitledBox
	templateSelector list.Model
	templatePreview  viewport.Model
	focus            int // 0=templateSelector, 1=templatePreview
	layout           SlotEditorScreenLayout
}

func NewSlotEditorScreen(templateManager *core.TemplateManager, slotInstance *core.SlotInstance, width, height int) SlotEditorScreen {
	focusedTitledBox := NewTitledBox(
		slotEditorStyle.Global.FocusedBox,
		slotEditorStyle.Global.BoxTitle,
	)
	blurredTitledBox := NewTitledBox(
		slotEditorStyle.Global.BlurredBox,
		slotEditorStyle.Global.BoxTitle,
	)

	templateSelectorItems := []list.Item{}
	for _, t := range templateManager.List(slotInstance.Definition.Namespace) {
		templateSelectorItems = append(templateSelectorItems, templateSelectorItem{template: t})
	}
	if len(templateSelectorItems) == 0 {
		templateSelectorItems = append(templateSelectorItems, templateSelectorItem{template: nil}) // [Empty]
	}

	templateSelector := list.New(templateSelectorItems, templateSelectorDelegate{}, 0, 0)
	templateSelector.SetShowTitle(false)
	templateSelector.SetShowStatusBar(false)
	templateSelector.SetFilteringEnabled(false)
	templateSelector.SetShowHelp(false)
	templateSelector.SetShowPagination(false)

	templatePreview := viewport.New(0, 0)

	s := SlotEditorScreen{
		templateManager:  templateManager,
		slotInstance:     slotInstance,
		focusedTitledBox: focusedTitledBox,
		blurredTitledBox: blurredTitledBox,
		templateSelector: templateSelector,
		templatePreview:  templatePreview,
	}
	s.UpdateSizes(width, height)
	return s
}

func (model *SlotEditorScreen) Init() tea.Cmd {
	return nil
}

func (model *SlotEditorScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.UpdateSizes(msg.Width, msg.Height)

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonLeft:
				if msg.X < model.layout.leftPanelWidth {
					// 左面板
					model.focus = 0
					itemIndex := model.templateSelector.Paginator.Page*model.templateSelector.Paginator.PerPage + (msg.Y - 1)
					items := model.templateSelector.Items()
					if itemIndex >= 0 && itemIndex < len(items) {
						if itemIndex == model.templateSelector.Index() {
							// 已选中, 确认选择
							if item, ok := items[itemIndex].(templateSelectorItem); ok {
								if item.template != nil {
									model.slotInstance.Embed(item.template.NewInstance())
								}
								return model, func() tea.Msg { return RouteBackMsg{} }
							}
						} else {
							// 未选中, 选中该 item
							model.templateSelector.Select(itemIndex)
							model.refreshTemplatePreview()
						}
					}
				} else {
					// 右面板
					model.focus = 1
				}
			case tea.MouseButtonWheelUp:
				if model.focus == 0 {
					var cmd tea.Cmd
					model.templateSelector, cmd = model.templateSelector.Update(tea.KeyMsg{Type: tea.KeyUp})
					cmds = append(cmds, cmd)
					model.refreshTemplatePreview()
				} else {
					var cmd tea.Cmd
					model.templatePreview, cmd = model.templatePreview.Update(tea.KeyMsg{Type: tea.KeyUp})
					cmds = append(cmds, cmd)
				}
			case tea.MouseButtonWheelDown:
				if model.focus == 0 {
					var cmd tea.Cmd
					model.templateSelector, cmd = model.templateSelector.Update(tea.KeyMsg{Type: tea.KeyDown})
					cmds = append(cmds, cmd)
					model.refreshTemplatePreview()
				} else {
					var cmd tea.Cmd
					model.templatePreview, cmd = model.templatePreview.Update(tea.KeyMsg{Type: tea.KeyDown})
					cmds = append(cmds, cmd)
				}
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return model, tea.Quit
		case "esc":
			return model, func() tea.Msg { return RouteBackMsg{} }
		case "left":
			model.focus = 0
		case "right":
			model.focus = 1
		case "enter":
			if model.focus == 0 {
				if item, ok := model.templateSelector.SelectedItem().(templateSelectorItem); ok {
					if item.template != nil {
						model.slotInstance.Embed(item.template.NewInstance())
					}
					return model, func() tea.Msg { return RouteBackMsg{} }
				}
			}
		case "up", "down":
			if model.focus == 0 {
				var cmd tea.Cmd
				model.templateSelector, cmd = model.templateSelector.Update(msg)
				cmds = append(cmds, cmd)
				model.refreshTemplatePreview()
			} else {
				var cmd tea.Cmd
				model.templatePreview, cmd = model.templatePreview.Update(msg)
				cmds = append(cmds, cmd)
			}
		default:
			if model.focus == 0 {
				var cmd tea.Cmd
				model.templateSelector, cmd = model.templateSelector.Update(msg)
				cmds = append(cmds, cmd)
				model.refreshTemplatePreview()
			} else {
				var cmd tea.Cmd
				model.templatePreview, cmd = model.templatePreview.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return model, tea.Batch(cmds...)
}

func (model *SlotEditorScreen) View() string {
	var body string
	if model.focus == 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			model.focusedTitledBox.Render(fmt.Sprintf("Select Template: %s", model.slotInstance.Definition.Namespace), model.templateSelector.View(), model.layout.leftPanelWidth, model.layout.height-2),
			model.blurredTitledBox.Render("Preview", model.templatePreview.View(), model.layout.rightPanelWidth, model.layout.height-2),
		)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			model.blurredTitledBox.Render(fmt.Sprintf("Select Template: %s", model.slotInstance.Definition.Namespace), model.templateSelector.View(), model.layout.leftPanelWidth, model.layout.height-2),
			model.focusedTitledBox.Render("Preview", model.templatePreview.View(), model.layout.rightPanelWidth, model.layout.height-2),
		)
	}

	var tooltip string
	if model.focus == 0 {
		tooltip = slotEditorStyle.Global.Tooltip.Render("←→: focus | ↑↓: navigate | Enter: select | Esc: quit")
	} else {
		tooltip = slotEditorStyle.Global.Tooltip.Render("←→: focus | Esc: quit")
	}

	return body + "\n" + tooltip
}

func (model *SlotEditorScreen) UpdateSizes(width, height int) {
	model.layout.update(width, height)
	model.templateSelector.SetSize(model.layout.leftPanelWidth-2, model.layout.contentHeight)
	model.templatePreview.Width = model.layout.rightPanelWidth - 2
	model.templatePreview.Height = model.layout.contentHeight
	model.refreshTemplatePreview()
}

// endregion

// region 内部方法

func (model *SlotEditorScreen) refreshTemplatePreview() {
	if item, ok := model.templateSelector.SelectedItem().(templateSelectorItem); ok && item.template != nil {
		model.templatePreview.SetContent(item.template.Content)
	} else {
		model.templatePreview.SetContent("")
	}
}

// endregion

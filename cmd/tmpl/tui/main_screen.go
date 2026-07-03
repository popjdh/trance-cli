package tui

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"trance-cli/cmd/tmpl/core"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// region 样式

var mainStyle = struct {
	Global struct {
		FocusedBox lipgloss.Style
		BlurredBox lipgloss.Style
		BoxTitle   lipgloss.Style
		Tooltip    lipgloss.Style
	}
	TemplateFlatTree struct {
		GuideLine        lipgloss.Style
		StatusEmpty      lipgloss.Style
		StatusIncomplete lipgloss.Style
		StatusNoInput    lipgloss.Style
		StatusComplete   lipgloss.Style
		Selected         lipgloss.Style
	}
	TemplatePreview struct {
		LineEnd     lipgloss.Style
		Levels      []lipgloss.Style
		Fragment    lipgloss.Style
		Literal     lipgloss.Style
		LiteralNull lipgloss.Style
	}
}{}

func init() {
	mainStyle.Global.FocusedBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("2"))
	mainStyle.Global.BlurredBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	mainStyle.Global.BoxTitle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	mainStyle.Global.Tooltip = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))

	mainStyle.TemplateFlatTree.GuideLine = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	mainStyle.TemplateFlatTree.StatusEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	mainStyle.TemplateFlatTree.StatusIncomplete = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	mainStyle.TemplateFlatTree.StatusNoInput = lipgloss.NewStyle().Foreground(lipgloss.Color("#8FA3B8"))
	mainStyle.TemplateFlatTree.StatusComplete = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	mainStyle.TemplateFlatTree.Selected = lipgloss.NewStyle().Background(lipgloss.Color("15")).Foreground(lipgloss.Color("0"))

	mainStyle.TemplatePreview.LineEnd = lipgloss.NewStyle().Foreground(lipgloss.Color("#858585"))
	mainStyle.TemplatePreview.Levels = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#7CB9FF")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD75E")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#E0A0FF")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#73FFD0")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB088")),
	}
	mainStyle.TemplatePreview.Fragment = lipgloss.NewStyle().Foreground(lipgloss.Color("#DCDCAA"))
	mainStyle.TemplatePreview.Literal = lipgloss.NewStyle().Foreground(lipgloss.Color("#B5CEA8"))
	mainStyle.TemplatePreview.LiteralNull = lipgloss.NewStyle().Foreground(lipgloss.Color("#F14C4C"))
}

// endregion

// region templateFlatTree

type templateFlatTreeItem struct {
	guideLinePrefix string // 导航线前缀
	displayName     string // 显示名
	level           int
	instance        *core.TemplateInstance // nil 表示空 slot
	slotInstance    *core.SlotInstance
}

func (i templateFlatTreeItem) FilterValue() string { return "" }

type templateFlatTreeItemDelegate struct{}

func (d templateFlatTreeItemDelegate) Height() int                             { return 1 }
func (d templateFlatTreeItemDelegate) Spacing() int                            { return 0 }
func (d templateFlatTreeItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d templateFlatTreeItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item := listItem.(templateFlatTreeItem)

	displayNameStyle := mainStyle.TemplateFlatTree.StatusEmpty
	if item.instance != nil {
		switch item.instance.State() {
		case core.TemplateInstanceStatusComplete:
			displayNameStyle = mainStyle.TemplateFlatTree.StatusComplete
		case core.TemplateInstanceStatusNoInput:
			displayNameStyle = mainStyle.TemplateFlatTree.StatusNoInput
		case core.TemplateInstanceStatusIncomplete:
			displayNameStyle = mainStyle.TemplateFlatTree.StatusIncomplete
		}
	}
	if index == m.Index() {
		displayNameStyle = displayNameStyle.Inherit(mainStyle.TemplateFlatTree.Selected)
	}

	fmt.Fprint(w,
		mainStyle.TemplateFlatTree.GuideLine.Render(item.guideLinePrefix)+
			displayNameStyle.Render(item.displayName),
	)
}

func buildTemplateFlatTree(rootSlotInst *core.SlotInstance) []list.Item {
	var result []list.Item

	rootTmplInst := rootSlotInst.Instances[0]

	if rootTmplInst == nil {
		result = append(result, templateFlatTreeItem{
			guideLinePrefix: "",
			displayName:     "[EMPTY]",
			level:           0,
			instance:        nil,
			slotInstance:    rootSlotInst,
		})
		return result
	}

	result = append(result, templateFlatTreeItem{
		guideLinePrefix: "",
		displayName:     fmt.Sprintf("[%s]", rootTmplInst.Definition.Name),
		level:           0,
		instance:        rootTmplInst,
		slotInstance:    rootSlotInst,
	})

	var walk func(parentTmplInst *core.TemplateInstance, level int, baseGuideLinePrefix string)
	walk = func(parentTmplInst *core.TemplateInstance, level int, baseGuideLinePrefix string) {
		for i := range parentTmplInst.Slots {
			isLast := i == len(parentTmplInst.Slots)-1
			slotInst := &parentTmplInst.Slots[i]
			childTmplInsts := slotInst.Instances
			for i, childTmplInst := range childTmplInsts {
				isChildLast := isLast && i == len(childTmplInsts)-1

				guideLinePrefix := baseGuideLinePrefix
				if isChildLast {
					guideLinePrefix += "└──"
				} else {
					guideLinePrefix += "├──"
				}
				if slotInst.Definition.Type == core.SlotTypeMulti {
					guideLinePrefix += "*"
				} else {
					guideLinePrefix += "·"
				}

				displayName := slotInst.Definition.Namespace
				if parentTmplInst != nil {
					parentNS := parentTmplInst.Definition.Namespace
					if strings.HasPrefix(displayName, parentNS+".") {
						displayName = ">" + displayName[len(parentNS)+1:]
					}
				}
				if slotInst.Definition.Key != slotInst.Definition.Namespace {
					displayName = slotInst.Definition.Key + ":" + displayName
				}
				if childTmplInst == nil {
					displayName = fmt.Sprintf("[%s:%s]", displayName, "EMPTY")
				} else {
					displayName = fmt.Sprintf("[%s:%s]", displayName, childTmplInst.Definition.Name)
				}

				result = append(result, templateFlatTreeItem{
					guideLinePrefix: guideLinePrefix,
					displayName:     displayName,
					level:           level,
					instance:        childTmplInst,
					slotInstance:    slotInst,
				})

				if childTmplInst != nil {
					nextBaseGuideLinePrefix := baseGuideLinePrefix
					if isChildLast {
						nextBaseGuideLinePrefix += "    "
					} else {
						nextBaseGuideLinePrefix += "│   "
					}
					walk(childTmplInst, level+1, nextBaseGuideLinePrefix)
				}
			}
		}
	}

	walk(rootTmplInst, 1, "")
	return result
}

// endregion

// region MainScreenLayout

type MainScreenLayout struct {
	width           int
	height          int
	leftPanelWidth  int
	rightPanelWidth int
	contentHeight   int
}

func (l *MainScreenLayout) update(width, height int) {
	l.width = width
	l.height = height
	l.leftPanelWidth = (width-4)/3 + 2
	l.rightPanelWidth = width - l.leftPanelWidth
	l.contentHeight = height - 4
}

// endregion

// region MainScreen

type MainScreen struct {
	templateManager  *core.TemplateManager
	rootSlotInstance *core.SlotInstance
	focusedTitledBox TitledBox
	blurredTitledBox TitledBox
	templateFlatTree list.Model
	templatePreview  viewport.Model
	focus            int // 0=templateFlatTree, 1=templatePreview
	layout           MainScreenLayout
}

func NewMainScreen(templateManager *core.TemplateManager, rootSlotInstance *core.SlotInstance) MainScreen {
	focusedTitledBox := NewTitledBox(
		mainStyle.Global.FocusedBox,
		mainStyle.Global.BoxTitle,
	)
	blurredTitledBox := NewTitledBox(
		mainStyle.Global.BlurredBox,
		mainStyle.Global.BoxTitle,
	)

	templateFlatTree := list.New(buildTemplateFlatTree(rootSlotInstance), templateFlatTreeItemDelegate{}, 25, 42)
	templateFlatTree.SetShowTitle(false)
	templateFlatTree.SetShowStatusBar(false)
	templateFlatTree.SetFilteringEnabled(false)
	templateFlatTree.SetShowHelp(false)
	templateFlatTree.SetShowPagination(false)

	templatePreview := viewport.New(51, 42)

	s := MainScreen{
		templateManager:  templateManager,
		rootSlotInstance: rootSlotInstance,
		focusedTitledBox: focusedTitledBox,
		blurredTitledBox: blurredTitledBox,
		templateFlatTree: templateFlatTree,
		templatePreview:  templatePreview,
		focus:            0,
	}
	s.UpdateSizes(80, 45) // 非 TTY 默认尺寸
	return s
}

func (model *MainScreen) Init() tea.Cmd {
	if model.rootSlotInstance.Instances[0] == nil {
		return func() tea.Msg {
			return RouteSlotEditorMsg{SlotInstance: model.rootSlotInstance}
		}
	}
	return nil
}

func (model *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
					itemIndex := model.templateFlatTree.Paginator.Page*model.templateFlatTree.Paginator.PerPage + (msg.Y - 1)
					items := model.templateFlatTree.Items()
					if itemIndex >= 0 && itemIndex < len(items) {
						if itemIndex == model.templateFlatTree.Index() {
							// 已选中, 进入编辑
							if item, ok := items[itemIndex].(templateFlatTreeItem); ok {
								if item.instance == nil && item.slotInstance != nil {
									cmds = append(cmds, func() tea.Msg {
										return RouteSlotEditorMsg{SlotInstance: item.slotInstance}
									})
								} else if item.instance != nil && item.instance.State() != core.TemplateInstanceStatusNoInput {
									cmds = append(cmds, func() tea.Msg {
										return RouteLiteralsEditorMsg{TemplateInstance: item.instance}
									})
								}
							}
						} else {
							// 未选中, 选中该 item
							model.templateFlatTree.Select(itemIndex)
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
					model.templateFlatTree, cmd = model.templateFlatTree.Update(tea.KeyMsg{Type: tea.KeyUp})
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
					model.templateFlatTree, cmd = model.templateFlatTree.Update(tea.KeyMsg{Type: tea.KeyDown})
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
		case "ctrl+c", "esc":
			return model, tea.Quit
		case "left":
			model.focus = 0
		case "right":
			model.focus = 1
		case "up", "down":
			if model.focus == 0 {
				var cmd tea.Cmd
				model.templateFlatTree, cmd = model.templateFlatTree.Update(msg)
				cmds = append(cmds, cmd)
				model.refreshTemplatePreview()
			} else {
				var cmd tea.Cmd
				model.templatePreview, cmd = model.templatePreview.Update(msg)
				cmds = append(cmds, cmd)
			}
		case "enter":
			if model.focus == 0 {
				if item, ok := model.templateFlatTree.SelectedItem().(templateFlatTreeItem); ok {
					if item.instance == nil && item.slotInstance != nil {
						cmds = append(cmds, func() tea.Msg {
							return RouteSlotEditorMsg{
								SlotInstance: item.slotInstance,
							}
						})
					} else if item.instance != nil && item.instance.State() != core.TemplateInstanceStatusNoInput {
						cmds = append(cmds, func() tea.Msg {
							return RouteLiteralsEditorMsg{TemplateInstance: item.instance}
						})
					}
				}
			}
		case "backspace", "delete":
			if model.focus == 0 {
				if item, ok := model.templateFlatTree.SelectedItem().(templateFlatTreeItem); ok {
					if item.instance != nil && item.slotInstance != nil {
						item.slotInstance.Remove(item.instance)
						model.refreshTemplateFlatTree()
						model.refreshTemplatePreview()
					}
				}
			}
		case " ":
			result := model.templateManager.Render(model.rootSlotInstance.Instances[0], false)
			cmds = append(cmds, func() tea.Msg {
				return ResultMsg{Result: result}
			})
		default:
			if model.focus == 0 {
				var cmd tea.Cmd
				model.templateFlatTree, cmd = model.templateFlatTree.Update(msg)
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

func (model *MainScreen) View() string {
	var body string
	if model.focus == 0 {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			model.focusedTitledBox.Render("Structure", model.templateFlatTree.View(), model.layout.leftPanelWidth, model.layout.height-2),
			model.blurredTitledBox.Render("Preview", model.templatePreview.View(), model.layout.rightPanelWidth, model.layout.height-2),
		)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			model.blurredTitledBox.Render("Structure", model.templateFlatTree.View(), model.layout.leftPanelWidth, model.layout.height-2),
			model.focusedTitledBox.Render("Preview", model.templatePreview.View(), model.layout.rightPanelWidth, model.layout.height-2),
		)
	}

	var tooltip string
	if model.focus == 0 {
		tooltip = mainStyle.Global.Tooltip.Render("←→: focus | ↑↓: navigate | Enter: edit | Backspace/Delete: delete | Space: generate | Esc: quit")
	} else {
		tooltip = mainStyle.Global.Tooltip.Render("←→: focus | Esc: quit")
	}

	return body + "\n" + tooltip
}

func (model *MainScreen) UpdateSizes(width, height int) {
	model.layout.update(width, height)
	model.templateFlatTree.SetSize(model.layout.leftPanelWidth-2, model.layout.contentHeight)
	model.templatePreview.Width = model.layout.rightPanelWidth - 2
	model.templatePreview.Height = model.layout.contentHeight
	model.refreshTemplateFlatTree()
	model.refreshTemplatePreview()
}

// endregion

// region 内部方法

func (model *MainScreen) refreshTemplateFlatTree() {
	model.templateFlatTree.SetItems(buildTemplateFlatTree(model.rootSlotInstance))
}

func (model *MainScreen) refreshTemplatePreview() {
	item, ok := model.templateFlatTree.SelectedItem().(templateFlatTreeItem)
	if !ok || item.instance == nil {
		model.templatePreview.SetContent("")
		return
	}

	raw := model.templateManager.Render(item.instance, true)
	lines := strings.Split(raw, "\n")
	levels := mainStyle.TemplatePreview.Levels
	baseLevel := item.level

	var out strings.Builder
	var styleStack []lipgloss.Style

	// 获取当前栈顶样式的辅助函数
	currentStyle := func() lipgloss.Style {
		if len(styleStack) == 0 {
			return lipgloss.NewStyle()
		}
		return styleStack[len(styleStack)-1]
	}

	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}

		// 使用滑动窗口的方式扫描单行字符串
		for len(line) > 0 {
			// 寻找下一个特殊符号的位置
			idx := strings.IndexAny(line, "⟦⟧⟪⟫")

			// 没有找到特殊符号, 直接渲染剩余部分并结束本行
			if idx == -1 {
				out.WriteString(currentStyle().Render(line))
				break
			}

			// 如果符号前有普通文本, 先将其渲染并写入
			if idx > 0 {
				out.WriteString(currentStyle().Render(line[:idx]))
				line = line[idx:] // 截断已处理的文本
			}

			// 此时 line 必定以特殊符号开头, 检查各种模式
			switch {
			case strings.HasPrefix(line, "⟦f|"):
				styleStack = append(styleStack, mainStyle.TemplatePreview.Fragment)
				line = line[len("⟦f|"):]
			case strings.HasPrefix(line, "⟪=|"):
				styleStack = append(styleStack, mainStyle.TemplatePreview.Literal)
				line = line[len("⟪=|"):]
			case strings.HasPrefix(line, "⟪x|"):
				styleStack = append(styleStack, mainStyle.TemplatePreview.LiteralNull)
				line = line[len("⟪x|"):]
			case strings.HasPrefix(line, "⟧"):
				if len(styleStack) > 0 {
					styleStack = styleStack[:len(styleStack)-1]
				}
				line = line[len("⟧"):]
			case strings.HasPrefix(line, "⟫"):
				if len(styleStack) > 0 {
					styleStack = styleStack[:len(styleStack)-1]
				}
				line = line[len("⟫"):]
			case strings.HasPrefix(line, "⟦"):
				pipeIdx := strings.IndexByte(line, '|')
				if pipeIdx > len("⟦") {
					levelStr := line[len("⟦"):pipeIdx]
					if level, err := strconv.Atoi(levelStr); err == nil {
						styleStack = append(styleStack, levels[(baseLevel+level-1)%len(levels)])
						line = line[pipeIdx+1:]
						continue
					}
				}
				// 解析数字失败时的 fallback 处理 (将其视为普通文本)
				out.WriteString(currentStyle().Render(line[:len("⟦")]))
				line = line[len("⟦"):]
			default:
				// 处理意外的特殊符号, 避免死循环
				r, size := utf8.DecodeRuneInString(line)
				out.WriteString(currentStyle().Render(string(r)))
				line = line[size:]
			}
		}
		out.WriteString(mainStyle.TemplatePreview.LineEnd.Render("$"))
	}

	model.templatePreview.SetContent(out.String())
}

// endregion

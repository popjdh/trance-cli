package tui

import (
	"trance-cli/cmd/tmpl/core"

	tea "github.com/charmbracelet/bubbletea"
)

// region 屏幕切换消息

type RouteLiteralsEditorMsg struct {
	TemplateInstance *core.TemplateInstance
}

type RouteSlotEditorMsg struct {
	SlotInstance *core.SlotInstance
}

type RouteBackMsg struct{}

type ResultMsg struct {
	Result string
}

// endregion

// region 屏幕常量

type Screen int

const (
	screenMain Screen = iota
	screenLiteralsEditor
	screenSlotEditor
)

// endregion

// region RootModel

type RootModel struct {
	screen               Screen
	mainScreen           *MainScreen
	literalsEditorScreen *LiteralsEditorScreen
	slotEditorScreen     *SlotEditorScreen
	templateManager      *core.TemplateManager
	rootSlotInstance     *core.SlotInstance
	width                int
	height               int

	Result string
}

func NewRootModel(tmplManager *core.TemplateManager, rootSlotInst *core.SlotInstance) RootModel {
	mainScreen := NewMainScreen(tmplManager, rootSlotInst)
	return RootModel{
		screen:           screenMain,
		mainScreen:       &mainScreen,
		templateManager:  tmplManager,
		rootSlotInstance: rootSlotInst,
	}
}

func (model RootModel) Init() tea.Cmd {
	return model.mainScreen.Init()
}

func (model RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
	case RouteLiteralsEditorMsg:
		model.screen = screenLiteralsEditor
		literalsEditorScreen := NewLiteralEditorScreen(msg.TemplateInstance, model.width, model.height)
		model.literalsEditorScreen = &literalsEditorScreen
		return model, nil
	case RouteSlotEditorMsg:
		templates := model.templateManager.List(msg.SlotInstance.Definition.Namespace)
		if len(templates) == 0 {
			return model, func() tea.Msg { return RouteBackMsg{} }
		}
		if len(templates) == 1 {
			msg.SlotInstance.Embed(model.templateManager.NewInstance(templates[0]))
			return model, func() tea.Msg { return RouteBackMsg{} }
		}
		model.screen = screenSlotEditor
		slotEditorScreen := NewSlotEditorScreen(model.templateManager, msg.SlotInstance, model.width, model.height)
		model.slotEditorScreen = &slotEditorScreen
		return model, nil
	case RouteBackMsg:
		model.screen = screenMain
		model.mainScreen.UpdateSizes(model.width, model.height)
		return model, nil
	case ResultMsg:
		model.Result = msg.Result
		return model, tea.Quit
	}

	switch model.screen {
	case screenMain:
		_, cmd := model.mainScreen.Update(msg)
		return model, cmd
	case screenLiteralsEditor:
		_, cmd := model.literalsEditorScreen.Update(msg)
		return model, cmd
	case screenSlotEditor:
		_, cmd := model.slotEditorScreen.Update(msg)
		return model, cmd
	}
	return model, nil
}

func (model RootModel) View() string {
	switch model.screen {
	case screenMain:
		return model.mainScreen.View()
	case screenLiteralsEditor:
		return model.literalsEditorScreen.View()
	case screenSlotEditor:
		return model.slotEditorScreen.View()
	}
	return ""
}

// endregion

package ssh

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tuiAppStyle              = lipgloss.NewStyle().Padding(1, 1)
	tuiTitleStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	tuiListItemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	tuiListSelectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	tuiHelpStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingTop(1)
)

type TuiListItem struct {
	host string
}

func (listItem TuiListItem) Title() string       { return listItem.host }
func (listItem TuiListItem) Description() string { return "" }
func (listItem TuiListItem) FilterValue() string { return listItem.host }

type TuiListItemDelegate struct{}

func (listItemDelegate TuiListItemDelegate) Height() int  { return 1 }
func (listItemDelegate TuiListItemDelegate) Spacing() int { return 0 }
func (listItemDelegate TuiListItemDelegate) Update(msg tea.Msg, model *list.Model) tea.Cmd {
	return nil
}
func (listItemDelegate TuiListItemDelegate) Render(writer io.Writer, model list.Model, index int, listItem list.Item) {
	style := tuiListItemStyle
	str := listItem.(TuiListItem).Title()
	if index == model.Index() {
		style = tuiListSelectedItemStyle
		str = fmt.Sprintf("> %s", str)
	}
	_, _ = fmt.Fprint(writer, style.Render(str))
}

type TuiModel struct {
	teaList           list.Model
	teaTextInput      textinput.Model
	originalListItems []list.Item
	quitting          bool
	selectedHost      string
}

func newTuiModel(hosts []string) TuiModel {
	teaListItems := make([]list.Item, len(hosts))
	for i, host := range hosts {
		teaListItems[i] = TuiListItem{host: host}
	}

	teaList := list.New(teaListItems, TuiListItemDelegate{}, 0, 0)
	teaList.Title = "Select an SSH Host"
	teaList.SetShowStatusBar(false)
	teaList.SetFilteringEnabled(false)
	teaList.Styles.Title = tuiTitleStyle
	teaList.Styles.HelpStyle = tuiHelpStyle

	teaTextInput := textinput.New()
	teaTextInput.Placeholder = ""
	teaTextInput.Focus()
	teaTextInput.CharLimit = 42
	teaTextInput.Width = 50

	return TuiModel{teaList: teaList, teaTextInput: teaTextInput, originalListItems: teaListItems}
}

func (model TuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (model TuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width, height := tuiAppStyle.GetFrameSize()
		model.teaList.SetSize(msg.Width-width, msg.Height-height-1)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			model.quitting = true
			return model, tea.Quit
		case "enter":
			listItem, ok := model.teaList.SelectedItem().(TuiListItem)
			if ok {
				model.selectedHost = listItem.host
				model.quitting = true
				return model, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	model.teaTextInput, cmd = model.teaTextInput.Update(msg)
	searchWord := strings.ToLower(model.teaTextInput.Value())
	var filteredListItems []list.Item
	for _, listItem := range model.originalListItems {
		if strings.Contains(strings.ToLower(listItem.FilterValue()), searchWord) {
			filteredListItems = append(filteredListItems, listItem)
		}
	}
	model.teaList.SetItems(filteredListItems)
	model.teaList, _ = model.teaList.Update(msg)
	return model, cmd
}

func (model TuiModel) View() string {
	if model.quitting {
		return ""
	}
	return tuiAppStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			model.teaTextInput.View(),
			model.teaList.View(),
		),
	)
}

func RunSelector(hosts []string) (string, error) {
	program := tea.NewProgram(newTuiModel(hosts), tea.WithAltScreen())
	model, err := program.Run()
	if err != nil {
		return "", fmt.Errorf("TUI 运行错误\n%w", err)
	}
	result := model.(TuiModel)
	if result.selectedHost == "" {
		return "", fmt.Errorf("未选择主机")
	}
	return result.selectedHost, nil
}

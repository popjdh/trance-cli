package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TitledBox 带标题的边框面板
type TitledBox struct {
	BoxStyle   lipgloss.Style
	TitleStyle lipgloss.Style
}

// NewTitledBox 创建带标题的边框面板
func NewTitledBox(boxStyle lipgloss.Style, titleStyle lipgloss.Style) TitledBox {
	return TitledBox{
		BoxStyle:   boxStyle,
		TitleStyle: titleStyle,
	}
}

// Render 渲染带标题的面板
func (titledBox TitledBox) Render(title string, body string, width int, height int) string {
	border := titledBox.BoxStyle.GetBorderStyle()
	fg := titledBox.BoxStyle.GetBorderTopForeground()
	borderStyler := lipgloss.NewStyle().Foreground(fg).Render

	topLeft := borderStyler(border.TopLeft)
	topRight := borderStyler(border.TopRight)
	renderedTitle := titledBox.TitleStyle.Render(title)

	cellsShort := max(0, width-lipgloss.Width(topLeft+topRight+renderedTitle))
	gap := strings.Repeat(border.Top, cellsShort)
	top := topLeft + renderedTitle + borderStyler(gap) + topRight

	contentWidth := max(0, width-2)

	bottom := titledBox.BoxStyle.Copy().
		BorderTop(false).
		Width(contentWidth).
		Height(height - 1).
		Render(body)

	return top + "\n" + bottom
}

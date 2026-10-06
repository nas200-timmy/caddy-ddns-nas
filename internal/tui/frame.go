package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle = lipgloss.NewStyle().Bold(true)
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	btnActive   = lipgloss.NewStyle().Background(lipgloss.Color("203")).Foreground(lipgloss.Color("255")).Bold(true)
	btnInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// button 是对话框页脚的一个按钮。
type button struct {
	label  string
	active bool // 当前高亮（Enter 触发）
}

// renderFooter 渲染 Debian 风格按钮行，如 [ <上一步> ]  [ <确定> ]。
func renderFooter(btns []button) string {
	parts := make([]string, 0, len(btns))
	for _, b := range btns {
		label := " <" + b.label + "> "
		if b.active {
			parts = append(parts, btnActive.Render(label))
		} else {
			parts = append(parts, btnInactive.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// renderDialog 把标题、内容、按钮行装进 Debian 安装器风格对话框。
func renderDialog(title, content, footer string, width int) string {
	const h, v = "─", "│"
	inner := width - 2
	if inner < 20 {
		inner = 20
	}

	// 顶边框：┌─┤ 标题 ├──────┐
	titlePart := "┤ " + title + " ├"
	top := "┌" + h
	if lipgloss.Width(titlePart) <= inner {
		top += titlePart + strings.Repeat(h, inner-lipgloss.Width(titlePart))
	} else {
		top += strings.Repeat(h, inner)
	}
	top += "┐"

	var rows []string
	for _, line := range strings.Split(content, "\n") {
		w := lipgloss.Width(line)
		if w <= inner {
			line += strings.Repeat(" ", inner-w)
		}
		rows = append(rows, v+line+v)
	}

	sep := "├" + strings.Repeat(h, inner) + "┤"
	fw := lipgloss.Width(footer)
	if fw <= inner {
		footer += strings.Repeat(" ", inner-fw)
	}
	bottom := "└" + strings.Repeat(h, inner) + "┘"

	return top + "\n" + strings.Join(rows, "\n") + "\n" + sep + "\n" + v + footer + v + "\n" + bottom
}

// maskSecret 脱敏显示凭证。
func maskSecret(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

package tui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cddns/internal/site"
)

// schemeItem 是源站协议选择列表的一项。
type schemeItem struct {
	label string
	value string
}

func (i schemeItem) FilterValue() string { return i.label }

type schemeDelegate struct{}

func (d schemeDelegate) Height() int                               { return 1 }
func (d schemeDelegate) Spacing() int                              { return 0 }
func (d schemeDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d schemeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it := item.(schemeItem)
	if index == m.Index() {
		sel := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("203")).Render(it.label)
		fmt.Fprintf(w, " ▶ %s", sel)
		return
	}
	fmt.Fprintf(w, "   %s", it.label)
}

var schemeItems = []list.Item{
	schemeItem{label: "HTTP（明文，默认）", value: "http"},
	schemeItem{label: "HTTPS（源站有证书）", value: "https"},
}

// portStep 收集源站协议、端口并做连通性检查。
type portStep struct {
	list  list.Model
	input textinput.Model
	focus int // 0=协议列表，1=端口输入框
	err   string
}

func newPortStep() *portStep {
	l := list.New(schemeItems, schemeDelegate{}, 0, 2)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	ti := textinput.New()
	ti.Placeholder = "如 8080"
	ti.CharLimit = 5
	return &portStep{list: l, input: ti}
}

func (s *portStep) title() string { return "源站协议与端口" }

func (s *portStep) init(w *wizard) tea.Cmd {
	s.input.SetValue(w.data.originPort)
	if w.data.originScheme == "" {
		w.data.originScheme = "http"
	}
	for i, it := range schemeItems {
		if it.(schemeItem).value == w.data.originScheme {
			s.list.Select(i)
			break
		}
	}
	s.err = ""
	return s.setFocus(0)
}

func (s *portStep) setFocus(i int) tea.Cmd {
	s.focus = i
	if i == 1 {
		s.input.Focus()
		return textinput.Blink
	}
	s.input.Blur()
	return nil
}

func (s *portStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, navNone
	}

	if s.focus == 0 {
		switch key.String() {
		case "enter":
			if sel, ok := s.list.SelectedItem().(schemeItem); ok {
				w.data.originScheme = sel.value
			}
			return s.setFocus(1), navNone
		case "up", "down", "j", "k":
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			if sel, ok := s.list.SelectedItem().(schemeItem); ok {
				w.data.originScheme = sel.value
			}
			return cmd, navNone
		case "esc":
			return nil, navBack
		}
		return nil, navNone
	}

	switch key.String() {
	case "enter":
		v := strings.TrimSpace(s.input.Value())
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			s.err = "端口必须是 1-65535 的数字"
			return nil, navNone
		}
		s.err = ""
		w.data.originPort = v
		// 同步做连通性检查（超时 3 秒），结果供汇总页展示。
		ok, note := site.CheckOrigin(w.data.origin, p, w.data.originScheme)
		w.data.healthOK = ok
		w.data.healthNote = note
		return nil, navNext
	case "esc", "up", "tab":
		return s.setFocus(0), navNone
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return cmd, navNone
}

func (s *portStep) view(w *wizard) string {
	width := w.width
	if width < 60 {
		width = 78
	}
	s.list.SetWidth(width - 10)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 选择源站协议："))
	b.WriteString("\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 请输入源站端口："))
	b.WriteString("\n\n ")
	b.WriteString(s.input.View())
	switch {
	case s.err != "":
		b.WriteString("\n\n " + errStyle.Render("✗ "+s.err))
	case w.data.healthOK:
		b.WriteString("\n\n " + okStyle.Render("✓ "+w.data.healthNote))
	case w.data.healthNote != "":
		b.WriteString("\n\n " + warnStyle.Render("✗ "+w.data.healthNote+"（可继续，部署后请确认源站在线）"))
	}
	b.WriteString("\n")
	return b.String()
}

func (s *portStep) footer(w *wizard) []button {
	return []button{{label: "上一步"}, {label: "确定", active: true}}
}

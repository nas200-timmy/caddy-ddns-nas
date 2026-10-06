package tui

import (
	"errors"
	"net"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

func validateOrigin(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return errors.New("请输入源站地址")
	}
	if net.ParseIP(v) != nil {
		return nil
	}
	if !hostnameRE.MatchString(v) {
		return errors.New("格式不正确，示例：192.168.1.10 / 100.64.0.2 / nas.local")
	}
	return nil
}

// originStep 收集源站 IP 或域名。
type originStep struct {
	input textinput.Model
	err   string
}

func newOriginStep() *originStep {
	ti := textinput.New()
	ti.Placeholder = "如 192.168.1.10、100.64.0.2 或 nas.local"
	ti.CharLimit = 253
	return &originStep{input: ti}
}

func (s *originStep) title() string { return "源站地址" }

func (s *originStep) init(w *wizard) tea.Cmd {
	s.input.SetValue(w.data.origin)
	s.input.Focus()
	return textinput.Blink
}

func (s *originStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, navNone
	}
	switch key.String() {
	case "enter":
		if err := validateOrigin(s.input.Value()); err != nil {
			s.err = err.Error()
			return nil, navNone
		}
		s.err = ""
		w.data.origin = strings.TrimSpace(s.input.Value())
		return nil, navNext
	case "esc":
		return nil, navBack
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return cmd, navNone
}

func (s *originStep) view(w *wizard) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 请输入源站 IP 或域名（被代理的目标地址）："))
	b.WriteString("\n\n ")
	b.WriteString(s.input.View())
	if s.err != "" {
		b.WriteString("\n\n " + errStyle.Render("✗ "+s.err))
	} else {
		b.WriteString("\n\n " + hintStyle.Render("支持内网 IP、Tailscale IP、FRP 地址。"))
	}
	b.WriteString("\n")
	return b.String()
}

func (s *originStep) footer(w *wizard) []button {
	return []button{{label: "上一步"}, {label: "确定", active: true}}
}

package tui

import (
	"errors"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var domainRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

func validateDomain(v string) error {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return errors.New("请输入转发域名")
	}
	if !domainRE.MatchString(v) {
		return errors.New("域名格式不正确，示例：nas.example.com")
	}
	return nil
}

// domainStep 收集对外转发域名。
type domainStep struct {
	input textinput.Model
	err   string
}

func newDomainStep() *domainStep {
	ti := textinput.New()
	ti.Placeholder = "如 nas.example.com"
	ti.CharLimit = 253
	return &domainStep{input: ti}
}

func (s *domainStep) title() string { return "转发域名" }

func (s *domainStep) init(w *wizard) tea.Cmd {
	s.input.SetValue(w.data.domain)
	s.input.Focus()
	return textinput.Blink
}

func (s *domainStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, navNone
	}
	switch key.String() {
	case "enter":
		if err := validateDomain(s.input.Value()); err != nil {
			s.err = err.Error()
			return nil, navNone
		}
		s.err = ""
		w.data.domain = strings.ToLower(strings.TrimSpace(s.input.Value()))
		return nil, navNext
	case "esc":
		return nil, navBack
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return cmd, navNone
}

func (s *domainStep) view(w *wizard) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 请输入对外转发域名："))
	b.WriteString("\n\n ")
	b.WriteString(s.input.View())
	b.WriteString("\n\n " + hintStyle.Render("部署时将自动：写入 A 记录 → 用 DNS-01 验证签发免费证书 → 开启代理。"))
	if s.err != "" {
		b.WriteString("\n\n " + errStyle.Render("✗ "+s.err))
	}
	b.WriteString("\n")
	return b.String()
}

func (s *domainStep) footer(w *wizard) []button {
	return []button{{label: "上一步"}, {label: "确定", active: true}}
}

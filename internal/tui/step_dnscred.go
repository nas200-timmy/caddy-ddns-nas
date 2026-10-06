package tui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cddns/internal/config"
)

// providerItem 是服务商选择列表的一项。
type providerItem struct {
	label string
	desc  string
	value config.DNSProvider
}

func (i providerItem) FilterValue() string { return i.label }

type providerDelegate struct{}

func (d providerDelegate) Height() int                               { return 1 }
func (d providerDelegate) Spacing() int                              { return 0 }
func (d providerDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d providerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it := item.(providerItem)
	if index == m.Index() {
		sel := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("203")).Render(it.label)
		fmt.Fprintf(w, " ▶ %s —— %s", sel, it.desc)
		return
	}
	fmt.Fprintf(w, "   %s —— %s", it.label, it.desc)
}

var providerItems = []list.Item{
	providerItem{label: "阿里云 DNS", desc: "RAM 子账户 AccessKey（建议 AliyunDNSFullAccess）", value: config.ProviderAliyun},
	providerItem{label: "腾讯云 DNSPod", desc: "子权限 SecretId / SecretKey", value: config.ProviderTencentCloud},
	providerItem{label: "Cloudflare", desc: "API Token（Zone.DNS Edit 权限）", value: config.ProviderCloudflare},
}

// dnsCredStep 收集 DNS 服务商与凭证。
type dnsCredStep struct {
	list   list.Model
	fields []textinput.Model // 依据服务商动态生成
	focus  int               // 0=列表，1..len(fields)=输入框
	err    string
}

func newDNSCredStep() *dnsCredStep {
	l := list.New(providerItems, providerDelegate{}, 0, 3)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	return &dnsCredStep{list: l}
}

func (s *dnsCredStep) title() string { return "DNS 凭证" }

// saveFields 把当前输入框内容按服务商存回数据（切换服务商/焦点时防止丢失）。
func (s *dnsCredStep) saveFields(w *wizard) {
	switch w.data.provider {
	case config.ProviderCloudflare:
		if len(s.fields) > 0 {
			w.data.token = strings.TrimSpace(s.fields[0].Value())
		}
	default:
		if len(s.fields) == 2 {
			w.data.secretID = strings.TrimSpace(s.fields[0].Value())
			w.data.secretKey = strings.TrimSpace(s.fields[1].Value())
		}
	}
}

// rebuildFields 按当前服务商重建输入框并回填已输入的值。
func (s *dnsCredStep) rebuildFields(w *wizard) {
	switch w.data.provider {
	case config.ProviderCloudflare:
		s.fields = []textinput.Model{newSecretInput("API Token")}
		s.fields[0].SetValue(w.data.token)
	default:
		s.fields = []textinput.Model{
			newSecretInput("SecretId"),
			newSecretInput("SecretKey"),
		}
		s.fields[0].SetValue(w.data.secretID)
		s.fields[1].SetValue(w.data.secretKey)
	}
}

func newSecretInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 200
	return ti
}

func (s *dnsCredStep) init(w *wizard) tea.Cmd {
	s.rebuildFields(w)
	s.list.SetItems(providerItems)
	for i, it := range providerItems {
		if it.(providerItem).value == w.data.provider {
			s.list.Select(i)
			break
		}
	}
	s.err = ""
	return s.focusField(0)
}

func (s *dnsCredStep) focusField(i int) tea.Cmd {
	s.focus = i
	for j := range s.fields {
		s.fields[j].Blur()
	}
	if i >= 1 && i <= len(s.fields) {
		s.fields[i-1].Focus()
		return textinput.Blink
	}
	return nil
}

func (s *dnsCredStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, navNone
	}

	// 列表获得焦点时，由列表消费按键。
	if s.focus == 0 {
		switch key.String() {
		case "enter":
			sel, ok := s.list.SelectedItem().(providerItem)
			if ok {
				w.data.provider = sel.value
			}
			return s.focusField(1), navNone
		case "up", "down", "j", "k":
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			sel, ok := s.list.SelectedItem().(providerItem)
			if ok && sel.value != w.data.provider {
				s.saveFields(w)
				w.data.provider = sel.value
				s.rebuildFields(w)
			}
			return cmd, navNone
		case "esc":
			return nil, navBack
		}
		return nil, navNone
	}

	// 输入框焦点。
	switch key.String() {
	case "tab", "down":
		next := s.focus + 1
		if next > len(s.fields) {
			next = 0
		}
		return s.focusField(next), navNone
	case "shift+tab", "up":
		prev := s.focus - 1
		if prev < 0 {
			prev = len(s.fields)
		}
		return s.focusField(prev), navNone
	case "esc":
		s.saveFields(w)
		return s.focusField(0), navNone
	case "enter":
		if s.focus < len(s.fields) {
			return s.focusField(s.focus + 1), navNone
		}
		if err := s.validate(w); err != nil {
			s.err = err.Error()
			return nil, navNone
		}
		s.err = ""
		return nil, navNext
	}
	var cmd tea.Cmd
	s.fields[s.focus-1], cmd = s.fields[s.focus-1].Update(msg)
	return cmd, navNone
}

func (s *dnsCredStep) validate(w *wizard) error {
	switch w.data.provider {
	case config.ProviderAliyun, config.ProviderTencentCloud:
		w.data.secretID = strings.TrimSpace(s.fields[0].Value())
		w.data.secretKey = strings.TrimSpace(s.fields[1].Value())
		if w.data.secretID == "" || w.data.secretKey == "" {
			return errors.New("SecretId 与 SecretKey 都不能为空")
		}
	case config.ProviderCloudflare:
		w.data.token = strings.TrimSpace(s.fields[0].Value())
		if w.data.token == "" {
			return errors.New("API Token 不能为空")
		}
	default:
		return errors.New("请选择 DNS 服务商")
	}
	return nil
}

func (s *dnsCredStep) view(w *wizard) string {
	width := w.width
	if width < 60 {
		width = 78
	}
	s.list.SetWidth(width - 10)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 选择 DNS 服务商："))
	b.WriteString("\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 凭证（用于写入 A 记录与证书 DNS-01 验证）："))
	b.WriteString("\n\n")
	for i := range s.fields {
		b.WriteString(" " + s.fields[i].View() + "\n")
	}
	b.WriteString("\n " + hintStyle.Render("凭证仅保存在本机配置文件（权限 600），不会上传。"))
	if s.err != "" {
		b.WriteString("\n\n " + errStyle.Render("✗ "+s.err))
	}
	b.WriteString("\n")
	return b.String()
}

func (s *dnsCredStep) footer(w *wizard) []button {
	return []button{{label: "上一步"}, {label: "确定", active: true}}
}

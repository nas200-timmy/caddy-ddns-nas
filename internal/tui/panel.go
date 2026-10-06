package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cddns/internal/config"
	"cddns/internal/site"
)

type panelModel struct {
	cfg     *config.Config
	cfgPath string

	width  int
	height int
	health map[string]string // domain → 健康检查结果
	rerun  bool
}

type healthDoneMsg struct{ notes map[string]string }

// RunPanel 打开日常管理面板；返回 true 表示用户选择重跑配置向导。
func RunPanel(cfgPath string) (bool, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return false, err
	}
	p := tea.NewProgram(&panelModel{cfg: cfg, cfgPath: cfgPath})
	m, err := p.Run()
	if err != nil {
		return false, err
	}
	return m.(*panelModel).rerun, nil
}

func (m *panelModel) Init() tea.Cmd {
	return func() tea.Msg {
		notes := map[string]string{}
		for _, s := range m.cfg.Sites {
			ok, note := site.CheckOrigin(s.Origin, s.OriginPort, s.OriginScheme)
			if ok {
				notes[s.Domain] = okStyle.Render("✓ " + note)
			} else {
				notes[s.Domain] = warnStyle.Render("✗ " + note)
			}
		}
		return healthDoneMsg{notes: notes}
	}
}

func (m *panelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.rerun = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	case healthDoneMsg:
		m.health = msg.notes
	}
	return m, nil
}

// certStatus 检查域名的证书文件是否已签发。
func certStatus(domain string) string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	cert := filepath.Join(dataHome, "caddy", "certificates", "local", domain, domain+".crt")
	if _, err := os.Stat(cert); err == nil {
		return okStyle.Render("✓ 已签发")
	}
	return hintStyle.Render("未签发（cddns run 启动后自动申请）")
}

func (m *panelModel) View() string {
	width := m.width
	if width < 60 {
		width = 78
	}
	var content strings.Builder
	content.WriteString("\n")
	if len(m.cfg.Sites) == 0 {
		content.WriteString("  尚未配置任何站点。\n\n  请先运行: cddns setup\n")
	} else {
		content.WriteString(labelStyle.Render("  站点："))
		content.WriteString("\n\n")
		for _, s := range m.cfg.Sites {
			h := hintStyle.Render("检测中...")
			if note, ok := m.health[s.Domain]; ok {
				h = note
			}
			scheme := s.OriginScheme
			if scheme == "" {
				scheme = "http"
			}
			fmt.Fprintf(&content, "    %s → %s://%s:%d\n", s.Domain, scheme, s.Origin, s.OriginPort)
			fmt.Fprintf(&content, "      证书: %s  源站: %s\n", certStatus(s.Domain), h)
		}
		content.WriteString("\n")
		fmt.Fprintf(&content, "  DNS 服务商: %s\n", providerLabel(m.cfg.Credential.Provider))
		fmt.Fprintf(&content, "  配置文件:   %s\n", m.cfgPath)
	}
	content.WriteString("\n")
	content.WriteString(hintStyle.Render("  按 Enter 重跑配置向导，q 退出。"))
	content.WriteString("\n")
	return renderDialog("cddns 管理面板", content.String(), renderFooter([]button{{label: "配置向导", active: true}}), width)
}

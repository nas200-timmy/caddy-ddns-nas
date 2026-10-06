package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type quitMsg struct{}

// doneStep 部署完成后的收尾页。
type doneStep struct{}

func (s *doneStep) title() string { return "完成" }

func (s *doneStep) init(w *wizard) tea.Cmd {
	if w.doneHold <= 0 {
		return func() tea.Msg { return quitMsg{} }
	}
	return tea.Tick(w.doneHold, func(time.Time) tea.Msg { return quitMsg{} })
}

func (s *doneStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	switch msg.(type) {
	case tea.KeyMsg, quitMsg:
		return nil, navQuit
	}
	return nil, navNone
}

func (s *doneStep) view(w *wizard) string {
	d := w.data
	lines := []string{
		"",
		okStyle.Render("  ✓ 配置完成！"),
		"",
		fmt.Sprintf("   配置已保存到 %s", w.cfgPath),
		fmt.Sprintf("   转发域名  %s → %s://%s:%s", d.domain, d.originScheme, d.origin, d.originPort),
	}
	if d.publicIP != "" {
		lines = append(lines, fmt.Sprintf("   公网 IP   %s", d.publicIP))
	}
	lines = append(lines,
		"",
		"  下一步：",
		"    cddns run      启动代理服务（systemd 单元随交付版本提供）",
		"    cddns setup    重新运行本向导修改配置",
		"",
		"  按任意键退出。",
		"",
	)
	return strings.Join(lines, "\n")
}

func (s *doneStep) footer(w *wizard) []button {
	return []button{{label: "退出", active: true}}
}

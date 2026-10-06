package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// welcomeStep 展示说明并做环境自检。
type welcomeStep struct{}

func (s *welcomeStep) title() string { return "欢迎" }

func (s *welcomeStep) init(w *wizard) tea.Cmd {
	// 端口检查即时完成，直接记录结果。
	w.data.port80Free = portFree(80)
	w.data.port443Free = portFree(443)
	return nil
}

func (s *welcomeStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		return nil, navNext
	}
	return nil, navNone
}

func (s *welcomeStep) view(w *wizard) string {
	check := func(name string, free bool) string {
		if free {
			return okStyle.Render(fmt.Sprintf("  ✓ %s 端口：空闲", name))
		}
		return warnStyle.Render(fmt.Sprintf("  ✗ %s 端口：已被占用（代理服务需要监听）", name))
	}
	return strings.Join([]string{
		"",
		"本向导将一步步完成以下配置：",
		"  1. 源站地址（被代理的 IP 或域名，支持内网 IP / Tailscale / FRP）",
		"  2. 源站端口",
		"  3. DNS 服务商凭证（用于写入 A 记录与证书 DNS-01 验证）",
		"  4. 对外转发域名",
		"",
		"部署时将自动：写入 A 记录 → 签发免费证书 → 开启反向代理。",
		"",
		"环境自检：",
		check("80", w.data.port80Free),
		check("443", w.data.port443Free),
		"",
		"操作：Enter 确认，Esc 返回上一步，Ctrl+C 退出。",
		"",
	}, "\n")
}

func (s *welcomeStep) footer(w *wizard) []button {
	return []button{{label: "确定", active: true}}
}

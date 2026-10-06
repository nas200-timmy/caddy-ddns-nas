package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cddns/internal/config"
)

// summaryStep 展示配置摘要，等待用户确认部署。
type summaryStep struct{}

func (s *summaryStep) title() string { return "确认配置" }

func (s *summaryStep) init(w *wizard) tea.Cmd { return nil }

func (s *summaryStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, navNone
	}
	switch key.String() {
	case "enter":
		return nil, navNext
	case "esc":
		return nil, navBack
	}
	return nil, navNone
}

func providerLabel(p config.DNSProvider) string {
	switch p {
	case config.ProviderAliyun:
		return "阿里云 DNS"
	case config.ProviderTencentCloud:
		return "腾讯云 DNSPod"
	case config.ProviderCloudflare:
		return "Cloudflare"
	}
	return string(p)
}

func (s *summaryStep) view(w *wizard) string {
	d := w.data
	cred := ""
	switch d.provider {
	case config.ProviderCloudflare:
		cred = "Token: " + maskSecret(d.token)
	default:
		cred = "SecretId: " + maskSecret(d.secretID) + "  SecretKey: " + maskSecret(d.secretKey)
	}
	health := hintStyle.Render("未检查")
	if d.healthOK {
		health = okStyle.Render("✓ " + d.healthNote)
	} else if d.healthNote != "" {
		health = warnStyle.Render("✗ " + d.healthNote)
	}
	return strings.Join([]string{
		"",
		labelStyle.Render(" 配置摘要："),
		"",
		fmt.Sprintf("   源站      %s://%s:%s", d.originScheme, d.origin, d.originPort),
		fmt.Sprintf("   健康检查  %s", health),
		fmt.Sprintf("   服务商    %s", providerLabel(d.provider)),
		fmt.Sprintf("   凭证      %s", cred),
		fmt.Sprintf("   转发域名  %s", d.domain),
		"",
		" 确认无误后按 Enter 开始部署。",
		"",
	}, "\n")
}

func (s *summaryStep) footer(w *wizard) []button {
	return []button{{label: "上一步"}, {label: "部署", active: true}}
}

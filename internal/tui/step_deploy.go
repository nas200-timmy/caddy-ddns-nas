package tui

import (
	"context"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cddns/internal/ddns"
	"cddns/internal/site"
)

// deployTask 是部署流程中的一步。
type deployTask struct {
	name string
	fn   func(w *wizard) error
}

type taskDoneMsg struct {
	idx int
	err error
}

// deployStep 依次执行部署任务并展示进度。
type deployStep struct {
	tasks   []deployTask
	results []string // 与 tasks 对齐："" 未开始 / "✓" / "✗"
	idx     int
	failed  string
}

func newDeployStep() *deployStep {
	s := &deployStep{}
	s.tasks = []deployTask{
		// 部署流水线：配置落盘 → 探测公网 IP → 校验凭证 → 写 A 记录 → 生成代理配置 → 签发证书。
		{name: "保存配置", fn: func(w *wizard) error {
			return w.data.toConfig().Save(w.cfgPath)
		}},
		{name: "探测公网 IP", fn: func(w *wizard) error {
			ctx := context.Background()
			if w.cfg.PublicIP != "" {
				w.data.publicIP = w.cfg.PublicIP
				return nil
			}
			ip, err := w.detectIP(ctx)
			if err != nil {
				return err
			}
			w.data.publicIP = ip
			return nil
		}},
		{name: "校验 DNS 凭证", fn: func(w *wizard) error {
			ctx := context.Background()
			p, err := w.newDNSProvider(w.data.toConfig().Credential)
			if err != nil {
				return err
			}
			_, err = ddns.ResolveZone(ctx, p, w.data.domain)
			return err
		}},
		{name: "写入 A 记录", fn: func(w *wizard) error {
			ctx := context.Background()
			p, err := w.newDNSProvider(w.data.toConfig().Credential)
			if err != nil {
				return err
			}
			return ddns.UpsertA(ctx, p, w.data.domain, w.data.publicIP)
		}},
		{name: "生成代理配置", fn: func(w *wizard) error {
			caddyfilePath := filepath.Join(filepath.Dir(w.cfgPath), "Caddyfile")
			return site.WriteCaddyfile(caddyfilePath, w.data.toConfig())
		}},
		{name: "签发证书", fn: func(w *wizard) error {
			caddyfilePath := filepath.Join(filepath.Dir(w.cfgPath), "Caddyfile")
			site.ApplyCredentialEnv(w.data.toConfig())
			return w.issueCert(caddyfilePath, w.data.domain, w.certTimeout)
		}},
	}
	s.results = make([]string, len(s.tasks))
	return s
}

func (s *deployStep) title() string { return "部署" }

func (s *deployStep) init(w *wizard) tea.Cmd {
	s.idx = 0
	s.failed = ""
	for i := range s.results {
		s.results[i] = ""
	}
	return s.run(w, 0)
}

func (s *deployStep) run(w *wizard, i int) tea.Cmd {
	t := s.tasks[i]
	return func() tea.Msg {
		return taskDoneMsg{idx: i, err: t.fn(w)}
	}
}

func (s *deployStep) update(w *wizard, msg tea.Msg) (tea.Cmd, navAction) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return nil, navBack
		case "r":
			if s.failed != "" {
				s.failed = ""
				s.results[s.idx] = ""
				return s.run(w, s.idx), navNone
			}
		}
	case taskDoneMsg:
		if msg.err != nil {
			s.results[msg.idx] = "✗"
			s.failed = s.tasks[msg.idx].name + " 失败：" + msg.err.Error()
			return nil, navNone
		}
		s.results[msg.idx] = "✓"
		if msg.idx+1 < len(s.tasks) {
			s.idx = msg.idx + 1
			return s.run(w, s.idx), navNone
		}
		// 全部完成，自动进入完成页。
		return nil, navNext
	}
	return nil, navNone
}

func (s *deployStep) view(w *wizard) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(" 部署进度："))
	b.WriteString("\n\n")
	done := 0
	for i, t := range s.tasks {
		status := dimStyle.Render(" · ")
		switch s.results[i] {
		case "✓":
			status = okStyle.Render(" ✓ ")
			done++
		case "✗":
			status = errStyle.Render(" ✗ ")
		}
		b.WriteString(" " + status + t.name + "\n")
	}
	b.WriteString("\n " + renderBar(done, len(s.tasks), 30))
	if s.failed != "" {
		b.WriteString("\n\n " + errStyle.Render("✗ "+s.failed))
		b.WriteString("\n\n " + hintStyle.Render("按 r 重试，Esc 返回修改配置。"))
	}
	b.WriteString("\n")
	return b.String()
}

// renderBar 渲染简单进度条。
func renderBar(done, total, width int) string {
	if total <= 0 {
		return ""
	}
	filled := width * done / total
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func (s *deployStep) footer(w *wizard) []button {
	if s.failed != "" {
		return []button{{label: "上一步"}, {label: "重试", active: true}}
	}
	return []button{{label: "上一步"}}
}

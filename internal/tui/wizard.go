// Package tui 实现 cddns 的终端界面：Debian 安装器风格的配置向导。
package tui

import (
	"context"
	"fmt"
	"net"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cddns/internal/caddyembed"
	"cddns/internal/config"
	"cddns/internal/ddns"
)

// navAction 描述步骤希望向导如何翻页。
// 向导在同一轮 Update 中同步应用导航——按键队列快于异步消息，
// 若用异步消息翻页，后续按键会落在旧页面上。
type navAction int

const (
	navNone navAction = iota
	navNext
	navBack
	navQuit
)

// step 是向导中的一页。
type step interface {
	title() string
	init(w *wizard) tea.Cmd
	update(w *wizard, msg tea.Msg) (tea.Cmd, navAction)
	view(w *wizard) string
	footer(w *wizard) []button
}

// wizard 是向导的顶层模型。
type wizard struct {
	cfgPath string
	cfg     *config.Config
	data    *wizardData

	steps []step
	idx   int

	width  int
	height int

	// doneHold 是完成页自动退出的等待时间（测试中置 0 立即退出）。
	doneHold time.Duration

	// 依赖注入点：部署任务调用，测试中可替换。
	detectIP       func(ctx context.Context) (string, error)
	newDNSProvider func(cred config.Credential) (ddns.Provider, error)
	issueCert      func(caddyfilePath, domain string, timeout time.Duration) error

	// certTimeout 是部署时等待证书签发的最长时间。
	certTimeout time.Duration

	quitting bool
}

// RunWizard 启动配置向导；cfgPath 是配置文件路径。
func RunWizard(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	w := newWizard(cfgPath, cfg, 5*time.Second)
	p := tea.NewProgram(w, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// newWizard 组装向导模型；已有配置会被回填到表单。
func newWizard(cfgPath string, cfg *config.Config, doneHold time.Duration) *wizard {
	data := &wizardData{}
	data.fromConfig(cfg)
	w := &wizard{
		cfgPath:        cfgPath,
		cfg:            cfg,
		data:           data,
		doneHold:       doneHold,
		detectIP:       ddns.DetectPublicIP,
		newDNSProvider: ddns.NewProvider,
		issueCert:      caddyembed.IssueCert,
		certTimeout:    3 * time.Minute,
	}
	w.steps = []step{
		&welcomeStep{},
		newOriginStep(),
		newPortStep(),
		newDNSCredStep(),
		newDomainStep(),
		&summaryStep{},
		newDeployStep(),
		&doneStep{},
	}
	return w
}

func (w *wizard) Init() tea.Cmd {
	return w.steps[w.idx].init(w)
}

func (w *wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width, w.height = msg.Width, msg.Height
		return w, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			w.quitting = true
			return w, tea.Quit
		}
	}
	cmd, nav := w.steps[w.idx].update(w, msg)
	switch nav {
	case navQuit:
		w.quitting = true
		return w, tea.Quit
	case navNext:
		if w.idx < len(w.steps)-1 {
			w.idx++
			return w, tea.Batch(cmd, w.steps[w.idx].init(w))
		}
	case navBack:
		if w.idx > 0 {
			w.idx--
			return w, tea.Batch(cmd, w.steps[w.idx].init(w))
		}
	}
	return w, cmd
}

func (w *wizard) View() string {
	width := w.width
	if width < 60 {
		width = 78
	}
	dialog := renderDialog(
		fmt.Sprintf("cddns 配置向导 · 步骤 %d/%d · %s", w.idx+1, len(w.steps), w.steps[w.idx].title()),
		w.steps[w.idx].view(w),
		renderFooter(w.steps[w.idx].footer(w)),
		width,
	)
	if w.height > 0 {
		dialog = lipgloss.Place(width, w.height, lipgloss.Center, lipgloss.Center, dialog)
	}
	return dialog
}

// portFree 检查端口是否空闲。
func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

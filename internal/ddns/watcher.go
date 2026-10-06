package ddns

import (
	"context"
	"log"
	"time"

	"cddns/internal/config"
)

// Watcher 周期探测公网 IP，变化时刷新所有站点域名的 A 记录。
type Watcher struct {
	Cred     config.Credential
	Domains  []string
	Interval time.Duration

	// 依赖注入点：测试可替换。
	Detect      func(ctx context.Context) (string, error)
	NewProvider func(cred config.Credential) (Provider, error)
	Logger      *log.Logger
}

// Run 阻塞运行直到 ctx 取消；探测/更新失败只记日志不中断（下次周期重试）。
func (w *Watcher) Run(ctx context.Context) error {
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	detect := w.Detect
	if detect == nil {
		detect = DetectPublicIP
	}
	newProvider := w.NewProvider
	if newProvider == nil {
		newProvider = NewProvider
	}
	logger := w.Logger
	if logger == nil {
		logger = log.Default()
	}
	var last string
	for {
		ip, err := detect(ctx)
		if err != nil {
			logger.Printf("[ddns] 探测公网 IP 失败: %v", err)
		} else if ip != last {
			if err := w.updateAll(ctx, newProvider, ip); err != nil {
				logger.Printf("[ddns] 更新 A 记录失败: %v", err)
			} else {
				last = ip
				logger.Printf("[ddns] 公网 IP 已刷新: %s", ip)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

func (w *Watcher) updateAll(ctx context.Context, newProvider func(config.Credential) (Provider, error), ip string) error {
	p, err := newProvider(w.Cred)
	if err != nil {
		return err
	}
	for _, d := range w.Domains {
		if err := UpsertA(ctx, p, d, ip); err != nil {
			return err
		}
	}
	return nil
}

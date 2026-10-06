// Package service 是 headless 服务：
// 生成代理配置、运行内嵌 Caddy、周期 DDNS 看门狗与配置热重载。
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"cddns/internal/caddyembed"
	"cddns/internal/config"
	"cddns/internal/ddns"
	"cddns/internal/site"
)

const watchInterval = 5 * time.Second

// Run 启动服务并阻塞直到 ctx 取消。
// 配置缺失/无效时不退出：保持运行等待向导写入配置（检测到后自动热重载）。
func Run(ctx context.Context, cfgPath string) error {
	s := &service{cfgPath: cfgPath}
	if err := s.apply(); err != nil {
		log.Printf("[service] 初始配置未就绪，等待向导配置: %v", err)
	}
	defer func() {
		s.stopWatcher()
		caddyembed.Stop()
	}()

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			mod, err := fileModTime(s.cfgPath)
			if errors.Is(err, os.ErrNotExist) {
				continue // 尚未配置，安静等待
			}
			if err != nil {
				log.Printf("[service] 检查配置失败: %v", err)
				continue
			}
			if !mod.Equal(s.lastMod) {
				s.lastMod = mod
				log.Printf("[service] 检测到配置变更，热重载中...")
				if err := s.apply(); err != nil {
					log.Printf("[service] 热重载失败（保留旧配置运行）: %v", err)
				}
			}
		}
	}
}

type service struct {
	cfgPath       string
	lastMod       time.Time
	watcherCancel context.CancelFunc
}

// apply 读取配置 → 生成 Caddyfile → 启动/热重载 Caddy → 重启 DDNS 看门狗。
func (s *service) apply() error {
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("配置无效: %w", err)
	}
	caddyfilePath := filepath.Join(filepath.Dir(s.cfgPath), "Caddyfile")
	if err := site.WriteCaddyfile(caddyfilePath, cfg); err != nil {
		return err
	}
	site.ApplyCredentialEnv(cfg)
	if err := caddyembed.Load(caddyfilePath); err != nil {
		return err
	}

	// 重启 DDNS 看门狗（凭证/域名可能已变化）
	s.stopWatcher()
	wctx, cancel := context.WithCancel(context.Background())
	s.watcherCancel = cancel
	go runWatcher(wctx, cfg)

	if mod, err := fileModTime(s.cfgPath); err == nil {
		s.lastMod = mod
	}
	return nil
}

func (s *service) stopWatcher() {
	if s.watcherCancel != nil {
		s.watcherCancel()
		s.watcherCancel = nil
	}
}

func runWatcher(ctx context.Context, cfg *config.Config) {
	domains := make([]string, 0, len(cfg.Sites))
	for _, st := range cfg.Sites {
		domains = append(domains, st.Domain)
	}
	w := &ddns.Watcher{
		Cred:     cfg.Credential,
		Domains:  domains,
		Interval: 5 * time.Minute,
	}
	if err := w.Run(ctx); err != nil {
		log.Printf("[ddns] 看门狗退出: %v", err)
	}
}

func fileModTime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return fi.ModTime(), nil
}

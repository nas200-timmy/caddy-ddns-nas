// Package caddyembed 在进程内运行 Caddy 引擎。
package caddyembed

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/caddyserver/caddy/v2"
	_ "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile" // 注册 caddyfile 适配器
	"github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard" // 注册标准 HTTP 指令（reverse_proxy 等）

	// DNS 模块：证书 DNS-01 验证。
	_ "github.com/caddy-dns/alidns"
	_ "github.com/caddy-dns/cloudflare"
	_ "github.com/caddy-dns/tencentcloud"
)

// Load 适配并加载 Caddyfile，启动或热重载内嵌 Caddy。
func Load(caddyfilePath string) error {
	cfgJSON, _, err := caddycmd.LoadConfig(caddyfilePath, "caddyfile")
	if err != nil {
		return fmt.Errorf("适配 Caddyfile %s: %w", caddyfilePath, err)
	}
	if err := caddy.Load(cfgJSON, false); err != nil {
		return fmt.Errorf("加载代理配置: %w", err)
	}
	return nil
}

// Run 加载 Caddyfile 后阻塞直到 ctx 取消，随后优雅停止 Caddy。
func Run(ctx context.Context, caddyfilePath string) error {
	if err := Load(caddyfilePath); err != nil {
		return err
	}
	<-ctx.Done()
	return caddy.Stop()
}

// Stop 停止内嵌 Caddy。
func Stop() error {
	return caddy.Stop()
}

// IssueCert 加载 Caddyfile 触发 DNS-01 证书签发（异步），
// 轮询 domain 的 TLS 握手直到证书就绪或超时；无论成败都停止内嵌 Caddy，
// 已签发的证书保存在 Caddy 本地存储中，后续启动直接复用。
func IssueCert(caddyfilePath, domain string, timeout time.Duration) error {
	if err := Load(caddyfilePath); err != nil {
		return err
	}
	defer Stop()

	deadline := time.Now().Add(timeout)
	for {
		if conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 3 * time.Second},
			"tcp",
			net.JoinHostPort("127.0.0.1", "443"),
			&tls.Config{ServerName: domain, InsecureSkipVerify: true},
		); err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待 %s 的证书签发超时（%v），请检查 DNS 凭证与域名解析", domain, timeout)
		}
		time.Sleep(2 * time.Second)
	}
}

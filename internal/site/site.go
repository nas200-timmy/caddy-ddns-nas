// Package site 负责站点配置：Caddyfile 生成与源站健康检查。
package site

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cddns/internal/config"
)

// 凭证环境变量名：Caddyfile 通过 {$VAR} 占位符引用，
// 由 ApplyCredentialEnv 在加载前写入进程环境，凭证不落入 Caddyfile 文件。
const (
	EnvAliyunID     = "CDDNS_ALIDNS_ACCESS_KEY_ID"
	EnvAliyunSecret = "CDDNS_ALIDNS_ACCESS_KEY_SECRET"
	EnvTencentID    = "CDDNS_TENCENTCLOUD_SECRET_ID"
	EnvTencentKey   = "CDDNS_TENCENTCLOUD_SECRET_KEY"
	EnvCloudflare   = "CDDNS_CLOUDFLARE_API_TOKEN"
)

// ApplyCredentialEnv 把凭证写入进程环境变量。
func ApplyCredentialEnv(cfg *config.Config) {
	switch cfg.Credential.Provider {
	case config.ProviderAliyun:
		os.Setenv(EnvAliyunID, cfg.Credential.SecretID)
		os.Setenv(EnvAliyunSecret, cfg.Credential.SecretKey)
	case config.ProviderTencentCloud:
		os.Setenv(EnvTencentID, cfg.Credential.SecretID)
		os.Setenv(EnvTencentKey, cfg.Credential.SecretKey)
	case config.ProviderCloudflare:
		os.Setenv(EnvCloudflare, cfg.Credential.Token)
	}
}

// Generate 根据配置生成 Caddyfile 内容。
// 全局 tls 块按服务商配置 DNS-01 验证，所有站点自动 HTTPS。
func Generate(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("\tadmin off\n")
	if cfg.ACMECA != "" {
		fmt.Fprintf(&b, "\tacme_ca %s\n", cfg.ACMECA)
	}
	b.WriteString("\tacme_dns ")
	switch cfg.Credential.Provider {
	case config.ProviderAliyun:
		b.WriteString("alidns {\n")
		b.WriteString("\t\taccess_key_id {$" + EnvAliyunID + "}\n")
		b.WriteString("\t\taccess_key_secret {$" + EnvAliyunSecret + "}\n")
		b.WriteString("\t}\n")
	case config.ProviderTencentCloud:
		b.WriteString("tencentcloud {\n")
		b.WriteString("\t\tsecret_id {$" + EnvTencentID + "}\n")
		b.WriteString("\t\tsecret_key {$" + EnvTencentKey + "}\n")
		b.WriteString("\t}\n")
	case config.ProviderCloudflare:
		b.WriteString("cloudflare {$" + EnvCloudflare + "}\n")
	}
	b.WriteString("}\n\n")
	for _, s := range cfg.Sites {
		fmt.Fprintf(&b, "%s {\n", s.Domain)
		fmt.Fprintf(&b, "\treverse_proxy %s\n", UpstreamAddr(s))
		if s.OriginScheme == "https" && s.OriginTLSInsecure {
			b.WriteString("\ttransport http {\n\t\ttls_insecure_skip_verify\n\t}\n")
		}
		b.WriteString("}\n\n")
	}
	// 末尾多出的空行会被 Caddy 判定为“未格式化”，去掉后与 caddy fmt 输出一致。
	return strings.TrimSuffix(b.String(), "\n")
}

// UpstreamAddr 计算反向代理上游地址（IPv6 字面量加方括号）。
func UpstreamAddr(s config.Site) string {
	host := s.Origin
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	scheme := "http"
	if s.OriginScheme == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, s.OriginPort)
}

// WriteCaddyfile 生成并写入 Caddyfile。
func WriteCaddyfile(path string, cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}
	if err := os.WriteFile(path, []byte(Generate(cfg)), 0o644); err != nil {
		return fmt.Errorf("写入 Caddyfile %s: %w", path, err)
	}
	return nil
}

// CheckOrigin 检测源站连通性：https 源站做 TLS 握手验证，http 源站做 TCP 检查。
func CheckOrigin(origin string, port int, scheme string) (bool, string) {
	if scheme == "https" {
		return checkTLS(origin, port)
	}
	return CheckTCP(origin, port)
}

// checkTLS 对源站做 TLS 握手（不校验证书链，只验证握手可达性）。
func checkTLS(origin string, port int) (bool, string) {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 3 * time.Second},
		"tcp",
		net.JoinHostPort(origin, strconv.Itoa(port)),
		&tls.Config{ServerName: origin, InsecureSkipVerify: true},
	)
	if err != nil {
		return false, err.Error()
	}
	conn.Close()
	return true, fmt.Sprintf("TLS 握手成功 %s:%d", origin, port)
}

// CheckTCP 检测源站 TCP 连通性，返回结果与说明。
func CheckTCP(origin string, port int) (bool, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(origin, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return false, err.Error()
	}
	conn.Close()
	return true, fmt.Sprintf("TCP %s:%d 连接成功", origin, port)
}

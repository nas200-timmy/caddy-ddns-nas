package site

import (
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"

	"cddns/internal/config"
)

// Caddy 启动时若 Caddyfile 与 caddy fmt 结果不一致会打印告警，
// 这里保证生成结果本身就是规范格式（含结尾不多个空行）。
func TestGenerateIsFormatted(t *testing.T) {
	cases := []*config.Config{
		{
			Credential: config.Credential{Provider: config.ProviderAliyun},
			Sites:      []config.Site{{Origin: "127.0.0.1", OriginPort: 8080, Domain: "nas.example.com"}},
		},
		{
			Credential: config.Credential{Provider: config.ProviderCloudflare},
			ACMECA:     "https://127.0.0.1:1/dir",
			Sites: []config.Site{
				{Origin: "nas200.top", OriginPort: 5667, OriginScheme: "https", OriginTLSInsecure: true, Domain: "a.example.com"},
				{Origin: "::1", OriginPort: 9000, Domain: "b.example.com"},
			},
		},
	}
	for _, cfg := range cases {
		raw := Generate(cfg)
		if got := string(caddyfile.Format([]byte(raw))); got != raw {
			t.Errorf("生成的 Caddyfile 不是规范格式:\n生成: %q\n规范: %q", raw, got)
		}
	}
}

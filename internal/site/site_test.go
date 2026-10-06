package site

import (
	"strings"
	"testing"

	"os"

	"cddns/internal/config"
)

func TestGenerateAliyun(t *testing.T) {
	cfg := &config.Config{
		Credential: config.Credential{Provider: config.ProviderAliyun},
		ACMECA:     "https://ca.example.com/dir",
		Sites: []config.Site{
			{Origin: "127.0.0.1", OriginPort: 8080, Domain: "nas.example.com"},
			{Origin: "::1", OriginPort: 9000, Domain: "v6.example.com"},
		},
	}
	out := Generate(cfg)
	for _, want := range []string{
		"admin off",
		"acme_ca https://ca.example.com/dir",
		"acme_dns alidns {",
		"access_key_id {$" + EnvAliyunID + "}",
		"access_key_secret {$" + EnvAliyunSecret + "}",
		"nas.example.com {",
		"reverse_proxy http://127.0.0.1:8080",
		"v6.example.com {",
		"reverse_proxy http://[::1]:9000",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Caddyfile 缺少 %q:\n%s", want, out)
		}
	}
}

func TestGenerateProviders(t *testing.T) {
	cases := []struct {
		provider config.DNSProvider
		want     []string
	}{
		{config.ProviderAliyun, []string{"acme_dns alidns {", "access_key_id", "access_key_secret"}},
		{config.ProviderTencentCloud, []string{"acme_dns tencentcloud {", "secret_id", "secret_key"}},
		{config.ProviderCloudflare, []string{"acme_dns cloudflare {$" + EnvCloudflare + "}"}},
	}
	for _, c := range cases {
		cfg := &config.Config{
			Credential: config.Credential{Provider: c.provider},
			Sites:      []config.Site{{Origin: "127.0.0.1", OriginPort: 80, Domain: "nas.example.com"}},
		}
		out := Generate(cfg)
		for _, want := range c.want {
			if !strings.Contains(out, want) {
				t.Errorf("%s: Caddyfile 缺少 %q:\n%s", c.provider, want, out)
			}
		}
	}
}

func TestApplyCredentialEnv(t *testing.T) {
	cfg := &config.Config{
		Credential: config.Credential{Provider: config.ProviderAliyun, SecretID: "ak", SecretKey: "sk"},
	}
	ApplyCredentialEnv(cfg)
	if got := os.Getenv(EnvAliyunID); got != "ak" {
		t.Fatalf("%s = %q, 期望 ak", EnvAliyunID, got)
	}
	if got := os.Getenv(EnvAliyunSecret); got != "sk" {
		t.Fatalf("%s = %q, 期望 sk", EnvAliyunSecret, got)
	}
}

func TestUpstreamAddr(t *testing.T) {
	cases := []struct {
		site config.Site
		want string
	}{
		{config.Site{Origin: "127.0.0.1", OriginPort: 8080}, "http://127.0.0.1:8080"},
		{config.Site{Origin: "nas200.top", OriginPort: 5667, OriginScheme: "https"}, "https://nas200.top:5667"},
		{config.Site{Origin: "::1", OriginPort: 9000, OriginScheme: "https"}, "https://[::1]:9000"},
	}
	for _, c := range cases {
		if got := UpstreamAddr(c.site); got != c.want {
			t.Errorf("UpstreamAddr(%+v) = %q, 期望 %q", c.site, got, c.want)
		}
	}
}

func TestGenerateHTTPSOrigin(t *testing.T) {
	cfg := &config.Config{
		Credential: config.Credential{Provider: config.ProviderAliyun},
		Sites: []config.Site{
			{Origin: "nas200.top", OriginPort: 5667, OriginScheme: "https", OriginTLSInsecure: true, Domain: "ipv4.nas200.top"},
		},
	}
	out := Generate(cfg)
	for _, want := range []string{
		"reverse_proxy https://nas200.top:5667",
		"transport http {",
		"tls_insecure_skip_verify",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Caddyfile 缺少 %q:\n%s", want, out)
		}
	}
}

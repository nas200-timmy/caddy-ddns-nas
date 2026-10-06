package tui

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/libdns/libdns"

	"cddns/internal/config"
	"cddns/internal/ddns"
)

var errZoneNotFound = errors.New("zone not found")

// fakeProvider 是向导全流程测试用的内存版 libdns provider。
type fakeProvider struct {
	records map[string][]libdns.Record
}

func (f *fakeProvider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	recs, ok := f.records[zone]
	if !ok {
		return nil, errZoneNotFound
	}
	return recs, nil
}

func (f *fakeProvider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	f.records[zone] = append(f.records[zone], recs...)
	return recs, nil
}

// SetRecords 按 (name, type) 语义替换整个 RRset。
func (f *fakeProvider) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	zoneRecs := f.records[zone]
	for _, want := range recs {
		wrr := want.RR()
		var kept []libdns.Record
		for _, r := range zoneRecs {
			rr := r.RR()
			if rr.Name == wrr.Name && rr.Type == wrr.Type {
				continue
			}
			kept = append(kept, r)
		}
		zoneRecs = append(kept, want)
	}
	f.records[zone] = zoneRecs
	return recs, nil
}

func (f *fakeProvider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return nil, nil
}

// TestWizardFullWalkthrough 用按键流完整走一遍向导，断言配置与 A 记录落盘。
func TestWizardFullWalkthrough(t *testing.T) {
	// 本地起一个真实监听端口作为源站，保证健康检查快速通过。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	fake := &fakeProvider{records: map[string][]libdns.Record{"example.com": {}}}

	input := strings.Join([]string{
		"\r",              // 欢迎 → 源站
		"127.0.0.1", "\r", // 源站 → 协议与端口
		"\r",                     // 协议列表（HTTP）→ 端口输入框
		strconv.Itoa(port), "\r", // 端口（健康检查通过）→ 凭证
		"\r",            // 服务商列表 → SecretId
		"test-ak", "\r", // SecretId → SecretKey
		"test-sk", "\r", // SecretKey → 域名
		"nas.example.com", "\r", // 域名 → 汇总
		"\r", // 部署（任务完成后自动进入完成页并退出）
	}, "")

	w := newWizard(cfgPath, &config.Config{}, 0) // doneHold=0：完成页立即退出
	w.detectIP = func(ctx context.Context) (string, error) { return "1.2.3.4", nil }
	w.newDNSProvider = func(cred config.Credential) (ddns.Provider, error) { return fake, nil }
	w.issueCert = func(caddyfilePath, domain string, timeout time.Duration) error { return nil }
	p := tea.NewProgram(w, tea.WithInput(strings.NewReader(input)), tea.WithoutRenderer())
	final, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	if fw := final.(*wizard); !fw.quitting {
		t.Fatal("向导没有正常退出")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sites) != 1 {
		t.Fatalf("期望 1 个站点，实际 %d", len(cfg.Sites))
	}
	s := cfg.Sites[0]
	if s.Origin != "127.0.0.1" || s.OriginPort != port || s.Domain != "nas.example.com" {
		t.Fatalf("站点配置不正确: %+v", s)
	}
	if cfg.Credential.Provider != config.ProviderAliyun {
		t.Fatalf("provider = %q, 期望 alidns", cfg.Credential.Provider)
	}
	if cfg.Credential.SecretID != "test-ak" || cfg.Credential.SecretKey != "test-sk" {
		t.Fatalf("凭证不正确: %+v", cfg.Credential)
	}

	// A 记录已通过部署任务写入
	recs := fake.records["example.com"]
	if len(recs) != 1 {
		t.Fatalf("A 记录数不正确: %+v", recs)
	}
	rr := recs[0].RR()
	if rr.Name != "nas" || rr.Type != "A" || rr.Data != "1.2.3.4" {
		t.Fatalf("A 记录不正确: %+v", rr)
	}
}

// TestWizardPrefill 验证重跑向导时已有配置回填。
func TestWizardPrefill(t *testing.T) {
	cfg := &config.Config{
		Credential: config.Credential{
			Provider: config.ProviderCloudflare,
			Token:    "my-cf-token",
		},
		Sites: []config.Site{{Origin: "10.0.0.8", OriginPort: 9000, OriginScheme: "https", Domain: "nas.example.com"}},
	}
	w := newWizard("/tmp/unused.yaml", cfg, 0)
	if w.data.origin != "10.0.0.8" || w.data.originPort != "9000" || w.data.originScheme != "https" {
		t.Fatalf("源站回填不正确: %+v", w.data)
	}
	if w.data.domain != "nas.example.com" || w.data.provider != config.ProviderCloudflare || w.data.token != "my-cf-token" {
		t.Fatalf("域名/凭证回填不正确: %+v", w.data)
	}
}

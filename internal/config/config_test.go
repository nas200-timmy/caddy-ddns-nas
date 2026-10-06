package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{
		Credential: Credential{Provider: ProviderAliyun, SecretID: "ak", SecretKey: "sk"},
		Sites:      []Site{{Origin: "127.0.0.1", OriginPort: 8080, Domain: "nas.example.com"}},
	}
}

func TestValidateOK(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
}

func TestValidateProviders(t *testing.T) {
	for _, p := range Providers {
		cfg := validConfig()
		cfg.Credential.Provider = p
		if err := cfg.Validate(); err != nil {
			t.Errorf("服务商 %s 应被接受: %v", p, err)
		}
	}

	cfg := validConfig()
	cfg.Credential.Provider = "aliyun" // 拼错的服务商
	err := cfg.Validate()
	if err == nil {
		t.Fatal("未知服务商应报错")
	}
	if !strings.Contains(err.Error(), "alidns") {
		t.Errorf("错误信息应列出可选项，实际: %v", err)
	}
}

func TestValidateScheme(t *testing.T) {
	for _, scheme := range []string{"", "http", "https"} {
		cfg := validConfig()
		cfg.Sites[0].OriginScheme = scheme
		if err := cfg.Validate(); err != nil {
			t.Errorf("协议 %q 应被接受: %v", scheme, err)
		}
	}
	cfg := validConfig()
	cfg.Sites[0].OriginScheme = "ftp"
	if err := cfg.Validate(); err == nil {
		t.Fatal("非法协议应报错")
	}
}

func TestDefaultPathEnvOverride(t *testing.T) {
	if got := DefaultPath(); got != defaultPath {
		t.Fatalf("默认路径 = %q, 期望 %q", got, defaultPath)
	}
	if got := DefaultDataDir(); got != defaultDataDir {
		t.Fatalf("默认数据目录 = %q, 期望 %q", got, defaultDataDir)
	}

	t.Setenv("CDDNS_CONFIG", "/tmp/custom.yaml")
	t.Setenv("CDDNS_DATA_DIR", "/tmp/custom-data")
	if got := DefaultPath(); got != "/tmp/custom.yaml" {
		t.Fatalf("CDDNS_CONFIG 未生效: %q", got)
	}
	if got := DefaultDataDir(); got != "/tmp/custom-data" {
		t.Fatalf("CDDNS_DATA_DIR 未生效: %q", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	want := validConfig()
	want.Sites[0].OriginScheme = "https"
	want.Sites[0].OriginTLSInsecure = true
	want.ACMECA = "https://ca.example.com/dir"
	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("配置文件权限 = %o, 期望 600（含凭证）", perm)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Credential != want.Credential || got.ACMECA != want.ACMECA {
		t.Fatalf("往返不一致: %+v", got)
	}
	if len(got.Sites) != 1 || got.Sites[0] != want.Sites[0] {
		t.Fatalf("站点往返不一致: %+v", got.Sites)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("文件不存在不应报错: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("空配置应校验失败")
	}
}

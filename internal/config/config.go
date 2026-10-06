// Package config 定义并持久化 cddns 的配置文件。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultPath    = "/etc/cddns/config.yaml"
	defaultDataDir = "/var/lib/cddns"
)

// DefaultPath 返回配置文件路径；CDDNS_CONFIG 可覆盖（容器内挂载、非 root 部署）。
func DefaultPath() string {
	if p := os.Getenv("CDDNS_CONFIG"); p != "" {
		return p
	}
	return defaultPath
}

// DefaultDataDir 返回数据目录（证书存储）；CDDNS_DATA_DIR 可覆盖。
func DefaultDataDir() string {
	if d := os.Getenv("CDDNS_DATA_DIR"); d != "" {
		return d
	}
	return defaultDataDir
}

// DNSProvider 表示受支持的 DNS 服务商。
type DNSProvider string

const (
	ProviderAliyun       DNSProvider = "alidns"     // 阿里云 DNS
	ProviderTencentCloud DNSProvider = "dnspod"     // 腾讯云 DNSPod
	ProviderCloudflare   DNSProvider = "cloudflare" // Cloudflare
)

// Providers 列出所有受支持的服务商。
var Providers = []DNSProvider{ProviderAliyun, ProviderTencentCloud, ProviderCloudflare}

// Valid 判断服务商是否受支持。
func (p DNSProvider) Valid() bool {
	for _, q := range Providers {
		if p == q {
			return true
		}
	}
	return false
}

// Credential 保存 DNS 控制凭证。
type Credential struct {
	Provider DNSProvider `yaml:"provider"`
	// 阿里云 / 腾讯云使用子账户密钥对。
	SecretID  string `yaml:"secret_id,omitempty"`
	SecretKey string `yaml:"secret_key,omitempty"`
	// Cloudflare 使用 API Token。
	Token string `yaml:"token,omitempty"`
}

// Site 描述一个"域名 → 源站"的转发站点。
type Site struct {
	Origin     string `yaml:"origin"`      // 源站 IP 或域名
	OriginPort int    `yaml:"origin_port"` // 源站端口
	Domain     string `yaml:"domain"`      // 对外域名
	// OriginScheme 源站协议：http / https（默认 http）。
	OriginScheme string `yaml:"origin_scheme,omitempty"`
	// OriginTLSInsecure 源站为 https 且证书不可信（如自签名）时跳过校验。
	OriginTLSInsecure bool `yaml:"origin_tls_insecure,omitempty"`
}

// Config 是 cddns 的完整配置。
type Config struct {
	Credential Credential `yaml:"credential"`
	Sites      []Site     `yaml:"sites"`
	// PublicIP 手动指定本机公网 IP（为空则自动探测）。
	PublicIP string `yaml:"public_ip,omitempty"`
	// ACMECA 自定义 ACME CA 目录地址（如 Let's Encrypt staging 或 Pebble），为空用默认 CA。
	ACMECA string `yaml:"acme_ca,omitempty"`
}

// Load 从 path 读取配置；文件不存在时返回零值配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	return &c, nil
}

// Save 将配置写入 path，自动创建父目录，文件权限 0600。
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("写入配置 %s: %w", path, err)
	}
	return nil
}

// Validate 检查配置是否完整。
func (c *Config) Validate() error {
	if c.Credential.Provider == "" {
		return errors.New("缺少 DNS 服务商凭证")
	}
	if !c.Credential.Provider.Valid() {
		return fmt.Errorf("不支持的 DNS 服务商 %q（可选：%s）", c.Credential.Provider, providerNames())
	}
	if len(c.Sites) == 0 {
		return errors.New("至少需要一个转发站点")
	}
	for i, s := range c.Sites {
		if s.Origin == "" || s.Domain == "" {
			return fmt.Errorf("站点 %d: 源站和域名不能为空", i+1)
		}
		if s.OriginPort < 1 || s.OriginPort > 65535 {
			return fmt.Errorf("站点 %d: 端口 %d 超出范围", i+1, s.OriginPort)
		}
		switch s.OriginScheme {
		case "", "http", "https":
		default:
			return fmt.Errorf("站点 %d: 源站协议 %q 无效（可选 http / https）", i+1, s.OriginScheme)
		}
	}
	return nil
}

// providerNames 返回受支持服务商的可读列表，用于错误提示。
func providerNames() string {
	names := make([]string, len(Providers))
	for i, p := range Providers {
		names[i] = string(p)
	}
	return strings.Join(names, " / ")
}

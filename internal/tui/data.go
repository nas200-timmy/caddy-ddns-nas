package tui

import (
	"strconv"
	"strings"

	"cddns/internal/config"
)

// wizardData 收集向导各步骤的输入，最终转为 config.Config。
type wizardData struct {
	origin       string
	originScheme string // http / https
	originPort   string
	provider     config.DNSProvider
	secretID     string
	secretKey    string
	token        string
	domain       string

	// 环境自检与健康检查结果（欢迎页与端口页填充）。
	port80Free  bool
	port443Free bool
	healthOK    bool
	healthNote  string

	// 部署阶段探测到的本机公网 IP。
	publicIP string
}

// fromConfig 用已有配置回填表单（重跑向导时生效）。
func (d *wizardData) fromConfig(c *config.Config) {
	if len(c.Sites) > 0 {
		s := c.Sites[0]
		d.origin = s.Origin
		d.originScheme = s.OriginScheme
		d.originPort = strconv.Itoa(s.OriginPort)
		d.domain = s.Domain
	}
	d.provider = c.Credential.Provider
	d.secretID = c.Credential.SecretID
	d.secretKey = c.Credential.SecretKey
	d.token = c.Credential.Token
	if d.provider == "" {
		d.provider = config.ProviderAliyun
	}
}

// toConfig 把表单数据转为配置。
func (d *wizardData) toConfig() *config.Config {
	port, _ := strconv.Atoi(d.originPort)
	scheme := d.originScheme
	if scheme == "" {
		scheme = "http"
	}
	return &config.Config{
		Credential: config.Credential{
			Provider:  d.provider,
			SecretID:  strings.TrimSpace(d.secretID),
			SecretKey: strings.TrimSpace(d.secretKey),
			Token:     strings.TrimSpace(d.token),
		},
		Sites: []config.Site{{
			Origin:       strings.TrimSpace(d.origin),
			OriginPort:   port,
			OriginScheme: scheme,
			Domain:       strings.ToLower(strings.TrimSpace(d.domain)),
		}},
	}
}

// Package ddns 负责公网 IP 探测、DNS zone 解析与 A 记录管理。
// 统一基于 libdns 接口实现：同一套 provider 库也服务于 Caddy 的 DNS-01 证书验证。
package ddns

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/libdns/alidns"
	"github.com/libdns/cloudflare"
	"github.com/libdns/libdns"
	"github.com/libdns/tencentcloud"
	"golang.org/x/net/publicsuffix"

	"cddns/internal/config"
)

// Provider 是 ddns 所需的 libdns 能力集合。
type Provider interface {
	libdns.RecordGetter
	libdns.RecordAppender
	libdns.RecordSetter
}

// NewProvider 根据凭证构建 libdns 服务商客户端。
func NewProvider(cred config.Credential) (Provider, error) {
	switch cred.Provider {
	case config.ProviderAliyun:
		return &alidns.Provider{
			CredentialInfo: alidns.CredentialInfo{
				AccessKeyID:     cred.SecretID,
				AccessKeySecret: cred.SecretKey,
			},
		}, nil
	case config.ProviderTencentCloud:
		return &tencentcloud.Provider{
			SecretId:  cred.SecretID,
			SecretKey: cred.SecretKey,
		}, nil
	case config.ProviderCloudflare:
		return &cloudflare.Provider{
			APIToken: cred.Token,
		}, nil
	default:
		return nil, fmt.Errorf("不支持的 DNS 服务商: %q", cred.Provider)
	}
}

// ResolveZone 确定 fqdn 所属的 DNS zone：
// 以 publicsuffix 得到的注册域为起点逐级探测，第一个能查询成功的即视为 zone。
// 探测同时起到校验凭证是否具备该域名权限的作用。
func ResolveZone(ctx context.Context, p Provider, fqdn string) (string, error) {
	eTLD1, err := publicsuffix.EffectiveTLDPlusOne(strings.TrimSuffix(fqdn, "."))
	if err != nil {
		return "", fmt.Errorf("解析域名 %s: %w", fqdn, err)
	}
	labels := strings.Split(strings.TrimSuffix(fqdn, "."), ".")
	min := len(strings.Split(eTLD1, "."))
	if len(labels) < min {
		return "", fmt.Errorf("域名 %s 不完整", fqdn)
	}
	for n := min; n <= len(labels); n++ {
		zone := strings.Join(labels[len(labels)-n:], ".")
		if _, err := p.GetRecords(ctx, zone); err == nil {
			return zone, nil
		}
	}
	return "", fmt.Errorf("无法访问 %s 的 DNS zone，请确认凭证具有该域名的权限", fqdn)
}

// UpsertA 将 fqdn 的 A 记录指向 ip：
// SetRecords 按 (name, type) 语义替换整个 RRset，创建与更新共用一条路径；
// 值已一致时跳过，避免无谓的 API 调用。
func UpsertA(ctx context.Context, p Provider, fqdn, ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return fmt.Errorf("无效的 IPv4 地址: %q", ip)
	}
	zone, err := ResolveZone(ctx, p, fqdn)
	if err != nil {
		return err
	}
	name := relativeName(fqdn, zone)

	recs, err := p.GetRecords(ctx, zone)
	if err != nil {
		return fmt.Errorf("查询 %s 的记录: %w", zone, err)
	}
	for _, r := range recs {
		rr := r.RR()
		if rr.Type == "A" && normalizeName(rr.Name) == normalizeName(name) && rr.Data == addr.String() {
			return nil // 已指向目标 IP，无需更新
		}
	}

	want := libdns.Address{Name: name, TTL: 600 * time.Second, IP: addr}
	if _, err := p.SetRecords(ctx, zone, []libdns.Record{want}); err != nil {
		return fmt.Errorf("设置 A 记录 %s → %s: %w", fqdn, ip, err)
	}
	return nil
}

// relativeName 计算记录在 zone 内的相对名；zone 顶点记为 "@"。
func relativeName(fqdn, zone string) string {
	name := strings.TrimSuffix(fqdn, ".")
	zone = strings.TrimSuffix(zone, ".")
	if name == zone {
		return "@"
	}
	if strings.HasSuffix(name, "."+zone) {
		return strings.TrimSuffix(name, "."+zone)
	}
	return name
}

// normalizeName 统一记录名比较格式：去尾部点，空名视同 "@"。
func normalizeName(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" {
		return "@"
	}
	return name
}

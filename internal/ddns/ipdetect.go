package ddns

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

// ipEchoServices 是用于探测公网 IPv4 的 echo 服务。
var ipEchoServices = []string{
	"https://4.icanhazip.com",
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://myip.ipip.net",
}

// DetectPublicIP 并发探测多个 echo 服务，返回多数一致的 IPv4。
func DetectPublicIP(ctx context.Context) (string, error) {
	return detectPublicIP(ctx, ipEchoServices)
}

// detectPublicIP 并发请求 endpoints，取出现次数最多的合法 IPv4。
func detectPublicIP(ctx context.Context, endpoints []string) (string, error) {
	results := make(chan string, len(endpoints))
	for _, u := range endpoints {
		go func(u string) {
			results <- queryEcho(ctx, u)
		}(u)
	}
	counts := map[string]int{}
	for range endpoints {
		select {
		case ip := <-results:
			if ip != "" {
				counts[ip]++
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	best, bestN := "", 0
	for ip, n := range counts {
		if n > bestN {
			best, bestN = ip, n
		}
	}
	if best == "" {
		return "", errors.New("无法探测公网 IPv4（所有探测源均失败），请检查网络或稍后重试")
	}
	return best, nil
}

var ipv4RE = regexp.MustCompile(`\b(\d{1,3}\.){3}\d{1,3}\b`)

// queryEcho 请求单个探测源并提取 IPv4；失败返回空串。
func queryEcho(ctx context.Context, url string) string {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return ""
	}
	s := ipv4RE.FindString(string(body))
	if net.ParseIP(s) == nil {
		return ""
	}
	return s
}

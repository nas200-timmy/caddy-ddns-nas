package caddyembed

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cddns/internal/site"
)

// TestProxyIntegration 验证内嵌 Caddy 能真实代理 HTTP 流量。
func TestProxyIntegration(t *testing.T) {
	// 源站
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello-from-origin")
	}))
	defer origin.Close()
	originPort := origin.Listener.Addr().(*net.TCPAddr).Port

	proxyPort := freePort(t)
	dir := t.TempDir()
	caddyfile := filepath.Join(dir, "Caddyfile")
	content := fmt.Sprintf("{\n\tadmin off\n}\nhttp://127.0.0.1:%d {\n\treverse_proxy 127.0.0.1:%d\n}\n", proxyPort, originPort)
	if err := os.WriteFile(caddyfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Load(caddyfile); err != nil {
		t.Fatalf("启动内嵌 Caddy 失败: %v", err)
	}
	t.Cleanup(func() { Stop() })

	// 轮询直到代理可用（Caddy 启动是异步的）
	var got string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			got = string(b)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got != "hello-from-origin" {
		t.Fatalf("代理响应 = %q, 期望 hello-from-origin", got)
	}
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// canBind443 探测当前环境能否监听 443：Caddy 启动站点需要它。
// 非特权环境（例如 CI 里的普通用户）会因权限不足绑不上。
func canBind443() bool {
	ln, err := net.Listen("tcp", ":443")
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// TestIssueCertConfigValid 验证 DNS-01 配置能被 Caddy 解析并启动，
// 假凭证下签发必然失败，IssueCert 应在超时后报错（而非配置错误）。
// 该路径需要真实监听 443，无法绑定的环境跳过而不是报失败。
func TestIssueCertConfigValid(t *testing.T) {
	if !canBind443() {
		t.Skip("无法绑定 :443（需 root 或 net.ipv4.ip_unprivileged_port_start=0），跳过证书签发路径验证")
	}

	dir := t.TempDir()
	caddyfile := filepath.Join(dir, "Caddyfile")
	t.Setenv(site.EnvAliyunID, "fake-id")
	t.Setenv(site.EnvAliyunSecret, "fake-secret")

	content := fmt.Sprintf(`{
	admin off
	acme_ca https://127.0.0.1:1/dir
	acme_dns alidns {
		access_key_id {$%s}
		access_key_secret {$%s}
	}
}

nas.example.com {
	reverse_proxy 127.0.0.1:1
}
`, site.EnvAliyunID, site.EnvAliyunSecret)
	if err := os.WriteFile(caddyfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := IssueCert(caddyfile, "nas.example.com", 6*time.Second)
	if err == nil {
		t.Fatal("假凭证下不应签发成功")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("期望超时错误，实际: %v", err)
	}
}

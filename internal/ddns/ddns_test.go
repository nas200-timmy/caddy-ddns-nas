package ddns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/libdns/libdns"

	"cddns/internal/config"
)

var errZoneNotFound = errors.New("zone not found")

// fakeProvider 是内存版 libdns provider，用于单元测试。
type fakeProvider struct {
	mu      sync.Mutex
	records map[string][]libdns.Record
}

func newFakeProvider(zones ...string) *fakeProvider {
	f := &fakeProvider{records: map[string][]libdns.Record{}}
	for _, z := range zones {
		f.records[z] = nil
	}
	return f
}

func (f *fakeProvider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	recs, ok := f.records[zone]
	if !ok {
		return nil, errZoneNotFound
	}
	return append([]libdns.Record(nil), recs...), nil
}

func (f *fakeProvider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[zone] = append(f.records[zone], recs...)
	return recs, nil
}

// SetRecords 按 (name, type) 语义替换整个 RRset。
func (f *fakeProvider) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

func TestResolveZone(t *testing.T) {
	p := newFakeProvider("example.com")
	zone, err := ResolveZone(context.Background(), p, "nas.example.com")
	if err != nil || zone != "example.com" {
		t.Fatalf("zone=%q err=%v, 期望 example.com", zone, err)
	}
	if _, err := ResolveZone(context.Background(), p, "other.org"); err == nil {
		t.Fatal("域名不在任何 zone 中时应报错")
	}
}

func TestUpsertA(t *testing.T) {
	ctx := context.Background()
	p := newFakeProvider("example.com")

	if err := UpsertA(ctx, p, "nas.example.com", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	recs := p.records["example.com"]
	if len(recs) != 1 {
		t.Fatalf("记录数不正确: %+v", recs)
	}
	rr := recs[0].RR()
	if rr.Name != "nas" || rr.Type != "A" || rr.Data != "1.2.3.4" {
		t.Fatalf("记录不正确: %+v", rr)
	}

	// 值相同 → 跳过，不新增
	if err := UpsertA(ctx, p, "nas.example.com", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if len(p.records["example.com"]) != 1 {
		t.Fatalf("值相同不应新增记录: %+v", p.records["example.com"])
	}

	// 值变化 → 替换原记录
	if err := UpsertA(ctx, p, "nas.example.com", "5.6.7.8"); err != nil {
		t.Fatal(err)
	}
	recs = p.records["example.com"]
	if len(recs) != 1 || recs[0].RR().Data != "5.6.7.8" {
		t.Fatalf("更新失败: %+v", recs)
	}

	// 非法 IP → 报错
	if err := UpsertA(ctx, p, "nas.example.com", "not-an-ip"); err == nil {
		t.Fatal("非法 IP 应报错")
	}
}

func TestDetectPublicIP(t *testing.T) {
	ip := "203.0.113.7"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ip + "\n"))
	}))
	defer ts.Close()

	got, err := detectPublicIP(context.Background(), []string{ts.URL, ts.URL, "http://127.0.0.1:1"})
	if err != nil || got != ip {
		t.Fatalf("got=%q err=%v, 期望 %q", got, err, ip)
	}

	if _, err := detectPublicIP(context.Background(), []string{"http://127.0.0.1:1"}); err == nil {
		t.Fatal("全部失败时应报错")
	}
}

func TestWatcher(t *testing.T) {
	fake := newFakeProvider("example.com")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seq := []string{"1.2.3.4", "5.6.7.8"}
	i := 0
	w := &Watcher{
		Cred:     config.Credential{Provider: config.ProviderAliyun},
		Domains:  []string{"nas.example.com"},
		Interval: 50 * time.Millisecond,
		Detect: func(ctx context.Context) (string, error) {
			if i < len(seq) {
				ip := seq[i]
				i++
				return ip, nil
			}
			return seq[len(seq)-1], nil
		},
		NewProvider: func(cred config.Credential) (Provider, error) { return fake, nil },
	}

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	recs := fake.records["example.com"]
	if len(recs) != 1 || recs[0].RR().Data != "5.6.7.8" {
		t.Fatalf("看门狗未更新记录: %+v", recs)
	}
}

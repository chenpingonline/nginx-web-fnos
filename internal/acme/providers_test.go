package acme

import (
	"context"
	"encoding/json"
	dnswire "github.com/miekg/dns"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-acme/lego/v5/challenge/dns01"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "acme-worker" {
		// Only this test executable bypasses DNS propagation; the production binary
		// always uses RunWorker without overrides. The real provider still runs.
		conn, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			os.Exit(2)
		}
		server := &dnswire.Server{PacketConn: conn, Handler: dnswire.HandlerFunc(func(w dnswire.ResponseWriter, r *dnswire.Msg) {
			answer := new(dnswire.Msg)
			answer.SetReply(r)
			if len(r.Question) > 0 && r.Question[0].Qtype == dnswire.TypeSOA {
				rr, _ := dnswire.NewRR("example.com. 60 IN SOA ns.example.com. hostmaster.example.com. 1 60 60 60 60")
				answer.Answer = []dnswire.RR{rr}
			}
			_ = w.WriteMsg(answer)
		})}
		go server.ActivateAndServe()
		options := dns01.NewOptions()
		options.RecursiveNameservers = []string{conn.LocalAddr().String()}
		dns01.SetDefaultClient(dns01.NewClient(options))
		os.Exit(runWorker([]dns01.ChallengeOption{dns01.PropagationWait(0, true)}))
	}
	os.Exit(m.Run())
}

func TestNativeCatalogAndValidation(t *testing.T) {
	if len(catalog.Providers) != 39 {
		t.Fatal("unexpected scope", len(catalog.Providers))
	}
	groups := map[string]int{}
	for _, p := range catalog.Providers {
		groups[p.Group]++
		seen := map[string]bool{}
		for _, f := range p.Fields {
			if seen[f.Key] {
				t.Fatal("duplicate field", p.Code, f.Key)
			}
			seen[f.Key] = true
		}
	}
	if groups["国内"] != 16 || groups["国际"] != 23 {
		t.Fatal(groups)
	}
	in := validInput()
	in.Provider = "dnsla"
	in.DNSConfig = map[string]string{"PATH": "injected"}
	if validate(&in) == nil {
		t.Fatal("accepted arbitrary environment")
	}
	in.Provider = "exec"
	in.DNSConfig = nil
	if validate(&in) == nil {
		t.Fatal("accepted external script provider")
	}
	in.Provider = "cloudflare"
	in.DNSConfig = map[string]string{"CF_DNS_API_TOKEN": "test-secret"}
	if err := validate(&in); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerEnvironmentIsolation(t *testing.T) {
	t.Setenv("ALICLOUD_ACCESS_KEY", "unrelated-account")
	t.Setenv("CF_DNS_API_TOKEN_FILE", "/old/account")
	t.Setenv("CF_API_KEY", "inherited-key")
	in := validInput()
	in.DNSConfig = map[string]string{"CF_DNS_API_TOKEN": "selected-account"}
	in.PropagationSeconds = 180
	env := strings.Join(workerEnvironment(in), "\n")
	for _, forbidden := range []string{"unrelated-account", "/old/account", "inherited-key"} {
		if strings.Contains(env, forbidden) {
			t.Fatal("inherited credential leaked")
		}
	}
	if !strings.Contains(env, "CF_DNS_API_TOKEN=selected-account") || !strings.Contains(env, "CLOUDFLARE_PROPAGATION_TIMEOUT=180") {
		t.Fatal("missing selected configuration")
	}
	if os.Getenv("CF_API_KEY") != "inherited-key" {
		t.Fatal("parent environment was changed")
	}
}

func TestNativeWorkerWithPebble(t *testing.T) {
	endpoint := os.Getenv("ACME_TEST_DIRECTORY")
	if endpoint == "" {
		t.Skip("local Pebble integration")
	}
	if !strings.HasPrefix(endpoint, "https://localhost:") && !strings.HasPrefix(endpoint, "https://127.0.0.1:") {
		t.Fatal("loopback only")
	}
	var creates, removes atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer worker-test-token" {
			t.Error("wrong isolated token")
			w.WriteHeader(401)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []map[string]string{{"id": "test-zone", "name": "example.com"}}})
		case http.MethodPost:
			creates.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]string{"id": "test-record"}})
		case http.MethodDelete:
			removes.Add(1)
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			w.WriteHeader(405)
		}
	}))
	defer api.Close()
	in := validInput()
	in.CA = "custom"
	in.DirectoryURL = endpoint
	in.RotateKey = true
	in.DNSConfig = map[string]string{"CF_DNS_API_TOKEN": "worker-test-token", "CLOUDFLARE_BASE_URL": api.URL}
	if err := validate(&in); err != nil {
		t.Fatal(err)
	}
	r := &Record{Input: in, Job: Job{Message: "parent-stage"}}
	saves := 0
	run := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := issue(ctx, r, func() error { saves++; return nil }, func(s string) { r.Job.Message = s }); err != nil {
			t.Fatal(err)
		}
	}
	run()
	if saves < 2 || r.Registration == nil || !r.Pending || len(r.Resource.PrivateKey) == 0 {
		t.Fatal("missing worker checkpoints")
	}
	first, _ := leaf(r.Resource)
	account := r.Registration.Location
	r.Pending = false
	r.Force = true
	run()
	second, _ := leaf(r.Resource)
	if first.SerialNumber.Cmp(second.SerialNumber) == 0 || r.Registration.Location != account {
		t.Fatal("worker renewal failed")
	}
	if creates.Load() == 0 || creates.Load() != removes.Load() {
		t.Fatal("provider cleanup mismatch", creates.Load(), removes.Load())
	}
}

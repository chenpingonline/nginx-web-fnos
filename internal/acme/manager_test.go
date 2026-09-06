package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-acme/lego/v5/certificate"
)

func validInput() Input {
	return Input{Name: "test", CA: "staging", Email: "admin@example.com", Domains: []string{"example.com", "*.example.com"}, Provider: "cloudflare", KeyType: "ec256", AcceptTerms: true, Credentials: Credentials{Token: "secret-token-test"}}
}
func TestRemoveForCertificate(t *testing.T) {
	m, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	unrelated, err := m.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	r := clone(m.records[second.ID])
	r.Job.CertificateID = first.ID
	if err := m.save(r); err != nil {
		t.Fatal(err)
	}
	m.active = second.ID
	if err := m.RemoveForCertificate(first.ID); err == nil {
		t.Fatal("removed active renewal")
	}
	if len(m.List()) != 3 {
		t.Fatal("active check partially removed jobs")
	}
	m.active = ""
	if err := m.RemoveForCertificate(first.ID); err != nil {
		t.Fatal(err)
	}
	jobs := m.List()
	if len(jobs) != 1 || jobs[0].ID != unrelated.ID {
		t.Fatal("incorrect jobs removed", jobs)
	}
	for _, id := range []string{first.ID, second.ID} {
		if _, err := os.Stat(filepath.Join(m.dir, id+".json")); !os.IsNotExist(err) {
			t.Fatal("credentials remain on disk", err)
		}
	}
	reloaded, err := New(m.dir, nil)
	if err != nil || len(reloaded.List()) != 1 {
		t.Fatal("deletion did not persist", err)
	}
}
func testResource(t *testing.T, now time.Time) *certificate.Resource {
	t.Helper()
	k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	c := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.com"}, DNSNames: []string{"example.com", "*.example.com"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(90 * 24 * time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, c, c, &k.PublicKey, k)
	if e != nil {
		t.Fatal(e)
	}
	pk, e := x509.MarshalPKCS8PrivateKey(k)
	if e != nil {
		t.Fatal(e)
	}
	return &certificate.Resource{Domains: []string{"example.com"}, Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk})}
}
func TestDurableIssueRetryDeployAndSecretIsolation(t *testing.T) {
	now := time.Now().UTC()
	dir := t.TempDir()
	issues := 0
	deployments := 0
	fail := true
	var target string
	deploy := func(id, name string, r *certificate.Resource) (string, error) {
		deployments++
		target = id
		if fail {
			return "", errors.New("failure")
		}
		return id, nil
	}
	m, e := New(dir, deploy)
	if e != nil {
		t.Fatal(e)
	}
	m.now = func() time.Time { return now }
	m.issue = func(ctx context.Context, r *Record, save func() error, stage func(string)) error {
		issues++
		r.AccountKey = []byte("secret-account-key")
		r.Resource = testResource(t, now)
		r.Pending = true
		return save()
	}
	job, e := m.Create(validInput())
	if e != nil {
		t.Fatal(e)
	}
	m.step(context.Background())
	if issues != 1 || deployments != 1 || target != job.ID {
		t.Fatal("issue or stable target failure")
	}
	failed := m.List()[0]
	if failed.Status != "failed" || !failed.NextAttempt.After(now) {
		t.Fatalf("missing backoff: %+v", failed)
	}
	raw, _ := json.Marshal(m.List())
	for _, secret := range []string{"secret-token-test", "secret-account-key", "private_key", "credentials"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("secret exposed", secret)
		}
	}
	info, e := os.Stat(filepath.Join(dir, job.ID+".json"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions", e)
	}
	// A restart must retry the already issued certificate, not create another ACME order.
	m, e = New(dir, deploy)
	if e != nil {
		t.Fatal(e)
	}
	now = now.Add(10 * time.Minute)
	m.now = func() time.Time { return now }
	m.issue = func(context.Context, *Record, func() error, func(string)) error {
		t.Fatal("reissued pending certificate")
		return nil
	}
	fail = false
	m.step(context.Background())
	ready := m.List()[0]
	if ready.Status != "ready" || ready.CertificateID != job.ID || deployments != 2 {
		t.Fatalf("deployment not recovered: %+v", ready)
	}
	if !m.Manages(job.ID) {
		t.Fatal("missing managed certificate guard")
	}
	if e = m.Action(job.ID, "pause"); e != nil {
		t.Fatal(e)
	}
	now = now.Add(100 * 24 * time.Hour)
	m.step(context.Background())
	if deployments != 2 {
		t.Fatal("paused task executed")
	}
	if e = m.Action(job.ID, "delete"); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(dir, job.ID+".json")); !os.IsNotExist(e) {
		t.Fatal("credentials not removed")
	}
}
func TestValidation(t *testing.T) {
	cases := []func(*Input){func(i *Input) { i.Domains = []string{"127.0.0.1"} }, func(i *Input) { i.Domains = []string{"foo.*.com"} }, func(i *Input) { i.AcceptTerms = false }, func(i *Input) { i.CA = "zerossl" }, func(i *Input) { i.CA = "custom"; i.DirectoryURL = "http://example.com/acme" }, func(i *Input) { i.Email = "" }, func(i *Input) { i.Provider = "shell" }, func(i *Input) { i.PropagationSeconds = 1801 }, func(i *Input) { i.Credentials.EABKID = "only-kid" }}
	for n, change := range cases {
		in := validInput()
		change(&in)
		if validate(&in) == nil {
			t.Fatalf("accepted invalid case %d", n)
		}
	}
	in := validInput()
	in.Domains = []string{"EXAMPLE.COM.", "example.com", "*.example.com"}
	if e := validate(&in); e != nil || len(in.Domains) != 2 {
		t.Fatal("normalization failed", e)
	}
	for _, p := range []string{"cloudflare", "alidns", "tencentcloud"} {
		in := validInput()
		in.Provider = p
		in.Credentials.AccessID = "id"
		in.Credentials.Secret = "secret"
		if e := validate(&in); e != nil {
			t.Fatal(e)
		}
		if _, e := dnsProvider(in); e != nil {
			t.Fatal("provider initialization", p, e)
		}
	}
}
func TestJobSingleFlightAndInterruptedRecovery(t *testing.T) {
	dir := t.TempDir()
	m, e := New(dir, func(id, name string, r *certificate.Resource) (string, error) { return id, nil })
	if e != nil {
		t.Fatal(e)
	}
	job, e := m.Create(validInput())
	if e != nil {
		t.Fatal(e)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	m.issue = func(context.Context, *Record, func() error, func(string)) error {
		close(started)
		<-release
		return errors.New("credential secret-token-test error")
	}
	go func() { m.step(context.Background()); close(done) }()
	<-started
	if e = m.Action(job.ID, "delete"); e == nil {
		t.Fatal("removed running job")
	}
	m.step(context.Background())
	restarted, e := New(dir, m.deploy)
	if e != nil {
		t.Fatal(e)
	}
	if restarted.List()[0].Status != "queued" {
		t.Fatal("interrupted job not requeued")
	}
	close(release)
	<-done
	if strings.Contains(m.List()[0].Message, "secret-token-test") {
		t.Fatal("credential leaked")
	}
}

func TestReissueDomainsPreservesCredentialsAndCertificate(t *testing.T) {
	m, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	j, err := m.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	r := clone(m.records[j.ID])
	r.Resource = testResource(t, time.Now())
	r.Pending = true
	r.Job.CertificateID = "existing-cert"
	r.AccountKey = []byte("saved-account")
	r.Job.Enabled = false
	if err = m.save(r); err != nil {
		t.Fatal(err)
	}
	job, err := m.Reissue(j.ID, []string{"NAscp.cn", "*.nascp.cn", "nascp.cn"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(job.Domains, ",") != "nascp.cn,*.nascp.cn" || job.CertificateID != "existing-cert" || !job.Enabled || job.Status != "queued" {
		t.Fatal(job)
	}
	reloaded, err := New(m.dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved := reloaded.records[j.ID]
	if saved.Resource != nil || saved.Pending || !saved.Force || saved.RenewAt != nil {
		t.Fatal("old order could be renewed/deployed")
	}
	if saved.Input.Credentials.Token != "secret-token-test" || string(saved.AccountKey) != "saved-account" {
		t.Fatal("lost private configuration")
	}
	public, _ := json.Marshal(job)
	if strings.Contains(string(public), "secret-token-test") {
		t.Fatal("credential exposed")
	}
	deployments := 0
	m.issue = func(_ context.Context, r *Record, _ func() error, _ func(string)) error {
		return errors.New("simulated failure")
	}
	m.deploy = func(id, name string, resource *certificate.Resource) (string, error) { deployments++; return id, nil }
	m.step(context.Background())
	if deployments != 0 || m.List()[0].CertificateID != "existing-cert" {
		t.Fatal("failed order affected deployed certificate")
	}
	m.issue = func(_ context.Context, r *Record, _ func() error, _ func(string)) error {
		r.Resource = testResource(t, time.Now())
		r.Pending = true
		return nil
	}
	if _, err = m.Reissue(j.ID, job.Domains); err != nil {
		t.Fatal(err)
	}
	m.step(context.Background())
	if deployments != 1 || m.List()[0].CertificateID != "existing-cert" {
		t.Fatal("replacement changed reference")
	}
}
func TestReissueRejectsInvalidOrActiveWithoutMutation(t *testing.T) {
	m, _ := New(t.TempDir(), nil)
	j, _ := m.Create(validInput())
	before, _ := json.Marshal(m.records[j.ID])
	if _, err := m.Reissue(j.ID, []string{"https://example.com"}); err == nil {
		t.Fatal("invalid domain accepted")
	}
	after, _ := json.Marshal(m.records[j.ID])
	if string(before) != string(after) {
		t.Fatal("invalid edit persisted")
	}
	m.active = j.ID
	if _, err := m.Reissue(j.ID, []string{"new.example.com"}); err == nil {
		t.Fatal("active edit accepted")
	}
	if _, err := m.Reissue("missing", []string{"example.com"}); err == nil {
		t.Fatal("missing job accepted")
	}
}

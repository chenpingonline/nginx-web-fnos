package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/go-acme/lego/v5/certificate"
)

func acmeResource(t *testing.T, serial int64) *certificate.Resource {
	t.Helper()
	k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	c := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour * 24)}
	der, e := x509.CreateCertificate(rand.Reader, c, c, &k.PublicKey, k)
	if e != nil {
		t.Fatal(e)
	}
	key, e := x509.MarshalPKCS8PrivateKey(k)
	if e != nil {
		t.Fatal(e)
	}
	return &certificate.Resource{Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})}
}
func TestACMEDeploymentPreservesIDAndDraft(t *testing.T) {
	s := testService(t)
	id := domain.RandomID()
	first := acmeResource(t, 1)
	if _, e := s.deployACME(id, "test", first); e != nil {
		t.Fatal(e)
	}
	state := s.State()
	state.Dirty = true
	state.DraftRevisionID = "draft-marker"
	if e := s.store.Update(func(next *State) error {
		next.Dirty = state.Dirty
		next.DraftRevisionID = state.DraftRevisionID
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	old, e := os.Readlink(filepath.Join(s.paths.CertificateDir, id))
	if e != nil {
		t.Fatal(e)
	}
	second := acmeResource(t, 2)
	if _, e = s.deployACME(id, "test", second); e != nil {
		t.Fatal(e)
	}
	got := s.State()
	if len(got.Certificates) != 1 || got.Certificates[0].ID != id || got.Certificates[0].SerialNumber != "2" || !got.Dirty || got.DraftRevisionID != "draft-marker" {
		t.Fatal("renewal changed ID or draft", got)
	}
	current, e := os.Readlink(filepath.Join(s.paths.CertificateDir, id))
	if e != nil || old == current {
		t.Fatal("version not switched", e)
	}
	bad := acmeResource(t, 3)
	bad.PrivateKey = first.PrivateKey
	if _, e = s.deployACME(id, "test", bad); e == nil {
		t.Fatal("accepted mismatched key")
	}
	final, _ := os.Readlink(filepath.Join(s.paths.CertificateDir, id))
	if final != current {
		t.Fatal("failed certificate replaced valid version")
	}
	if e = s.DeleteCertificate(id); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.paths.CertificateDir, ".acme-versions", id)); !os.IsNotExist(e) {
		t.Fatal("private versions not removed")
	}
}

func TestACMEDeploymentWithRealNginx(t *testing.T) {
	binary := os.Getenv("NGINX_TEST_BIN")
	if binary == "" {
		t.Skip("set NGINX_TEST_BIN for live TLS renewal and rollback")
	}
	s := testService(t)
	if e := os.MkdirAll(filepath.Dir(s.paths.NginxBin), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(binary, s.paths.NginxBin); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Dir(s.paths.MimeTypes), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(s.paths.MimeTypes, []byte("types { text/html html; }"), 0644); e != nil {
		t.Fatal(e)
	}
	id := domain.RandomID()
	first := acmeResource(t, 10)
	if _, e := s.deployACME(id, "live", first); e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	_, e = s.CreateRule(ProxyRule{Name: "live", Enabled: true, ListenPort: port, Domains: []string{"example.com"}, TLS: true, CertificateID: id, UpstreamScheme: "http", UpstreamHost: "127.0.0.1", UpstreamPort: 18080})
	if e != nil {
		t.Fatal(e)
	}
	// Use a minimal installed configuration: this test targets TLS deployment, not optional renderer modules.
	active := fmt.Sprintf("pid %s; error_log %s; events {} http { server { listen 127.0.0.1:%d ssl; ssl_certificate %s; ssl_certificate_key %s; location / { return 200 'ok'; } } }", s.paths.NginxPID, s.paths.NginxErrorLog, port, filepath.Join(s.paths.CertificateDir, id, "fullchain.pem"), filepath.Join(s.paths.CertificateDir, id, "privkey.pem"))
	if e = os.WriteFile(s.paths.NginxMaster, []byte(active), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.NginxStart(); e != nil {
		t.Fatal(e)
	}
	defer s.NginxStop()
	serial := func() string {
		t.Helper()
		c, e := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", fmt.Sprintf("127.0.0.1:%d", port), &tls.Config{InsecureSkipVerify: true, ServerName: "example.com"})
		if e != nil {
			t.Fatal(e)
		}
		defer c.Close()
		return c.ConnectionState().PeerCertificates[0].SerialNumber.String()
	}
	if serial() != "10" {
		t.Fatal("initial certificate not served")
	}
	// Deliberately invalid draft must not be installed during a certificate renewal.
	if e = s.store.Update(func(state *State) error {
		state.Dirty = true
		state.Settings.Logging.CustomFormat = "$unknown_draft_only"
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	second := acmeResource(t, 11)
	if _, e = s.deployACME(id, "live", second); e != nil {
		t.Fatal(e)
	}
	if serial() != "11" {
		t.Fatal("renewed certificate not served")
	}
	third := acmeResource(t, 12)
	if e = s.nginx.DeployCertificate(id, third.Certificate, third.PrivateKey, func() error { return errors.New("simulated state save failure") }); e == nil {
		t.Fatal("expected deployment rollback")
	}
	if serial() != "11" {
		t.Fatal("rollback did not restore previous certificate")
	}
	if !s.State().Dirty || s.State().Settings.Logging.CustomFormat != "$unknown_draft_only" {
		t.Fatal("draft changed")
	}
}

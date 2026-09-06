package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	acmemanager "github.com/chenpingonline/nginx-web-fnos/internal/acme"
	"github.com/go-acme/lego/v5/certificate"
)

func (s *AppService) ACME() *acmemanager.Manager       { return s.acme }
func (s *AppService) MaintainACME(ctx context.Context) { s.acme.Run(ctx) }
func (s *AppService) deployACME(id, name string, r *certificate.Resource) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r == nil {
		return "", errors.New("没有可部署的证书")
	}
	pair, err := tls.X509KeyPair(r.Certificate, r.PrivateKey)
	if err != nil {
		return "", err
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return "", err
	}
	if !cert.NotAfter.After(time.Now()) || cert.NotBefore.After(time.Now().Add(5*time.Minute)) {
		return "", errors.New("证书不在有效期内")
	}
	sum := sha256.Sum256(cert.Raw)
	h := strings.ToUpper(hex.EncodeToString(sum[:]))
	parts := []string{}
	for i := 0; i < len(h); i += 2 {
		parts = append(parts, h[i:i+2])
	}
	meta := CertificateMeta{ID: id, Name: name, Subject: cert.Subject.String(), DNSNames: cert.DNSNames, SerialNumber: cert.SerialNumber.String(), NotBefore: cert.NotBefore, NotAfter: cert.NotAfter, Fingerprint: strings.Join(parts, ":"), CreatedAt: time.Now().UTC()}
	for _, ip := range cert.IPAddresses {
		meta.IPAddresses = append(meta.IPAddresses, ip.String())
	}
	err = s.nginx.DeployCertificate(id, r.Certificate, r.PrivateKey, func() error {
		return s.store.Update(func(state *State) error {
			for i, old := range state.Certificates {
				if old.ID == id {
					meta.CreatedAt = old.CreatedAt
					state.Certificates[i] = meta
					return nil
				}
			}
			state.Certificates = append(state.Certificates, meta)
			return nil
		})
	})
	return id, err
}

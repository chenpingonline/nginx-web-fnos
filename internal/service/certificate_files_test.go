package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertificateImportMethods(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"example.test"}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	for _, method := range []string{"", "pem", "file", "path"} {
		t.Run("method_"+method, func(t *testing.T) {
			s := testService(t)
			input := CertificateInput{Method: method, Certificate: certPEM, PrivateKey: keyPEM}
			if method == "path" {
				root := t.TempDir()
				input.CertificatePath = filepath.Join(root, "cert.pem")
				input.PrivateKeyPath = filepath.Join(root, "key.pem")
				if err := os.WriteFile(input.CertificatePath, []byte(certPEM), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(input.PrivateKeyPath, []byte(keyPEM), 0600); err != nil {
					t.Fatal(err)
				}
				input.Certificate, input.PrivateKey = "", ""
			}
			meta, err := s.ImportCertificate(input)
			if err != nil {
				t.Fatal(err)
			}
			savedKey := filepath.Join(s.paths.CertificateDir, meta.ID, "privkey.pem")
			info, err := os.Stat(savedKey)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatalf("private key mode %v", info.Mode())
			}
			data, err := os.ReadFile(savedKey)
			if err != nil || string(data) != keyPEM {
				t.Fatal("private key content changed")
			}
			if method == "path" {
				if err := os.Remove(input.PrivateKeyPath); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(savedKey); err != nil {
					t.Fatal("import did not preserve independent copy")
				}
			}
		})
	}
	s := testService(t)
	for _, input := range []CertificateInput{
		{Method: "acme", Certificate: certPEM, PrivateKey: keyPEM},
		{Method: "path", Certificate: certPEM, PrivateKey: keyPEM},
		{Method: "file", Certificate: certPEM, PrivateKey: certPEM},
		{Method: "path", CertificatePath: "relative.pem", PrivateKeyPath: "key.pem"},
	} {
		if _, err := s.ImportCertificate(input); err == nil {
			t.Fatal("accepted invalid certificate input")
		}
	}
	if len(s.State().Certificates) != 0 {
		t.Fatal("invalid imports modified state")
	}
}

func TestReadCertificateFileBounds(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "cert.pem")
	if err := os.WriteFile(file, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative.pem", root, filepath.Join(root, "missing"), file} {
		if _, err := readCertificateFile(path, 4); err == nil {
			t.Fatalf("accepted invalid file %s", path)
		}
	}
	if data, err := readCertificateFile(file, 5); err != nil || data != "12345" {
		t.Fatalf("read %q: %v", data, err)
	}
}

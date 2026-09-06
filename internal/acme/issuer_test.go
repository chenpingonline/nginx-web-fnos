package acme

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
)

type recordingDNS struct{ present, cleanup int }

func (p *recordingDNS) Present(context.Context, string, string, string) error {
	p.present++
	return nil
}
func (p *recordingDNS) CleanUp(context.Context, string, string, string) error {
	p.cleanup++
	return nil
}

// Run against local Pebble with PEBBLE_VA_ALWAYS_VALID=1 and its test CA trusted
// through LEGO_CA_CERTIFICATES. This exercises real ACME messages, not live DNS APIs.
func TestIssuerWithPebble(t *testing.T) {
	endpoint := os.Getenv("ACME_TEST_DIRECTORY")
	if endpoint == "" {
		t.Skip("set ACME_TEST_DIRECTORY for local Pebble protocol integration")
	}
	u, e := url.Parse(endpoint)
	if e != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		t.Fatal("test endpoint must be loopback")
	}
	in := validInput()
	if algorithm := os.Getenv("ACME_TEST_KEY_TYPE"); algorithm != "" {
		in.KeyType = algorithm
	}
	in.CA = "custom"
	in.DirectoryURL = endpoint
	in.RotateKey = true
	in.Credentials.EABKID = os.Getenv("ACME_TEST_EAB_KID")
	in.Credentials.EABHMAC = os.Getenv("ACME_TEST_EAB_HMAC")
	if e := validate(&in); e != nil {
		t.Fatal(e)
	}
	r := &Record{Input: in}
	dns := &recordingDNS{}
	saves := 0
	run := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if e := issueWithDNS(ctx, r, func() error { saves++; return nil }, func(string) {}, func(Input) (challenge.Provider, error) { return dns, nil }, []dns01.ChallengeOption{dns01.PropagationWait(0, true)}); e != nil {
			t.Fatal(e)
		}
	}
	run()
	if r.Registration == nil || !r.Pending || saves < 3 || dns.present == 0 || dns.present != dns.cleanup {
		t.Fatal("incomplete issue lifecycle")
	}
	first, e := leaf(r.Resource)
	if e != nil {
		t.Fatal(e)
	}
	assertAlgorithm := func(certKey any) {
		t.Helper()
		switch in.KeyType {
		case "ec256":
			key, ok := certKey.(*ecdsa.PublicKey)
			if !ok || key.Curve.Params().BitSize != 256 {
				t.Fatal("wrong ECDSA key")
			}
		case "rsa2048":
			key, ok := certKey.(*rsa.PublicKey)
			if !ok || key.N.BitLen() != 2048 {
				t.Fatal("wrong RSA key")
			}
		}
	}
	assertAlgorithm(first.PublicKey)
	oldKey := append([]byte(nil), r.Resource.PrivateKey...)
	accountKey := append([]byte(nil), r.AccountKey...)
	accountURI := r.Registration.Location
	for _, name := range in.Domains {
		found := false
		for _, san := range first.DNSNames {
			if san == name {
				found = true
			}
		}
		if !found {
			t.Fatal("missing SAN", name)
		}
	}
	saved, e := legacyRecordJSON(r)
	if e != nil {
		t.Fatal(e)
	}
	restored := &Record{}
	if e = json.Unmarshal(saved, restored); e != nil {
		t.Fatal(e)
	}
	r = restored
	if !bytes.Equal(oldKey, r.Resource.PrivateKey) {
		t.Fatal("persisted key material lost")
	}
	r.Pending = false
	r.Force = true
	run()
	second, e := leaf(r.Resource)
	if e != nil {
		t.Fatal(e)
	}
	assertAlgorithm(second.PublicKey)
	if first.SerialNumber.Cmp(second.SerialNumber) == 0 || bytes.Equal(oldKey, r.Resource.PrivateKey) {
		t.Fatal("renewal did not issue a new certificate and rotate key")
	}
	if !bytes.Equal(accountKey, r.AccountKey) || accountURI != r.Registration.Location {
		t.Fatal("ACME account changed during renewal")
	}
	r.Pending = false
	r.Force = false
	run()
	if r.Pending {
		t.Fatal("issued before renewal window")
	}
}

// Reproduce the private record format written by our lego v4 integration.
func legacyRecordJSON(r *Record) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}
	if r.Registration != nil {
		fields["registration"], err = json.Marshal(map[string]any{"body": r.Registration.Account, "uri": r.Registration.Location})
		if err != nil {
			return nil, err
		}
	}
	if r.Resource != nil {
		fields["resource"], err = json.Marshal(map[string]any{"domain": r.Input.Domains[0], "certUrl": r.Resource.CertURL, "certStableUrl": r.Resource.CertStableURL})
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

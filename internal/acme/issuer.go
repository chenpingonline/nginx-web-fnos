package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/go-acme/lego/v5/certificate"
	"github.com/go-acme/lego/v5/challenge"
	"github.com/go-acme/lego/v5/challenge/dns01"
	"github.com/go-acme/lego/v5/lego"
	legolog "github.com/go-acme/lego/v5/log"
	"github.com/go-acme/lego/v5/providers/dns/alidns"
	"github.com/go-acme/lego/v5/providers/dns/cloudflare"
	"github.com/go-acme/lego/v5/providers/dns/tencentcloud"
	"github.com/go-acme/lego/v5/registration"
)

// Provider logs can include full API responses. Expose only the safe task stages above.
func init() { legolog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) }

func issue(ctx context.Context, r *Record, checkpoint func() error, progress func(string)) error {
	if len(r.Input.DNSConfig) > 0 || (r.Input.Provider != "cloudflare" && r.Input.Provider != "alidns" && r.Input.Provider != "tencentcloud") {
		return issueInWorker(ctx, r, checkpoint, progress)
	}
	return issueWithDNS(ctx, r, checkpoint, progress, dnsProvider, nil)
}
func issueWithDNS(ctx context.Context, r *Record, checkpoint func() error, progress func(string), factory func(Input) (challenge.Provider, error), options []dns01.ChallengeOption) error {
	if len(r.AccountKey) == 0 {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}
		r.AccountKey, err = x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return err
		}
		if err = checkpoint(); err != nil {
			return err
		}
	}
	key, err := x509.ParsePKCS8PrivateKey(r.AccountKey)
	if err != nil {
		return err
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return fmt.Errorf("invalid account key")
	}
	user := &account{email: r.Input.Email, key: signer, registration: r.Registration}
	cfg := lego.NewConfig(user)
	cfg.CADirURL = r.Input.DirectoryURL
	cfg.UserAgent = "nginx-web"
	keyType := map[string]certcrypto.KeyType{"rsa2048": certcrypto.RSA2048, "rsa4096": certcrypto.RSA4096, "ec256": certcrypto.EC256, "ec384": certcrypto.EC384}[r.Input.KeyType]
	cfg.Certificate.Timeout = 10 * time.Minute
	cfg.HTTPClient.Timeout = 45 * time.Second

	progress("连接证书机构")
	client, err := lego.NewClient(cfg)
	if err != nil {
		return err
	}
	if user.registration == nil {
		progress("注册 ACME 账户")
		if r.Input.Credentials.EABKID != "" {
			user.registration, err = client.Registration.RegisterWithExternalAccountBinding(ctx, registration.RegisterEABOptions{TermsOfServiceAgreed: true, Kid: r.Input.Credentials.EABKID, HmacEncoded: r.Input.Credentials.EABHMAC})
		} else {
			user.registration, err = client.Registration.Register(ctx, registration.RegisterOptions{TermsOfServiceAgreed: true})
		}
		if err != nil {
			return err
		}
		r.Registration = user.registration
		if err = checkpoint(); err != nil {
			return err
		}
	}
	if r.Resource != nil && !r.Force {
		progress("检查续期时间")
		if cert, e := leaf(r.Resource); e == nil {
			renewAt := cert.NotAfter.Add(-cert.NotAfter.Sub(cert.NotBefore) / 3)
			if info, e := client.Certificate.GetRenewalInfo(ctx, cert); e == nil {
				if suggested := info.ShouldRenewAt(time.Now(), cert.NotAfter.Sub(time.Now())); suggested != nil && suggested.Before(cert.NotAfter) {
					renewAt = *suggested
				}
			}
			r.RenewAt = &renewAt
			if renewAt.After(time.Now()) {
				return checkpoint()
			}
		}
	}
	provider, err := factory(r.Input)
	if err != nil {
		return err
	}
	if err = client.Challenge.SetDNS01Provider(provider, options...); err != nil {
		return err
	}
	progress("DNS 验证与证书签发")
	var resource *certificate.Resource
	if r.Resource != nil {
		previous := *r.Resource
		previous.KeyType = keyType
		if r.Input.RotateKey {
			// v5 Renew does not forward Resource.KeyType when generating a key.
			// Supply a freshly generated key to preserve the selected algorithm.
			rotated, e := certcrypto.GeneratePrivateKey(keyType)
			if e != nil {
				return e
			}
			previous.PrivateKey = certcrypto.PEMEncode(rotated)
		}
		resource, err = client.Certificate.Renew(ctx, previous, &certificate.RenewOptions{Bundle: true})
	} else {
		resource, err = client.Certificate.Obtain(ctx, certificate.ObtainRequest{Domains: r.Input.Domains, KeyType: keyType, Bundle: true})
	}
	if err != nil {
		return err
	}
	r.Resource = resource
	r.Pending = true
	if c, e := leaf(resource); e == nil {
		at := c.NotAfter.Add(-c.NotAfter.Sub(c.NotBefore) / 3)
		r.RenewAt = &at
	}
	return checkpoint()
}
func dnsProvider(in Input) (challenge.Provider, error) {
	timeout := time.Duration(in.PropagationSeconds) * time.Second
	switch in.Provider {
	case "cloudflare":
		c := cloudflare.NewDefaultConfig()
		c.AuthToken = in.Credentials.Token
		c.ZoneToken = in.Credentials.Token
		c.PropagationTimeout = timeout
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
		return cloudflare.NewDNSProviderConfig(c)
	case "alidns":
		c := alidns.NewDefaultConfig()
		c.APIKey = in.Credentials.AccessID
		c.SecretKey = in.Credentials.Secret
		c.PropagationTimeout = timeout
		c.HTTPTimeout = 30 * time.Second
		return alidns.NewDNSProviderConfig(c)
	default:
		c := tencentcloud.NewDefaultConfig()
		c.SecretID = in.Credentials.AccessID
		c.SecretKey = in.Credentials.Secret
		c.PropagationTimeout = timeout
		c.HTTPTimeout = 30 * time.Second
		return tencentcloud.NewDNSProviderConfig(c)
	}
}

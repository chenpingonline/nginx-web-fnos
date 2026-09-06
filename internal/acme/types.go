// Package acme manages private ACME accounts and durable certificate jobs.
package acme

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"time"

	legoacme "github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certificate"
	"golang.org/x/net/idna"
)

type Input struct {
	DNSConfig          map[string]string `json:"dns_config,omitempty"`
	Name               string            `json:"name"`
	CA                 string            `json:"ca"`
	DirectoryURL       string            `json:"directory_url,omitempty"`
	Email              string            `json:"email"`
	Domains            []string          `json:"domains"`
	Provider           string            `json:"provider"`
	KeyType            string            `json:"key_type"`
	RotateKey          bool              `json:"rotate_key"`
	AcceptTerms        bool              `json:"accept_terms"`
	PropagationSeconds int               `json:"propagation_seconds"`
	Credentials        Credentials       `json:"credentials"`
}
type Credentials struct {
	Token    string `json:"token,omitempty"`
	AccessID string `json:"access_id,omitempty"`
	Secret   string `json:"secret,omitempty"`
	EABKID   string `json:"eab_kid,omitempty"`
	EABHMAC  string `json:"eab_hmac,omitempty"`
}
type Job struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	CA            string     `json:"ca"`
	Domains       []string   `json:"domains"`
	Provider      string     `json:"provider"`
	KeyType       string     `json:"key_type"`
	Enabled       bool       `json:"enabled"`
	Status        string     `json:"status"`
	Message       string     `json:"message"`
	CertificateID string     `json:"certificate_id,omitempty"`
	NotAfter      *time.Time `json:"not_after,omitempty"`
	NextAttempt   time.Time  `json:"next_attempt"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Failures      int        `json:"failures"`
}

// Record is private. It must never be sent by an API or included in configuration revisions.
type Record struct {
	Job          Job                       `json:"job"`
	Input        Input                     `json:"input"`
	AccountKey   []byte                    `json:"account_key"`
	Registration *legoacme.ExtendedAccount `json:"registration,omitempty"`
	Resource     *certificate.Resource     `json:"resource,omitempty"`
	Pending      bool                      `json:"pending"`
	RenewAt      *time.Time                `json:"renew_at,omitempty"`
	Force        bool                      `json:"force"`
}
type account struct {
	email        string
	key          crypto.Signer
	registration *legoacme.ExtendedAccount
}

func (a *account) GetEmail() string                           { return a.email }
func (a *account) GetPrivateKey() crypto.Signer               { return a.key }
func (a *account) GetRegistration() *legoacme.ExtendedAccount { return a.registration }
func leaf(resource *certificate.Resource) (*x509.Certificate, error) {
	if resource == nil {
		return nil, errors.New("missing certificate")
	}
	b, _ := pem.Decode(resource.Certificate)
	if b == nil {
		return nil, errors.New("invalid certificate")
	}
	return x509.ParseCertificate(b.Bytes)
}
func validate(in *Input) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	if len([]rune(in.Name)) > 80 {
		return errors.New("名称不能超过 80 个字符")
	}
	m, err := mail.ParseAddress(in.Email)
	if err != nil || m.Address != in.Email {
		return errors.New("请输入有效的电子邮箱")
	}
	if !in.AcceptTerms {
		return errors.New("请先同意所选证书机构的服务条款")
	}
	switch in.CA {
	case "letsencrypt":
		in.DirectoryURL = "https://acme-v02.api.letsencrypt.org/directory"
	case "staging":
		in.DirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	case "zerossl":
		in.DirectoryURL = "https://acme.zerossl.com/v2/DV90"
	case "custom":
		u, e := url.Parse(in.DirectoryURL)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("自定义 ACME Directory 必须是 HTTPS 地址，不得包含用户名或密码")
		}
	default:
		return errors.New("不支持的证书机构")
	}
	if (in.Credentials.EABKID == "") != (in.Credentials.EABHMAC == "") {
		return errors.New("EAB KID 和 HMAC Key 必须同时填写")
	}
	if in.Credentials.EABHMAC != "" {
		in.Credentials.EABHMAC = strings.TrimRight(strings.TrimSpace(in.Credentials.EABHMAC), "=")
		decoded, e := base64.RawURLEncoding.DecodeString(in.Credentials.EABHMAC)
		if e != nil || len(decoded) < 32 {
			return errors.New("EAB HMAC Key 必须是有效的 Base64URL 密钥（至少 32 字节）")
		}
	}
	if in.CA == "zerossl" && in.Credentials.EABKID == "" {
		return errors.New("ZeroSSL 需要 EAB KID 和 HMAC Key")
	}
	if err := validateDNS(in); err != nil {
		return err
	}
	for _, s := range []string{in.Credentials.Token, in.Credentials.AccessID, in.Credentials.Secret, in.Credentials.EABKID, in.Credentials.EABHMAC} {
		if len(s) > 8192 {
			return errors.New("认证信息过长")
		}
	}
	switch in.KeyType {
	case "rsa2048", "rsa4096", "ec256", "ec384":
	default:
		return errors.New("不支持的密钥算法")
	}
	if in.PropagationSeconds == 0 {
		in.PropagationSeconds = 180
	}
	if in.PropagationSeconds < 30 || in.PropagationSeconds > 1800 {
		return errors.New("DNS 等待时间应为 30 至 1800 秒")
	}
	if len(in.Domains) == 0 || len(in.Domains) > 100 {
		return errors.New("请填写 1 至 100 个域名")
	}
	seen := map[string]bool{}
	domains := []string{}
	for _, d := range in.Domains {
		d = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
		wild := strings.HasPrefix(d, "*.")
		base := strings.TrimPrefix(d, "*.")
		base, e := idna.Lookup.ToASCII(base)
		if e != nil || len(base) > 253 || net.ParseIP(base) != nil || !strings.Contains(base, ".") {
			return errors.New("仅支持完整域名和通配符域名，不支持 IP 地址")
		}
		for _, label := range strings.Split(base, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return errors.New("域名格式不正确")
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
					return errors.New("域名格式不正确")
				}
			}
		}
		d = base
		if wild {
			d = "*." + base
		}
		if !seen[d] {
			domains = append(domains, d)
			seen[d] = true
		}
	}
	in.Domains = domains
	if in.Name == "" {
		in.Name = domains[0]
	}
	return nil
}

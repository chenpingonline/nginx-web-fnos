package acme

import (
	_ "embed"
	"encoding/json"
	"errors"
	"strings"
)

//go:embed dns_catalog.json
var catalogJSON []byte

type DNSField struct {
	Multiline   bool   `json:"multiline"`
	Key         string `json:"key"`
	Description string `json:"description"`
	Advanced    bool   `json:"advanced"`
	Secret      bool   `json:"secret"`
}
type DNSProvider struct {
	Group       string     `json:"group"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	URL         string     `json:"url"`
	Fields      []DNSField `json:"fields"`
}
type DNSCatalog struct {
	Version   string        `json:"version"`
	Providers []DNSProvider `json:"providers"`
}

var catalog = func() DNSCatalog {
	var c DNSCatalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(err)
	}
	return c
}()

func Providers() DNSCatalog { return catalog }
func providerDefinition(code string) *DNSProvider {
	for i := range catalog.Providers {
		if catalog.Providers[i].Code == code {
			return &catalog.Providers[i]
		}
	}
	return nil
}
func validateDNS(in *Input) error {
	p := providerDefinition(in.Provider)
	if p == nil {
		return errors.New("不支持的 DNS 服务商")
	}
	allowed := map[string]bool{}
	for _, f := range p.Fields {
		allowed[f.Key] = true
	}
	for k, v := range in.DNSConfig {
		if !allowed[k] {
			return errors.New("DNS 配置包含不属于所选服务商的字段")
		}
		if len(v) > 65536 || strings.ContainsRune(v, 0) {
			return errors.New("DNS 配置值无效或过长")
		}
	}
	if len(in.DNSConfig) > 0 {
		return nil
	}
	// Keep the original three-provider credential format readable after upgrade.
	switch in.Provider {
	case "cloudflare":
		if in.Credentials.Token == "" {
			return errors.New("请输入 Cloudflare API Token")
		}
	case "alidns", "tencentcloud":
		if in.Credentials.AccessID == "" || in.Credentials.Secret == "" {
			return errors.New("请输入 DNS 服务商的 ID 和 Secret")
		}
	}
	return nil
}

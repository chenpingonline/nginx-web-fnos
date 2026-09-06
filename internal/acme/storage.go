package acme

import (
	"encoding/json"
	legoacme "github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"

	"github.com/go-acme/lego/v5/certificate"
)

// lego deliberately excludes PEM bytes from Resource JSON. Persist those bytes
// explicitly in our private record so restart/renewal never loses key material.
type recordAlias Record
type material struct {
	Chain  []byte `json:"chain"`
	Key    []byte `json:"key"`
	Issuer []byte `json:"issuer"`
	CSR    []byte `json:"csr"`
}
type recordJSON struct {
	*recordAlias
	Material *material `json:"material,omitempty"`
}

func (r Record) MarshalJSON() ([]byte, error) {
	out := recordJSON{recordAlias: (*recordAlias)(&r)}
	if r.Resource != nil {
		out.Material = &material{r.Resource.Certificate, r.Resource.PrivateKey, r.Resource.IssuerCertificate, r.Resource.CSR}
	}
	return json.Marshal(out)
}
func (r *Record) UnmarshalJSON(b []byte) error {
	out := recordJSON{recordAlias: (*recordAlias)(r)}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	if out.Material != nil {
		if r.Resource == nil {
			r.Resource = &certificate.Resource{}
		}
		r.Resource.Certificate = out.Material.Chain
		r.Resource.PrivateKey = out.Material.Key
		r.Resource.IssuerCertificate = out.Material.Issuer
		r.Resource.CSR = out.Material.CSR
	}
	// v4 stored accounts as {body, uri}; v5 flattens the body and uses accountURL.
	if r.Registration != nil && r.Registration.Location == "" {
		var legacy struct {
			Registration struct {
				Body legoacme.Account `json:"body"`
				URI  string           `json:"uri"`
			} `json:"registration"`
		}
		if err := json.Unmarshal(b, &legacy); err != nil {
			return err
		}
		if legacy.Registration.URI != "" {
			r.Registration = &legoacme.ExtendedAccount{Account: legacy.Registration.Body, Location: legacy.Registration.URI}
		}
	}
	if r.Resource != nil {
		if len(r.Resource.Domains) == 0 {
			r.Resource.Domains = append([]string(nil), r.Input.Domains...)
		}
		if r.Resource.KeyType == "" {
			r.Resource.KeyType = map[string]certcrypto.KeyType{"rsa2048": certcrypto.RSA2048, "rsa4096": certcrypto.RSA4096, "ec256": certcrypto.EC256, "ec384": certcrypto.EC384}[r.Input.KeyType]
		}
	}
	return nil
}

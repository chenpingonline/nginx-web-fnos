package acme

import (
	"bytes"
	"encoding/json"
	legoacme "github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"
	"testing"
	"time"
)

func TestReadLegacyRecord(t *testing.T) {
	in := validInput()
	in.KeyType = "ec256"
	r := &Record{Input: in, AccountKey: []byte("stored-account-key"), Resource: testResource(t, time.Now()), Registration: &legoacme.ExtendedAccount{Account: legoacme.Account{Status: "valid"}, Location: "https://ca.example/acct/42"}}
	raw, err := legacyRecordJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	var restored Record
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Registration.Location != r.Registration.Location || restored.Registration.Status != "valid" {
		t.Fatal("legacy account was lost")
	}
	if !bytes.Equal(restored.AccountKey, r.AccountKey) || !bytes.Equal(restored.Resource.PrivateKey, r.Resource.PrivateKey) || !bytes.Equal(restored.Resource.Certificate, r.Resource.Certificate) {
		t.Fatal("legacy key material was lost")
	}
	if restored.Resource.KeyType != certcrypto.EC256 || len(restored.Resource.Domains) != len(in.Domains) {
		t.Fatal("legacy certificate metadata was not restored")
	}
	raw, err = json.Marshal(&restored)
	if err != nil {
		t.Fatal(err)
	}
	var again Record
	if err = json.Unmarshal(raw, &again); err != nil {
		t.Fatal(err)
	}
	if again.Registration.Location != r.Registration.Location || !bytes.Equal(again.Resource.PrivateKey, r.Resource.PrivateKey) {
		t.Fatal("v5 record round trip lost data")
	}
}

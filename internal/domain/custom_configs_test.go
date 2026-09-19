package domain

import "testing"

func TestValidateCustomConfigs(t *testing.T) {
	valid := []CustomConfig{{Name: "custom-1panel.conf", Content: "server { return 200; }"}, {Name: "custom-tcp.stream", Content: "server { listen 9000; }"}}
	if err := ValidateCustomConfigs(valid); err != nil {
		t.Fatal(err)
	}
	for _, configs := range [][]CustomConfig{
		{{Name: "010-generated.conf", Content: "server {}"}},
		{{Name: "custom-empty.conf", Content: "  "}},
		{{Name: "custom-a.conf", Content: "x"}, {Name: "CUSTOM-A.CONF", Content: "y"}},
	} {
		if err := ValidateCustomConfigs(configs); err == nil {
			t.Fatal("accepted invalid custom config", configs)
		}
	}
}

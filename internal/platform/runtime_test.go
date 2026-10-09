package platform

import "testing"

func TestRuntimeRejectsAccidentalPublicFnOSListener(t *testing.T) {
	for _, test := range []struct {
		mode, listen string
		valid        bool
	}{
		{"", "", true}, {"fnos", "", true}, {"fnos", ":8080", false},
		{"standalone", "", true}, {"standalone", "127.0.0.1:8080", true},
		{"standalone", "[::]:8080", true}, {"standalone", ":0", false},
		{"standalone", ":http", false}, {"standalone", "8080", false}, {"typo", "", false},
	} {
		t.Setenv("FNPROXY_MODE", test.mode)
		t.Setenv("FNPROXY_LISTEN", test.listen)
		_, err := LoadRuntime()
		if (err == nil) != test.valid {
			t.Fatalf("mode=%q listen=%q: %v", test.mode, test.listen, err)
		}
	}
}

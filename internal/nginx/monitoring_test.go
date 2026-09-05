package nginx

import "testing"

func TestBasicStatusParser(t *testing.T) {
	status, err := parseBasicStatus("Active connections: 4\nserver accepts handled requests\n 10 10 24\nReading: 0 Writing: 1 Waiting: 3\n")
	if err != nil || status.Requests != 24 || status.Connections != 3 {
		t.Fatalf("incorrect status: %+v %v", status, err)
	}
	for _, input := range []string{"<html>Error</html>", "Active connections: -1\nheader\n1 1 1\nReading: 0 Writing: 1 Waiting: 0"} {
		if _, err := parseBasicStatus(input); err == nil {
			t.Fatal("malformed status accepted")
		}
	}
}

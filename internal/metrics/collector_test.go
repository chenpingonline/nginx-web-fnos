package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func collectorFixture(t *testing.T) (*Collector, time.Time) {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	c := New(filepath.Join(root, "http-metrics.log"), filepath.Join(root, "history.json"), now)
	if err := os.WriteFile(c.logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return c, now
}

func appendLog(t *testing.T, path string, now time.Time, rule string, status int) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "{\"time\":%.3f,\"rule\":%q,\"status\":%d,\"bytes\":256}\n", float64(now.UnixMilli())/1000, rule, status)
}

func TestSamplingExcludesSelfTrafficAndLeavesGaps(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, &Sample{PID: 7, Requests: 10, Connections: 2}, true)
	c.Collect(now.Add(5*time.Second), &Sample{PID: 7, Requests: 21, Connections: 4}, true)
	r := c.Snapshot(now.Add(5*time.Second), 15, "")
	if r.RPS == nil || *r.RPS != 2 || r.Connections == nil || *r.Connections != 4 {
		t.Fatalf("bad sample: %+v", r)
	}
	if r.Points[0].RPS != nil {
		t.Fatal("missing history rendered as zero")
	}
	c.Collect(now.Add(10*time.Second), &Sample{PID: 7, Requests: 22, Connections: 0}, true)
	if r := c.Snapshot(now.Add(10*time.Second), 15, ""); r.RPS == nil || *r.RPS != 0 {
		t.Fatalf("idle traffic includes monitoring request: %+v", r.RPS)
	}
	c.Collect(now.Add(90*time.Second), &Sample{PID: 7, Requests: 100, Connections: 0}, true)
	if r := c.Snapshot(now.Add(90*time.Second), 15, ""); r.RPS != nil {
		t.Fatal("outage interval must not produce a rate")
	}
	c.Collect(now.Add(95*time.Second), &Sample{PID: 8, Requests: 1, Connections: 0}, true)
	if r := c.Snapshot(now.Add(95*time.Second), 15, ""); r.RPS != nil {
		t.Fatal("restart must reset baseline")
	}
	if r := c.Snapshot(now.Add(130*time.Second), 15, ""); r.Available || r.RPS != nil || r.Connections != nil {
		t.Fatal("stale live values exposed")
	}
}

func TestResponseRatesUseCompletedLogsAndCurrentInterval(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, &Sample{PID: 7, Requests: 1}, true)
	if c.Snapshot(now, 15, "").ResponseRPS != nil {
		t.Fatal("first sample must not have a live response rate")
	}
	// Ten requests have arrived, while only three have completed so far.
	appendLog(t, c.logPath, now.Add(time.Second), "alpha", 200)
	appendLog(t, c.logPath, now.Add(2*time.Second), "alpha", 404)
	appendLog(t, c.logPath, now.Add(3*time.Second), "beta", 502)
	c.Collect(now.Add(5*time.Second), &Sample{PID: 7, Requests: 12}, true)
	r := c.Snapshot(now.Add(5*time.Second), 15, "")
	assertRate(t, "received requests", r.RPS, 2)
	assertRate(t, "completed responses", r.ResponseRPS, 0.6)
	assertRate(t, "selected rule responses", c.Snapshot(now.Add(5*time.Second), 15, "alpha").ResponseRPS, 0.4)
	assertRate(t, "idle rule responses", c.Snapshot(now.Add(5*time.Second), 15, "idle").ResponseRPS, 0)
	assertRate(t, "response chart", r.Points[len(r.Points)-1].ResponseRPS, 0.6)
	if r.Points[0].ResponseRPS != nil {
		t.Fatal("unobserved response history must remain null")
	}
	appendLog(t, c.logPath, now.Add(6*time.Second), "beta", 200)
	c.Collect(now.Add(10*time.Second), &Sample{PID: 7, Requests: 13}, true)
	r = c.Snapshot(now.Add(10*time.Second), 15, "")
	assertRate(t, "latest interval rather than all-history mean", r.ResponseRPS, 0.2)
	assertRate(t, "bucket response average", r.Points[len(r.Points)-1].ResponseRPS, 0.4)
	assertRate(t, "idle selected rule latest interval", c.Snapshot(now.Add(10*time.Second), 15, "alpha").ResponseRPS, 0)
	c.Collect(now.Add(15*time.Second), &Sample{PID: 7, Requests: 14}, false)
	if c.Snapshot(now.Add(15*time.Second), 15, "").ResponseRPS != nil {
		t.Fatal("logging-disabled live response rate must be null")
	}
	c.Collect(now.Add(20*time.Second), &Sample{PID: 7, Requests: 15}, true)
	if c.Snapshot(now.Add(20*time.Second), 15, "").ResponseRPS != nil {
		t.Fatal("first interval after missing logging coverage must be null")
	}
	c.Collect(now.Add(25*time.Second), &Sample{PID: 7, Requests: 16}, true)
	assertRate(t, "observed idle responses", c.Snapshot(now.Add(25*time.Second), 15, "").ResponseRPS, 0)
	if c.Snapshot(now.Add(45*time.Second), 15, "").ResponseRPS != nil {
		t.Fatal("stale live response rate must be null")
	}
	c.Collect(now.Add(50*time.Second), &Sample{PID: 7, Requests: 17}, true)
	if c.Snapshot(now.Add(50*time.Second), 15, "").ResponseRPS != nil {
		t.Fatal("a collection gap must not be presented as live traffic")
	}
	c.Collect(now.Add(55*time.Second), &Sample{PID: 8, Requests: 1}, true)
	if c.Snapshot(now.Add(55*time.Second), 15, "").ResponseRPS != nil {
		t.Fatal("a replaced Nginx process must start a new rate baseline")
	}
}

func TestResponseRateAtMinuteBoundaryAndHistoryRecovery(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now.Add(55*time.Second), &Sample{PID: 1, Requests: 1}, true)
	appendLog(t, c.logPath, now.Add(56*time.Second), "alpha", 200)
	appendLog(t, c.logPath, now.Add(57*time.Second), "alpha", 200)
	appendLog(t, c.logPath, now.Add(62*time.Second), "alpha", 200)
	c.Collect(now.Add(65*time.Second), &Sample{PID: 1, Requests: 5}, true)
	r := c.Snapshot(now.Add(65*time.Second), 15, "")
	assertRate(t, "first minute observed seconds", r.Points[len(r.Points)-2].ResponseRPS, 0.4)
	assertRate(t, "second minute observed seconds", r.Points[len(r.Points)-1].ResponseRPS, 0.2)
	for _, point := range c.Snapshot(now.Add(65*time.Second), 1440, "").Points {
		if point.Requests != nil && *point.Requests > 0 {
			assertRate(t, "five-minute weighted response average", point.ResponseRPS, 0.3)
		}
	}
	c.Flush(now.Add(65 * time.Second))
	appendLog(t, c.logPath, now.Add(66*time.Second), "alpha", 200)
	restored := New(c.logPath, c.historyPath, now.Add(70*time.Second))
	restored.Collect(now.Add(70*time.Second), &Sample{PID: 1, Requests: 7}, true)
	if restored.Snapshot(now.Add(70*time.Second), 15, "").ResponseRPS != nil {
		t.Fatal("checkpoint recovery must not produce a live backlog spike")
	}
	restored.Collect(now.Add(75*time.Second), &Sample{PID: 1, Requests: 8}, true)
	assertRate(t, "recovered idle rate", restored.Snapshot(now.Add(75*time.Second), 15, "").ResponseRPS, 0)
}

func TestLogsRotationCheckpointAndRuleIsolation(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, &Sample{PID: 1, Requests: 1}, true)
	appendLog(t, c.logPath, now.Add(time.Second), "alpha", 200)
	appendLog(t, c.logPath, now.Add(2*time.Second), "beta", 502)
	appendLog(t, c.logPath, now.Add(3*time.Second), "default", 404)
	c.Collect(now.Add(5*time.Second), &Sample{PID: 1, Requests: 5}, true)
	c.Flush(now.Add(5 * time.Second))
	appendLog(t, c.logPath, now.Add(6*time.Second), "alpha", 503)
	if err := os.Rename(c.logPath, c.logPath+".1"); err != nil {
		t.Fatal(err)
	}
	appendLog(t, c.logPath, now.Add(7*time.Second), "beta", 200)
	restarted := New(c.logPath, c.historyPath, now.Add(8*time.Second))
	restarted.Collect(now.Add(10*time.Second), &Sample{PID: 1, Requests: 8}, true)
	restarted.Collect(now.Add(15*time.Second), &Sample{PID: 1, Requests: 9}, true)
	r := restarted.Snapshot(now.Add(15*time.Second), 60, "")
	if r.Counts.Requests != 5 || r.Counts.Errors != 2 || r.Counts.Bytes != 1280 {
		t.Fatalf("lost or duplicated records: %+v", r.Counts)
	}
	if r.Counts.ClientErrors == nil || *r.Counts.ClientErrors != 1 {
		t.Fatalf("4xx count was lost during rotation or checkpoint restore: %+v", r.Counts)
	}
	assertRate(t, "combined error rate", r.ErrorRate, 60)
	assertRate(t, "client error rate", r.ClientErrorRate, 20)
	assertRate(t, "server error rate", r.ServerErrorRate, 40)
	if r.Rules["alpha"].Requests != 2 || r.Rules["alpha"].Errors != 1 || r.Rules["default"].Requests != 0 {
		t.Fatalf("rule attribution incorrect: %+v", r.Rules)
	}
	selected := restarted.Snapshot(now.Add(15*time.Second), 60, "beta")
	if selected.Counts.Requests != 2 || selected.ErrorRate == nil || *selected.ErrorRate != 50 || selected.Connections != nil {
		t.Fatalf("selected scope incorrect: %+v", selected)
	}
	if len(restarted.Snapshot(now.Add(15*time.Second), 1440, "").Points) != 288 {
		t.Fatal("24h aggregation must be bounded")
	}
}

func assertRate(t *testing.T, label string, rate *float64, want float64) {
	t.Helper()
	if rate == nil || math.Abs(*rate-want) > 0.000001 {
		t.Fatalf("%s: got %v, want %v", label, rate, want)
	}
}

func TestErrorClassesAndNoRequests(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, &Sample{PID: 1, Requests: 1}, true)
	c.Collect(now.Add(5*time.Second), &Sample{PID: 1, Requests: 2}, true)
	empty := c.Snapshot(now.Add(5*time.Second), 15, "")
	if empty.ErrorRate != nil || empty.ClientErrorRate != nil || empty.ServerErrorRate != nil {
		t.Fatalf("no requests must not produce a percentage: %+v", empty)
	}
	if empty.Counts.ClientErrors == nil || *empty.Counts.ClientErrors != 0 {
		t.Fatal("an observed interval without requests has a known zero 4xx count")
	}
	for _, point := range empty.Points {
		if point.ErrorRate != nil || point.ClientErrorRate != nil || point.ServerErrorRate != nil {
			t.Fatal("empty chart point must not produce a percentage")
		}
	}
	for i, status := range []int{399, 400, 404, 499, 500, 503, 599, 200} {
		appendLog(t, c.logPath, now.Add(time.Duration(i+6)*time.Second), "alpha", status)
	}
	c.Collect(now.Add(15*time.Second), &Sample{PID: 1, Requests: 11}, true)
	r := c.Snapshot(now.Add(15*time.Second), 15, "alpha")
	if r.Counts.Requests != 8 || r.Counts.Errors != 3 || r.Counts.ClientErrors == nil || *r.Counts.ClientErrors != 3 {
		t.Fatalf("status classes are incorrect: %+v", r.Counts)
	}
	assertRate(t, "combined error rate", r.ErrorRate, 75)
	assertRate(t, "client error rate", r.ClientErrorRate, 37.5)
	assertRate(t, "server error rate", r.ServerErrorRate, 37.5)
	last := r.Points[len(r.Points)-1]
	assertRate(t, "point combined error rate", last.ErrorRate, 75)
	assertRate(t, "point client error rate", last.ClientErrorRate, 37.5)
	assertRate(t, "point server error rate", last.ServerErrorRate, 37.5)
}

func TestLegacyHistoryPreservesUnknownClientErrorsAndTimeScopes(t *testing.T) {
	c, now := collectorFixture(t)
	old := now.Add(-20 * time.Minute)
	// Version 1 checkpoints predate the client_errors field. Keep these requests
	// and their known server errors without inventing a zero client-error count.
	saved := fmt.Sprintf(`{"version":1,"since":%q,"buckets":{%q:{"time":%d,"log_seconds":60,"counts":{"requests":4,"errors":1},"rules":{"legacy":{"requests":4,"errors":1}}}}}`, old.Format(time.RFC3339), fmt.Sprint(old.Unix()), old.Unix())
	if err := os.WriteFile(c.historyPath, []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	c = New(c.logPath, c.historyPath, now)
	c.Collect(now, &Sample{PID: 1, Requests: 1}, true)
	appendLog(t, c.logPath, now.Add(time.Second), "legacy", 404)
	appendLog(t, c.logPath, now.Add(2*time.Second), "modern", 502)
	c.Collect(now.Add(5*time.Second), &Sample{PID: 1, Requests: 4}, true)
	r := c.Snapshot(now.Add(5*time.Second), 60, "")
	if r.Counts.Requests != 6 || r.Counts.Errors != 2 || r.Counts.ClientErrors != nil || r.ErrorRate != nil || r.ClientErrorRate != nil {
		t.Fatalf("old history was lost or unknown client errors were assumed zero: %+v", r)
	}
	assertRate(t, "known historical server error rate", r.ServerErrorRate, 100.0/3)
	legacy := r.Rules["legacy"]
	if legacy.Requests != 5 || legacy.Errors != 1 || legacy.ClientErrors != nil {
		t.Fatalf("per-rule legacy uncertainty was lost: %+v", legacy)
	}
	modern := c.Snapshot(now.Add(5*time.Second), 60, "modern")
	assertRate(t, "unrelated rule combined error rate", modern.ErrorRate, 100)
	assertRate(t, "unrelated rule client error rate", modern.ClientErrorRate, 0)
	assertRate(t, "unrelated rule server error rate", modern.ServerErrorRate, 100)
	recent := c.Snapshot(now.Add(5*time.Second), 15, "")
	if recent.Counts.Requests != 2 || recent.Counts.ClientErrors == nil || *recent.Counts.ClientErrors != 1 {
		t.Fatalf("older requests leaked into the selected time range: %+v", recent)
	}
	assertRate(t, "recent combined error rate", recent.ErrorRate, 100)
	assertRate(t, "recent client error rate", recent.ClientErrorRate, 50)
	assertRate(t, "recent server error rate", recent.ServerErrorRate, 50)
	for _, point := range c.Snapshot(now.Add(5*time.Second), 1440, "").Points {
		if point.Requests != nil && *point.Requests == 4 {
			if point.ErrorRate != nil || point.ClientErrorRate != nil {
				t.Fatal("legacy chart point fabricated combined/client rates")
			}
			assertRate(t, "legacy chart server error rate", point.ServerErrorRate, 25)
		}
	}
	c.Flush(now.Add(5 * time.Second))
	reloaded := New(c.logPath, c.historyPath, now.Add(6*time.Second)).Snapshot(now.Add(6*time.Second), 60, "")
	if reloaded.Counts.Requests != 6 || reloaded.Counts.ClientErrors != nil || reloaded.ErrorRate != nil {
		t.Fatalf("checkpoint upgrade discarded history or uncertainty: %+v", reloaded)
	}
	encoded, err := json.Marshal(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"error_rate", "client_error_rate"} {
		if value, ok := payload[key]; !ok || value != nil {
			t.Fatalf("unknown %s must be explicit JSON null", key)
		}
	}
}

func TestCountsAggregationKeepsUnknownsWithoutAliasing(t *testing.T) {
	knownClientErrors := uint64(2)
	known := Counts{Requests: 5, ClientErrors: &knownClientErrors}
	legacy := Counts{Requests: 5, Errors: 1}
	for _, order := range [][]Counts{{known, legacy}, {legacy, known}} {
		var total Counts
		for _, value := range order {
			total.add(value)
		}
		if total.Requests != 10 || total.Errors != 1 || total.ClientErrors != nil {
			t.Fatalf("unknown propagation depends on aggregation order: %+v", total)
		}
	}
	var total Counts
	total.add(known)
	snapshot := total
	total.add(known)
	if *known.ClientErrors != 2 || *snapshot.ClientErrors != 2 || *total.ClientErrors != 4 {
		t.Fatal("aggregation mutated previously returned counters")
	}
}

func TestPartialRecordDisabledLoggingAndRetention(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, &Sample{PID: 1, Requests: 1}, true)
	line := fmt.Sprintf("{\"time\":%d,\"rule\":\"alpha\",\"status\":502,\"bytes\":10}", now.Unix()+1)
	os.WriteFile(c.logPath, []byte(line), 0o600)
	c.Collect(now.Add(5*time.Second), &Sample{PID: 1, Requests: 2}, true)
	if c.data.Cursor.Offset != 0 {
		t.Fatal("partial record committed")
	}
	f, _ := os.OpenFile(c.logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString("\n")
	f.Close()
	c.Collect(now.Add(10*time.Second), &Sample{PID: 1, Requests: 3}, true)
	if r := c.Snapshot(now.Add(10*time.Second), 15, ""); r.Counts.Errors != 1 {
		t.Fatal("partial record not recovered")
	}
	c.Collect(now.Add(15*time.Second), &Sample{PID: 1, Requests: 4}, false)
	if r := c.Snapshot(now.Add(15*time.Second), 15, ""); r.Logging || r.ObservedSeconds != 0 {
		t.Fatalf("disabled/partial logging coverage incorrect: %v", r.ObservedSeconds)
	}
	c.Collect(now.Add(Retention+time.Hour), nil, false)
	if len(c.data.Buckets) != 0 {
		t.Fatal("expired history retained")
	}
}

func TestLongHistoryWindowsAndPersistence(t *testing.T) {
	c, now := collectorFixture(t)
	c.data.Since = now.Add(-31 * 24 * time.Hour)
	// Two unequal coverage intervals per day prove rates are weighted by time
	// and error percentages by requests rather than averaging percentages.
	for day := 0; day <= 31; day++ {
		for offset, seconds := range []float64{10, 30} {
			minute := now.Add(-time.Duration(day)*24*time.Hour - time.Duration(offset)*time.Minute).Unix()
			client := uint64(offset + 1)
			counts := Counts{Requests: uint64(seconds * seconds), Errors: 1, ClientErrors: &client}
			b := c.bucket(minute)
			b.Seconds, b.LogSeconds = seconds, seconds
			b.Requests, b.Counts = seconds*seconds, counts
			b.Rules["alpha"] = counts
		}
	}
	c.prune(now)
	c.Flush(now)
	restored := New(c.logPath, c.historyPath, now)
	if restored.data.Buckets[now.Add(-29*24*time.Hour).Unix()] == nil {
		t.Fatal("month history lost after restart")
	}
	if restored.data.Buckets[now.Add(-31*24*time.Hour).Unix()] != nil {
		t.Fatal("expired history restored")
	}
	for _, window := range []struct{ minutes, points, days int }{
		{15, 15, 1}, {60, 60, 1}, {300, 300, 1}, {1440, 288, 1}, {10080, 168, 7}, {43200, 240, 30},
	} {
		t.Run(fmt.Sprint(window.minutes), func(t *testing.T) {
			if !ValidRange(window.minutes) {
				t.Fatal("range rejected")
			}
			for _, rule := range []string{"", "alpha"} {
				r := restored.Snapshot(now, window.minutes, rule)
				if len(r.Points) != window.points || r.Counts.Requests != uint64(window.days*1000) || r.ObservedSeconds != float64(window.days*40) {
					t.Fatalf("incorrect range aggregation: points=%d requests=%d seconds=%v", len(r.Points), r.Counts.Requests, r.ObservedSeconds)
				}
				if r.ErrorRate == nil || math.Abs(*r.ErrorRate-0.5) > 1e-9 || r.Rules["alpha"].Requests != r.Counts.Requests {
					t.Fatal("error rate or rule totals changed")
				}
				var gaps int
				for _, point := range r.Points {
					if point.Requests == nil {
						gaps++
						continue
					}
					if point.ResponseRPS == nil || math.Abs(*point.ResponseRPS-25) > 1e-9 {
						// Minute-sized windows retain each interval's own rate.
						if window.minutes > 300 {
							t.Fatal("long-range rate not weighted by coverage")
						}
					}
				}
				if gaps == 0 {
					t.Fatal("missing history filled with zero")
				}
			}
		})
	}
	for _, invalid := range []int{-1, 0, 301, 43201} {
		if ValidRange(invalid) {
			t.Fatalf("accepted invalid range %d", invalid)
		}
	}
}

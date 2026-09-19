package metrics

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"testing"
	"time"
)

func TestLimitAnalysisCoverageIsolationAndPersistence(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, nil, true)
	snapshot := domain.RateLimitSnapshot{ID: "policy", Name: "原策略", RuleName: "接口", Settings: domain.RateLimitSettings{Enabled: true, RequestsPerSecond: 10, Burst: 20}}
	ingest := func(rule, req, conn string, status int, legacy bool) {
		metadata, _ := json.Marshal(snapshot)
		record := map[string]any{"time": float64(now.UnixMilli()) / 1000, "rule": rule, "status": status, "method": "GET", "uri": "/api", "request_time": 0.1, "limit_policy": base64.StdEncoding.EncodeToString(metadata)}
		if !legacy {
			record["limit_req_status"] = req
			record["limit_conn_status"] = conn
		}
		data, _ := json.Marshal(record)
		c.ingest(data, now)
	}
	ingest("a", "", "", 503, true)              // Legacy 503 is unknown, not a limit event.
	ingest("a", "PASSED", "PASSED", 503, false) // An upstream 503 is not rejected.
	ingest("a", "REJECTED", "", 503, false)
	ingest("a", "", "REJECTED", 429, false)
	ingest("a", "DELAYED", "PASSED", 200, false)
	ingest("a", "REJECTED_DRY_RUN", "PASSED", 200, false)
	ingest("a", "DELAYED_DRY_RUN", "PASSED", 200, false)
	ingest("b", "REJECTED", "REJECTED", 200, false) // Deduplicate total; independent reasons.
	ingest("a", "INVALID", "REJECTED", 503, false)  // Invalid instrumentation is unknown.
	r := c.Snapshot(now, 60, "")
	l := r.Analysis.Limits
	if l.Total != 9 || l.Covered != 7 || l.Rejected != 3 || l.RequestRejected != 2 || l.ConnectionRejected != 2 || l.Delayed != 1 || l.AffectedRules != 2 {
		t.Fatalf("incorrect limits: %+v", l)
	}
	if l.Rate == nil || *l.Rate != float64(3)*100/7 {
		t.Fatalf("wrong denominator: %+v", l)
	}
	a := c.Snapshot(now, 60, "a").Analysis.Limits
	if a.Rejected != 2 || a.AffectedRules != 1 || len(a.Recent) != 3 {
		t.Fatalf("scope leaked: %+v", a)
	}
	point := r.Points[len(r.Points)-1]
	if point.LimitRequestRejected == nil || *point.LimitRequestRejected != 2 {
		t.Fatalf("missing trend: %+v", point)
	}
	snapshot.Name = "新策略"
	snapshot.Settings.RequestsPerSecond = 25
	now = now.Add(time.Second)
	ingest("a", "REJECTED", "", 503, false)
	a = c.Snapshot(now, 60, "a").Analysis.Limits
	if !a.Rules[0].PolicyChanged || a.Recent[0].Policy.Name != "新策略" || a.Recent[1].Policy.Name != "原策略" {
		t.Fatalf("historical policy overwritten: %+v", a)
	}
	// Other errors must not evict independent limit samples.
	for i := 0; i < 250; i++ {
		ingest("a", "PASSED", "", 500, false)
	}
	c.Flush(now)
	restored := New(c.logPath, c.historyPath, now)
	a = restored.Snapshot(now, 60, "a").Analysis.Limits
	if len(a.Recent) != 4 || a.Rejected != 3 || a.Rules[0].Policy.Name != "新策略" {
		t.Fatalf("limit history was lost: %+v", a)
	}
	restored.prune(now.Add(Retention + time.Minute))
	if len(restored.data.LimitRecent) != 0 {
		t.Fatal("expired samples retained")
	}
}

func TestLimitLegacyAndNullFieldsRemainUnknown(t *testing.T) {
	c, now := collectorFixture(t)
	c.Collect(now, nil, true)
	for _, fields := range []string{"", `,"limit_req_status":null,"limit_conn_status":"REJECTED"`, `,"limit_req_status":"REJECTED"`} {
		c.ingest([]byte(fmt.Sprintf(`{"time":%d,"rule":"a","status":503%s}`, now.Unix(), fields)), now)
	}
	r := c.Snapshot(now, 60, "")
	if r.Analysis.Limits.Covered != 0 || r.Analysis.Limits.Rate != nil || len(r.Analysis.Limits.Recent) != 0 || r.Points[len(r.Points)-1].LimitRequestRejected != nil {
		t.Fatalf("legacy treated as known: %+v", r.Analysis.Limits)
	}
	for _, sample := range r.Analysis.Recent {
		if sample.LimitRequest != "" || sample.LimitConnection != "" {
			t.Fatal("unknown limit state mislabeled")
		}
	}
}

// Package metrics collects bounded, minute-resolution HTTP history without an external database.
package metrics

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

const (
	Retention         = 30 * 24 * time.Hour
	checkpointVersion = 2
)

// ValidRange is shared by the API and collector so unsupported windows cannot
// silently return a different time range.
func ValidRange(minutes int) bool {
	switch minutes {
	case 15, 60, 300, 1440, 10080, 43200:
		return true
	default:
		return false
	}
}

type Sample struct {
	PID         int
	Requests    uint64
	Connections int
}

type Counts struct {
	Requests                 uint64     `json:"requests"`
	Errors                   uint64     `json:"errors"`        // Server errors (5xx); retained for checkpoint compatibility.
	ClientErrors             *uint64    `json:"client_errors"` // Nil when requests include history collected before 4xx tracking.
	Bytes                    uint64     `json:"bytes"`
	LastSeen                 *time.Time `json:"last_seen,omitempty"`
	TimedRequests            uint64     `json:"timed_requests,omitempty"`
	RequestTimeMillis        float64    `json:"request_time_ms,omitempty"`
	TimedUpstreams           uint64     `json:"timed_upstreams,omitempty"`
	UpstreamTimeMillis       float64    `json:"upstream_time_ms,omitempty"`
	TimedUpstreamHeaders     uint64     `json:"timed_upstream_headers,omitempty"`
	UpstreamHeaderTimeMillis float64    `json:"upstream_header_time_ms,omitempty"`
}

func (c *Counts) add(other Counts) {
	// Empty counts are neutral. Once nonempty legacy history contributes an
	// unknown 4xx count, keep that uncertainty throughout the aggregation.
	if c.ClientErrors == nil && c.Requests == 0 {
		zero := uint64(0)
		c.ClientErrors = &zero
	}
	if other.ClientErrors == nil && other.Requests > 0 {
		c.ClientErrors = nil
	} else if c.ClientErrors != nil && other.ClientErrors != nil {
		total := *c.ClientErrors + *other.ClientErrors
		c.ClientErrors = &total
	}
	c.Requests += other.Requests
	c.Errors += other.Errors
	c.Bytes += other.Bytes
	c.TimedRequests += other.TimedRequests
	c.RequestTimeMillis += other.RequestTimeMillis
	c.TimedUpstreams += other.TimedUpstreams
	c.UpstreamTimeMillis += other.UpstreamTimeMillis
	c.TimedUpstreamHeaders += other.TimedUpstreamHeaders
	c.UpstreamHeaderTimeMillis += other.UpstreamHeaderTimeMillis
	if other.LastSeen != nil && (c.LastSeen == nil || other.LastSeen.After(*c.LastSeen)) {
		t := *other.LastSeen
		c.LastSeen = &t
	}
}

type Bucket struct {
	Time        int64                     `json:"time"`
	Requests    float64                   `json:"requests"`
	Seconds     float64                   `json:"seconds"`
	Connections float64                   `json:"connections"`
	LogSeconds  float64                   `json:"log_seconds"`
	Counts      Counts                    `json:"counts"`
	Rules       map[string]Counts         `json:"rules,omitempty"`
	Analysis    map[string]analysisBucket `json:"analysis,omitempty"`
}

type checkpoint struct {
	LimitRecent []RequestSample   `json:"limit_recent,omitempty"`
	Version     int               `json:"version"`
	Since       time.Time         `json:"since"`
	Buckets     map[int64]*Bucket `json:"buckets"`
	Cursor      cursor            `json:"cursor"`
	Recent      []RequestSample   `json:"recent,omitempty"`
}

type Point struct {
	LimitRequestRejected            *uint64   `json:"limit_request_rejected"`
	LimitConnectionRejected         *uint64   `json:"limit_connection_rejected"`
	LimitDelayed                    *uint64   `json:"limit_delayed"`
	Time                            time.Time `json:"time"`
	RPS                             *float64  `json:"rps"`
	ResponseRPS                     *float64  `json:"response_rps"`
	Connections                     *float64  `json:"connections"`
	ErrorRate                       *float64  `json:"error_rate"`
	ClientErrorRate                 *float64  `json:"client_error_rate"`
	ServerErrorRate                 *float64  `json:"server_error_rate"`
	Requests                        *uint64   `json:"requests"`
	AverageRequestTimeMillis        *float64  `json:"average_request_time_ms"`
	AverageUpstreamHeaderTimeMillis *float64  `json:"average_upstream_header_time_ms"`
	AverageUpstreamTimeMillis       *float64  `json:"average_upstream_time_ms"`
}

type Result struct {
	Since           time.Time         `json:"since"`
	SampledAt       *time.Time        `json:"sampled_at"`
	Available       bool              `json:"available"`
	Logging         bool              `json:"logging"`
	Issue           string            `json:"issue,omitempty"`
	HistoryIssue    string            `json:"history_issue,omitempty"`
	RPS             *float64          `json:"rps"`
	ResponseRPS     *float64          `json:"response_rps"`
	Connections     *int              `json:"connections"`
	Counts          Counts            `json:"counts"`
	ErrorRate       *float64          `json:"error_rate"`
	ClientErrorRate *float64          `json:"client_error_rate"`
	ServerErrorRate *float64          `json:"server_error_rate"`
	ObservedSeconds float64           `json:"observed_seconds"`
	Rules           map[string]Counts `json:"rules"`
	Points          []Point           `json:"points"`
	Analysis        RequestAnalysis   `json:"analysis"`
}

type Collector struct {
	mu                   sync.Mutex
	data                 checkpoint
	logPath, historyPath string
	previous             *Sample
	previousAt           time.Time
	latest               *Sample
	sampledAt            *time.Time
	rps                  *float64
	responseRPS          *float64
	responseRules        map[string]float64
	intervalResponses    uint64
	intervalRules        map[string]uint64
	logging              bool
	issue, historyIssue  string
	lastSave             time.Time
}

func New(logPath, historyPath string, now time.Time) *Collector {
	c := &Collector{logPath: logPath, historyPath: historyPath, data: checkpoint{Version: checkpointVersion, Since: now, Buckets: map[int64]*Bucket{}}}
	data, err := os.ReadFile(historyPath)
	if err == nil {
		var saved checkpoint
		if json.Unmarshal(data, &saved) == nil && (saved.Version == 1 || saved.Version == checkpointVersion) && saved.Buckets != nil && !saved.Since.IsZero() {
			if saved.Version == 1 {
				migrateCheckpointV1(&saved)
			}
			c.data = saved
			c.prune(now)
			return c
		}
		c.historyIssue = "历史数据无法读取，本次从当前时间重新采集"
	} else if !errors.Is(err, os.ErrNotExist) {
		c.historyIssue = "历史数据无法读取，本次从当前时间重新采集"
	}
	// First installation starts now; existing log lines are not synthetic historical coverage.
	if info, err := os.Stat(logPath); err == nil {
		c.data.Cursor = cursor{ID: fileID(info), Offset: info.Size()}
	}
	return c
}

func migrateCheckpointV1(saved *checkpoint) {
	// Version 1 counted upgraded WebSocket/SSE connection lifetimes as HTTP
	// latency. Status and request totals remain valid, but latency aggregates in
	// a scope containing a 101 cannot be separated after the fact.
	contaminated := map[int64]map[string]bool{}
	mark := func(minute int64, rule string) {
		if contaminated[minute] == nil {
			contaminated[minute] = map[string]bool{}
		}
		contaminated[minute][""] = true
		if rule != "" && rule != "default" {
			contaminated[minute][rule] = true
		}
	}
	for minute, bucket := range saved.Buckets {
		for rule, analysis := range bucket.Analysis {
			if analysis.StatusCodes["101"] > 0 {
				mark(minute, rule)
			}
		}
	}
	for _, sample := range saved.Recent {
		if sample.Status == 101 {
			mark(sample.Time.Unix()/60*60, sample.Rule)
		}
	}
	for minute, scopes := range contaminated {
		bucket := saved.Buckets[minute]
		if bucket == nil {
			continue
		}
		if scopes[""] {
			clearCountsLatency(&bucket.Counts)
		}
		for rule := range scopes {
			if rule == "" {
				continue
			}
			counts := bucket.Rules[rule]
			clearCountsLatency(&counts)
			bucket.Rules[rule] = counts
		}
		for rule, analysis := range bucket.Analysis {
			if scopes[rule] {
				clearAnalysisLatency(&analysis)
				bucket.Analysis[rule] = analysis
			}
		}
	}
	saved.Version = checkpointVersion
}

func clearCountsLatency(counts *Counts) {
	counts.TimedRequests = 0
	counts.RequestTimeMillis = 0
	counts.TimedUpstreams = 0
	counts.UpstreamTimeMillis = 0
	counts.TimedUpstreamHeaders = 0
	counts.UpstreamHeaderTimeMillis = 0
}

func clearAnalysisLatency(analysis *analysisBucket) {
	clearAnalysisCountLatency(&analysis.Counts)
	analysis.Slow = 0
	for key, count := range analysis.Paths {
		clearAnalysisCountLatency(&count)
		analysis.Paths[key] = count
	}
	for key, count := range analysis.Backends {
		clearAnalysisCountLatency(&count)
		analysis.Backends[key] = count
	}
}

func clearAnalysisCountLatency(count *AnalysisCount) {
	count.TimedRequests = 0
	count.RequestTimeMillis = 0
	count.TimedUpstreams = 0
	count.UpstreamTimeMillis = 0
	count.TimedUpstreamHeaders = 0
	count.UpstreamHeaderMillis = 0
	count.MaxRequestTimeMillis = 0
}

func (c *Collector) bucket(minute int64) *Bucket {
	b := c.data.Buckets[minute]
	if b == nil {
		b = &Bucket{Time: minute, Rules: map[string]Counts{}, Analysis: map[string]analysisBucket{}}
		c.data.Buckets[minute] = b
	}
	if b.Rules == nil {
		b.Rules = map[string]Counts{}
	}
	if b.Analysis == nil {
		b.Analysis = map[string]analysisBucket{}
	}
	return b
}

func (c *Collector) prune(now time.Time) {
	cutoff := now.Add(-Retention).Unix() / 60 * 60
	for minute := range c.data.Buckets {
		if minute < cutoff {
			delete(c.data.Buckets, minute)
		}
	}
	recent := c.data.Recent[:0]
	for _, sample := range c.data.Recent {
		if !sample.Time.Before(now.Add(-Retention)) {
			recent = append(recent, sample)
		}
	}
	c.data.Recent = recent
	limits := c.data.LimitRecent[:0]
	for _, sample := range c.data.LimitRecent {
		if !sample.Time.Before(now.Add(-Retention)) {
			limits = append(limits, sample)
		}
	}
	c.data.LimitRecent = limits
}

// Collect is called by the service, independent of dashboard clients.
func (c *Collector) Collect(now time.Time, sample *Sample, logging bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune(now)
	c.latest, c.rps, c.responseRPS = sample, nil, nil
	c.responseRules = nil
	c.intervalResponses, c.intervalRules = 0, map[string]uint64{}
	t := now
	c.sampledAt = &t
	seconds := now.Sub(c.previousAt).Seconds()
	continuous := seconds > 0 && seconds <= 15
	c.issue = ""
	logOK := false
	if logging {
		if err := c.readLogs(now); err != nil {
			c.issue = err.Error()
		} else {
			logOK = true
		}
	}
	if sample != nil && c.previous != nil && sample.PID == c.previous.PID && sample.Requests >= c.previous.Requests && continuous {
		delta := sample.Requests - c.previous.Requests
		// One request per successful interval belongs to our private status poll.
		if delta > 0 {
			delta--
		}
		rate := float64(delta) / seconds
		c.rps = &rate
		c.distribute(c.previousAt, now, func(b *Bucket, duration float64) {
			b.Requests += rate * duration
			b.Seconds += duration
			b.Connections += float64(sample.Connections) * duration
		})
	}
	if sample != nil && c.previous != nil && continuous && logging && c.logging && logOK {
		c.distribute(c.previousAt, now, func(b *Bucket, duration float64) { b.LogSeconds += duration })
		if sample.PID == c.previous.PID {
			// Only consecutive, fully consumed polls form a live rate. Buffered
			// access logs may arrive slightly after completion; recovery/backlog
			// reads must never appear as a burst of current responses.
			rate := float64(c.intervalResponses) / seconds
			c.responseRPS = &rate
			c.responseRules = make(map[string]float64, len(c.intervalRules))
			for id, count := range c.intervalRules {
				c.responseRules[id] = float64(count) / seconds
			}
		}
	}
	c.logging = logging && logOK
	c.previous, c.previousAt = sample, now
	if c.lastSave.IsZero() || now.Sub(c.lastSave) >= time.Minute {
		c.save(now)
	}
}

func (c *Collector) distribute(from, to time.Time, add func(*Bucket, float64)) {
	for from.Before(to) {
		minute := from.Unix() / 60 * 60
		end := time.Unix(minute+60, 0)
		if end.After(to) {
			end = to
		}
		add(c.bucket(minute), end.Sub(from).Seconds())
		from = end
	}
}

func (c *Collector) save(now time.Time) {
	data, err := json.Marshal(c.data)
	if err == nil {
		err = fileutil.WriteFileAtomic(c.historyPath, data, 0o600)
	}
	if err != nil {
		c.historyIssue = "历史统计保存失败，当前数据仅保留在内存"
	} else {
		c.historyIssue = ""
		c.lastSave = now
	}
}

func (c *Collector) Flush(now time.Time) { c.mu.Lock(); defer c.mu.Unlock(); c.save(now) }

func ratio(errors, requests uint64) *float64 {
	if requests == 0 {
		return nil
	}
	v := float64(errors) / float64(requests) * 100
	return &v
}

func errorRates(counts Counts) (total, client, server *float64) {
	server = ratio(counts.Errors, counts.Requests)
	if counts.ClientErrors != nil {
		client = ratio(*counts.ClientErrors, counts.Requests)
		total = ratio(counts.Errors+*counts.ClientErrors, counts.Requests)
	}
	return
}

func (c *Collector) Snapshot(now time.Time, minutes int, ruleID string) Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !ValidRange(minutes) {
		minutes = 60
	}
	r := Result{Since: c.data.Since, SampledAt: c.sampledAt, Logging: c.logging, Issue: c.issue, HistoryIssue: c.historyIssue, Rules: map[string]Counts{}, Points: []Point{}}
	fresh := c.sampledAt != nil && now.Sub(*c.sampledAt) <= 15*time.Second
	if fresh && c.latest != nil {
		r.Available = true
		v := c.latest.Connections
		r.Connections = &v
		r.RPS = c.rps
		r.ResponseRPS = c.responseRPS
		if ruleID != "" && c.responseRPS != nil {
			v := c.responseRules[ruleID]
			r.ResponseRPS = &v
		}
	}
	end := now.Unix() / 60 * 60
	start := end - int64(minutes-1)*60
	var analysis analysisBucket
	analysis.ensure()
	step := int64(60)
	switch minutes {
	case 1440:
		step = 300
	case 10080:
		step = 3600
	case 43200:
		step = 10800
	}
	for from := start; from <= end; from += step {
		point := Point{Time: time.Unix(from, 0).UTC()}
		var requests, seconds, connections, logSeconds float64
		var counts Counts
		var pointAnalysis analysisBucket
		pointAnalysis.ensure()
		for minute := from; minute < from+step && minute <= end; minute += 60 {
			b := c.data.Buckets[minute]
			if b == nil {
				continue
			}
			requests += b.Requests
			seconds += b.Seconds
			connections += b.Connections
			logSeconds += b.LogSeconds
			if ruleID == "" {
				counts.add(b.Counts)
			} else {
				counts.add(b.Rules[ruleID])
			}
			for id, value := range b.Rules {
				total := r.Rules[id]
				total.add(value)
				r.Rules[id] = total
			}
			pointAnalysis.add(b.Analysis[ruleID])
			analysis.add(b.Analysis[ruleID])
		}
		if seconds > 0 && ruleID == "" {
			rate, conn := requests/seconds, connections/seconds
			point.RPS, point.Connections = &rate, &conn
		}
		if logSeconds > 0 || counts.Requests > 0 {
			v := counts.Requests
			point.Requests = &v
			point.ErrorRate, point.ClientErrorRate, point.ServerErrorRate = errorRates(counts)
			if logSeconds > 0 {
				rate := float64(counts.Requests) / logSeconds
				point.ResponseRPS = &rate
			}
			if ruleID != "" && logSeconds > 0 {
				rate := float64(counts.Requests) / logSeconds
				point.RPS = &rate
			}
			if pointAnalysis.Limits.Covered > 0 {
				v := pointAnalysis.Limits
				point.LimitRequestRejected = &v.RequestRejected
				point.LimitConnectionRejected = &v.ConnectionRejected
				point.LimitDelayed = &v.Delayed
			}
			point.AverageRequestTimeMillis = pointAnalysis.Counts.averageRequestTime()
			point.AverageUpstreamHeaderTimeMillis = pointAnalysis.Counts.averageUpstreamHeaderTime()
			point.AverageUpstreamTimeMillis = pointAnalysis.Counts.averageUpstreamTime()
		}
		r.Counts.add(counts)
		r.ObservedSeconds += logSeconds
		r.Points = append(r.Points, point)
	}
	r.ErrorRate, r.ClientErrorRate, r.ServerErrorRate = errorRates(r.Counts)
	recent := make([]RequestSample, 0, len(c.data.Recent))
	cutoff := now.Add(-time.Duration(minutes) * time.Minute)
	for i := len(c.data.Recent) - 1; i >= 0; i-- {
		sample := c.data.Recent[i]
		if sample.Time.Before(cutoff) || (ruleID != "" && sample.Rule != ruleID) {
			continue
		}
		recent = append(recent, sample)
		if len(recent) >= 50 {
			break
		}
	}
	r.Analysis = buildRequestAnalysis(analysis, recent)
	r.Analysis.Limits.Total = r.Counts.Requests
	r.Analysis.Limits.Recent = []RequestSample{}
	for i := len(c.data.LimitRecent) - 1; i >= 0; i-- {
		sample := c.data.LimitRecent[i]
		if sample.Time.Before(cutoff) || (ruleID != "" && sample.Rule != ruleID) {
			continue
		}
		r.Analysis.Limits.Recent = append(r.Analysis.Limits.Recent, sample)
		if len(r.Analysis.Limits.Recent) >= 50 {
			break
		}
	}
	if ruleID != "" {
		r.Connections, r.RPS = nil, nil
		if r.ObservedSeconds > 0 {
			v := float64(r.Counts.Requests) / r.ObservedSeconds
			r.RPS = &v
		}
	}
	return r
}

type logRecord struct {
	LimitRequest       string                    `json:"limit_req_status"`
	LimitConnection    string                    `json:"limit_conn_status"`
	LimitPolicy        string                    `json:"limit_policy"`
	HasLimitStatus     bool                      `json:"-"`
	Policy             *domain.RateLimitSnapshot `json:"-"`
	Time               float64                   `json:"time"`
	Rule               string                    `json:"rule"`
	Status             int                       `json:"status"`
	Bytes              uint64                    `json:"bytes"`
	Method             string                    `json:"method"`
	URI                string                    `json:"uri"`
	RequestTime        float64                   `json:"request_time"`
	HasRequestTime     bool                      `json:"-"`
	Upstream           string                    `json:"upstream"`
	UpstreamStatus     string                    `json:"upstream_status"`
	UpstreamHeaderTime string                    `json:"upstream_header_time"`
	UpstreamTime       string                    `json:"upstream_time"`
}

func (c *Collector) ingest(line []byte, now time.Time) {
	var record logRecord
	var raw map[string]json.RawMessage
	if json.Unmarshal(line, &raw) != nil || json.Unmarshal(line, &record) != nil || record.Time <= 0 || math.IsNaN(record.Time) || record.Status < 100 || record.Status > 599 || len(record.Rule) > 80 || record.Rule == "" {
		return
	}
	_, record.HasRequestTime = raw["request_time"]
	_, hasReq := raw["limit_req_status"]
	_, hasConn := raw["limit_conn_status"]
	record.HasLimitStatus = hasReq && hasConn && string(raw["limit_req_status"]) != "null" && string(raw["limit_conn_status"]) != "null" && validLimitStatus(record.LimitRequest, true) && validLimitStatus(record.LimitConnection, false)
	if record.HasLimitStatus {
		record.Policy = parseLimitPolicy(record.LimitPolicy)
	} else {
		record.LimitRequest = ""
		record.LimitConnection = ""
	}
	record.Method = strings.ToUpper(strings.TrimSpace(record.Method))
	if len(record.Method) > 16 {
		record.Method = record.Method[:16]
	}
	record.URI = strings.TrimSpace(record.URI)
	if len(record.URI) > 512 {
		record.URI = record.URI[:512]
	}
	record.Upstream = strings.TrimSpace(record.Upstream)
	if len(record.Upstream) > 256 {
		record.Upstream = record.Upstream[:256]
	}
	t := time.UnixMilli(int64(record.Time * 1000)).UTC()
	if t.Before(now.Add(-Retention)) || t.After(now.Add(time.Minute)) || t.Before(c.data.Since) {
		return
	}
	clientErrors := uint64(0)
	if record.Status >= 400 && record.Status < 500 {
		clientErrors = 1
	}
	count := Counts{Requests: 1, ClientErrors: &clientErrors, Bytes: record.Bytes, LastSeen: &t}
	detail := analysisCount(record)
	count.TimedRequests, count.RequestTimeMillis = detail.TimedRequests, detail.RequestTimeMillis
	count.TimedUpstreams, count.UpstreamTimeMillis = detail.TimedUpstreams, detail.UpstreamTimeMillis
	count.TimedUpstreamHeaders, count.UpstreamHeaderTimeMillis = detail.TimedUpstreamHeaders, detail.UpstreamHeaderMillis
	if record.Status >= 500 {
		count.Errors = 1
	}
	b := c.bucket(t.Unix() / 60 * 60)
	b.Counts.add(count)
	global := b.Analysis[""]
	global.ingest(record)
	b.Analysis[""] = global
	c.intervalResponses++
	// The default virtual host contributes to totals, but never to an unrelated rule.
	if record.Rule != "default" {
		if _, found := b.Rules[record.Rule]; !found && len(b.Rules) >= 4096 {
			return
		}
		value := b.Rules[record.Rule]
		value.add(count)
		b.Rules[record.Rule] = value
		scoped := b.Analysis[record.Rule]
		scoped.ingest(record)
		b.Analysis[record.Rule] = scoped
		c.intervalRules[record.Rule]++
	}
	if limitCounts(record).Rejected > 0 || limitCounts(record).Delayed > 0 || record.Status == 101 || record.Status >= 400 || (record.HasRequestTime && record.RequestTime*1000 >= slowRequestMillis) {
		requestMillis := rawRequestTimeMillis(record)
		upstreamHeaderMillis := rawUpstreamTimeMillis(record.UpstreamHeaderTime)
		upstreamMillis := rawUpstreamTimeMillis(record.UpstreamTime)
		c.data.Recent = append(c.data.Recent, RequestSample{LimitRequest: record.LimitRequest, LimitConnection: record.LimitConnection, Policy: record.Policy, Time: t, Rule: record.Rule, Method: record.Method, URI: record.URI, Status: record.Status, Bytes: record.Bytes, RequestTimeMillis: requestMillis, Upstream: record.Upstream, UpstreamStatus: record.UpstreamStatus, UpstreamHeaderTime: upstreamHeaderMillis, UpstreamTimeMillis: upstreamMillis})
		if limitCounts(record).Rejected+limitCounts(record).Delayed > 0 {
			c.data.LimitRecent = append(c.data.LimitRecent, c.data.Recent[len(c.data.Recent)-1])
			if len(c.data.LimitRecent) > maxRecentRequests {
				c.data.LimitRecent = append([]RequestSample(nil), c.data.LimitRecent[len(c.data.LimitRecent)-maxRecentRequests:]...)
			}
		}
		if len(c.data.Recent) > maxRecentRequests {
			c.data.Recent = append([]RequestSample(nil), c.data.Recent[len(c.data.Recent)-maxRecentRequests:]...)
		}
	}
}

func rawRequestTimeMillis(record logRecord) *float64 {
	if !record.HasRequestTime || record.RequestTime < 0 {
		return nil
	}
	value := record.RequestTime * 1000
	return &value
}

func rawUpstreamTimeMillis(raw string) *float64 {
	value, ok := parseUpstreamTime(raw)
	if !ok {
		return nil
	}
	value *= 1000
	return &value
}

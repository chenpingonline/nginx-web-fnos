// Package metrics collects bounded, minute-resolution HTTP history without an external database.
package metrics

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"sync"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

const Retention = 30 * 24 * time.Hour

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
	Requests     uint64     `json:"requests"`
	Errors       uint64     `json:"errors"`        // Server errors (5xx); retained for checkpoint compatibility.
	ClientErrors *uint64    `json:"client_errors"` // Nil when requests include history collected before 4xx tracking.
	Bytes        uint64     `json:"bytes"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
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
	if other.LastSeen != nil && (c.LastSeen == nil || other.LastSeen.After(*c.LastSeen)) {
		t := *other.LastSeen
		c.LastSeen = &t
	}
}

type Bucket struct {
	Time        int64             `json:"time"`
	Requests    float64           `json:"requests"`
	Seconds     float64           `json:"seconds"`
	Connections float64           `json:"connections"`
	LogSeconds  float64           `json:"log_seconds"`
	Counts      Counts            `json:"counts"`
	Rules       map[string]Counts `json:"rules,omitempty"`
}

type checkpoint struct {
	Version int               `json:"version"`
	Since   time.Time         `json:"since"`
	Buckets map[int64]*Bucket `json:"buckets"`
	Cursor  cursor            `json:"cursor"`
}

type Point struct {
	Time            time.Time `json:"time"`
	RPS             *float64  `json:"rps"`
	ResponseRPS     *float64  `json:"response_rps"`
	Connections     *float64  `json:"connections"`
	ErrorRate       *float64  `json:"error_rate"`
	ClientErrorRate *float64  `json:"client_error_rate"`
	ServerErrorRate *float64  `json:"server_error_rate"`
	Requests        *uint64   `json:"requests"`
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
	c := &Collector{logPath: logPath, historyPath: historyPath, data: checkpoint{Version: 1, Since: now, Buckets: map[int64]*Bucket{}}}
	data, err := os.ReadFile(historyPath)
	if err == nil {
		var saved checkpoint
		if json.Unmarshal(data, &saved) == nil && saved.Version == 1 && saved.Buckets != nil && !saved.Since.IsZero() {
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

func (c *Collector) bucket(minute int64) *Bucket {
	b := c.data.Buckets[minute]
	if b == nil {
		b = &Bucket{Time: minute, Rules: map[string]Counts{}}
		c.data.Buckets[minute] = b
	}
	if b.Rules == nil {
		b.Rules = map[string]Counts{}
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
		}
		r.Counts.add(counts)
		r.ObservedSeconds += logSeconds
		r.Points = append(r.Points, point)
	}
	r.ErrorRate, r.ClientErrorRate, r.ServerErrorRate = errorRates(r.Counts)
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
	Time   float64 `json:"time"`
	Rule   string  `json:"rule"`
	Status int     `json:"status"`
	Bytes  uint64  `json:"bytes"`
}

func (c *Collector) ingest(line []byte, now time.Time) {
	var record logRecord
	if json.Unmarshal(line, &record) != nil || record.Time <= 0 || math.IsNaN(record.Time) || record.Status < 100 || record.Status > 599 || len(record.Rule) > 80 || record.Rule == "" {
		return
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
	if record.Status >= 500 {
		count.Errors = 1
	}
	b := c.bucket(t.Unix() / 60 * 60)
	b.Counts.add(count)
	c.intervalResponses++
	// The default virtual host contributes to totals, but never to an unrelated rule.
	if record.Rule != "default" {
		if _, found := b.Rules[record.Rule]; !found && len(b.Rules) >= 4096 {
			return
		}
		value := b.Rules[record.Rule]
		value.add(count)
		b.Rules[record.Rule] = value
		c.intervalRules[record.Rule]++
	}
}

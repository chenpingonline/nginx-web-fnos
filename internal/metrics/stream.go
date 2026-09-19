package metrics

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/fileutil"
)

type StreamCounts struct {
	Sessions uint64  `json:"sessions"`
	Errors   uint64  `json:"errors"`
	Rejected uint64  `json:"rejected"`
	Sent     uint64  `json:"sent"`
	Received uint64  `json:"received"`
	Duration float64 `json:"duration"`
}

func (c *StreamCounts) add(v StreamCounts) {
	c.Sessions += v.Sessions
	c.Errors += v.Errors
	c.Rejected += v.Rejected
	c.Sent += v.Sent
	c.Received += v.Received
	c.Duration += v.Duration
}

type StreamSample struct {
	Time     time.Time `json:"time"`
	Rule     string    `json:"rule"`
	Protocol string    `json:"protocol"`
	Status   int       `json:"status"`
	Sent     uint64    `json:"sent"`
	Received uint64    `json:"received"`
	Duration float64   `json:"duration"`
	Client   string    `json:"client"`
	Upstream string    `json:"upstream"`
	Limit    string    `json:"limit"`
}
type StreamRank struct {
	StreamCounts
	Rule     string `json:"rule"`
	Protocol string `json:"protocol"`
}
type StreamPoint struct {
	StreamCounts
	Time time.Time `json:"time"`
}
type StreamResult struct {
	StreamCounts
	ActiveConnections *uint64        `json:"active_connections"`
	Since             time.Time      `json:"since"`
	Ready             bool           `json:"ready"`
	LoggingRules      []string       `json:"logging_rules"`
	Issue             string         `json:"issue,omitempty"`
	Rules             []StreamRank   `json:"rules"`
	Recent            []StreamSample `json:"recent"`
	Points            []StreamPoint  `json:"points"`
}
type streamCheckpoint struct {
	Version int                             `json:"version"`
	Since   time.Time                       `json:"since"`
	Cursor  cursor                          `json:"cursor"`
	Buckets map[int64]map[string]StreamRank `json:"buckets"`
	Recent  []StreamSample                  `json:"recent"`
}
type StreamCollector struct {
	mu                                        sync.Mutex
	data                                      streamCheckpoint
	logPath, historyPath, issue, historyIssue string
	lastSave                                  time.Time
}

func NewStream(logPath, historyPath string, now time.Time) *StreamCollector {
	c := &StreamCollector{logPath: logPath, historyPath: historyPath, data: streamCheckpoint{Version: 1, Since: now, Buckets: map[int64]map[string]StreamRank{}}}
	if b, err := os.ReadFile(historyPath); err == nil {
		var saved streamCheckpoint
		if json.Unmarshal(b, &saved) == nil && saved.Version == 1 && saved.Buckets != nil && !saved.Since.IsZero() {
			c.data = saved
			c.prune(now)
			return c
		}
		c.historyIssue = "TCP/UDP 历史统计无法读取，已重新开始采集"
	} else if !os.IsNotExist(err) {
		c.historyIssue = "TCP/UDP 历史统计无法读取，已重新开始采集"
	}
	if info, err := os.Stat(logPath); err == nil {
		c.data.Cursor = cursor{ID: fileID(info), Offset: info.Size()}
	}
	return c
}
func (c *StreamCollector) ingest(line []byte, now time.Time) {
	var v struct {
		StreamSample
		Timestamp float64 `json:"time"`
	}
	if json.Unmarshal(line, &v) != nil || math.IsNaN(v.Timestamp) || math.IsInf(v.Timestamp, 0) || v.Timestamp <= 0 || v.Rule == "" || len(v.Rule) > 128 || (v.Protocol != "TCP" && v.Protocol != "UDP") || v.Status < 200 || v.Status > 599 || v.Duration < 0 || math.IsNaN(v.Duration) || math.IsInf(v.Duration, 0) {
		return
	}
	v.StreamSample.Time = time.UnixMilli(int64(math.Round(v.Timestamp * 1000)))
	if v.Time.Before(c.data.Since) || v.Time.Before(now.Add(-Retention)) || v.Time.After(now.Add(time.Minute)) {
		return
	}
	minute := v.Time.Unix() / 60 * 60
	if c.data.Buckets[minute] == nil {
		c.data.Buckets[minute] = map[string]StreamRank{}
	}
	key := v.Rule + "/" + v.Protocol
	rank := c.data.Buckets[minute][key]
	rank.Rule = v.Rule
	rank.Protocol = v.Protocol
	count := StreamCounts{Sessions: 1, Sent: v.Sent, Received: v.Received, Duration: v.Duration}
	if v.Status >= 400 {
		count.Errors = 1
	}
	if v.Limit == "REJECTED" {
		count.Rejected = 1
	}
	rank.add(count)
	c.data.Buckets[minute][key] = rank
	c.data.Recent = append(c.data.Recent, v.StreamSample)
	if len(c.data.Recent) > 200 {
		c.data.Recent = c.data.Recent[len(c.data.Recent)-200:]
	}
}
func (c *StreamCollector) prune(now time.Time) {
	cutoff := now.Add(-Retention)
	for m := range c.data.Buckets {
		if m < cutoff.Unix()/60*60 {
			delete(c.data.Buckets, m)
		}
	}
	recent := c.data.Recent[:0]
	for _, s := range c.data.Recent {
		if !s.Time.Before(cutoff) {
			recent = append(recent, s)
		}
	}
	c.data.Recent = recent
}
func (c *StreamCollector) save(now time.Time) {
	b, err := json.Marshal(c.data)
	if err == nil {
		err = fileutil.WriteFileAtomic(c.historyPath, b, 0600)
	}
	if err != nil {
		c.historyIssue = "TCP/UDP 历史保存失败，数据暂存内存"
	} else {
		c.historyIssue = ""
		c.lastSave = now
	}
}
func (c *StreamCollector) Collect(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prune(now)
	c.issue = ""
	if err := readMetricLogs(c.logPath, &c.data.Cursor, func(line []byte) { c.ingest(line, now) }); err != nil {
		c.issue = err.Error()
	}
	if c.lastSave.IsZero() || now.Sub(c.lastSave) >= time.Minute {
		c.save(now)
	}
}
func (c *StreamCollector) Flush(now time.Time) { c.mu.Lock(); defer c.mu.Unlock(); c.save(now) }
func (c *StreamCollector) Snapshot(now time.Time, minutes int, rule string) StreamResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !ValidRange(minutes) {
		minutes = 60
	}
	r := StreamResult{Since: c.data.Since, Issue: c.issue, Rules: []StreamRank{}, Recent: []StreamSample{}, Points: []StreamPoint{}, LoggingRules: []string{}}
	if c.historyIssue != "" {
		r.Issue = c.historyIssue
	}
	end := now.Unix() / 60 * 60
	start := end - int64(minutes-1)*60
	step := int64(60)
	if minutes >= 43200 {
		step = 21600
	} else if minutes >= 10080 {
		step = 3600
	} else if minutes >= 1440 {
		step = 300
	}
	ranks := map[string]StreamRank{}
	points := map[int64]StreamCounts{}
	for minute, b := range c.data.Buckets {
		if minute < start || minute > end {
			continue
		}
		for key, v := range b {
			if rule != "" && v.Rule != rule {
				continue
			}
			r.add(v.StreamCounts)
			rank := ranks[key]
			rank.Rule = v.Rule
			rank.Protocol = v.Protocol
			rank.add(v.StreamCounts)
			ranks[key] = rank
			t := start + (minute-start)/step*step
			p := points[t]
			p.add(v.StreamCounts)
			points[t] = p
		}
	}
	for _, rank := range ranks {
		r.Rules = append(r.Rules, rank)
	}
	sort.Slice(r.Rules, func(i, j int) bool {
		if r.Rules[i].Sessions == r.Rules[j].Sessions {
			return r.Rules[i].Rule < r.Rules[j].Rule
		}
		return r.Rules[i].Sessions > r.Rules[j].Sessions
	})
	for t := start; t <= end; t += step {
		if t+step <= c.data.Since.Unix() {
			continue
		}
		r.Points = append(r.Points, StreamPoint{Time: time.Unix(t, 0), StreamCounts: points[t]})
	}
	for _, s := range c.data.Recent {
		if s.Time.Unix() >= start && !s.Time.After(now) && (rule == "" || s.Rule == rule) {
			r.Recent = append(r.Recent, s)
		}
	}
	sort.SliceStable(r.Recent, func(i, j int) bool { return r.Recent[i].Time.After(r.Recent[j].Time) })
	if len(r.Recent) > 50 {
		r.Recent = r.Recent[:50]
	}
	return r
}

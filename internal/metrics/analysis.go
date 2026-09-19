package metrics

import (
	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxAnalysisPaths    = 128
	maxAnalysisBackends = 64
	maxRecentRequests   = 200
	slowRequestMillis   = 1000
)

type AnalysisCount struct {
	Requests             uint64  `json:"requests"`
	ClientErrors         uint64  `json:"client_errors"`
	Errors               uint64  `json:"errors"`
	Bytes                uint64  `json:"bytes"`
	TimedRequests        uint64  `json:"timed_requests"`
	RequestTimeMillis    float64 `json:"request_time_ms"`
	TimedUpstreams       uint64  `json:"timed_upstreams"`
	UpstreamTimeMillis   float64 `json:"upstream_time_ms"`
	TimedUpstreamHeaders uint64  `json:"timed_upstream_headers"`
	UpstreamHeaderMillis float64 `json:"upstream_header_time_ms"`
	MaxRequestTimeMillis float64 `json:"max_request_time_ms"`
}

func (c *AnalysisCount) add(other AnalysisCount) {
	c.Requests += other.Requests
	c.ClientErrors += other.ClientErrors
	c.Errors += other.Errors
	c.Bytes += other.Bytes
	c.TimedRequests += other.TimedRequests
	c.RequestTimeMillis += other.RequestTimeMillis
	c.TimedUpstreams += other.TimedUpstreams
	c.UpstreamTimeMillis += other.UpstreamTimeMillis
	c.TimedUpstreamHeaders += other.TimedUpstreamHeaders
	c.UpstreamHeaderMillis += other.UpstreamHeaderMillis
	if other.MaxRequestTimeMillis > c.MaxRequestTimeMillis {
		c.MaxRequestTimeMillis = other.MaxRequestTimeMillis
	}
}

func (c AnalysisCount) averageRequestTime() *float64 {
	if c.TimedRequests == 0 {
		return nil
	}
	value := c.RequestTimeMillis / float64(c.TimedRequests)
	return &value
}

func (c AnalysisCount) averageUpstreamTime() *float64 {
	if c.TimedUpstreams == 0 {
		return nil
	}
	value := c.UpstreamTimeMillis / float64(c.TimedUpstreams)
	return &value
}

func (c AnalysisCount) averageUpstreamHeaderTime() *float64 {
	if c.TimedUpstreamHeaders == 0 {
		return nil
	}
	value := c.UpstreamHeaderMillis / float64(c.TimedUpstreamHeaders)
	return &value
}

type analysisBucket struct {
	Limits      LimitCounts              `json:"limits"`
	LimitRules  map[string]LimitRank     `json:"limit_rules,omitempty"`
	Counts      AnalysisCount            `json:"counts"`
	Slow        uint64                   `json:"slow"`
	StatusCodes map[string]uint64        `json:"status_codes,omitempty"`
	Methods     map[string]uint64        `json:"methods,omitempty"`
	Paths       map[string]AnalysisCount `json:"paths,omitempty"`
	Backends    map[string]AnalysisCount `json:"backends,omitempty"`
}

func (a *analysisBucket) ensure() {
	if a.LimitRules == nil {
		a.LimitRules = map[string]LimitRank{}
	}
	if a.StatusCodes == nil {
		a.StatusCodes = map[string]uint64{}
	}
	if a.Methods == nil {
		a.Methods = map[string]uint64{}
	}
	if a.Paths == nil {
		a.Paths = map[string]AnalysisCount{}
	}
	if a.Backends == nil {
		a.Backends = map[string]AnalysisCount{}
	}
}

func (a *analysisBucket) add(other analysisBucket) {
	a.ensure()
	a.Counts.add(other.Counts)
	a.Limits.add(other.Limits)
	for key, value := range other.LimitRules {
		row := a.LimitRules[key]
		row.add(value)
		a.LimitRules[key] = row
	}
	a.Slow += other.Slow
	for key, value := range other.StatusCodes {
		a.StatusCodes[key] += value
	}
	for key, value := range other.Methods {
		a.Methods[key] += value
	}
	for key, value := range other.Paths {
		current := a.Paths[key]
		current.add(value)
		a.Paths[key] = current
	}
	for key, value := range other.Backends {
		current := a.Backends[key]
		current.add(value)
		a.Backends[key] = current
	}
}

func (a *analysisBucket) ingest(record logRecord) {
	a.ensure()
	count := analysisCount(record)
	limits := limitCounts(record)
	a.Limits.add(limits)
	if limits.Rejected+limits.Delayed > 0 {
		if _, found := a.LimitRules[record.Rule]; found || len(a.LimitRules) < 4096 {
			row := a.LimitRules[record.Rule]
			row.add(LimitRank{LimitCounts: limits, Rule: record.Rule, Policy: record.Policy, LastSeen: time.UnixMilli(int64(record.Time * 1000)).UTC()})
			a.LimitRules[record.Rule] = row
		}
	}
	a.Counts.add(count)
	a.StatusCodes[strconv.Itoa(record.Status)]++
	if record.Method != "" {
		a.Methods[record.Method]++
	}
	if record.Status != 101 && record.RequestTime*1000 >= slowRequestMillis {
		a.Slow++
	}
	addRankedCount(a.Paths, record.URI, count, maxAnalysisPaths)
	addRankedCount(a.Backends, record.Upstream, count, maxAnalysisBackends)
}

func analysisCount(record logRecord) AnalysisCount {
	count := AnalysisCount{Requests: 1, Bytes: record.Bytes}
	if record.Status >= 400 && record.Status < 500 {
		count.ClientErrors = 1
	}
	if record.Status >= 500 {
		count.Errors = 1
	}
	// A 101 entry measures the lifetime of an upgraded connection, not HTTP
	// response latency. Keep it in status/path totals, but exclude it from
	// latency averages, maxima, and slow-request counts.
	if record.Status == 101 {
		return count
	}
	if record.RequestTime >= 0 && record.HasRequestTime {
		count.TimedRequests = 1
		count.RequestTimeMillis = record.RequestTime * 1000
		count.MaxRequestTimeMillis = count.RequestTimeMillis
	}
	if value, ok := parseUpstreamTime(record.UpstreamTime); ok {
		count.TimedUpstreams = 1
		count.UpstreamTimeMillis = value * 1000
	}
	if value, ok := parseUpstreamTime(record.UpstreamHeaderTime); ok {
		count.TimedUpstreamHeaders = 1
		count.UpstreamHeaderMillis = value * 1000
	}
	return count
}

func addRankedCount(target map[string]AnalysisCount, key string, count AnalysisCount, limit int) {
	key = strings.TrimSpace(key)
	if key == "" || key == "-" {
		return
	}
	if _, found := target[key]; !found && len(target) >= limit {
		return
	}
	current := target[key]
	current.add(count)
	target[key] = current
}

func parseUpstreamTime(raw string) (float64, bool) {
	var total float64
	var found bool
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ':' || r == ' '
	}) {
		value, err := strconv.ParseFloat(part, 64)
		if err == nil && value >= 0 {
			total += value
			found = true
		}
	}
	return total, found
}

type RequestSample struct {
	LimitRequest       string                    `json:"limit_req_status,omitempty"`
	LimitConnection    string                    `json:"limit_conn_status,omitempty"`
	Policy             *domain.RateLimitSnapshot `json:"limit_policy,omitempty"`
	Time               time.Time                 `json:"time"`
	Rule               string                    `json:"rule"`
	Method             string                    `json:"method,omitempty"`
	URI                string                    `json:"uri,omitempty"`
	Status             int                       `json:"status"`
	Bytes              uint64                    `json:"bytes"`
	RequestTimeMillis  *float64                  `json:"request_time_ms"`
	Upstream           string                    `json:"upstream,omitempty"`
	UpstreamStatus     string                    `json:"upstream_status,omitempty"`
	UpstreamHeaderTime *float64                  `json:"upstream_header_time_ms"`
	UpstreamTimeMillis *float64                  `json:"upstream_time_ms"`
}

type AnalysisRank struct {
	Key                             string   `json:"key"`
	Requests                        uint64   `json:"requests"`
	ClientErrors                    uint64   `json:"client_errors"`
	Errors                          uint64   `json:"errors"`
	Bytes                           uint64   `json:"bytes"`
	AverageRequestTimeMillis        *float64 `json:"average_request_time_ms"`
	AverageUpstreamHeaderTimeMillis *float64 `json:"average_upstream_header_time_ms"`
	AverageUpstreamTimeMillis       *float64 `json:"average_upstream_time_ms"`
	MaxRequestTimeMillis            *float64 `json:"max_request_time_ms"`
}

type AnalysisValue struct {
	Key   string `json:"key"`
	Count uint64 `json:"count"`
}

type RequestAnalysis struct {
	Limits                          LimitAnalysis   `json:"limits"`
	AverageRequestTimeMillis        *float64        `json:"average_request_time_ms"`
	AverageUpstreamHeaderTimeMillis *float64        `json:"average_upstream_header_time_ms"`
	AverageUpstreamTimeMillis       *float64        `json:"average_upstream_time_ms"`
	MaxRequestTimeMillis            *float64        `json:"max_request_time_ms"`
	SlowRequests                    uint64          `json:"slow_requests"`
	StatusCodes                     []AnalysisValue `json:"status_codes"`
	Methods                         []AnalysisValue `json:"methods"`
	Paths                           []AnalysisRank  `json:"paths"`
	Backends                        []AnalysisRank  `json:"backends"`
	Recent                          []RequestSample `json:"recent"`
}

func buildRequestAnalysis(bucket analysisBucket, recent []RequestSample) RequestAnalysis {
	result := RequestAnalysis{
		Limits:                          buildLimitAnalysis(bucket),
		AverageRequestTimeMillis:        bucket.Counts.averageRequestTime(),
		AverageUpstreamHeaderTimeMillis: bucket.Counts.averageUpstreamHeaderTime(),
		AverageUpstreamTimeMillis:       bucket.Counts.averageUpstreamTime(),
		SlowRequests:                    bucket.Slow,
		StatusCodes:                     sortedValues(bucket.StatusCodes),
		Methods:                         sortedValues(bucket.Methods),
		Paths:                           sortedRanks(bucket.Paths, 8),
		Backends:                        sortedRanks(bucket.Backends, 8),
		Recent:                          recent,
	}
	if bucket.Counts.TimedRequests > 0 {
		value := bucket.Counts.MaxRequestTimeMillis
		result.MaxRequestTimeMillis = &value
	}
	return result
}

func sortedValues(values map[string]uint64) []AnalysisValue {
	result := make([]AnalysisValue, 0, len(values))
	for key, count := range values {
		result = append(result, AnalysisValue{Key: key, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Key < result[j].Key
		}
		return result[i].Count > result[j].Count
	})
	return result
}

func sortedRanks(values map[string]AnalysisCount, limit int) []AnalysisRank {
	result := make([]AnalysisRank, 0, len(values))
	for key, count := range values {
		item := AnalysisRank{Key: key, Requests: count.Requests, ClientErrors: count.ClientErrors, Errors: count.Errors, Bytes: count.Bytes, AverageRequestTimeMillis: count.averageRequestTime(), AverageUpstreamHeaderTimeMillis: count.averageUpstreamHeaderTime(), AverageUpstreamTimeMillis: count.averageUpstreamTime()}
		if count.TimedRequests > 0 {
			value := count.MaxRequestTimeMillis
			item.MaxRequestTimeMillis = &value
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Requests == result[j].Requests {
			return result[i].Key < result[j].Key
		}
		return result[i].Requests > result[j].Requests
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

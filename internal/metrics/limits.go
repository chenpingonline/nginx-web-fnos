package metrics

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
)

type LimitCounts struct {
	Covered            uint64 `json:"covered"`
	Rejected           uint64 `json:"rejected"`
	RequestRejected    uint64 `json:"request_rejected"`
	ConnectionRejected uint64 `json:"connection_rejected"`
	Delayed            uint64 `json:"delayed"`
}

func (c *LimitCounts) add(v LimitCounts) {
	c.Covered += v.Covered
	c.Rejected += v.Rejected
	c.RequestRejected += v.RequestRejected
	c.ConnectionRejected += v.ConnectionRejected
	c.Delayed += v.Delayed
}
func limitCounts(r logRecord) LimitCounts {
	if !r.HasLimitStatus {
		return LimitCounts{}
	}
	c := LimitCounts{Covered: 1}
	if r.LimitRequest == "REJECTED" {
		c.RequestRejected = 1
	}
	if r.LimitConnection == "REJECTED" {
		c.ConnectionRejected = 1
	}
	if c.RequestRejected+c.ConnectionRejected > 0 {
		c.Rejected = 1
	}
	// A delayed request may subsequently fail for another reason. This is a
	// processing outcome, not proof of a successful upstream response.
	if r.LimitRequest == "DELAYED" && c.Rejected == 0 {
		c.Delayed = 1
	}
	return c
}

type LimitRank struct {
	LimitCounts
	Rule          string                    `json:"rule"`
	Policy        *domain.RateLimitSnapshot `json:"policy,omitempty"`
	PolicyChanged bool                      `json:"policy_changed"`
	LastSeen      time.Time                 `json:"last_seen"`
}

func (r *LimitRank) add(v LimitRank) {
	r.LimitCounts.add(v.LimitCounts)
	if r.Policy != nil && v.Policy != nil && *r.Policy != *v.Policy {
		r.PolicyChanged = true
	}
	r.PolicyChanged = r.PolicyChanged || v.PolicyChanged
	if v.LastSeen.After(r.LastSeen) {
		r.LastSeen = v.LastSeen
		r.Policy = v.Policy
	}
	r.Rule = v.Rule
}

type LimitAnalysis struct {
	Recent []RequestSample `json:"recent"`
	LimitCounts
	Total         uint64      `json:"total"`
	Rate          *float64    `json:"rate"`
	Rules         []LimitRank `json:"rules"`
	AffectedRules int         `json:"affected_rules"`
}

func buildLimitAnalysis(a analysisBucket) LimitAnalysis {
	result := LimitAnalysis{LimitCounts: a.Limits, Rules: make([]LimitRank, 0, len(a.LimitRules))}
	if result.Covered > 0 {
		value := float64(result.Rejected) * 100 / float64(result.Covered)
		result.Rate = &value
	}
	for _, row := range a.LimitRules {
		result.Rules = append(result.Rules, row)
	}
	result.AffectedRules = len(result.Rules)
	sort.Slice(result.Rules, func(i, j int) bool {
		a, b := result.Rules[i], result.Rules[j]
		if a.Rejected != b.Rejected {
			return a.Rejected > b.Rejected
		}
		if a.Delayed != b.Delayed {
			return a.Delayed > b.Delayed
		}
		return a.Rule < b.Rule
	})
	return result
}
func parseLimitPolicy(value string) *domain.RateLimitSnapshot {
	if len(value) > 16384 {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil
	}
	var p domain.RateLimitSnapshot
	if json.Unmarshal(data, &p) != nil || p.ID == "" {
		return nil
	}
	return &p
}
func validLimitStatus(value string, request bool) bool {
	switch value {
	case "", "-", "PASSED", "REJECTED", "REJECTED_DRY_RUN":
		return true
	case "DELAYED", "DELAYED_DRY_RUN":
		return request
	}
	return false
}

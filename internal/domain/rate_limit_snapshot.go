package domain

// RateLimitSnapshot records the applied policy, not the current editable draft.
type RateLimitSnapshot struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	RuleName string            `json:"rule_name"`
	Settings RateLimitSettings `json:"settings"`
}

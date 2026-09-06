package service

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenpingonline/nginx-web-fnos/internal/domain"
	"github.com/chenpingonline/nginx-web-fnos/internal/metrics"
)

type DashboardRule struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Protocol      string          `json:"protocol"`
	Entry         string          `json:"entry"`
	ListenAddress string          `json:"listen_address"`
	Target        string          `json:"target"`
	ConfigState   string          `json:"config_state"`
	Enabled       bool            `json:"enabled"`
	Counts        *metrics.Counts `json:"counts"`
}

type CertificateAlert struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	NotAfter  time.Time `json:"not_after"`
	NotBefore time.Time `json:"not_before"`
	Days      int       `json:"days"`
	Severity  string    `json:"severity"`
	Rules     []string  `json:"rules"`
}

type Dashboard struct {
	Overview        Overview           `json:"overview"`
	Metrics         metrics.Result     `json:"metrics"`
	MonitoringReady bool               `json:"monitoring_ready"`
	AccessLogging   bool               `json:"access_logging"`
	AppliedKnown    bool               `json:"applied_known"`
	AppliedCount    int                `json:"applied_count"`
	AppliedPorts    []int              `json:"applied_ports"`
	Rules           []DashboardRule    `json:"rules"`
	Certificates    []CertificateAlert `json:"certificates"`
}

func (s *AppService) appliedState(state State) (State, bool) {
	data, err := os.ReadFile(s.paths.AppliedState())
	var applied State
	if err == nil && json.Unmarshal(data, &applied) == nil && applied.LastAppliedAt != nil && state.LastAppliedAt != nil && applied.LastAppliedAt.Equal(*state.LastAppliedAt) {
		return applied, true
	}
	// Upgrade path: successful revisions already contain the last applied state.
	if revisions, err := s.ListRevisions(); err == nil && len(revisions) > 0 {
		return revisions[0].State, true
	}
	if !state.Dirty && state.LastAppliedAt != nil {
		return state, true
	}
	return State{}, false
}

func (s *AppService) monitoringConfig() (bool, bool) {
	data, err := os.ReadFile(s.paths.NginxMaster)
	if err != nil {
		return false, false
	}
	master := string(data)
	return strings.Contains(master, "log_format fnproxy_metrics"), strings.Contains(master, "fnproxy_metrics buffer=")
}

func (s *AppService) MaintainMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer func() { s.metrics.Flush(time.Now()) }()
	collect := func() {
		ready, logging := s.monitoringConfig()
		var sample *metrics.Sample
		if ready {
			if value, err := s.nginx.BasicStatus(ctx); err == nil {
				sample = &metrics.Sample{PID: value.PID, Requests: value.Requests, Connections: value.Connections}
			}
		}
		s.metrics.Collect(time.Now(), sample, ready && logging)
	}
	collect()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}

func (s *AppService) Dashboard(minutes int, selected string) Dashboard {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	state := s.State()
	applied, known := s.appliedState(state)
	ready, logging := s.monitoringConfig()
	stats := s.metrics.Snapshot(now, minutes, selected)
	d := Dashboard{Overview: s.Overview(), Metrics: stats, MonitoringReady: ready, AccessLogging: logging, AppliedKnown: known, AppliedPorts: []int{}, Rules: []DashboardRule{}, Certificates: []CertificateAlert{}}
	if known {
		d.AppliedCount = domain.EnabledRuleCount(applied)
		d.AppliedPorts = domain.ActivePorts(applied)
	}
	// Shared runtime changes also need applying even if a rule itself is unchanged.
	sharedEqual := known && reflect.DeepEqual(state.Settings, applied.Settings) && reflect.DeepEqual(state.UpstreamPools, applied.UpstreamPools) && reflect.DeepEqual(state.RateLimitPolicies, applied.RateLimitPolicies)
	oldHTTP := map[string]domain.ProxyRule{}
	oldStream := map[string]domain.StreamRule{}
	for _, rule := range applied.Rules {
		oldHTTP[rule.ID] = rule
	}
	for _, rule := range applied.StreamRules {
		oldStream[rule.ID] = rule
	}
	for _, rule := range state.Rules {
		old, exists := oldHTTP[rule.ID]
		currentCompare, oldCompare := rule, old
		currentCompare.CreatedAt, currentCompare.UpdatedAt = time.Time{}, time.Time{}
		oldCompare.CreatedAt, oldCompare.UpdatedAt = time.Time{}, time.Time{}
		status := ruleConfigState(known, exists, old.Enabled, rule.Enabled, sharedEqual && reflect.DeepEqual(currentCompare, oldCompare))
		protocol := "HTTP"
		if rule.TLS {
			protocol = "HTTPS"
		}
		target := httpTarget(rule, state)
		row := DashboardRule{ID: rule.ID, Name: rule.Name, Protocol: protocol, Entry: strings.Join(rule.Domains, ", ") + ":" + itoa(rule.ListenPort), ListenAddress: dashboardListenAddress("0.0.0.0", rule.ListenPort), Target: target, ConfigState: status, Enabled: rule.Enabled}
		if stats.ObservedSeconds > 0 || stats.Rules[rule.ID].Requests > 0 {
			value := stats.Rules[rule.ID]
			if value.Requests == 0 {
				value.ClientErrors = new(uint64)
			}
			row.Counts = &value
		}
		d.Rules = append(d.Rules, row)
		delete(oldHTTP, rule.ID)
	}
	for _, rule := range state.StreamRules {
		old, exists := oldStream[rule.ID]
		currentCompare, oldCompare := rule, old
		// Legacy snapshots may encode absent lists as null rather than [].
		domain.NormalizeStreamRule(&currentCompare)
		domain.NormalizeStreamRule(&oldCompare)
		currentCompare.CreatedAt, currentCompare.UpdatedAt = time.Time{}, time.Time{}
		oldCompare.CreatedAt, oldCompare.UpdatedAt = time.Time{}, time.Time{}
		status := ruleConfigState(known, exists, old.Enabled, rule.Enabled, sharedEqual && reflect.DeepEqual(currentCompare, oldCompare))
		target := poolOrHost(rule.UpstreamPoolID, rule.UpstreamHost, rule.UpstreamPort, state)
		if len(rule.SNIRoutes) > 0 {
			target = "SNI 多目标分流"
		}
		d.Rules = append(d.Rules, DashboardRule{ID: rule.ID, Name: rule.Name, Protocol: strings.ToUpper(rule.Protocol), Entry: rule.ListenAddress + ":" + itoa(rule.ListenPort), ListenAddress: dashboardListenAddress(rule.ListenAddress, rule.ListenPort), Target: target, ConfigState: status, Enabled: rule.Enabled})
		delete(oldStream, rule.ID)
	}
	// Deleted drafts may still be forwarding until the new configuration is applied.
	for _, rule := range oldHTTP {
		if rule.Enabled {
			protocol := "HTTP"
			if rule.TLS {
				protocol = "HTTPS"
			}
			row := DashboardRule{ID: rule.ID, Name: rule.Name, Protocol: protocol, Entry: strings.Join(rule.Domains, ", ") + ":" + itoa(rule.ListenPort), ListenAddress: dashboardListenAddress("0.0.0.0", rule.ListenPort), Target: httpTarget(rule, applied), ConfigState: "pending_delete", Enabled: true}
			if stats.ObservedSeconds > 0 || stats.Rules[rule.ID].Requests > 0 {
				value := stats.Rules[rule.ID]
				if value.Requests == 0 {
					value.ClientErrors = new(uint64)
				}
				row.Counts = &value
			}
			d.Rules = append(d.Rules, row)
		}
	}
	for _, rule := range oldStream {
		if rule.Enabled {
			d.Rules = append(d.Rules, DashboardRule{ID: rule.ID, Name: rule.Name, Protocol: strings.ToUpper(rule.Protocol), Entry: rule.ListenAddress + ":" + itoa(rule.ListenPort), ListenAddress: dashboardListenAddress(rule.ListenAddress, rule.ListenPort), Target: poolOrHost(rule.UpstreamPoolID, rule.UpstreamHost, rule.UpstreamPort, applied), ConfigState: "pending_delete", Enabled: true})
		}
	}
	sort.Slice(d.Rules, func(i, j int) bool { return d.Rules[i].Name < d.Rules[j].Name })
	d.Certificates = certificateAlerts(state, applied, now)
	return d
}

func dashboardListenAddress(address string, port int) string {
	// Both HTTP's port-only listen and stream's wildcard listen bind IPv4.
	if address == "" || address == "*" {
		address = "0.0.0.0"
	}
	return net.JoinHostPort(address, itoa(port))
}

func ruleConfigState(known, exists, oldEnabled, enabled, equal bool) string {
	if !known {
		if enabled {
			return "unknown"
		}
		return "disabled"
	}
	if equal && exists {
		if enabled {
			return "applied"
		}
		return "disabled"
	}
	if !enabled && !oldEnabled {
		return "disabled"
	}
	return "pending"
}

func certificateAlerts(state, applied State, now time.Time) []CertificateAlert {
	refs := map[string]map[string]bool{}
	add := func(id, name string) {
		if id != "" {
			if refs[id] == nil {
				refs[id] = map[string]bool{}
			}
			refs[id][name] = true
		}
	}
	for _, snapshot := range []State{state, applied} {
		for _, rule := range snapshot.Rules {
			if rule.Enabled && rule.TLS {
				add(rule.CertificateID, rule.Name)
			}
		}
		for _, rule := range snapshot.StreamRules {
			if rule.Enabled && rule.TLSMode == "terminate" {
				add(rule.CertificateID, rule.Name)
			}
		}
	}
	alerts := []CertificateAlert{}
	for _, cert := range state.Certificates {
		if len(refs[cert.ID]) == 0 || (cert.NotAfter.After(now.Add(30*24*time.Hour)) && !cert.NotBefore.After(now)) {
			continue
		}
		severity := "warning"
		if !cert.NotAfter.After(now) || cert.NotBefore.After(now) {
			severity = "danger"
		}
		days := int(math.Ceil(cert.NotAfter.Sub(now).Hours() / 24))
		names := []string{}
		for name := range refs[cert.ID] {
			names = append(names, name)
		}
		sort.Strings(names)
		alerts = append(alerts, CertificateAlert{ID: cert.ID, Name: cert.Name, NotAfter: cert.NotAfter, NotBefore: cert.NotBefore, Days: days, Severity: severity, Rules: names})
	}
	sort.Slice(alerts, func(i, j int) bool { return alerts[i].NotAfter.Before(alerts[j].NotAfter) })
	return alerts
}

func poolOrHost(id, host string, port int, state State) string {
	if id != "" {
		for _, pool := range state.UpstreamPools {
			if pool.ID == id {
				return pool.Name + "（服务组）"
			}
		}
		return "后端服务组"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return host + ":" + itoa(port)
}

func httpTarget(rule domain.ProxyRule, state State) string {
	for _, location := range rule.Locations {
		if location.Enabled {
			return "多个路径 / 目标"
		}
	}
	loc := rule.RootLocation
	switch loc.BackendType {
	case "static":
		return "静态文件 · " + loc.StaticPath
	case "return":
		return "固定响应 / 跳转"
	case "stub_status":
		return "Nginx 状态"
	}
	if loc.UpstreamPoolID != "" || loc.UpstreamHost != "" {
		return poolOrHost(loc.UpstreamPoolID, loc.UpstreamHost, loc.UpstreamPort, state)
	}
	return poolOrHost(rule.UpstreamPoolID, rule.UpstreamHost, rule.UpstreamPort, state)
}

func itoa(value int) string { return strconv.Itoa(value) }

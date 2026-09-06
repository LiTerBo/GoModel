package modeldata

import (
	"sort"
	"time"
)

// CapabilityErrorRow is one audit entry carrying a runtime mismatch verdict.
type CapabilityErrorRow struct {
	Provider  string
	Model     string
	Kind      string // auditlog capability_error value (type_mismatch / capability_mismatch)
	Timestamp time.Time
}

// ModelCapabilityErrors is the per-model rollup the dashboard WARN reads.
type ModelCapabilityErrors struct {
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	LatestKind  string    `json:"latest_kind"`
	LastSeen    time.Time `json:"last_seen"`
	Occurrences int       `json:"occurrences"`
}

// AggregateCapabilityErrors rolls runtime mismatch rows up per provider/model,
// keeping the latest verdict and an occurrence count. Rows whose model is
// unknown are skipped; they cannot be attached to a dashboard row.
func AggregateCapabilityErrors(rows []CapabilityErrorRow) []ModelCapabilityErrors {
	type agg struct {
		kind      string
		lastSeen  time.Time
		latestAt  time.Time
		count     int
		hasLatest bool
	}
	byModel := map[string]*agg{}
	for _, r := range rows {
		if r.Provider == "" || r.Model == "" || r.Kind == "" {
			continue
		}
		key := r.Provider + "/" + r.Model
		a, ok := byModel[key]
		if !ok {
			a = &agg{}
			byModel[key] = a
		}
		a.count++
		if !a.hasLatest || r.Timestamp.After(a.latestAt) {
			a.latestAt = r.Timestamp
			a.kind = r.Kind
			a.lastSeen = r.Timestamp
			a.hasLatest = true
		}
	}
	out := make([]ModelCapabilityErrors, 0, len(byModel))
	for key, a := range byModel {
		var provider, model string
		for i := 0; i < len(key); i++ {
			if key[i] == '/' {
				provider, model = key[:i], key[i+1:]
				break
			}
		}
		out = append(out, ModelCapabilityErrors{
			Provider:    provider,
			Model:       model,
			LatestKind:  a.kind,
			LastSeen:    a.lastSeen,
			Occurrences: a.count,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

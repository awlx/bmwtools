package data

import (
	"sort"
)

// ProviderStats represents statistics for a provider
type ProviderStats struct {
	Provider     string  `json:"provider"`
	Count        int     `json:"count"`
	SuccessCount int     `json:"successful_count"`
	FailedCount  int     `json:"failed_count"`
	Total        int     `json:"total"`
	SuccessRate  float64 `json:"success_rate"`
	FailureRate  float64 `json:"failure_rate"`
}

// GroupProviders groups the sessions of the manager by clean operator name.
// Sessions without a known operator are left out.
func (m *Manager) GroupProviders() map[string]interface{} {
	groups := make(map[string]*ProviderStats)
	for _, session := range m.sessions {
		name := CleanProviderName(session.Provider)
		if name == "" {
			continue
		}
		key := ProviderKey(name)
		g, ok := groups[key]
		if !ok {
			g = &ProviderStats{Provider: name}
			groups[key] = g
		}
		g.Count++
		if session.Failed {
			g.FailedCount++
		} else {
			g.SuccessCount++
		}
		g.Total = g.SuccessCount + g.FailedCount
	}

	allProviders := make([]ProviderStats, 0, len(groups))
	for _, g := range groups {
		if g.Total > 0 {
			g.SuccessRate = float64(g.SuccessCount) / float64(g.Total) * 100
			g.FailureRate = float64(g.FailedCount) / float64(g.Total) * 100
		}
		allProviders = append(allProviders, *g)
	}
	sort.Slice(allProviders, func(i, j int) bool {
		if allProviders[i].Total != allProviders[j].Total {
			return allProviders[i].Total > allProviders[j].Total
		}
		return allProviders[i].Provider < allProviders[j].Provider
	})

	top := func(count func(ProviderStats) int) []ProviderStats {
		out := make([]ProviderStats, 0, 5)
		for _, p := range allProviders {
			if count(p) > 0 {
				out = append(out, p)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return count(out[i]) > count(out[j]) })
		if len(out) > 5 {
			out = out[:5]
		}
		return out
	}

	return map[string]interface{}{
		"grouped_successful_providers": top(func(p ProviderStats) int { return p.SuccessCount }),
		"grouped_failed_providers":     top(func(p ProviderStats) int { return p.FailedCount }),
		"all_providers":                allProviders,
	}
}

package providers

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

type ProviderDifference struct {
	Field string `json:"field"`
	Saved string `json:"saved"`
	Live  string `json:"live"`
}

// ProfileComparison compares public configured fields, never secret values or provenance.
type ProfileComparison struct {
	Profile     string               `json:"profile"`
	Differences []ProviderDifference `json:"differences"`
	Inherited   []string             `json:"inherited,omitempty"`
}

func compareProfiles(host agent.ProviderHost, live agent.ProviderProfile, active bool, profiles []agent.ProviderProfile) []ProfileComparison {
	reader, ok := host.(agent.ProviderFieldReader)
	if !ok {
		return nil
	}
	actual := map[string]string{}
	if active {
		actual = reader.ProviderFields(live.Redacted())
	}
	comparisons := make([]ProfileComparison, 0, len(profiles))
	for _, p := range profiles {
		desired := reader.ProviderFields(p.Redacted())
		comparison := ProfileComparison{Profile: p.Name, Differences: []ProviderDifference{}}
		for _, field := range []string{"endpoint", "model", "protocol", "credential"} {
			saved, configured := desired[field]
			if !configured {
				comparison.Inherited = append(comparison.Inherited, field)
				continue
			}
			value := actual[field]
			if saved != value {
				comparison.Differences = append(comparison.Differences, ProviderDifference{Field: field, Saved: saved, Live: value})
			}
		}
		comparisons = append(comparisons, comparison)
	}
	return comparisons
}

func (st Status) comparison(name string) (ProfileComparison, bool) {
	for _, c := range st.Comparisons {
		if c.Profile == name {
			return c, true
		}
	}
	return ProfileComparison{}, false
}

func matchProfile(comparisons []ProfileComparison) (string, []string) {
	var matches []string
	for _, c := range comparisons {
		if len(c.Differences) == 0 {
			matches = append(matches, c.Profile)
		}
	}
	if len(matches) == 1 {
		return matches[0], matches
	}
	return "", matches
}

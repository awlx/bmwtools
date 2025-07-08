package data

import (
	"fmt"
	"testing"
)

// TestProviderMatching tests the provider matching functionality and helps identify why providers might end up as "Unknown"
func TestProviderMatching(t *testing.T) {
	// Define a set of test provider names (both valid and potentially problematic ones)
	testProviders := []struct {
		name        string
		shouldMatch bool // Whether it should match or be marked as "Unknown"
	}{
		// Common valid providers - should match
		{"IONITY", true},
		{"EnBW", true},
		{"Tesla Supercharger", true},
		{"Fastned", true},
		{"Allego", true},

		// Variations of valid providers - should match
		{"IONITY GmbH", true},
		{"EnBW mobility+", true},
		{"EnBW mobility plus", true},
		{"HPC/Ionity", true},
		{"DC/AC/EnBW", true},
		{"Tesla Supercharger V3", true},
		{"Allego B.V.", true},

		// Edge cases that might be marked as Unknown
		{"A", false},   // Single letter
		{"AB", false},  // Two letters
		{"123", false}, // Only numbers
		{"A1", false},  // Single letter with number
		{"--", false},  // Only special chars
		{" ", false},   // Only whitespace
		{"", false},    // Empty string
		{"??", false},  // Special characters

		// Unusual but valid providers - should match
		{"EWE GO", true},
		{"E.ON Drive", true},
		{"TOTAL Energies", true},
		{"Shell Recharge", true},

		// Mixed cases that should be tested
		{"en-bw", true},
		{"E-Wald", true},
		{"E/ON", true},
		{"E-ON", true},
		{"E ON", true},
	}

	// Create a map to track unknown providers
	unknownProviders := make(map[string]string)

	// Test each provider
	for _, tp := range testProviders {
		// Check if provider would be classified as "Unknown" based on the letterCount check in GroupProviders
		letterCount := countLetters(tp.name)
		if letterCount < 2 {
			unknownProviders[tp.name] = fmt.Sprintf("Too few letters (%d): %s", letterCount, tp.name)
		}

		// Test normalization
		normalized := normalizeProviderName(tp.name)
		t.Logf("Provider: %q -> Normalized: %q, Letter count: %d", tp.name, normalized, letterCount)

		// Check if it would be matched with a known provider
		matched := false
		for _, known := range []string{"IONITY", "EnBW", "Tesla", "Fastned", "Allego"} {
			similarity := calculateStringSimilarity(tp.name, known)
			if similarity >= 0.35 { // Using the SIMILARITY_THRESHOLD from GroupProviders
				matched = true
				t.Logf("  Matched with %q with similarity: %.2f", known, similarity)
				break
			}
		}

		if !matched && letterCount >= 2 && tp.shouldMatch {
			// This provider is not marked as "Unknown" due to letter count,
			// but failed to match with any known provider despite being expected to match
			unknownProviders[tp.name] = "Failed to match with any known provider"
		}

		// If we expect it to match but it doesn't, or vice versa, that's an issue
		if matched != tp.shouldMatch && letterCount >= 2 {
			t.Errorf("Provider %q: expected match=%v, got match=%v", tp.name, tp.shouldMatch, matched)
		}
	}

	// Log all providers that would end up as "Unknown"
	t.Log("\nProviders that would be marked as Unknown:")
	for provider, reason := range unknownProviders {
		t.Logf("  %q: %s", provider, reason)
	}
}

// countLetters is a helper function that counts the number of letters in a string
func countLetters(s string) int {
	letterCount := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letterCount++
		}
	}
	return letterCount
}

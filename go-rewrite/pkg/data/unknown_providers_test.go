package data

import (
	"testing"
)

// TestUnknownProviderAnalysis analyzes the unknown provider debug info
// to find patterns and fix issues with providers being marked as "Unknown"
func TestUnknownProviderAnalysis(t *testing.T) {
	// Create test data with sample providers that might have issues
	testSessions := []Session{
		{Provider: "IONITY", SocStart: 10, SocEnd: 20},
		{Provider: "EnBW", SocStart: 30, SocEnd: 40},
		{Provider: "", SocStart: 50, SocEnd: 60},     // Empty string
		{Provider: "A", SocStart: 25, SocEnd: 35},    // Single letter
		{Provider: "E-ON", SocStart: 40, SocEnd: 50}, // Should match but might not
		{Provider: "--", SocStart: 45, SocEnd: 55},   // Only special chars
		{Provider: "123", SocStart: 60, SocEnd: 70},  // Only numbers
		{Provider: "EnBW mobility+", SocStart: 15, SocEnd: 25},
		{Provider: "Tesla Supercharger", SocStart: 20, SocEnd: 30},
		{Provider: "AC/Tesla", SocStart: 40, SocEnd: 50}, // With charging type prefix
		{Provider: "Maingau Energie GmbH", SocStart: 30, SocEnd: 40},
		{Provider: "Shell Recharge Networks", SocStart: 35, SocEnd: 45},
	}

	// Create a data manager and set the test sessions
	manager := NewManager()
	manager.SetSessions(testSessions)

	// Get the grouped providers and debug info
	result := manager.GroupProviders()

	// Extract the unknown providers debug info
	unknownProvidersDebug := result["unknown_providers_debug"].([]UnknownProviderInfo)

	// Log the unknown providers found
	t.Logf("Found %d unknown providers:", len(unknownProvidersDebug))
	for _, info := range unknownProvidersDebug {
		t.Logf("  Original: %q, Reason: %s", info.OriginalValue, info.Reason)
	}

	// Analyze matching patterns for common providers
	t.Log("\nAnalyzing provider matching patterns:")
	analyzeProviderMatching(t, "IONITY", []string{"Ionity", "IONITY GmbH", "HPC/IONITY", "ionity ag"})
	analyzeProviderMatching(t, "EnBW", []string{"EnBW", "EnBW mobility+", "enbw mobility plus", "DC/EnBW"})
	analyzeProviderMatching(t, "Tesla", []string{"Tesla", "Tesla Supercharger", "AC/Tesla", "tesla charging"})

	// Propose potential fixes for issues
	t.Log("\nPotential fixes for unknown provider issues:")
	t.Log("1. Fix for empty strings:")
	t.Log("   - Always normalize empty provider to 'Unknown'")

	t.Log("2. Fix for providers with very few letters:")
	t.Log("   - Consider maintaining a whitelist of valid short providers")
	t.Log("   - ***REMOVED*** 'E.ON', 'EWE', 'RWE' are valid short names")

	t.Log("3. Fix for numeric-only or special character-only providers:")
	t.Log("   - Add special handling for specific numeric/character patterns")
	t.Log("   - Consider checking if they match address or location data")
}

// Helper function to analyze string similarity for specific provider patterns
func analyzeProviderMatching(t *testing.T, baseProvider string, variations []string) {
	t.Logf("Base provider: %q", baseProvider)
	for _, variation := range variations {
		similarity := calculateStringSimilarity(baseProvider, variation)
		normalized1 := normalizeProviderName(baseProvider)
		normalized2 := normalizeProviderName(variation)
		t.Logf("  Variation: %q -> Similarity: %.2f (Normalized: %q vs %q)",
			variation, similarity, normalized1, normalized2)
	}
}

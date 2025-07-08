package data

import (
	"regexp"
	"sort"
	"strings"
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

// normalizeProviderName normalizes a provider name for comparison
func normalizeProviderName(name string) string {
	if name == "" || name == "Unknown" {
		return "Unknown"
	}

	// Convert to lowercase
	normalized := strings.ToLower(name)

	// Remove charging type prefixes (very important for matching variants)
	prefixRegex := regexp.MustCompile(`^(hpc\/|dc\/|ac\/|hpc\/dc\/|hpc\/ac\/|dc\/ac\/|hpc\/dc\/ac\/)+`)
	normalized = prefixRegex.ReplaceAllString(normalized, "")

	// Remove common entity types
	entityRegex := regexp.MustCompile(`\b(gmbh|ag|ltd|llc|inc|kg|co\.|corporation|holding|mbh|group)\b`)
	normalized = entityRegex.ReplaceAllString(normalized, "")

	// Remove charging types
	chargingRegex := regexp.MustCompile(`\b(hpc|dc|ac|ultra|supercharger|fast|rapid|schnell|lader|charger|charging|punkt|power|station)\b`)
	normalized = chargingRegex.ReplaceAllString(normalized, "")

	// Remove country and region designations
	countryRegex := regexp.MustCompile(`\b(germany|deutschland|de|europe|european|eu|nord|süd|sud|west|ost|east|north|south)\b`)
	normalized = countryRegex.ReplaceAllString(normalized, "")

	// Remove common words that don't help identify the provider
	commonRegex := regexp.MustCompile(`\b(charging|station|network|services|mobility|energy|power|elektro|electric)\b`)
	normalized = commonRegex.ReplaceAllString(normalized, "")

	// Remove all special characters and numbers
	specialRegex := regexp.MustCompile(`[\/\-_.,()0-9]`)
	normalized = specialRegex.ReplaceAllString(normalized, " ")

	// Normalize whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	normalized = whitespaceRegex.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// Special handling for common suffixes
	normalized = strings.TrimSuffix(normalized, " mobility")
	normalized = strings.TrimSuffix(normalized, " plus")

	// If after all normalization the string is empty, return the original name
	if normalized == "" {
		return name
	}

	return normalized
}

// calculateStringSimilarity calculates similarity between two strings (0-1)
func calculateStringSimilarity(str1, str2 string) float64 {
	if str1 == "" || str2 == "" {
		return 0.0
	}

	if str1 == str2 {
		return 1.0
	}

	// Special case for Unknown providers
	if str1 == "Unknown" && str2 == "Unknown" {
		return 1.0
	}

	if str1 == "Unknown" || str2 == "Unknown" {
		// Don't match Unknown with anything else
		return 0.0
	}

	s1 := normalizeProviderName(str1)
	s2 := normalizeProviderName(str2)

	if s1 == "" || s2 == "" {
		return 0.1 // Small base similarity
	}

	// Exact match after normalization
	if s1 == s2 {
		return 1.0
	}

	// Expanded list of key brand names and common provider name fragments
	keyBrands := []string{
		// Major charging networks
		"enbw", "ionity", "tesla", "fastned", "allego", "totalenergies",
		"shell", "recharge", "evbox", "aral", "plugsurfing", "maingau",
		"vw", "volkswagen", "eon", "vattenfall", "innogy", "ewe", "elli",
		"rwe", "porsche", "bmw", "e on", "eon", "enel", "engie", "orlen",
		"mer", "mobilityplus", "mobility", "chargepoint", "charge", "envi",
		"electrify", "ladenetz", "endesa", "e-wald", "ewald", "hascharge",
		"enkoping", "fortum", "easy", "move", "greenflux", "newmotion",
		// Additional providers & variants
		"autostadt", "hyundai", "aufgeladen", "go", "ecopower", "ecotap",
		"pfalzwerke", "stadtwerke", "stadtwerk", "gemeinde", "gemeinden",
		"energy", "goingelectric", "ladepark", "ladepunkt", "super",
		"emobility", "sodetrel", "smatrics", "stromnetz", "virta", "freshmile",
		"jolt", "be emobil", "beemobil", "bee", "citywatt", "park", "flow",
		"eon drive", "eondrive", "hubject", "efa", "efacec", "grønn", "gronn",
		"kontakt", "circle", "cito", "charg", "schnell", "belectric", "clever",
		"plugsurfi", "webasto", "zunder", "keba", "okq8", "q8", "mover",
		// Country-specific fragments that may be meaningful
		"stad", "stadt", "statt", "city", "kommune", "kommun", "kommunal",
	}

	// Check for shared key brands - this is the most reliable indicator
	sharedBrands := 0
	for _, brand := range keyBrands {
		if strings.Contains(s1, brand) && strings.Contains(s2, brand) {
			sharedBrands++
			// For critical key brands, a single match is definitive
			if len(brand) >= 4 {
				lengthRatio := float64(min(len(s1), len(s2))) / float64(max(len(s1), len(s2)))
				return 0.85 + (0.15 * lengthRatio)
			}
		}
	}

	// If they share multiple brands/fragments (even short ones), that's significant
	if sharedBrands >= 2 {
		return 0.8
	}

	// Check if one contains the other
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		lengthRatio := float64(min(len(s1), len(s2))) / float64(max(len(s1), len(s2)))
		return 0.7 + (0.3 * lengthRatio)
	}

	// Token-based similarity
	tokens1 := strings.Fields(s1)
	tokens2 := strings.Fields(s2)

	// Find common tokens and partial token matches
	commonTokens := 0
	partialMatches := 0

	// Filter very short tokens (often not meaningful)
	var filteredTokens1, filteredTokens2 []string
	for _, t := range tokens1 {
		if len(t) > 1 {
			filteredTokens1 = append(filteredTokens1, t)
		}
	}
	for _, t := range tokens2 {
		if len(t) > 1 {
			filteredTokens2 = append(filteredTokens2, t)
		}
	}

	// If either has no tokens after filtering, use originals
	if len(filteredTokens1) == 0 {
		filteredTokens1 = tokens1
	}
	if len(filteredTokens2) == 0 {
		filteredTokens2 = tokens2
	}

	// Count exact matches and partial matches
	for _, t1 := range filteredTokens1 {
		for _, t2 := range filteredTokens2 {
			if t1 == t2 {
				// Exact match - weight by length
				commonTokens++
				break
			} else if len(t1) >= 3 && len(t2) >= 3 {
				// Check for partial matches on longer tokens
				if strings.Contains(t1, t2) || strings.Contains(t2, t1) {
					partialMatches++
					break
				}
			}
		}
	}

	totalTokens := float64(len(filteredTokens1) + len(filteredTokens2))
	if totalTokens == 0 {
		totalTokens = 1 // Prevent division by zero
	}

	if commonTokens > 0 || partialMatches > 0 {
		// Calculate weighted score that values exact matches more
		tokenSimilarity := (float64(commonTokens)*2.0 + float64(partialMatches)*1.0) / totalTokens

		// Apply progressive boosts for multiple matches
		if commonTokens > 1 {
			tokenSimilarity += 0.15 // Big boost for multiple exact matches
		} else if commonTokens == 1 {
			tokenSimilarity += 0.1 // Modest boost for a single exact match
		} else if partialMatches > 1 {
			tokenSimilarity += 0.05 // Small boost for multiple partial matches
		}

		return tokenSimilarity
	}

	// Character-based similarity for short strings
	if len(s1) < 6 || len(s2) < 6 {
		commonChars := 0
		for _, char := range s1 {
			if strings.ContainsRune(s2, char) {
				commonChars++
			}
		}
		charSimilarity := float64(commonChars) / float64(max(len(s1), len(s2)))
		// Boost similarity for short strings with common characters
		if charSimilarity > 0.3 {
			charSimilarity += 0.1
		}
		return charSimilarity
	}

	// First letter matching gives a small boost
	if len(s1) > 0 && len(s2) > 0 && s1[0] == s2[0] {
		return 0.2
	}

	// Default low similarity
	return 0.1
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GroupProviders groups similar providers together
func (m *Manager) GroupProviders() map[string]interface{} {
	// Create maps for tracking provider stats - we'll use groupedProviders directly

	// Calculate similarity threshold
	const SIMILARITY_THRESHOLD = 0.35 // Further lowered to be more aggressive in matching providers

	// First, collect all original providers and their types
	originalProviders := []map[string]interface{}{}

	for _, session := range m.sessions {
		provider := session.Provider
		if provider == "" {
			provider = "Unknown"
		}

		isSuccessful := session.SocEnd > session.SocStart
		providerType := "failed"
		if isSuccessful {
			providerType = "successful"
		}

		originalProviders = append(originalProviders, map[string]interface{}{
			"provider": provider,
			"count":    1,
			"type":     providerType,
		})
	}

	// Group similar providers
	groupedProviders := make(map[string]*ProviderStats)

	// Process each provider
	for _, providerObj := range originalProviders {
		provider := providerObj["provider"].(string)
		count := providerObj["count"].(int)
		providerType := providerObj["type"].(string)

		// Skip empty providers or handle spaces/special characters that might be causing "Unknown"
		if provider == "" {
			provider = "Unknown"
		} else {
			// Trim any leading/trailing whitespace
			provider = strings.TrimSpace(provider)
			// Fix some common patterns that might lead to Unknown
			provider = strings.Replace(provider, "  ", " ", -1)
			// If it's just special characters or extremely short, mark as Unknown
			if len(strings.TrimSpace(strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
					return r
				}
				return -1
			}, provider))) < 2 {
				provider = "Unknown"
			}
		}

		// Initialize counters
		successCount := 0
		failedCount := 0

		if providerType == "successful" {
			successCount = count
		} else {
			failedCount = count
		}

		// Find best match in existing grouped providers
		bestMatchKey := ""
		bestSimilarity := 0.0

		for existingProvider := range groupedProviders {
			similarity := calculateStringSimilarity(provider, existingProvider)

			if similarity > bestSimilarity {
				bestSimilarity = similarity
				bestMatchKey = existingProvider
			}
		}

		// Special case for Unknown providers - use a lower threshold
		matchThreshold := SIMILARITY_THRESHOLD
		if provider == "Unknown" {
			matchThreshold = 0.25 // Even lower threshold for matching Unknown providers
		}

		// If good match found, merge with it
		if bestMatchKey != "" && bestSimilarity >= matchThreshold {
			existing := groupedProviders[bestMatchKey]
			existing.Count += count
			existing.SuccessCount += successCount
			existing.FailedCount += failedCount
			existing.Total = existing.SuccessCount + existing.FailedCount
		} else {
			// Otherwise add as new provider
			groupedProviders[provider] = &ProviderStats{
				Provider:     provider,
				Count:        count,
				SuccessCount: successCount,
				FailedCount:  failedCount,
				Total:        successCount + failedCount,
			}
		}
	}

	// Calculate success and failure rates
	for _, stats := range groupedProviders {
		total := float64(stats.Total)
		if total > 0 {
			stats.SuccessRate = float64(stats.SuccessCount) / total * 100
			stats.FailureRate = float64(stats.FailedCount) / total * 100
		}
	}

	// Convert to slices for sorting
	var successfulProviders []ProviderStats
	var failedProviders []ProviderStats
	const MIN_SESSIONS = 50 // Minimum number of sessions to include in the stats

	for _, stats := range groupedProviders {
		// Only include providers with at least MIN_SESSIONS total sessions
		if stats.Total >= MIN_SESSIONS {
			if stats.SuccessCount > 0 {
				successfulProviders = append(successfulProviders, *stats)
			}

			if stats.FailedCount > 0 {
				failedProviders = append(failedProviders, *stats)
			}
		}
	}

	// Sort by count in descending order
	sort.Slice(successfulProviders, func(i, j int) bool {
		return successfulProviders[i].SuccessCount > successfulProviders[j].SuccessCount
	})

	sort.Slice(failedProviders, func(i, j int) bool {
		return failedProviders[i].FailedCount > failedProviders[j].FailedCount
	})

	// Limit to top 5 for each category
	if len(successfulProviders) > 5 {
		successfulProviders = successfulProviders[:5]
	}

	if len(failedProviders) > 5 {
		failedProviders = failedProviders[:5]
	}

	// Get all providers (including those with fewer than MIN_SESSIONS) for the "all_providers" list
	var allProviders []ProviderStats
	for _, stats := range groupedProviders {
		allProviders = append(allProviders, *stats)
	}

	// Sort all providers by total count
	sort.Slice(allProviders, func(i, j int) bool {
		return allProviders[i].Total > allProviders[j].Total
	})

	// Return the result
	return map[string]interface{}{
		"grouped_successful_providers": successfulProviders,
		"grouped_failed_providers":     failedProviders,
		"all_providers":                allProviders,
	}
}

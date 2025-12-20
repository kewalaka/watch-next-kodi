package kodi

import (
	"sort"
	"strings"
)

// FuzzySearch filters and ranks items based on the query.
// It returns a subset of items that match the query, sorted by relevance.
func FuzzySearch(items []MediaItem, query string) []MediaItem {
	query = strings.ToLower(query)
	var results []struct {
		item  MediaItem
		score int
	}

	for _, item := range items {
		// Search against Title and ShowTitle
		target := strings.ToLower(item.Title)
		if item.ShowTitle != "" {
			target += " " + strings.ToLower(item.ShowTitle)
		}

		// 0. Exact Prefix (Highest Priority) - "bre" -> "breaking bad"
		if strings.HasPrefix(target, query) {
			results = append(results, struct {
				item  MediaItem
				score int
			}{item, -50}) // Extremely low score = best rank
			continue
		}

		// 1. Exact match or Contains (Medium Priority)
		if strings.Contains(target, query) {
			results = append(results, struct {
				item  MediaItem
				score int
			}{item, 0}) // 0 score for "good" match
			continue
		}

		// 2. Levenshtein Distance (Fuzzy)
		// Only consider if the distance is within a reasonable threshold (e.g., 30% of length or absolute value)
		dist := levenshtein(query, target)
		threshold := len(query) / 2
		if threshold < 3 {
			threshold = 3
		}

		if dist <= threshold {
			results = append(results, struct {
				item  MediaItem
				score int
			}{item, dist})
		}
	}

	// Sort results: Lower score is better
	sort.Slice(results, func(i, j int) bool {
		return results[i].score < results[j].score
	})

	out := make([]MediaItem, len(results))
	for i, r := range results {
		out[i] = r.item
	}
	// Limit to top 20 to keep UI snappy
	if len(out) > 20 {
		return out[:20]
	}
	return out
}

func levenshtein(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n > m {
		r1, r2 = r2, r1
		n, m = m, n
	}

	currentRow := make([]int, n+1)
	for i := 0; i <= n; i++ {
		currentRow[i] = i
	}

	for i := 1; i <= m; i++ {
		previousRow := currentRow
		currentRow = make([]int, n+1)
		currentRow[0] = i
		for j := 1; j <= n; j++ {
			add, del, change := previousRow[j]+1, currentRow[j-1]+1, previousRow[j-1]
			if r1[j-1] != r2[i-1] {
				change++
			}
			currentRow[j] = min(add, min(del, change))
		}
	}
	return currentRow[n]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

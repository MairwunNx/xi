package texting

import (
	"fmt"
	"strings"
)

// ExpandIndicesAndRanges parses index strings that can be either single indices or ranges.
// Returns a deduplicated slice of all indices within bounds [0, maxIndex].
//
// Examples:
//   - "5" -> [5]
//   - "1-14" -> [1,2,3,...,14]
//   - "7-9" -> [7,8,9]
//   - ["0", "3-7", "12"] -> [0,3,4,5,6,7,12]
//
// Invalid indices or ranges are silently skipped.
func ExpandIndicesAndRanges(indicesStrs []string, maxIndex int) []int {
	var result []int
	seen := make(map[int]bool)

	for _, str := range indicesStrs {
		str = strings.TrimSpace(str)

		// Check if it's a range (contains "-")
		if strings.Contains(str, "-") {
			parts := strings.Split(str, "-")
			if len(parts) != 2 {
				continue // Invalid range format, skip
			}

			var start, end int
			if _, err := fmt.Sscanf(parts[0], "%d", &start); err != nil {
				continue // Invalid start, skip
			}
			if _, err := fmt.Sscanf(parts[1], "%d", &end); err != nil {
				continue // Invalid end, skip
			}

			// Validate range
			if start > end || start < 0 || end > maxIndex {
				continue // Invalid range, skip
			}

			// Add all indices in range
			for i := start; i <= end; i++ {
				if !seen[i] {
					result = append(result, i)
					seen[i] = true
				}
			}
		} else {
			// Single index
			var index int
			if _, err := fmt.Sscanf(str, "%d", &index); err != nil {
				continue // Invalid index, skip
			}

			if index >= 0 && index <= maxIndex && !seen[index] {
				result = append(result, index)
				seen[index] = true
			}
		}
	}

	return result
}


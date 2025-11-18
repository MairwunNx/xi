package indices

import (
	"fmt"
	"strings"
	"ximanager/sources/tracing"
)

// Expand parses index strings that can be either single indices or ranges.
// Returns a deduplicated slice of all indices within bounds [0, maxIndex].
//
// Examples:
//   - "5" -> [5]
//   - "1-14" -> [1,2,3,...,14]
//   - "7-9" -> [7,8,9]
//   - ["0", "3-7", "12"] -> [0,3,4,5,6,7,12]
//
// Invalid indices or ranges are logged and skipped.
func Expand(log *tracing.Logger, indicesStrs []string, maxIndex int) []int {
	var result []int
	seen := make(map[int]bool)
	invalidCount := 0

	for _, str := range indicesStrs {
		str = strings.TrimSpace(str)

		// Check if it's a range (contains "-")
		if strings.Contains(str, "-") {
			parts := strings.Split(str, "-")
			if len(parts) != 2 {
				log.W("Invalid range format, expected 'start-end'", "range", str)
				invalidCount++
				continue
			}

			var start, end int
			if _, err := fmt.Sscanf(parts[0], "%d", &start); err != nil {
				log.W("Failed to parse range start", "range", str, "start", parts[0], tracing.InnerError, err)
				invalidCount++
				continue
			}
			if _, err := fmt.Sscanf(parts[1], "%d", &end); err != nil {
				log.W("Failed to parse range end", "range", str, "end", parts[1], tracing.InnerError, err)
				invalidCount++
				continue
			}

			// Validate range
			if start > end {
				log.W("Invalid range: start > end", "range", str, "start", start, "end", end)
				invalidCount++
				continue
			}
			if start < 0 || end > maxIndex {
				log.W("Range out of bounds", "range", str, "start", start, "end", end, "max_index", maxIndex)
				invalidCount++
				continue
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
				log.W("Failed to parse index", "index_str", str, tracing.InnerError, err)
				invalidCount++
				continue
			}

			if index < 0 || index > maxIndex {
				log.W("Index out of bounds", "index", index, "max_index", maxIndex)
				invalidCount++
				continue
			}

			if !seen[index] {
				result = append(result, index)
				seen[index] = true
			}
		}
	}

	if invalidCount > 0 {
		log.W("Finished parsing indices with errors", "invalid_count", invalidCount, "valid_count", len(result), "total_input", len(indicesStrs))
	}

	return result
}
package main

const defaultMaxCandidates = 20

// mergeCandidates combines two candidate slices with first-seen-wins
// deduplication by schema name, capped at maxCandidates.
// Primary candidates (static) take precedence over secondary (doctype).
func mergeCandidates(primary, secondary []SchemaCandidate, maxCandidates int) []SchemaCandidate {
	if maxCandidates <= 0 {
		maxCandidates = defaultMaxCandidates
	}

	seen := make(map[string]struct{}, len(primary)+len(secondary))
	result := make([]SchemaCandidate, 0, len(primary)+len(secondary))

	add := func(candidates []SchemaCandidate) {
		for _, c := range candidates {
			if len(result) >= maxCandidates {
				return
			}
			if _, dup := seen[c.Schema]; dup {
				continue
			}
			seen[c.Schema] = struct{}{}
			result = append(result, c)
		}
	}

	add(primary)
	add(secondary)
	return result
}

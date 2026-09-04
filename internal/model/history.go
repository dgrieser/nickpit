package model

// SnapshotFinding returns a non-recursive deep copy suitable for immutable
// history storage. JSON cloning keeps pointer-owned downstream representations
// and suggestion slices independent from the live finding.
func SnapshotFinding(f Finding) (FindingSnapshot, error) {
	clone, err := (&ReviewResult{Findings: []Finding{f}}).Clone()
	if err != nil {
		return FindingSnapshot{}, err
	}
	f = clone.Findings[0]
	return FindingSnapshot{
		ID:              f.ID,
		Title:           f.Title,
		Body:            f.Body,
		ConfidenceScore: f.ConfidenceScore,
		Priority:        f.Priority,
		CodeLocation:    f.CodeLocation,
		Suggestions:     f.Suggestions,
		Verification:    f.Verification,
		Finalization:    f.Finalization,
		Summarization:   f.Summarization,
		State:           f.State,
		Resolution:      f.Resolution,
	}, nil
}

// FindingFromSnapshot restores one history entry as a standalone finding.
func FindingFromSnapshot(s FindingSnapshot) Finding {
	return Finding{
		ID:              s.ID,
		Title:           s.Title,
		Body:            s.Body,
		ConfidenceScore: s.ConfidenceScore,
		Priority:        s.Priority,
		CodeLocation:    s.CodeLocation,
		Suggestions:     s.Suggestions,
		Verification:    s.Verification,
		Finalization:    s.Finalization,
		Summarization:   s.Summarization,
		State:           s.State,
		Resolution:      s.Resolution,
	}
}

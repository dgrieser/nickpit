package reviewmd

// UpdateJobReply associates asynchronous acknowledgement/follow-up notes with
// the original question, so a late follow-up cannot consume a newer question.
type UpdateJobReply struct {
	JobID  string `json:"job_id"`
	NoteID int    `json:"note_id"`
	Phase  string `json:"phase"`
}

func UpdateJobReplyMarker(reply UpdateJobReply) string {
	marker, _ := encodeMarker(MarkerOpen+"update-reply:", reply)
	return marker
}

func ReadUpdateJobReply(body string) UpdateJobReply {
	var out UpdateJobReply
	budget := &carrierBudget{}
	scanMarkers(StripHistory(body), MarkerOpen+"update-reply:", func(raw string) bool {
		if !budget.allow() {
			return false
		}
		n, ok := decodeMarker(raw, &out)
		budget.spend(n)
		return !ok
	})
	return out
}

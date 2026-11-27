package taskprotocol

// BusinessComplete reports whether one finished path settles the business check.
// Monitoring always needs both paths; an HTTP proof mismatch is a settled check.
func BusinessComplete(kind string, progress PathProgress) bool {
	if !progress.Done {
		return false
	}
	switch kind {
	case "rss":
		return progress.Valid
	case "verification":
		return progress.Path.Outcome == "OK"
	default:
		return false
	}
}

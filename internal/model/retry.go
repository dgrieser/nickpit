package model

import (
	"fmt"
	"time"
)

// RetryCounter renders the retry number for a progress line, with the maximum
// as a denominator when a retry count bounds the loop ("2/5") and bare
// otherwise ("2"). A max of zero or less means "no count bounds this retry" —
// either nothing does, or something other than a count does, such as the
// cumulative rate-limit wait that bounds 429 retries. It is the caller's job
// to pass the bound that actually applies to the counter it is rendering; the
// callers' own "0 = unlimited" config semantics are separate and must be
// translated, not passed through.
func RetryCounter(retry, max int) string {
	if max > 0 {
		return fmt.Sprintf("%d/%d", retry, max)
	}
	return fmt.Sprintf("%d", retry)
}

// RetryCountLabel renders a retry total in prose: "1 retry", "3 retries".
func RetryCountLabel(retries int) string {
	if retries == 1 {
		return "1 retry"
	}
	return fmt.Sprintf("%d retries", retries)
}

// RetryLine renders the retry progress message every layer shares, so all the
// retries in one run read the same way: "2/5 network error, waiting 4s". The
// leading "retry" word comes from the progress line's state column, so it is
// not repeated here. A zero wait omits the wait for retries that fire
// immediately.
func RetryLine(retry, max int, reason string, wait time.Duration) string {
	msg := RetryCounter(retry, max)
	if reason != "" {
		msg += " " + reason
	}
	if wait > 0 {
		msg += ", waiting " + HumanWait(wait)
	}
	return msg
}

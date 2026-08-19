package model

import (
	"fmt"
	"time"
)

// RetryCounter renders the retry number for a progress line, with the maximum
// as a denominator when one bounds the loop ("2/5") and bare otherwise ("2").
// A max of zero or less means unbounded, matching the "0 = unlimited" retry
// semantics the config uses.
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

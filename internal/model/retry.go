package model

import (
	"fmt"
	"time"
)

// retryCounter renders the retry number for a progress line, with the bound as
// a denominator when a retry count bounds the loop ("2/5") and bare otherwise
// ("2"). A limit of zero or less means "no count bounds this retry" — either
// nothing does, or something other than a count does, such as the cumulative
// rate-limit wait that bounds 429 retries, or several budgets at once, in which
// case RetryBudgetLine names the one that applies.
func retryCounter(retry, limit int) string {
	if limit > 0 {
		return fmt.Sprintf("%d/%d", retry, limit)
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
//
// Use this form only where a single budget bounds the whole loop, so the
// fraction can only be read one way; where a call interleaves retries from
// several budgets, use RetryBudgetLine.
func RetryLine(retry, limit int, reason string, wait time.Duration) string {
	msg := retryCounter(retry, limit)
	if reason != "" {
		msg += " " + reason
	}
	if wait > 0 {
		msg += ", waiting " + HumanWait(wait)
	}
	return msg
}

// RetryBudgetLine renders a retry line for a call whose retries come from more
// than one budget: the counter in front counts every retry the call has made,
// of every kind — which is the total the outcome line reports — and the budget
// that bounds this one retry is named in a trailing gauge instead of shown as a
// denominator: "4 status=500, waiting 4s (2/5 request retries)". Two unlabelled
// fractions from different budgets in one progress stream read as a single
// sequence that jumps forward and then resets.
func RetryBudgetLine(retry int, reason string, wait time.Duration, gauge string) string {
	msg := RetryLine(retry, 0, reason, wait)
	if gauge != "" {
		msg += " (" + gauge + ")"
	}
	return msg
}

// RetryBudget renders the trailing gauge for RetryBudgetLine, naming the budget
// the fraction belongs to: "2/5 request retries". A limit of zero or less means
// no count bounds this budget, and yields no gauge.
func RetryBudget(spent, limit int, name string) string {
	if limit <= 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d %s", spent, limit, name)
}

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dgrieser/nickpit/internal/session"
)

func TestREPLDefersCompletionUntilCurrentTurnEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	stdin := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = stdin }()
	sess := session.New()
	sess.Result = cliTestReview()
	corrected, _ := sess.Result.Clone()
	corrected.Findings[0].Body = "Corrected evidence"
	events := make(chan cliUpdateCompletion, 1)
	second, release, applied := make(chan struct{}), make(chan struct{}), make(chan struct{})
	turn := func(question string) error {
		sess.Append(session.UserMessage(question))
		if question == "second" {
			close(second)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			if sess.Result.Findings[0].Body != "Old evidence" {
				t.Error("completion mutated active discussion turn")
			}
		}
		if question == "third" && sess.Result.Findings[0].Body != "Corrected evidence" {
			t.Error("later turn missed correction")
		}
		return nil
	}
	feedDone := make(chan struct{})
	go func() {
		defer close(feedDone)
		_, _ = fmt.Fprintln(writer, "first\nsecond")
		select {
		case <-second:
		case <-ctx.Done():
			return
		}
		events <- cliUpdateCompletion{Result: corrected}
		close(release)
		select {
		case <-applied:
		case <-ctx.Done():
			return
		}
		_, _ = fmt.Fprintln(writer, "third\n/exit")
		_ = writer.Close()
	}()
	apply := func(out cliUpdateCompletion) {
		if len(sess.Messages) != 2 {
			t.Errorf("completion displaced conversation: %d messages", len(sess.Messages))
		}
		if err := sess.RecordReviewUpdate(out.Result, "Evidence"); err != nil {
			t.Error(err)
		}
		close(applied)
	}
	if err := (&app{}).chatREPL(ctx, sess, turn, events, apply); err != nil {
		t.Fatal(err)
	}
	<-feedDone
	if len(sess.Messages) != 3 || len(sess.ReviewHistory) != 1 {
		t.Fatal("conversation or correction lost")
	}
}

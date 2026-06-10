package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func reviewerInfo() ProgressInfo {
	return ProgressInfo{
		AgentRole: "reviewer",
		AgentName: "#1",
		Model:     "gpt-5",
		Effort:    "high",
	}
}

func TestFormatProgressPlain(t *testing.T) {
	tests := []struct {
		name  string
		info  ProgressInfo
		stage Stage
		state State
		msg   string
		want  string
	}{
		{
			name:  "full bracket with turn state and msg",
			info:  reviewerInfo().WithTurn(2),
			stage: StageRequest,
			state: StateSent,
			msg:   "",
			want:  "Request    [reviewer #1 · gpt-5:high] #2 sent\n",
		},
		{
			name: "detail part",
			info: ProgressInfo{
				AgentRole: "verifier",
				AgentName: "#3",
				Detail:    "Missing err check",
				Turn:      1,
			},
			stage: StageVerify,
			state: StateDone,
			msg:   "conf=0.91",
			want:  "Verify     [verifier #3 · Missing err check] #1 done conf=0.91\n",
		},
		{
			name: "base url part",
			info: ProgressInfo{
				Model:   "gpt-5",
				Effort:  "high",
				BaseURL: "api.example.com",
			},
			stage: StageModel,
			state: StateReady,
			msg:   "16k context, temp=0.2",
			want:  "Model      [gpt-5:high @ api.example.com] ready 16k context, temp=0.2\n",
		},
		{
			name:  "empty bracket",
			info:  ProgressInfo{},
			stage: StagePublish,
			state: StateSkip,
			msg:   "source does not support publishing",
			want:  "Publish    skip source does not support publishing\n",
		},
		{
			name:  "no state",
			info:  reviewerInfo(),
			stage: StageAgent,
			state: StateNone,
			msg:   "3 retries, parallel",
			want:  "Agent      [reviewer #1 · gpt-5:high] 3 retries, parallel\n",
		},
		{
			name:  "no msg",
			info:  reviewerInfo().WithTurn(1),
			stage: StageResponse,
			state: StateDone,
			msg:   "",
			want:  "Response   [reviewer #1 · gpt-5:high] #1 done\n",
		},
		{
			name: "named agent uses colon join",
			info: ProgressInfo{
				AgentRole: "summarize",
				AgentName: "Summarize Review",
			},
			stage: StageSummarize,
			state: StateStart,
			msg:   "findings=4",
			want:  "Summarize  [summarize: Summarize Review] start findings=4\n",
		},
		{
			name:  "model without effort",
			info:  ProgressInfo{Model: "gpt-5"},
			stage: StageModel,
			state: StateRetry,
			msg:   "network error",
			want:  "Model      [gpt-5] retry network error\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatProgressLine(false, tt.info, tt.stage, tt.state, tt.msg)
			if got != tt.want {
				t.Errorf("formatProgressLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatProgressANSI(t *testing.T) {
	tests := []struct {
		name  string
		info  ProgressInfo
		stage Stage
		state State
		msg   string
		want  string
	}{
		{
			name:  "green state with bracket and turn",
			info:  reviewerInfo().WithTurn(2),
			stage: StageRequest,
			state: StateDone,
			want: "\x1b[33mRequest   \x1b[0m " +
				"\x1b[90m[\x1b[0m\x1b[34mreviewer #1\x1b[0m\x1b[90m · \x1b[0m\x1b[34mgpt-5\x1b[0m\x1b[90m:\x1b[0m\x1b[32mhigh\x1b[0m\x1b[90m]\x1b[0m " +
				"\x1b[32m#2\x1b[0m \x1b[32mdone\x1b[0m\n",
		},
		{
			name:  "red error state",
			info:  ProgressInfo{},
			stage: StagePublish,
			state: StateError,
			want:  "\x1b[33mPublish   \x1b[0m \x1b[31merror\x1b[0m\n",
		},
		{
			name:  "yellow retry state",
			info:  ProgressInfo{},
			stage: StageModel,
			state: StateRetry,
			want:  "\x1b[33mModel     \x1b[0m \x1b[33mretry\x1b[0m\n",
		},
		{
			name:  "gray skip state",
			info:  ProgressInfo{},
			stage: StageModelCheck,
			state: StateSkip,
			want:  "\x1b[33mModelCheck\x1b[0m \x1b[90mskip\x1b[0m\n",
		},
		{
			name:  "blue start state",
			info:  ProgressInfo{},
			stage: StageReview,
			state: StateStart,
			want:  "\x1b[33mReview    \x1b[0m \x1b[34mstart\x1b[0m\n",
		},
		{
			name:  "detail gray and base url magenta",
			info:  ProgressInfo{Model: "gpt-5", BaseURL: "api.example.com", Detail: "Missing err check"},
			stage: StageVerify,
			state: StateNone,
			want: "\x1b[33mVerify    \x1b[0m " +
				"\x1b[90m[\x1b[0m\x1b[34mgpt-5\x1b[0m\x1b[90m @ \x1b[0m\x1b[35mapi.example.com\x1b[0m" +
				"\x1b[90m · \x1b[0m\x1b[90mMissing err check\x1b[0m\x1b[90m]\x1b[0m\n",
		},
		{
			name:  "msg tail key-value colorized",
			info:  ProgressInfo{},
			stage: StageResult,
			state: StateOK,
			msg:   "findings=3",
			want:  "\x1b[33mResult    \x1b[0m \x1b[32mok\x1b[0m \x1b[34mfindings\x1b[0m\x1b[90m=\x1b[0m\x1b[32m3\x1b[0m\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatProgressLine(true, tt.info, tt.stage, tt.state, tt.msg)
			if got != tt.want {
				t.Errorf("formatProgressLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStageColumnWidthCoversAllStages(t *testing.T) {
	for _, stage := range allStages {
		if len(stage) > stageColumnWidth {
			t.Errorf("stage %q is longer than stageColumnWidth %d", stage, stageColumnWidth)
		}
	}
	// All brackets must start at the same column.
	wantCol := stageColumnWidth + 1
	for _, stage := range allStages {
		line := formatProgressLine(false, ProgressInfo{AgentRole: "x"}, stage, StateNone, "")
		if idx := strings.IndexByte(line, '['); idx != wantCol {
			t.Errorf("stage %q: bracket starts at column %d, want %d (line %q)", stage, idx, wantCol, line)
		}
	}
}

func TestProgressGatedByShowProgress(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, false, false)
	ctx := WithProgressInfo(context.Background(), reviewerInfo())
	l.Progress(ctx, StageRequest, StateSent, "msg")
	l.ProgressFor(reviewerInfo(), StageRequest, StateSent, "msg")
	l.ProgressToolCall(ctx, "inspect_file(path=foo.go)", "result=[ok]")
	if buf.Len() != 0 {
		t.Errorf("expected no output without SetShowProgress, got %q", buf.String())
	}
	l.SetShowProgress(true)
	l.Progress(ctx, StageRequest, StateSent, "")
	if got, want := buf.String(), "Request    [reviewer #1 · gpt-5:high] sent\n"; got != want {
		t.Errorf("Progress() wrote %q, want %q", got, want)
	}
}

func TestProgressToolCallPlain(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, false, false)
	l.SetShowProgress(true)
	ctx := WithProgressInfo(context.Background(), ProgressInfo{AgentRole: "reviewer", AgentName: "#2"})
	l.ProgressToolCall(ctx, "inspect_file(path=foo.go)", "result=[ok]")
	want := "Tool       [reviewer #2] inspect_file(path=foo.go) → result=[ok]\n"
	if got := buf.String(); got != want {
		t.Errorf("ProgressToolCall() wrote %q, want %q", got, want)
	}
}

func TestProgressInfoContextRoundTripAndVerbosePrefix(t *testing.T) {
	var nilCtx context.Context // nil-safety contract of ProgressInfoFromContext
	if _, ok := ProgressInfoFromContext(nilCtx); ok {
		t.Error("nil context should not carry info")
	}
	if _, ok := ProgressInfoFromContext(context.Background()); ok {
		t.Error("empty context should not carry info")
	}
	info := reviewerInfo().WithTurn(2)
	got, ok := ProgressInfoFromContext(WithProgressInfo(context.Background(), info))
	if !ok || got != info {
		t.Errorf("round trip = %+v, %t; want %+v, true", got, ok, info)
	}
	// Byte-compatible with the old formatAgentTag-based prefix.
	if got, want := info.VerbosePrefix(), "[reviewer: #1, turn: #2] "; got != want {
		t.Errorf("VerbosePrefix() = %q, want %q", got, want)
	}
	if got := (ProgressInfo{Model: "gpt-5"}).VerbosePrefix(); got != "" {
		t.Errorf("VerbosePrefix() without agent = %q, want empty", got)
	}
}

func TestProgressInfoLabel(t *testing.T) {
	tests := []struct {
		name string
		info ProgressInfo
		want string
	}{
		{"reviewer counter", ProgressInfo{AgentRole: "reviewer", AgentName: "#1"}, "reviewer #1"},
		{"named agent", ProgressInfo{AgentRole: "verify", AgentName: "Verify Findings"}, "verify: Verify Findings"},
		{"with detail", ProgressInfo{AgentRole: "verifier", AgentName: "#2", Detail: "Missing error handling"}, "verifier #2: Missing error handling"},
		{"with turn", ProgressInfo{AgentRole: "reviewer", AgentName: "#1", Turn: 3}, "reviewer #1 #3"},
		{"zero", ProgressInfo{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

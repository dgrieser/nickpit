package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func reviewerInfo() ProgressInfo {
	return ProgressInfo{
		AgentRole: "review",
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
			want:  "Request    [review #1 · gpt-5:high] #2 sent\n",
		},
		{
			name: "detail part",
			info: ProgressInfo{
				AgentRole: "verify",
				AgentName: "#3",
				Detail:    "Missing err check",
				Turn:      1,
			},
			stage: StageVerify,
			state: StateDone,
			msg:   "conf=0.91",
			want:  "Verify     [verify #3 · Missing err check] #1 done conf=0.91\n",
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
			want:  "Agent      [review #1 · gpt-5:high] 3 retries, parallel\n",
		},
		{
			name:  "no msg",
			info:  reviewerInfo().WithTurn(1),
			stage: StageResponse,
			state: StateDone,
			msg:   "",
			want:  "Response   [review #1 · gpt-5:high] #1 done\n",
		},
		{
			name:  "workflow bracket embedded",
			info:  ProgressInfo{Workflow: "default", WorkflowSource: "embedded", WorkflowSteps: 6},
			stage: StageAgent,
			state: StateNone,
			msg:   "Structured no nudges",
			want:  "Agent      [default · embedded · 6 steps] Structured no nudges\n",
		},
		{
			name:  "workflow bracket single step",
			info:  ProgressInfo{Workflow: "merge", WorkflowSource: "step", WorkflowSteps: 1},
			stage: StageAgent,
			state: StateNone,
			msg:   "Structured",
			want:  "Agent      [merge · step · 1 step] Structured\n",
		},
		{
			name:  "workflow bracket spec file",
			info:  ProgressInfo{Workflow: "security", WorkflowSource: "security.yaml", WorkflowSteps: 4},
			stage: StageAgent,
			state: StateNone,
			msg:   "Unstructured",
			want:  "Agent      [security · security.yaml · 4 steps] Unstructured\n",
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

func progressTestStyle(code, text string) string {
	if text == "" {
		return ""
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func progressTestGrey(text string) string {
	return progressTestStyle(progressColorGrey, text)
}

func progressTestLight(text string) string {
	return progressTestStyle(progressColorLightGrey, text)
}

func progressTestCounter(n string) string {
	return progressTestStyle(progressColorUnitGreen, "#") + progressTestStyle(progressColorNumberGreen, n)
}

func TestProgressPaletteValues(t *testing.T) {
	tests := map[string]string{
		"grey":              progressColorGrey,
		"darkGrey":          progressColorDarkGrey,
		"numberGreen":       progressColorNumberGreen,
		"unitGreen":         progressColorUnitGreen,
		"keyTurquoise":      progressColorKeyTurquoise,
		"keyTeal":           progressColorKeyTeal,
		"taskPink":          progressColorTaskPink,
		"urlPurpleBlue":     progressColorURLPurpleBlue,
		"profile":           progressColorProfile,
		"branchFromGold":    progressColorBranchFromGold,
		"branchToAquaGreen": progressColorBranchToAquaGreen,
	}
	want := map[string]string{
		"grey":              "38;5;244",
		"darkGrey":          "38;5;242",
		"numberGreen":       "38;5;118",
		"unitGreen":         "38;5;71",
		"keyTurquoise":      "38;5;116",
		"keyTeal":           "38;5;37",
		"taskPink":          "38;5;218",
		"urlPurpleBlue":     "38;5;105",
		"profile":           "38;5;216",
		"branchFromGold":    "38;5;214",
		"branchToAquaGreen": "38;5;48",
	}
	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("%s = %q, want %q", name, got, want[name])
		}
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
			want: progressTestStyle(progressStageStyles[StageRequest], "Request   ") + " " +
				progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, "review") + " " + progressTestCounter("1") +
				progressTestGrey(" · ") + progressTestStyle(progressColorMutedModel, "gpt-5") + progressTestGrey(":") +
				progressTestStyle(progressColorMutedModel, "high") + progressTestGrey("]") + " " +
				progressTestCounter("2") + " " + progressTestLight("done") + "\n",
		},
		{
			name:  "red error state",
			info:  ProgressInfo{},
			stage: StagePublish,
			state: StateError,
			want:  progressTestStyle(progressStageStyles[StagePublish], "Publish   ") + " " + progressTestStyle(progressColorErrorRed, "error") + "\n",
		},
		{
			name:  "yellow retry state",
			info:  ProgressInfo{},
			stage: StageModel,
			state: StateRetry,
			want:  progressTestStyle(progressStageStyles[StageModel], "Model     ") + " " + progressTestStyle(progressColorWarnYellow, "retry") + "\n",
		},
		{
			name:  "gray skip state",
			info:  ProgressInfo{},
			stage: StageModelCheck,
			state: StateSkip,
			want:  progressTestStyle(progressStageStyles[StageModelCheck], "ModelCheck") + " " + progressTestStyle(progressColorDarkGrey, "skip") + "\n",
		},
		{
			name:  "blue start state",
			info:  ProgressInfo{},
			stage: StageReview,
			state: StateStart,
			want:  progressTestStyle(progressStageStyles[StageReview], "Review    ") + " " + progressTestLight("start") + "\n",
		},
		{
			name:  "detail gray and base url magenta",
			info:  ProgressInfo{Model: "gpt-5", BaseURL: "api.example.com", Detail: "Missing err check"},
			stage: StageVerify,
			state: StateNone,
			want: progressTestStyle(progressStageStyles[StageVerify], "Verify    ") + " " +
				progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, "gpt-5") + progressTestGrey(" @ ") +
				progressTestStyle(progressColorURLPurpleBlue, "api.example.com") + progressTestGrey(" · ") +
				progressTestLight("Missing err check") + progressTestGrey("]") + "\n",
		},
		{
			name:  "msg tail key-value colorized",
			info:  ProgressInfo{},
			stage: StageResult,
			state: StateOK,
			msg:   "findings=3",
			want: progressTestStyle(progressStageStyles[StageResult], "Result    ") + " " + progressTestLight("ok") + " " +
				progressTestStyle(progressColorKeyTurquoise, "findings") + progressTestGrey("=") +
				progressTestStyle(progressColorNumberGreen, "3") + "\n",
		},
		{
			name:  "workflow bracket colorized",
			info:  ProgressInfo{Workflow: "default", WorkflowSource: "embedded", WorkflowSteps: 6},
			stage: StageAgent,
			state: StateNone,
			want: progressTestStyle(progressStageStyles[StageAgent], "Agent     ") + " " +
				progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, "default") + progressTestGrey(" · ") +
				progressTestLight("embedded") + progressTestGrey(" · ") +
				progressTestStyle(progressColorNumberGreen, "6") + " " + progressTestLight("steps") +
				progressTestGrey("]") + "\n",
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

func TestFormatProgressANSIExamples(t *testing.T) {
	const model = "Qwen3.5-122B-A10B-FP8"
	modelInfo := ProgressInfo{Model: model, Effort: "high", BaseURL: "https://llm.aihosting.mittwald.de/v1"}
	wantModel := progressTestStyle(progressStageStyles[StageModel], "Model     ") + " " +
		progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, model) + progressTestGrey(":") +
		progressTestStyle(progressColorTaskPink, "high") + progressTestGrey(" @ ") +
		progressTestStyle(progressColorURLPurpleBlue, "https://llm.aihosting.mittwald.de/v1") + progressTestGrey("]") + " " +
		progressTestLight("ready") + " " + progressTestStyle(progressColorNumberGreen, "120") +
		progressTestStyle(progressColorUnitGreen, "k") + " " + progressTestLight("context") + "\n"
	if got := formatProgressLine(true, modelInfo, StageModel, StateReady, "120k context"); got != wantModel {
		t.Fatalf("model line = %q, want %q", got, wantModel)
	}

	reviewInfo := ProgressInfo{Model: model, Effort: "high"}
	reviewMsg := "gitlab: [mittwald, ≥p3] on asylum/services/omvormer @ fix/imagebase-synchronized-race → master"
	wantReview := progressTestStyle(progressStageStyles[StageReview], "Review    ") + " " +
		progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, model) + progressTestGrey(":") +
		progressTestStyle(progressColorTaskPink, "high") + progressTestGrey("]") + " " +
		progressTestLight("start") + " " + progressTestLight("gitlab") + progressTestGrey(":") + " " +
		progressTestGrey("[") + progressTestStyle(progressColorProfile, "mittwald") + progressTestGrey(", ") +
		progressTestStyle(progressColorUnitGreen, "≥") + progressTestStyle(progressColorNumberGreen, "p3") +
		progressTestGrey("]") + " " + progressTestLight("on") + " " +
		progressTestStyle(progressColorTaskPink, "asylum") + progressTestGrey("/") +
		progressTestStyle(progressColorTaskPink, "services") + progressTestGrey("/") +
		progressTestStyle(progressColorTaskPink, "omvormer") + progressTestGrey(" @ ") +
		progressTestStyle(progressColorBranchFromGold, "fix") + progressTestGrey("/") +
		progressTestStyle(progressColorBranchFromGold, "imagebase-synchronized-race") + progressTestGrey(" → ") +
		progressTestStyle(progressColorBranchToAquaGreen, "master") + "\n"
	if got := formatProgressLine(true, reviewInfo, StageReview, StateStart, reviewMsg); got != wantReview {
		t.Fatalf("review line = %q, want %q", got, wantReview)
	}

	requestInfo := ProgressInfo{AgentRole: "context", AgentName: "Collect Context", Model: model, Effort: "high", Turn: 1}
	wantRequest := progressTestStyle(progressStageStyles[StageRequest], "Request   ") + " " +
		progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, "context") + progressTestGrey(": ") +
		progressTestStyle(progressColorTaskPink, "Collect Context") + progressTestGrey(" · ") +
		progressTestStyle(progressColorMutedModel, model) + progressTestGrey(":") +
		progressTestStyle(progressColorMutedModel, "high") + progressTestGrey("]") + " " +
		progressTestCounter("1") + " " + progressTestLight("sent") + "\n"
	if got := formatProgressLine(true, requestInfo, StageRequest, StateSent, ""); got != wantRequest {
		t.Fatalf("request line = %q, want %q", got, wantRequest)
	}

	toolMsg := `search(path=".", query="IsImageBaseCurrent", context_lines=0, max_results=0, case_sensitive=false) → result=[files=5, result_count=12]`
	wantTool := progressTestStyle(progressStageStyles[StageTool], "Tool      ") + " " +
		progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, "context") + progressTestGrey(": ") +
		progressTestStyle(progressColorTaskPink, "Collect Context") + progressTestGrey(" · ") +
		progressTestStyle(progressColorMutedModel, model) + progressTestGrey(":") +
		progressTestStyle(progressColorMutedModel, "high") + progressTestGrey("]") + " " +
		progressTestCounter("1") + " " + progressTestLight("search") + progressTestGrey("(") +
		progressTestStyle(progressColorKeyTurquoise, "path") + progressTestGrey("=") + progressTestStyle(progressColorStringGreen, `"."`) + progressTestGrey(",") + " " +
		progressTestStyle(progressColorKeyTurquoise, "query") + progressTestGrey("=") + progressTestStyle(progressColorStringGreen, `"IsImageBaseCurrent"`) + progressTestGrey(",") + " " +
		progressTestStyle(progressColorKeyTurquoise, "context_lines") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "0") + progressTestGrey(",") + " " +
		progressTestStyle(progressColorKeyTurquoise, "max_results") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "0") + progressTestGrey(",") + " " +
		progressTestStyle(progressColorKeyTurquoise, "case_sensitive") + progressTestGrey("=") + progressTestStyle(progressColorBoolGreen, "false") + progressTestGrey(")") + " " +
		progressTestGrey("→") + " " + progressTestStyle(progressColorKeyTurquoise, "result") + progressTestGrey("=") + progressTestGrey("[") +
		progressTestStyle(progressColorKeyTurquoise, "files") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "5") + progressTestGrey(",") + " " +
		progressTestStyle(progressColorKeyTurquoise, "result_count") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "12") + progressTestGrey("]") + "\n"
	if got := formatProgressLine(true, requestInfo, StageTool, StateNone, toolMsg); got != wantTool {
		t.Fatalf("tool line = %q, want %q", got, wantTool)
	}

	finalizeMsg := "findings_in=8 finalizer_findings=8 matched=8 omitted=0 ignored=0 findings_out=8 prompt_tokens=48709 completion_tokens=6684 total_tokens=55393"
	wantFinalize := progressTestStyle(progressStageStyles[StageFinalize], "Finalize  ") + " " +
		progressTestGrey("[") + progressTestStyle(progressColorKeyTeal, model) + progressTestGrey(":") +
		progressTestStyle(progressColorTaskPink, "high") + progressTestGrey("]") + " " +
		progressTestLight("done") + " " +
		progressTestStyle(progressColorKeyTurquoise, "findings_in") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "8") + " " +
		progressTestStyle(progressColorKeyTurquoise, "finalizer_findings") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "8") + " " +
		progressTestStyle(progressColorKeyTurquoise, "matched") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "8") + " " +
		progressTestStyle(progressColorKeyTurquoise, "omitted") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "0") + " " +
		progressTestStyle(progressColorKeyTurquoise, "ignored") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "0") + " " +
		progressTestStyle(progressColorKeyTurquoise, "findings_out") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "8") + " " +
		progressTestStyle(progressColorKeyTurquoise, "prompt_tokens") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "48709") + " " +
		progressTestStyle(progressColorKeyTurquoise, "completion_tokens") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "6684") + " " +
		progressTestStyle(progressColorKeyTurquoise, "total_tokens") + progressTestGrey("=") + progressTestStyle(progressColorNumberGreen, "55393") + "\n"
	if got := formatProgressLine(true, reviewInfo, StageFinalize, StateDone, finalizeMsg); got != wantFinalize {
		t.Fatalf("finalize line = %q, want %q", got, wantFinalize)
	}
}

func TestColorizeProgressNumbers(t *testing.T) {
	msg := "≤600s 120k #3 2/5 ≥p3 ∞"
	want := progressTestStyle(progressColorUnitGreen, "≤") + progressTestStyle(progressColorNumberGreen, "600") + progressTestStyle(progressColorUnitGreen, "s") + " " +
		progressTestStyle(progressColorNumberGreen, "120") + progressTestStyle(progressColorUnitGreen, "k") + " " +
		progressTestCounter("3") + " " +
		progressTestStyle(progressColorNumberGreen, "2") + progressTestStyle(progressColorUnitGreen, "/") + progressTestStyle(progressColorNumberGreen, "5") + " " +
		progressTestStyle(progressColorUnitGreen, "≥") + progressTestStyle(progressColorNumberGreen, "p3") + " " +
		progressTestStyle(progressColorUnitGreen, "∞")
	if got := colorizeProgressMessage(msg); got != want {
		t.Fatalf("numbers = %q, want %q", got, want)
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
	if got, want := buf.String(), "Request    [review #1 · gpt-5:high] sent\n"; got != want {
		t.Errorf("Progress() wrote %q, want %q", got, want)
	}
}

func TestProgressToolCallPlain(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, false, false)
	l.SetShowProgress(true)
	ctx := WithProgressInfo(context.Background(), ProgressInfo{AgentRole: "review", AgentName: "#2"})
	l.ProgressToolCall(ctx, "inspect_file(path=foo.go)", "result=[ok]")
	want := "Tool       [review #2] inspect_file(path=foo.go) → result=[ok]\n"
	if got := buf.String(); got != want {
		t.Errorf("ProgressToolCall() wrote %q, want %q", got, want)
	}
}

func TestProgressInfoContextRoundTrip(t *testing.T) {
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
}

func TestProgressInfoLabel(t *testing.T) {
	tests := []struct {
		name string
		info ProgressInfo
		want string
	}{
		{"review counter", ProgressInfo{AgentRole: "review", AgentName: "#1"}, "review #1"},
		{"named agent", ProgressInfo{AgentRole: "verify", AgentName: "Verify Findings"}, "verify: Verify Findings"},
		{"with detail", ProgressInfo{AgentRole: "verify", AgentName: "#2", Detail: "Missing error handling"}, "verify #2: Missing error handling"},
		{"with turn", ProgressInfo{AgentRole: "review", AgentName: "#1", Turn: 3}, "review #1 #3"},
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

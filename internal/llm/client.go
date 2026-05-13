package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrieser/nickpit/internal/logging"
	"github.com/dgrieser/nickpit/internal/model"
	"github.com/dgrieser/nickpit/prompts"
	openai "github.com/sashabaranov/go-openai"
)

var ErrInvalidJSON = errors.New("model returned invalid JSON")

var priorityPrefixPattern = regexp.MustCompile(`(?i)^\s*(?:\[\s*P[0-3]\s*\]\s*)+`)

func stripPriorityPrefix(title string) string {
	cleaned := priorityPrefixPattern.ReplaceAllString(title, "")
	return strings.TrimSpace(cleaned)
}

const reasoningBudgetExhaustedMessage = "llm: model exhausted token budget during reasoning without producing a response; try increasing max_tokens or switching to a non-reasoning model"

var reasoningEffortFallbackOrder = []string{"max", "xhigh", "high", "medium", "low", "minimal", "none", "off"}

// InvalidResponseError describes a model response that could not be parsed
// or that parsed but is missing required fields. RawContent holds the original
// model output so callers can append it to the conversation when asking the
// model to retry.
type InvalidResponseError struct {
	RawContent      string
	Reason          string
	MissingFields   []string
	ReasoningEffort string
	ToolsOmitted    bool
}

func (e *InvalidResponseError) Error() string {
	if len(e.MissingFields) > 0 {
		return fmt.Sprintf("model returned invalid JSON: %s (missing or invalid fields: %s)", e.Reason, strings.Join(e.MissingFields, ", "))
	}
	return fmt.Sprintf("model returned invalid JSON: %s", e.Reason)
}

func (e *InvalidResponseError) Is(target error) bool {
	return target == ErrInvalidJSON
}

type Client interface {
	Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
}

type OpenAIClient struct {
	baseURL            string
	apiKey             string
	model              string
	emptyMessagesLimit uint
	httpClient         *http.Client
	sdkClient          *openai.Client
	retrier            *Retrier
	logger             *logging.Logger
	transport          *capturingTransport
	allowedEfforts     map[string]struct{}
}

type SchemaKind string

const (
	SchemaKindReview   SchemaKind = "review"
	SchemaKindVerify   SchemaKind = "verify"
	SchemaKindFinalize SchemaKind = "finalize"
	SchemaKindJSON     SchemaKind = "json"
	SchemaKindText     SchemaKind = "text"
)

// ReasoningSink receives streaming reasoning content from collectStream.
// All methods must be nil-safe.
type ReasoningSink interface {
	Append(delta string)
	End()
}

// ResponseConstraints narrows what values are acceptable in a parsed agent response.
type ResponseConstraints struct {
	MinPriority        *int     // finding priority must be >= this value
	MaxPriority        *int     // finding priority must be <= this value
	AllowedCorrectness []string // overall_correctness must be one of these; nil means default enum
}

type ReviewRequest struct {
	SystemPrompt            string
	UserContent             string
	Messages                []Message
	NoToolsMessages         []Message
	Tools                   []ToolDefinition
	Schema                  json.RawMessage
	SchemaKind              SchemaKind
	Constraints             ResponseConstraints
	Model                   string
	MaxTokens               *int
	Temperature             *float64
	TopP                    *float64
	ExtraBody               map[string]any
	ParallelToolCalls       bool
	ReasoningEffort         string
	MaxReasoning            time.Duration
	MaxReasoningLoopRepeats int
	ReasoningSink           ReasoningSink
	SingleAttempt           bool
}

type Message struct {
	Role       string
	Content    string
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ReviewResponse struct {
	Findings               []model.Finding            `json:"findings"`
	OverallCorrectness     string                     `json:"overall_correctness"`
	OverallExplanation     string                     `json:"overall_explanation"`
	OverallConfidenceScore float64                    `json:"overall_confidence_score"`
	Verification           *model.FindingVerification `json:"verification,omitempty"`
	ToolCalls              []ToolCall                 `json:"tool_calls,omitempty"`
	RawResponse            string                     `json:"raw_response,omitempty"`
	TokensUsed             model.TokenUsage           `json:"tokens_used"`
	ReasoningEffort        string                     `json:"reasoning_effort,omitempty"`
	Reasoned               bool                       `json:"-"`
	ToolsOmitted           bool                       `json:"-"`
}

type capture struct {
	status string
	code   int
	header http.Header
	body   []byte
}

type extraBodyContextKey struct{}

type captureSlot struct {
	mu  sync.Mutex
	cap *capture
}

func (s *captureSlot) set(c *capture) {
	s.mu.Lock()
	s.cap = c
	s.mu.Unlock()
}

func (s *captureSlot) snapshot() *capture {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cap == nil {
		return nil
	}
	cloned := *s.cap
	if cloned.header != nil {
		cloned.header = cloned.header.Clone()
	}
	if cloned.body != nil {
		cloned.body = append([]byte(nil), cloned.body...)
	}
	return &cloned
}

type captureSlotContextKey struct{}

func contextWithCaptureSlot(ctx context.Context) (context.Context, *captureSlot) {
	slot := &captureSlot{}
	return context.WithValue(ctx, captureSlotContextKey{}, slot), slot
}

func captureSlotFromContext(ctx context.Context) *captureSlot {
	slot, _ := ctx.Value(captureSlotContextKey{}).(*captureSlot)
	return slot
}

type streamedResponse struct {
	content          string
	toolCalls        []ToolCall
	usage            model.TokenUsage
	reasoned         bool
	sawContent       bool
	sawToolCalls     bool
	sawUsage         bool
	lastFinishReason string
}

type toolCallBuilder struct {
	id        string
	name      string
	arguments strings.Builder
}

type streamReadError struct {
	err       error
	retryable bool
	partial   *streamedResponse
}

type llmHTTPStatusError struct {
	statusCode int
	status     string
	message    string
	cause      error
}

type ReasoningBudgetExhaustedError struct {
	ReasoningEffort string
}

type reasoningTimeoutController struct {
	limit   time.Duration
	cancel  context.CancelFunc
	timer   *time.Timer
	mu      sync.Mutex
	expired bool
	started bool
	stopped bool
}

func (e *streamReadError) Error() string {
	return e.err.Error()
}

func (e *streamReadError) Unwrap() error {
	return e.err
}

func (e *llmHTTPStatusError) Error() string {
	status := formatHTTPStatus(e.statusCode, e.status)
	if e.message == "" {
		return fmt.Sprintf("llm: api returned %s", status)
	}
	return fmt.Sprintf("llm: api returned %s: %s", status, e.message)
}

func (e *llmHTTPStatusError) Unwrap() error {
	return e.cause
}

func (e *ReasoningBudgetExhaustedError) Error() string {
	return reasoningBudgetExhaustedMessage
}

func newReasoningTimeoutController(limit time.Duration, cancel context.CancelFunc) *reasoningTimeoutController {
	if limit <= 0 {
		return nil
	}
	return &reasoningTimeoutController{limit: limit, cancel: cancel}
}

func (t *reasoningTimeoutController) Start() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return
	}
	t.started = true
	t.timer = time.AfterFunc(t.limit, func() {
		t.mu.Lock()
		if t.stopped {
			t.mu.Unlock()
			return
		}
		t.expired = true
		t.mu.Unlock()
		t.cancel()
	})
}

func (t *reasoningTimeoutController) Stop() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	if t.timer != nil {
		t.timer.Stop()
	}
}

func (t *reasoningTimeoutController) Expired() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.expired
}

func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	transport := &capturingTransport{base: http.DefaultTransport}
	httpClient := &http.Client{
		Transport: transport,
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = strings.TrimRight(baseURL, "/")
	config.HTTPClient = httpClient
	config.EmptyMessagesLimit = 100000

	return &OpenAIClient{
		baseURL:            strings.TrimRight(baseURL, "/"),
		apiKey:             apiKey,
		model:              model,
		emptyMessagesLimit: config.EmptyMessagesLimit,
		httpClient:         httpClient,
		sdkClient:          openai.NewClientWithConfig(config),
		retrier:            NewRetrier(),
		transport:          transport,
	}
}

func (c *OpenAIClient) SetLogger(logger *logging.Logger) {
	c.logger = logger
}

func (c *OpenAIClient) SetAllowedReasoningEfforts(efforts []string) {
	c.allowedEfforts = make(map[string]struct{}, len(efforts))
	for _, effort := range efforts {
		effort = strings.ToLower(strings.TrimSpace(effort))
		if effort != "" {
			c.allowedEfforts[effort] = struct{}{}
		}
	}
}

func KnownReasoningEfforts() []string {
	return append([]string(nil), reasoningEffortFallbackOrder...)
}

func requestPayloadForLog(payload openai.ChatCompletionRequest, extraBody map[string]any) (map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	for key, value := range extraBody {
		body[key] = value
	}
	if _, err := json.Marshal(body); err != nil {
		return nil, err
	}
	return body, nil
}

func contextWithExtraBody(ctx context.Context, extraBody map[string]any) context.Context {
	if len(extraBody) == 0 {
		return ctx
	}
	return context.WithValue(ctx, extraBodyContextKey{}, cloneRequestExtraBody(extraBody))
}

func extraBodyFromContext(ctx context.Context) map[string]any {
	extraBody, _ := ctx.Value(extraBodyContextKey{}).(map[string]any)
	return extraBody
}

func cloneRequestExtraBody(extraBody map[string]any) map[string]any {
	if extraBody == nil {
		return nil
	}
	cloned := make(map[string]any, len(extraBody))
	for key, value := range extraBody {
		cloned[key] = cloneRequestValue(value)
	}
	return cloned
}

func cloneRequestValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneRequestExtraBody(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneRequestValue(item)
		}
		return cloned
	case json.RawMessage:
		return cloneRawMessage(typed)
	case []byte:
		if typed == nil {
			return []byte(nil)
		}
		cloned := make([]byte, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return typed
	}
}

func cloneReviewRequest(req *ReviewRequest) ReviewRequest {
	cloned := *req
	cloned.Messages = cloneMessages(req.Messages)
	cloned.NoToolsMessages = cloneMessages(req.NoToolsMessages)
	cloned.Tools = cloneToolDefinitions(req.Tools)
	cloned.Schema = cloneRawMessage(req.Schema)
	cloned.ExtraBody = cloneRequestExtraBody(req.ExtraBody)
	if req.MaxTokens != nil {
		maxTokens := *req.MaxTokens
		cloned.MaxTokens = &maxTokens
	}
	if req.Temperature != nil {
		temperature := *req.Temperature
		cloned.Temperature = &temperature
	}
	if req.TopP != nil {
		topP := *req.TopP
		cloned.TopP = &topP
	}
	return cloned
}

func cloneMessages(messages []Message) []Message {
	if messages == nil {
		return nil
	}
	cloned := make([]Message, len(messages))
	copy(cloned, messages)
	for i := range cloned {
		cloned[i].ToolCalls = cloneToolCalls(messages[i].ToolCalls)
	}
	return cloned
}

func cloneToolCalls(toolCalls []ToolCall) []ToolCall {
	if toolCalls == nil {
		return nil
	}
	cloned := make([]ToolCall, len(toolCalls))
	copy(cloned, toolCalls)
	return cloned
}

func cloneToolDefinitions(tools []ToolDefinition) []ToolDefinition {
	if tools == nil {
		return nil
	}
	cloned := make([]ToolDefinition, len(tools))
	copy(cloned, tools)
	for i := range cloned {
		cloned[i].Parameters = cloneRawMessage(tools[i].Parameters)
	}
	return cloned
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func setRequestExtraBodyField(extraBody map[string]any, key string, value any) map[string]any {
	if extraBody == nil {
		extraBody = make(map[string]any)
	}
	extraBody[key] = value
	return extraBody
}

func injectExtraBody(req *http.Request) error {
	extraBody := extraBodyFromContext(req.Context())
	if len(extraBody) == 0 || req.Body == nil {
		return nil
	}

	data, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("llm: reading request body for extra_body: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return fmt.Errorf("llm: closing request body for extra_body: %w", err)
	}

	body := map[string]any{}
	if strings.TrimSpace(string(data)) != "" {
		if err := json.Unmarshal(data, &body); err != nil {
			return fmt.Errorf("llm: decoding request body for extra_body: %w", err)
		}
	}
	for key, value := range extraBody {
		body[key] = value
	}

	merged, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("llm: encoding request body with extra_body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(merged))
	req.ContentLength = int64(len(merged))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(merged)), nil
	}
	return nil
}

func (c *OpenAIClient) Review(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("llm: nil review request")
	}

	originalEffort := req.ReasoningEffort
	efforts := []string{originalEffort}
	if !req.SingleAttempt {
		for _, effort := range fallbackReasoningEfforts(originalEffort) {
			if attemptReasoningEffortAllowed(effort, c.allowedEfforts) {
				efforts = append(efforts, effort)
			}
		}
	}

	var lastBudgetErr *ReasoningBudgetExhaustedError
	var lastBudgetReq *ReviewRequest
	budgetExhausted := false
	var lastLoopErr *ReasoningLoopDetectedError
	var lastLoopReq *ReviewRequest
	loopDetected := false
	for attemptIndex, effort := range efforts {
		attemptReq := cloneReviewRequest(req)
		attemptReq.ReasoningEffort = effort
		if budgetExhausted {
			addReasoningBudgetRetryHint(&attemptReq)
		}
		if loopDetected {
			addReasoningLoopRetryHint(&attemptReq)
		}
		resp, err := c.reviewOnce(ctx, &attemptReq)
		if err == nil {
			return resp, nil
		}
		var budgetErr *ReasoningBudgetExhaustedError
		if errors.As(err, &budgetErr) {
			lastBudgetErr = budgetErr
			lastBudgetReq = &attemptReq
			if attemptIndex+1 < len(efforts) {
				budgetExhausted = true
				c.logf("Reasoning budget exhausted, retrying with lower effort: from=%q to=%q", effort, efforts[attemptIndex+1])
				continue
			}
			break
		}
		var loopErr *ReasoningLoopDetectedError
		if errors.As(err, &loopErr) {
			lastLoopErr = loopErr
			lastLoopReq = &attemptReq
			if attemptIndex+1 < len(efforts) {
				loopDetected = true
				c.logf("Reasoning loop detected, retrying with lower effort: from=%q to=%q", effort, efforts[attemptIndex+1])
				continue
			}
			break
		}
		if isReasoningEffortRejection(err, effort) {
			if attemptIndex+1 < len(efforts) {
				c.logf("Reasoning effort rejected by API, skipping effort: effort=%q error=%v", effort, err)
				continue
			}
			if attemptIndex > 0 {
				c.logf("Reasoning effort rejected by API, skipping effort: effort=%q error=%v", effort, err)
				continue
			}
			return nil, err
		}
		if attemptIndex > 0 {
			return nil, err
		}
		return nil, err
	}
	if lastBudgetReq != nil && len(lastBudgetReq.Tools) > 0 {
		noToolsReq := cloneReviewRequest(lastBudgetReq)
		noToolsReq.Messages = noToolsFallbackMessages(lastBudgetReq)
		noToolsReq.Tools = nil
		noToolsReq.ParallelToolCalls = false
		addReasoningBudgetRetryHint(&noToolsReq)
		c.logf("Retrying last budget-exhausted reasoning effort once without tools: effort=%q", noToolsReq.ReasoningEffort)
		noToolsResp, noToolsErr := c.reviewOnce(ctx, &noToolsReq)
		if noToolsErr == nil {
			noToolsResp.ToolsOmitted = true
			return noToolsResp, nil
		}
		var budgetErr *ReasoningBudgetExhaustedError
		if errors.As(noToolsErr, &budgetErr) {
			lastBudgetErr = budgetErr
		} else {
			var invalidResp *InvalidResponseError
			if errors.As(noToolsErr, &invalidResp) {
				invalidResp.ToolsOmitted = true
			}
			return nil, noToolsErr
		}
		c.logf("No-tools retry failed: effort=%q error=%v", noToolsReq.ReasoningEffort, noToolsErr)
	}
	if lastLoopReq != nil && len(lastLoopReq.Tools) > 0 && lastBudgetErr == nil {
		noToolsReq := cloneReviewRequest(lastLoopReq)
		noToolsReq.Messages = noToolsFallbackMessages(lastLoopReq)
		noToolsReq.Tools = nil
		noToolsReq.ParallelToolCalls = false
		addReasoningLoopRetryHint(&noToolsReq)
		c.logf("Retrying last loop-detected reasoning effort once without tools: effort=%q", noToolsReq.ReasoningEffort)
		noToolsResp, noToolsErr := c.reviewOnce(ctx, &noToolsReq)
		if noToolsErr == nil {
			noToolsResp.ToolsOmitted = true
			return noToolsResp, nil
		}
		var loopErr *ReasoningLoopDetectedError
		if errors.As(noToolsErr, &loopErr) {
			lastLoopErr = loopErr
		} else {
			var invalidResp *InvalidResponseError
			if errors.As(noToolsErr, &invalidResp) {
				invalidResp.ToolsOmitted = true
			}
			return nil, noToolsErr
		}
		c.logf("No-tools retry failed: effort=%q error=%v", noToolsReq.ReasoningEffort, noToolsErr)
	}
	if lastLoopErr != nil && lastBudgetErr == nil {
		return nil, lastLoopErr
	}
	if lastBudgetErr != nil {
		return nil, lastBudgetErr
	}
	return nil, fmt.Errorf("llm: internal error: reasoning fallback loop completed without returning")
}

func noToolsFallbackMessages(req *ReviewRequest) []Message {
	if req == nil {
		return nil
	}
	if len(req.NoToolsMessages) > 0 {
		return cloneMessages(req.NoToolsMessages)
	}
	return sanitizeMessagesForNoTools(req.Messages)
}

func sanitizeMessagesForNoTools(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	sanitized := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		next := msg
		next.ToolCalls = nil
		if next.Role == openai.ChatMessageRoleTool {
			next.Role = openai.ChatMessageRoleUser
			next.Name = ""
			next.ToolCallID = ""
		}
		sanitized = append(sanitized, next)
	}
	return sanitized
}

func addReasoningBudgetRetryHint(req *ReviewRequest) {
	addReasoningRetryHint(req, reasoningRetryHint(false))
}

func addReasoningLoopRetryHint(req *ReviewRequest) {
	addReasoningRetryHint(req, reasoningRetryHint(true))
}

func reasoningRetryHint(loopDetected bool) string {
	tmpl, err := prompts.Load("helper_reasoning_snippet.tmpl")
	if err != nil {
		panic(fmt.Sprintf("llm: load reasoning helper prompt: %v", err))
	}
	rendered, err := RenderPrompt(tmpl, struct {
		LoopDetected bool
	}{
		LoopDetected: loopDetected,
	})
	if err != nil {
		panic(fmt.Sprintf("llm: render reasoning helper prompt: %v", err))
	}
	return rendered
}

func addReasoningRetryHint(req *ReviewRequest, hint string) {
	if req == nil {
		return
	}
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return
	}
	if len(req.Messages) == 0 {
		req.UserContent = appendUserHint(req.UserContent, hint)
		return
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == openai.ChatMessageRoleUser {
			if strings.Contains(req.Messages[i].Content, hint) {
				return
			}
		}
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == openai.ChatMessageRoleUser && strings.TrimSpace(req.Messages[i].Content) != "" {
			req.Messages = append(req.Messages[:i+1], append([]Message{{Role: openai.ChatMessageRoleUser, Content: hint}}, req.Messages[i+1:]...)...)
			return
		}
	}
	req.Messages = append(req.Messages, Message{Role: openai.ChatMessageRoleUser, Content: hint})
}

func appendUserHint(content, hint string) string {
	content = strings.TrimSpace(content)
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return content
	}
	if strings.Contains(content, hint) {
		return content
	}
	if content == "" {
		return hint
	}
	return content + "\n\n" + hint
}

func fallbackReasoningEfforts(effort string) []string {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	for i, candidate := range reasoningEffortFallbackOrder {
		if normalized == candidate {
			return append([]string(nil), reasoningEffortFallbackOrder[i+1:]...)
		}
	}
	return []string{"low", "minimal", "none", "off"}
}

func attemptReasoningEffortAllowed(effort string, allowed map[string]struct{}) bool {
	if allowed == nil {
		return true
	}
	_, ok := allowed[strings.ToLower(strings.TrimSpace(effort))]
	return ok
}

func IsReasoningEffortRejection(err error, effort string) bool {
	return isReasoningEffortRejection(err, effort)
}

func isReasoningEffortRejection(err error, effort string) bool {
	var statusErr *llmHTTPStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.statusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
	default:
		return false
	}
	message := strings.ToLower(statusErr.message)
	if message == "" {
		message = strings.ToLower(statusErr.Error())
	}
	return strings.Contains(message, "reasoning_effort") ||
		strings.Contains(message, "reasoning effort") ||
		(strings.Contains(message, "reasoning") && (strings.Contains(message, "support") || strings.Contains(message, "supported") || strings.Contains(message, "invalid") || strings.Contains(message, "value"))) ||
		isOpaqueParameterValidationRejection(message, effort) ||
		isUnknownVariantRejection(message, effort)
}

func isOpaqueParameterValidationRejection(message, effort string) bool {
	if strings.TrimSpace(effort) == "" {
		return false
	}
	return strings.Contains(message, "failed to validate") &&
		strings.Contains(message, "parameter")
}

func isUnknownVariantRejection(message, effort string) bool {
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" {
		return false
	}
	return strings.Contains(message, "unknown variant `"+effort+"`") ||
		strings.Contains(message, "unknown variant \""+effort+"\"")
}

func (c *OpenAIClient) reviewOnce(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("llm: nil review request")
	}

	payload := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: buildMessages(req),
		Tools:    buildTools(req.Tools),
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	if len(req.Tools) > 0 {
		payload.ParallelToolCalls = req.ParallelToolCalls
	}
	if payload.Model == "" {
		payload.Model = c.model
	}
	maxTokensLog := "unset"
	if req.MaxTokens != nil {
		payload.MaxTokens = *req.MaxTokens
		maxTokensLog = fmt.Sprintf("%d", *req.MaxTokens)
	}
	requestExtraBody := cloneRequestExtraBody(req.ExtraBody)

	temperatureLog := "unset"
	if req.Temperature != nil {
		payload.Temperature = float32(*req.Temperature)
		requestExtraBody = setRequestExtraBodyField(requestExtraBody, "temperature", *req.Temperature)
		temperatureLog = fmt.Sprintf("%.2f", *req.Temperature)
	}
	topPLog := "unset"
	if req.TopP != nil {
		payload.TopP = float32(*req.TopP)
		requestExtraBody = setRequestExtraBodyField(requestExtraBody, "top_p", *req.TopP)
		topPLog = fmt.Sprintf("%.2f", *req.TopP)
	}
	extraBodyLog := "unset"
	if len(requestExtraBody) > 0 {
		extraBodyLog = fmt.Sprintf("%d", len(requestExtraBody))
	}
	if req.ReasoningEffort != "" {
		payload.ReasoningEffort = req.ReasoningEffort
	}
	if len(req.Schema) > 0 {
		payload.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   responseFormatName(req.SchemaKind),
				Schema: json.RawMessage(req.Schema),
				Strict: true,
			},
		}
	}

	payloadForLog, err := requestPayloadForLog(payload, requestExtraBody)
	if err != nil {
		return nil, fmt.Errorf("llm: encoding request: %w", err)
	}

	c.logf(
		"LLM request prepared: model=%s endpoint=%s max_tokens=%s temperature=%s top_p=%s extra_body_fields=%s reasoning_effort=%s stream=%t messages=%d tools=%d",
		payload.Model,
		c.baseURL+"/chat/completions",
		maxTokensLog,
		temperatureLog,
		topPLog,
		extraBodyLog,
		payload.ReasoningEffort,
		true,
		len(payload.Messages),
		len(payload.Tools),
	)
	c.logHighlightedJSON("LLM request payload:", payloadForLog)

	streamed, err := c.reviewStream(ctx, payload, requestExtraBody, req.ReasoningSink, req.MaxReasoning, req.MaxReasoningLoopRepeats)
	if err != nil {
		return nil, err
	}
	c.logf(
		"LLM stream summary: content_chunks=%t tool_calls=%t reasoning_chunks=%t usage_chunk=%t last_finish_reason=%q raw_response_bytes=%d",
		streamed.sawContent,
		streamed.sawToolCalls,
		streamed.reasoned,
		streamed.sawUsage,
		streamed.lastFinishReason,
		len(streamed.content),
	)
	c.logRawModelResponse(streamed)

	if streamed.reasoned && !streamed.sawContent && streamed.lastFinishReason == string(openai.FinishReasonLength) {
		return nil, &ReasoningBudgetExhaustedError{ReasoningEffort: payload.ReasoningEffort}
	}

	toolCalls, content, recoveredXMLToolCalls := mergeContentToolCalls(streamed.toolCalls, streamed.content)
	if recoveredXMLToolCalls > 0 {
		c.logf("Recovered XML-style tool calls: recovered=%d total_tool_calls=%d", recoveredXMLToolCalls, len(toolCalls))
	}

	var resp *ReviewResponse
	if len(toolCalls) > 0 {
		resp = &ReviewResponse{ToolCalls: toolCalls}
	} else {
		var err error
		resp, err = parseReviewResponse(content, req.SchemaKind, req.Constraints)
		if err != nil {
			var invalidResp *InvalidResponseError
			if errors.As(err, &invalidResp) {
				invalidResp.ReasoningEffort = payload.ReasoningEffort
			}
			return nil, err
		}
	}
	resp.RawResponse = content
	resp.TokensUsed = streamed.usage
	resp.ReasoningEffort = payload.ReasoningEffort
	resp.Reasoned = streamed.reasoned
	c.logf(
		"Parsed LLM response: findings=%d tool_calls=%d prompt_tokens=%d completion_tokens=%d total_tokens=%d",
		len(resp.Findings),
		len(resp.ToolCalls),
		resp.TokensUsed.PromptTokens,
		resp.TokensUsed.CompletionTokens,
		resp.TokensUsed.TotalTokens,
	)
	return resp, nil
}

func responseFormatName(kind SchemaKind) string {
	if kind == SchemaKindJSON {
		return "json_response"
	}
	return "review_response"
}

func buildMessages(req *ReviewRequest) []openai.ChatCompletionMessage {
	if len(req.Messages) > 0 {
		sanitized := sanitizeMessageHistory(req.Messages)
		messages := make([]openai.ChatCompletionMessage, 0, len(sanitized))
		for _, msg := range sanitized {
			messages = append(messages, toOpenAIMessage(msg))
		}
		return messages
	}
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: req.SystemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: req.UserContent},
	}
}

func buildTools(tools []ToolDefinition) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	converted := make([]openai.Tool, 0, len(tools))
	for _, tool := range tools {
		converted = append(converted, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
				Strict:      true,
			},
		})
	}
	return converted
}

func toOpenAIMessage(msg Message) openai.ChatCompletionMessage {
	converted := openai.ChatCompletionMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}
	if len(msg.ToolCalls) > 0 {
		converted.ToolCalls = make([]openai.ToolCall, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			arguments, ok := NormalizeToolCallArguments(call.Arguments)
			if !ok || strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Name) == "" {
				continue
			}
			converted.ToolCalls = append(converted.ToolCalls, openai.ToolCall{
				ID:   call.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      call.Name,
					Arguments: arguments,
				},
			})
		}
		if len(converted.ToolCalls) == 0 {
			converted.ToolCalls = nil
		}
	}
	return converted
}

func sanitizeMessageHistory(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	sanitized := make([]Message, 0, len(messages))
	validToolCallIDs := make(map[string]struct{})
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			msg.ToolCalls = sanitizeToolCalls(msg.ToolCalls)
			for _, call := range msg.ToolCalls {
				validToolCallIDs[call.ID] = struct{}{}
			}
		}
		if msg.Role == openai.ChatMessageRoleTool {
			if _, ok := validToolCallIDs[msg.ToolCallID]; !ok {
				continue
			}
		}
		sanitized = append(sanitized, msg)
	}
	return sanitized
}

func sanitizeToolCalls(toolCalls []ToolCall) []ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	sanitized := make([]ToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Name) == "" {
			continue
		}
		arguments, ok := NormalizeToolCallArguments(call.Arguments)
		if !ok {
			continue
		}
		call.Arguments = arguments
		sanitized = append(sanitized, call)
	}
	return sanitized
}

func NormalizeToolCallArguments(arguments string) (string, bool) {
	var args any
	if err := LenientUnmarshal(arguments, &args); err != nil {
		return "", false
	}
	normalized, err := json.Marshal(args)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}

func (c *OpenAIClient) reviewStream(ctx context.Context, payload openai.ChatCompletionRequest, extraBody map[string]any, sink ReasoningSink, maxReasoning time.Duration, maxReasoningLoopRepeats int) (*streamedResponse, error) {
	ctx = contextWithExtraBody(ctx, extraBody)
	for attempt := 0; ; attempt++ {
		streamCtx, streamCancel := context.WithCancel(ctx)
		streamCtx, slot := contextWithCaptureSlot(streamCtx)
		var detector *reasoningLoopDetector
		if maxReasoningLoopRepeats > 0 {
			detector = newReasoningLoopDetector(streamCancel, maxReasoningLoopRepeats)
		}
		timeout := newReasoningTimeoutController(maxReasoning, streamCancel)
		c.logf("Sending LLM request: attempt=%d", attempt+1)

		stream, err := c.sdkClient.CreateChatCompletionStream(streamCtx, payload)
		capture := slot.snapshot()
		if capture != nil && capture.code != 0 {
			c.logf("LLM stream opened: status=%s", capture.status)
		}
		if err != nil {
			if status := statusCodeFromError(err, capture); status > 0 {
				statusErr := newLLMHTTPStatusError(err, capture)
				c.logf("LLM request failed: attempt=%d error=%v", attempt+1, statusErr)
				if body := httpErrorBody(err, capture); len(body) > 0 {
					c.logMaybeJSON("LLM raw response body:", body)
				}
				if !c.shouldRetryHTTPStatus(status, attempt) {
					return nil, statusErr
				}
				resp := responseFromCapture(capture)
				waitFor := c.retrier.Backoff(attempt, resp)
				c.logRetryHTTPStatus(status, attempt+1, waitFor)
				c.logf("Retrying request: status=%d backoff=%s", status, waitFor)
				if waitErr := c.retrier.Wait(ctx, attempt, resp); waitErr != nil {
					return nil, fmt.Errorf("llm: retry canceled: %w", waitErr)
				}
				continue
			}
			c.logf("LLM request failed: attempt=%d error=%v", attempt+1, err)
			if !isRetryableNetworkError(err) || attempt >= c.retrier.MaxRetries {
				return nil, fmt.Errorf("llm: request failed: %w", err)
			}
			if c.logger != nil {
				c.logger.PrintStatusLine("LLM request hit a network error, retrying...")
			}
			if waitErr := c.retrier.Wait(ctx, attempt, nil); waitErr != nil {
				return nil, fmt.Errorf("llm: request canceled: %w", waitErr)
			}
			continue
		}

		resp, streamErr := c.collectStream(stream, sink, detector, timeout)
		closeErr := stream.Close()
		timeout.Stop()
		streamCancel()
		if streamErr != nil {
			if closeErr != nil {
				c.logf("LLM stream close failed after error: %v", closeErr)
			}
			var loopErr *ReasoningLoopDetectedError
			if errors.As(streamErr, &loopErr) {
				loopErr.ReasoningEffort = payload.ReasoningEffort
				c.logf("Reasoning loop detected: effort=%q", payload.ReasoningEffort)
				if c.logger != nil {
					if loopErr.LoopStartContent != "" {
						c.logger.PrintBlock("Reasoning loop - content before repeat:", loopErr.LoopStartContent)
					}
					c.logger.PrintBlock("Reasoning loop - repeated portion (aborted):", loopErr.RepeatedContent)
				}
				return nil, loopErr
			}
			if timeout.Expired() {
				c.logf("Reasoning time limit exceeded: effort=%q limit=%s", payload.ReasoningEffort, maxReasoning)
				return nil, &ReasoningBudgetExhaustedError{ReasoningEffort: payload.ReasoningEffort}
			}
			var readErr *streamReadError
			if errors.As(streamErr, &readErr) {
				if isReasoningOnlyPeerInternalStreamError(readErr) {
					return nil, &ReasoningBudgetExhaustedError{ReasoningEffort: payload.ReasoningEffort}
				}
				if readErr.retryable && attempt < c.retrier.MaxRetries {
					if c.logger != nil {
						c.logger.PrintStatusLine("LLM stream hit a network error, retrying...")
					}
					c.logf("Retrying request: stream network error")
					if waitErr := c.retrier.Wait(ctx, attempt, nil); waitErr != nil {
						return nil, fmt.Errorf("llm: retry canceled: %w", waitErr)
					}
					continue
				}
			}
			return nil, streamErr
		}
		if closeErr != nil {
			return nil, fmt.Errorf("llm: closing stream: %w", closeErr)
		}
		return resp, nil
	}
}

func (c *OpenAIClient) shouldRetryHTTPStatus(status, attempt int) bool {
	if !c.retrier.ShouldRetry(status) {
		return false
	}
	if status == http.StatusTooManyRequests {
		return true
	}
	return attempt < c.retrier.MaxRetries
}

func (c *OpenAIClient) logRetryHTTPStatus(status, currentAttempt int, waitFor time.Duration) {
	if c.logger == nil {
		return
	}
	if status == http.StatusTooManyRequests {
		c.logger.PrintProgress("Model", fmt.Sprintf("rate limited (429), waiting %s before retry attempt %d", waitFor, currentAttempt+1))
		return
	}
	c.logger.PrintStatusLine(fmt.Sprintf("LLM request failed with status %d, retrying in %s...", status, waitFor))
}

func (c *OpenAIClient) collectStream(stream *openai.ChatCompletionStream, sink ReasoningSink, detector *reasoningLoopDetector, timeout *reasoningTimeoutController) (*streamedResponse, error) {
	var (
		contentBuilder   strings.Builder
		toolCalls        []*toolCallBuilder
		usage            model.TokenUsage
		reasoningStarted bool
		sawUsage         bool
		sawContent       bool
		sawToolCalls     bool
		lastFinishReason string
		receivedChunk    bool
	)
	// Lazy fallback: open an unlabeled section when no sink was provided by the caller.
	ownsSink := false
	ensureSink := func() ReasoningSink {
		if sink == nil && c.logger != nil {
			sink = c.logger.OpenReasoningSection("")
			ownsSink = sink != nil
		}
		return sink
	}
	endSink := func() {
		if ownsSink && reasoningStarted {
			if s := ensureSink(); s != nil {
				s.End()
			}
		}
	}
	partialResponse := func() *streamedResponse {
		return &streamedResponse{
			content:          contentBuilder.String(),
			toolCalls:        finalizeToolCalls(toolCalls),
			usage:            usage,
			reasoned:         reasoningStarted,
			sawContent:       sawContent,
			sawToolCalls:     sawToolCalls,
			sawUsage:         sawUsage,
			lastFinishReason: lastFinishReason,
		}
	}
	c.logf("LLM waiting for first stream chunk")

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if !sawUsage {
					endSink()
					return nil, &streamReadError{
						err:       fmt.Errorf("llm: reading stream: interrupted before final usage chunk"),
						retryable: true,
						partial:   partialResponse(),
					}
				}
				endSink()
				return partialResponse(), nil
			}
			endSink()
			if detector != nil && detector.Detected() {
				return nil, detector.MakeError()
			}
			return nil, &streamReadError{
				err:       fmt.Errorf("llm: reading stream: %w", err),
				retryable: isRetryableNetworkError(err),
				partial:   partialResponse(),
			}
		}

		if chunk.Usage != nil {
			sawUsage = true
			usage = model.TokenUsage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
		if !receivedChunk {
			receivedChunk = true
			c.logf("LLM first stream chunk received")
		}

		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if choice.FinishReason != "" {
				lastFinishReason = string(choice.FinishReason)
			}
			if choice.Delta.ReasoningContent != "" {
				if !reasoningStarted {
					reasoningStarted = true
					timeout.Start()
				}
				if s := ensureSink(); s != nil {
					s.Append(choice.Delta.ReasoningContent)
				}
				if detector != nil {
					detector.onDelta(choice.Delta.ReasoningContent)
				}
			}
			if choice.Delta.Content != "" {
				timeout.Stop()
				contentBuilder.WriteString(choice.Delta.Content)
				sawContent = true
			}
			if len(choice.Delta.ToolCalls) > 0 {
				timeout.Stop()
				sawToolCalls = true
				mergeToolCallDeltas(&toolCalls, choice.Delta.ToolCalls)
			}
		}
	}
}

func mergeToolCallDeltas(builders *[]*toolCallBuilder, deltas []openai.ToolCall) {
	for _, delta := range deltas {
		index := 0
		if delta.Index != nil && *delta.Index >= 0 {
			index = *delta.Index
		}
		for len(*builders) <= index {
			*builders = append(*builders, nil)
		}
		if (*builders)[index] == nil {
			(*builders)[index] = &toolCallBuilder{}
		}
		builder := (*builders)[index]
		if delta.ID != "" {
			builder.id = delta.ID
		}
		if delta.Function.Name != "" {
			builder.name = delta.Function.Name
		}
		if delta.Function.Arguments != "" {
			builder.arguments.WriteString(delta.Function.Arguments)
		}
	}
}

func finalizeToolCalls(builders []*toolCallBuilder) []ToolCall {
	if len(builders) == 0 {
		return nil
	}
	toolCalls := make([]ToolCall, 0, len(builders))
	for _, builder := range builders {
		if builder == nil {
			continue
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        builder.id,
			Name:      builder.name,
			Arguments: builder.arguments.String(),
		})
	}
	return toolCalls
}

type capturingTransport struct {
	base http.RoundTripper
}

func (t *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	slot := captureSlotFromContext(req.Context())
	if err := injectExtraBody(req); err != nil {
		if slot != nil {
			slot.set(nil)
		}
		return nil, err
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		if slot != nil {
			slot.set(nil)
		}
		return nil, err
	}

	captured := &capture{
		status: resp.Status,
		code:   resp.StatusCode,
		header: resp.Header.Clone(),
	}

	if strings.Contains(req.Header.Get("Accept"), "text/event-stream") &&
		resp.StatusCode >= http.StatusOK &&
		resp.StatusCode < http.StatusBadRequest {
		if slot != nil {
			slot.set(captured)
		}
		return resp, nil
	}

	data, readErr := readAndRestoreBody(resp)
	captured.body = data
	if slot != nil {
		slot.set(captured)
	}
	if readErr != nil {
		return nil, readErr
	}

	return resp, nil
}

func readAndRestoreBody(resp *http.Response) ([]byte, error) {
	if resp.Body == nil {
		return nil, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}

	resp.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

func responseFromCapture(c *capture) *http.Response {
	if c == nil || c.code == 0 {
		return nil
	}
	return &http.Response{
		Status:     c.status,
		StatusCode: c.code,
		Header:     c.header.Clone(),
	}
}

func statusCodeFromError(err error, c *capture) int {
	var statusErr *llmHTTPStatusError
	if errors.As(err, &statusErr) {
		if statusErr.statusCode > 0 {
			return statusErr.statusCode
		}
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatusCode > 0 {
			return apiErr.HTTPStatusCode
		}
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		if reqErr.HTTPStatusCode > 0 {
			return reqErr.HTTPStatusCode
		}
	}

	if c != nil {
		return c.code
	}

	return 0
}

func newLLMHTTPStatusError(err error, c *capture) *llmHTTPStatusError {
	statusCode := statusCodeFromError(err, c)
	status := ""
	if c != nil {
		status = c.status
	}
	message := ""

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatus != "" {
			status = apiErr.HTTPStatus
		}
		if apiErr.HTTPStatusCode > 0 {
			statusCode = apiErr.HTTPStatusCode
		}
		message = apiErr.Message
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		if reqErr.HTTPStatus != "" {
			status = reqErr.HTTPStatus
		}
		if reqErr.HTTPStatusCode > 0 {
			statusCode = reqErr.HTTPStatusCode
		}
		if message == "" {
			message = providerErrorMessage(reqErr.Body)
		}
		if message == "" {
			message = cleanHTTPErrorText(string(reqErr.Body))
		}
		if message == "" && reqErr.Err != nil {
			message = cleanHTTPErrorText(reqErr.Err.Error())
		}
	}

	if message == "" && c != nil {
		message = providerErrorMessage(c.body)
		if message == "" {
			message = cleanHTTPErrorText(string(c.body))
		}
	}
	message = cleanHTTPErrorText(message)

	return &llmHTTPStatusError{
		statusCode: statusCode,
		status:     status,
		message:    message,
		cause:      err,
	}
}

func httpErrorBody(err error, c *capture) []byte {
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) && len(reqErr.Body) > 0 {
		return reqErr.Body
	}
	if c != nil && len(c.body) > 0 {
		return c.body
	}
	return nil
}

func formatHTTPStatus(code int, status string) string {
	status = strings.TrimSpace(status)
	if status != "" {
		return status
	}
	if code <= 0 {
		return "unknown status"
	}
	if text := http.StatusText(code); text != "" {
		return fmt.Sprintf("%d %s", code, text)
	}
	return fmt.Sprintf("%d", code)
}

func providerErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return ""
	}
	return cleanHTTPErrorText(providerErrorMessageValue(value))
}

func providerErrorMessageValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"message", "detail", "error_description", "error"} {
			if message := providerErrorMessageValue(typed[key]); message != "" {
				return message
			}
		}
	case []any:
		var parts []string
		for _, item := range typed {
			if message := providerErrorMessageValue(item); message != "" {
				parts = append(parts, message)
			}
		}
		return strings.Join(parts, ", ")
	case string:
		return typed
	}
	return ""
}

func cleanHTTPErrorText(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	const maxErrorTextRunes = 1024
	runes := []rune(text)
	if len(runes) > maxErrorTextRunes {
		return string(runes[:maxErrorTextRunes]) + "..."
	}
	return text
}

func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isRetryableNetworkError(urlErr.Err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	return false
}

func isReasoningOnlyPeerInternalStreamError(err *streamReadError) bool {
	if err == nil || err.partial == nil {
		return false
	}
	if !err.partial.reasoned || err.partial.sawContent || err.partial.sawToolCalls {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "stream error:") &&
		strings.Contains(message, "INTERNAL_ERROR") &&
		strings.Contains(message, "received from peer")
}

func (c *OpenAIClient) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func (c *OpenAIClient) logRawModelResponse(streamed *streamedResponse) {
	if c.logger == nil || streamed == nil {
		return
	}
	c.logHighlightedJSON("LLM raw model response:", rawModelResponseForLog(streamed))
}

func (c *OpenAIClient) logJSON(label string, value any) {
	if c.logger != nil {
		c.logger.PrintJSON(label, value)
	}
}

func (c *OpenAIClient) logHighlightedJSON(label string, value any) {
	if c.logger == nil {
		return
	}
	c.logger.Printf("%s", label)
	c.logger.PrintJSON("", value)
}

func (c *OpenAIClient) logMaybeJSON(label string, data []byte) {
	if c.logger == nil {
		return
	}
	var value any
	if err := json.Unmarshal(data, &value); err == nil {
		c.logger.PrintJSON(label, value)
		return
	}
	c.logger.PrintBlock(label, string(data))
}

func rawModelResponseForLog(streamed *streamedResponse) any {
	if streamed == nil {
		return nil
	}
	return struct {
		Content          string           `json:"content"`
		ToolCalls        []ToolCall       `json:"tool_calls,omitempty"`
		Usage            model.TokenUsage `json:"usage"`
		Reasoned         bool             `json:"reasoned"`
		SawContent       bool             `json:"saw_content"`
		SawToolCalls     bool             `json:"saw_tool_calls"`
		SawUsage         bool             `json:"saw_usage"`
		LastFinishReason string           `json:"last_finish_reason,omitempty"`
	}{
		Content:          streamed.content,
		ToolCalls:        streamed.toolCalls,
		Usage:            streamed.usage,
		Reasoned:         streamed.reasoned,
		SawContent:       streamed.sawContent,
		SawToolCalls:     streamed.sawToolCalls,
		SawUsage:         streamed.sawUsage,
		LastFinishReason: streamed.lastFinishReason,
	}
}

func mergeContentToolCalls(structured []ToolCall, content string) ([]ToolCall, string, int) {
	xmlCalls, cleanedContent := parseXMLToolCalls(content)
	if len(structured) == 0 && len(xmlCalls) == 0 {
		return nil, content, 0
	}

	merged := make([]ToolCall, 0, len(structured)+len(xmlCalls))
	seen := make(map[string]struct{}, len(structured)+len(xmlCalls))
	for _, call := range structured {
		key := canonicalToolCallKey(call)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, call)
	}

	recoveredXMLToolCalls := 0
	for _, call := range xmlCalls {
		key := canonicalToolCallKey(call)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, call)
		recoveredXMLToolCalls++
	}
	return merged, cleanedContent, recoveredXMLToolCalls
}

func parseXMLToolCalls(content string) ([]ToolCall, string) {
	re := regexp.MustCompile(`(?s)<tool_call>\s*([A-Za-z_][A-Za-z0-9_]*)\s*(.*?)</tool_call>`)
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, content
	}

	calls := make([]ToolCall, 0, len(matches))
	for _, match := range matches {
		args := parseXMLToolCallArguments(match[2])
		arguments, err := json.Marshal(args)
		if err != nil {
			continue
		}
		calls = append(calls, ToolCall{
			ID:        fmt.Sprintf("xml_tool_call_%d", len(calls)+1),
			Name:      match[1],
			Arguments: string(arguments),
		})
	}

	cleaned := strings.TrimSpace(re.ReplaceAllString(content, ""))
	return calls, cleaned
}

func parseXMLToolCallArguments(content string) map[string]any {
	re := regexp.MustCompile(`(?s)<arg_key>\s*(.*?)\s*</arg_key>\s*<arg_value>\s*(.*?)\s*</arg_value>`)
	matches := re.FindAllStringSubmatch(content, -1)
	args := make(map[string]any, len(matches))
	for _, match := range matches {
		key := strings.TrimSpace(html.UnescapeString(match[1]))
		if key == "" {
			continue
		}
		args[key] = parseXMLToolCallArgumentValue(match[2])
	}
	return args
}

func parseXMLToolCallArgumentValue(value string) any {
	value = strings.TrimSpace(html.UnescapeString(value))
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed
	}
	return value
}

func canonicalToolCallKey(call ToolCall) string {
	var args any
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return call.Name + "\x00" + call.Arguments
	}
	normalized, err := json.Marshal(args)
	if err != nil {
		return call.Name + "\x00" + call.Arguments
	}
	return call.Name + "\x00" + string(normalized)
}

func parseReviewResponse(content string, kind SchemaKind, constraints ResponseConstraints) (*ReviewResponse, error) {
	if kind == SchemaKindText {
		return &ReviewResponse{}, nil
	}
	if kind == SchemaKindJSON {
		var parsed any
		if err := LenientUnmarshal(content, &parsed); err != nil {
			return nil, &InvalidResponseError{
				RawContent: content,
				Reason:     fmt.Sprintf("could not parse JSON: %v", err),
			}
		}
		return &ReviewResponse{}, nil
	}
	if kind == SchemaKindVerify {
		return parseVerifyResponse(content)
	}
	var parsed ReviewResponse
	if err := LenientUnmarshal(content, &parsed); err != nil {
		return nil, &InvalidResponseError{
			RawContent: content,
			Reason:     fmt.Sprintf("could not parse JSON: %v", err),
		}
	}
	for i := range parsed.Findings {
		parsed.Findings[i].Title = stripPriorityPrefix(parsed.Findings[i].Title)
	}
	if missing := missingResponseFields(&parsed, content, kind, constraints); len(missing) > 0 {
		return &parsed, &InvalidResponseError{
			RawContent:    content,
			Reason:        "response is missing required fields",
			MissingFields: missing,
		}
	}
	return &parsed, nil
}

func parseVerifyResponse(content string) (*ReviewResponse, error) {
	var verification model.FindingVerification
	if err := LenientUnmarshal(content, &verification); err != nil {
		return nil, &InvalidResponseError{
			RawContent: content,
			Reason:     fmt.Sprintf("could not parse JSON: %v", err),
		}
	}
	if missing := missingVerifyFields(content); len(missing) > 0 {
		return &ReviewResponse{Verification: &verification}, &InvalidResponseError{
			RawContent:    content,
			Reason:        "response is missing required fields",
			MissingFields: missing,
		}
	}
	if verification.Priority < 0 || verification.Priority > 3 {
		return &ReviewResponse{Verification: &verification}, &InvalidResponseError{
			RawContent:    content,
			Reason:        "response is missing required fields",
			MissingFields: []string{"priority (must be 0-3)"},
		}
	}
	return &ReviewResponse{Verification: &verification}, nil
}

func missingVerifyFields(content string) []string {
	var raw map[string]json.RawMessage
	_ = LenientUnmarshal(content, &raw)
	var missing []string
	for _, field := range []string{"valid", "priority", "confidence_score", "remarks"} {
		if _, ok := raw[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

func missingResponseFields(parsed *ReviewResponse, content string, kind SchemaKind, constraints ResponseConstraints) []string {
	var raw map[string]json.RawMessage
	_ = LenientUnmarshal(content, &raw)
	var missing []string
	if _, ok := raw["findings"]; !ok && parsed.Findings == nil {
		missing = append(missing, "findings")
	}
	missing = append(missing, missingFindingFields(parsed.Findings, raw["findings"], kind, constraints)...)
	if strings.TrimSpace(parsed.OverallCorrectness) == "" {
		missing = append(missing, "overall_correctness")
	} else {
		allowed := constraints.AllowedCorrectness
		if len(allowed) == 0 {
			allowed = []string{"patch is correct", "patch is incorrect"}
		}
		if !slices.Contains(allowed, parsed.OverallCorrectness) {
			missing = append(missing, fmt.Sprintf("overall_correctness (must be one of: %v)", allowed))
		}
	}
	if strings.TrimSpace(parsed.OverallExplanation) == "" {
		missing = append(missing, "overall_explanation")
	}
	if _, ok := raw["overall_confidence_score"]; !ok {
		missing = append(missing, "overall_confidence_score")
	}
	return missing
}

func missingFindingFields(findings []model.Finding, rawFindings json.RawMessage, kind SchemaKind, constraints ResponseConstraints) []string {
	if len(findings) == 0 {
		return nil
	}
	var rawItems []map[string]json.RawMessage
	if len(rawFindings) > 0 {
		_ = json.Unmarshal(rawFindings, &rawItems)
	}
	effectiveMin := 0
	effectiveMax := 3
	if constraints.MinPriority != nil {
		effectiveMin = *constraints.MinPriority
	}
	if constraints.MaxPriority != nil {
		effectiveMax = *constraints.MaxPriority
	}
	var missing []string
	for i, finding := range findings {
		var rawItem map[string]json.RawMessage
		if i < len(rawItems) {
			rawItem = rawItems[i]
		}
		_, hasPriorityKey := rawItem["priority"]
		if !hasPriorityKey || finding.Priority == nil {
			missing = append(missing, fmt.Sprintf("findings[%d].priority", i))
			continue
		}
		if *finding.Priority < effectiveMin || *finding.Priority > effectiveMax {
			missing = append(missing, fmt.Sprintf("findings[%d].priority (must be %d-%d)", i, effectiveMin, effectiveMax))
		}
		if kind == SchemaKindFinalize {
			missing = append(missing, missingFinalizeFindingFields(i, rawItem, finding)...)
		}
	}
	return missing
}

func missingFinalizeFindingFields(i int, rawItem map[string]json.RawMessage, finding model.Finding) []string {
	var missing []string
	if _, ok := rawItem["verification"]; !ok || finding.Verification == nil {
		missing = append(missing, fmt.Sprintf("findings[%d].verification", i))
	} else {
		missing = append(missing, missingNestedFields(fmt.Sprintf("findings[%d].verification", i), rawItem["verification"], []string{"valid", "priority", "confidence_score", "remarks"})...)
	}
	if _, ok := rawItem["finalization"]; !ok || finding.Finalization == nil {
		missing = append(missing, fmt.Sprintf("findings[%d].finalization", i))
	} else {
		missing = append(missing, missingNestedFields(fmt.Sprintf("findings[%d].finalization", i), rawItem["finalization"], []string{"title", "body", "priority", "remarks"})...)
		if finding.Finalization.Priority < 0 || finding.Finalization.Priority > 3 {
			missing = append(missing, fmt.Sprintf("findings[%d].finalization.priority (must be 0-3)", i))
		}
	}
	return missing
}

func missingNestedFields(prefix string, raw json.RawMessage, fields []string) []string {
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	var missing []string
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			missing = append(missing, prefix+"."+field)
		}
	}
	return missing
}

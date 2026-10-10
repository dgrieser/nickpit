package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
)

// Protocol is one LLM wire API. It is the seam between the provider-neutral
// client core — retries, the reasoning-effort ladder, reasoning budgets, loop
// detection, response parsing — and the shape a provider's endpoint speaks.
// A protocol only translates: it renders a neutral CompletionRequest as a
// request body, decodes the streamed response into neutral StreamChunks, and
// reads a failed response into a ProviderError. The core owns the HTTP round
// trip, so every protocol inherits the same retry, backoff, and timeout
// behaviour, and nothing above the core ever sees a wire type.
type Protocol interface {
	// Name identifies the protocol in logs and configuration.
	Name() string
	// Capabilities reports what the wire API can express, independent of the
	// model behind it; a model probe can only narrow these down.
	Capabilities() Capabilities
	// Path is the endpoint path appended to the client's base URL.
	Path() string
	// SetHeaders adds the protocol's authentication and version headers for
	// one request. token is the current credential and may be empty.
	SetHeaders(header http.Header, token string)
	// EncodeRequest renders req as the JSON request body.
	EncodeRequest(req *CompletionRequest) (json.RawMessage, error)
	// NewEventReader decodes a successful streaming response body.
	NewEventReader(body io.Reader) EventReader
	// ParseError reads a failed response. The returned error's Status is the
	// status the client's retry policy acts on, which a protocol may remap
	// from the HTTP status when its error code says more (a quota that lasts
	// hours is not a rate limit worth waiting out).
	ParseError(status int, body []byte) *ProviderError
}

// ParamDropper is implemented by protocols that add optional request fields
// of their own accord. When the endpoint rejects one of them as unsupported,
// DropUnsupported records that, so later requests omit it, and reports
// whether the request should be re-sent at once.
type ParamDropper interface {
	DropUnsupported(err *ProviderError) bool
}

// Message roles. They are the neutral vocabulary of Message.Role; each
// protocol maps them onto its own (a system message may become instructions,
// a tool message a function-call output item).
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// CompletionRequest is one fully resolved model call in neutral form: the
// reasoning-effort ladder, finalization, and request trimming have already
// been applied, and every message is valid history.
type CompletionRequest struct {
	Model    string
	Messages []Message
	Tools    []ToolDefinition
	// ParallelToolCalls allows or forbids several tool calls per turn; nil
	// leaves it to the endpoint.
	ParallelToolCalls *bool
	// Schema constrains the response to a JSON schema named SchemaName when
	// the protocol supports structured output.
	Schema     json.RawMessage
	SchemaName string
	MaxTokens  *int
	// Sampling knobs. Only those the protocol lists in
	// Capabilities.SamplingParams reach it; the core drops the rest.
	Temperature       *float64
	TopP              *float64
	TopK              *int
	MinP              *float64
	PresencePenalty   *float64
	RepetitionPenalty *float64
	ReasoningEffort   string
	// ExtraBody holds raw provider fields merged into the request body
	// verbatim, overriding fields of the same name.
	ExtraBody map[string]any
}

// ReasoningKind says what reasoning text a stream carries.
type ReasoningKind string

const (
	// ReasoningRaw is the model's own reasoning tokens, verbatim.
	ReasoningRaw ReasoningKind = "raw"
	// ReasoningSummary is a provider-written summary of hidden reasoning.
	ReasoningSummary ReasoningKind = "summary"
	// ReasoningNone means the API returns no reasoning text at all.
	ReasoningNone ReasoningKind = "none"
)

// FinishReason is why the model stopped, in the neutral vocabulary the core
// acts on. A protocol maps its own reasons onto these; one without an
// equivalent passes through verbatim for the logs.
type FinishReason string

const (
	FinishStop          FinishReason = "stop"
	FinishLength        FinishReason = "length"
	FinishToolCalls     FinishReason = "tool_calls"
	FinishContentFilter FinishReason = "content_filter"
	// FinishNull is the literal "null" some OpenAI-compatible servers send.
	FinishNull FinishReason = "null"
)

// StreamChunk is one decoded piece of a streamed response. Every field is
// optional; an empty chunk is a keep-alive that proves the stream is alive.
type StreamChunk struct {
	Text          string
	Reasoning     string
	ReasoningKind ReasoningKind
	ToolCalls     []ToolCallDelta
	Usage         *model.TokenUsage
	FinishReason  FinishReason
	// StateItem is one piece of provider state to replay with this turn
	// (see ProviderState), in stream order.
	StateItem json.RawMessage
}

// ToolCallDelta is a fragment of a streamed tool call. Index identifies the
// call when the protocol numbers them; without it, a new ID starts a new call
// and ID-less fragments continue the latest one.
type ToolCallDelta struct {
	Index     *int
	ID        string
	Name      string
	Arguments string
}

// EventReader yields the chunks of one streamed response. Next returns io.EOF
// once the stream completed; any other error ends it early. A provider error
// reported inside the stream is returned as a *ProviderError.
type EventReader interface {
	Next() (StreamChunk, error)
}

// ProviderError is a provider's error report, from an error response or from
// inside a stream.
type ProviderError struct {
	// Status is the HTTP status the retry policy should treat this error as;
	// zero when the provider reported it mid-stream without one.
	Status  int
	Code    string
	Param   string
	Type    string
	Message string
}

// Error keeps the "error, <message>" form OpenAI-compatible SDKs report
// mid-stream errors in, which provider-specific detectors (a LiteLLM
// repeated-chunk report) match against.
func (e *ProviderError) Error() string {
	return "error, " + e.Message
}

// Capabilities is what a wire API can express. They describe the protocol,
// not a model: a model behind it may support less, which the model check
// probes for.
type Capabilities struct {
	// Reasoning is the kind of reasoning text the API can stream back.
	Reasoning ReasoningKind
	// ReasoningEffort reports whether a reasoning effort can be requested.
	ReasoningEffort bool
	// Tools reports function calling; ParallelToolCalls whether a request
	// can allow several calls per turn.
	Tools             bool
	ParallelToolCalls bool
	// StructuredOutput reports whether a JSON schema can constrain the
	// response.
	StructuredOutput bool
	// OutputTokenLimit reports whether a request can cap the output tokens.
	OutputTokenLimit bool
	// UnsupportedFields lists request-body fields the endpoint rejects. The
	// core strips them from a profile's extra_body, saying so in the log,
	// rather than sending a request that cannot succeed.
	UnsupportedFields []string
	// ReservedFields lists request-body fields the protocol itself owns —
	// streaming, storage, the conversation, the tool declaration form. A
	// profile's extra_body cannot override them: the core strips them (and
	// logs it), and the protocol enforces them in the body it encodes.
	ReservedFields []string
	// SamplingParams lists the sampling knobs the API accepts, by their
	// config names (temperature, top_p, top_k, min_p, presence_penalty,
	// repetition_penalty).
	SamplingParams []string
}

// AcceptsSampling reports whether name is one of SamplingParams.
func (c Capabilities) AcceptsSampling(name string) bool {
	return slices.Contains(c.SamplingParams, name)
}

// String renders the capabilities for logs and the model check.
func (c Capabilities) String() string {
	return fmt.Sprintf("reasoning=%s effort=%t tools=%t parallel_tools=%t structured_output=%t sampling=%s",
		c.Reasoning, c.ReasoningEffort, c.Tools, c.ParallelToolCalls, c.StructuredOutput, strings.Join(c.SamplingParams, ","))
}

// CapabilityReporter is implemented by clients that know what their wire API
// can express. Callers holding a plain Client use it to skip what the API
// cannot do instead of discovering it by failure.
type CapabilityReporter interface {
	Capabilities() Capabilities
}

// ClientCapabilities returns what client's API can express, or ok=false when
// the client does not report it.
func ClientCapabilities(client Client) (Capabilities, bool) {
	reporter, ok := client.(CapabilityReporter)
	if !ok {
		return Capabilities{}, false
	}
	return reporter.Capabilities(), true
}

// TokenSource supplies the credential sent with each request. Token returns
// one that is currently valid; a source whose credentials can be renewed
// also implements TokenRefresher.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// TokenRefresher renews a credential the server rejected as unauthorized.
type TokenRefresher interface {
	ForceRefresh(ctx context.Context) (string, error)
}

// AccountIdentifier is implemented by token sources that can name the account
// their credential belongs to. The identity must be stable for the account and
// must not reveal the credential; provider state is scoped to it.
type AccountIdentifier interface {
	AccountID() string
}

// StaticToken is a fixed API key.
type StaticToken string

// Token implements TokenSource.
func (t StaticToken) Token(context.Context) (string, error) { return string(t), nil }

// AccountID implements AccountIdentifier with a digest of the key, so two
// keys never share provider state while the key itself stays out of it.
func (t StaticToken) AccountID() string {
	if t == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(t))
	return "key:" + hex.EncodeToString(sum[:8])
}

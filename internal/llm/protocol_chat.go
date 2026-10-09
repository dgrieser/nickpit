package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/dgrieser/nickpit/internal/model"
	openai "github.com/sashabaranov/go-openai"
)

// ChatCompletionsProtocolName names the OpenAI Chat Completions protocol.
const ChatCompletionsProtocolName = "chat_completions"

// chatCompletionsProtocol speaks POST /chat/completions, the API every
// OpenAI-compatible server (OpenRouter, vLLM, LiteLLM, Mistral, DeepSeek, …)
// implements. go-openai is used here as a schema library only, so the request
// JSON and the stream semantics match what those servers have always seen.
type chatCompletionsProtocol struct{}

// ChatCompletionsProtocol returns the OpenAI Chat Completions protocol.
func ChatCompletionsProtocol() Protocol { return chatCompletionsProtocol{} }

func (chatCompletionsProtocol) Name() string { return ChatCompletionsProtocolName }

func (chatCompletionsProtocol) Capabilities() Capabilities {
	return Capabilities{
		// reasoning_content (or reasoning) carries the model's raw tokens on
		// the servers that stream them at all.
		Reasoning:         ReasoningRaw,
		ReasoningEffort:   true,
		Tools:             true,
		ParallelToolCalls: true,
		StructuredOutput:  true,
		// Open-weight servers accept every knob; they ride in the body as
		// extra fields where the OpenAI schema has none.
		SamplingParams: []string{"temperature", "top_p", "top_k", "min_p", "presence_penalty", "repetition_penalty"},
	}
}

func (chatCompletionsProtocol) Path() string { return "/chat/completions" }

func (chatCompletionsProtocol) SetHeaders(header http.Header, token string) {
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
}

func (chatCompletionsProtocol) EncodeRequest(req *CompletionRequest) (json.RawMessage, error) {
	payload, extraBody := chatCompletionPayload(req)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return mergeOrderedJSONObject(data, extraBody)
}

// chatCompletionPayload renders req as the typed request plus the fields that
// ride beside it. Sampling knobs are written into the extra fields as well as
// the typed request: the typed fields omit a zero value, and an explicit 0
// (temperature 0, min_p 0) must still reach the server.
func chatCompletionPayload(req *CompletionRequest) (openai.ChatCompletionRequest, map[string]any) {
	payload := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: chatMessages(req.Messages),
		Stream:   true,
		Tools:    chatTools(req.Tools),
		StreamOptions: &openai.StreamOptions{
			IncludeUsage: true,
		},
	}
	if len(req.Tools) > 0 {
		payload.ParallelToolCalls = req.ParallelToolCalls
	}
	if req.MaxTokens != nil {
		payload.MaxTokens = *req.MaxTokens
	}
	extraBody := cloneRequestExtraBody(req.ExtraBody)
	if req.Temperature != nil {
		payload.Temperature = float32(*req.Temperature)
		extraBody = setRequestExtraBodyField(extraBody, "temperature", *req.Temperature)
	}
	if req.TopP != nil {
		payload.TopP = float32(*req.TopP)
		extraBody = setRequestExtraBodyField(extraBody, "top_p", *req.TopP)
	}
	if req.TopK != nil {
		extraBody = setRequestExtraBodyField(extraBody, "top_k", *req.TopK)
	}
	if req.MinP != nil {
		extraBody = setRequestExtraBodyField(extraBody, "min_p", *req.MinP)
	}
	if req.PresencePenalty != nil {
		payload.PresencePenalty = float32(*req.PresencePenalty)
		extraBody = setRequestExtraBodyField(extraBody, "presence_penalty", *req.PresencePenalty)
	}
	if req.RepetitionPenalty != nil {
		extraBody = setRequestExtraBodyField(extraBody, "repetition_penalty", *req.RepetitionPenalty)
	}
	if req.ReasoningEffort != "" {
		payload.ReasoningEffort = req.ReasoningEffort
	}
	if len(req.Schema) > 0 {
		payload.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   req.SchemaName,
				Schema: json.RawMessage(req.Schema),
				// Strict is intentionally false: our schemas use optional
				// properties, which OpenAI strict mode forbids (it requires
				// every property in `required` and additionalProperties:false
				// on every object). Claiming strict would make any endpoint
				// that actually validates it reject every request.
				Strict: false,
			},
		}
	}
	return payload, extraBody
}

func chatMessages(messages []Message) []openai.ChatCompletionMessage {
	converted := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		converted = append(converted, chatMessage(msg))
	}
	return converted
}

func chatTools(tools []ToolDefinition) []openai.Tool {
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
				// Strict is intentionally not set: tool parameter schemas use
				// optional properties, which OpenAI strict mode forbids, so a
				// strict-validating endpoint would 400 every request.
			},
		})
	}
	return converted
}

func chatMessage(msg Message) openai.ChatCompletionMessage {
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

func (chatCompletionsProtocol) ParseError(status int, body []byte) *ProviderError {
	perr := &ProviderError{Status: status}
	var errResp openai.ErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
		perr.Message = errResp.Error.Message
		perr.Type = errResp.Error.Type
		if errResp.Error.Param != nil {
			perr.Param = *errResp.Error.Param
		}
		if errResp.Error.Code != nil {
			perr.Code = fmt.Sprint(errResp.Error.Code)
		}
	}
	if perr.Message == "" {
		perr.Message = providerErrorMessage(body)
	}
	if perr.Message == "" {
		perr.Message = cleanHTTPErrorText(string(body))
	}
	perr.Message = cleanHTTPErrorText(perr.Message)
	return perr
}

// chatStreamEmptyLineLimit bounds how many consecutive non-data lines a
// stream may send between two chunks before it is treated as broken.
const chatStreamEmptyLineLimit = 100000

var (
	chatStreamDataPrefix  = regexp.MustCompile(`^data:\s*`)
	chatStreamErrorPrefix = regexp.MustCompile(`^data:\s*{"error":`)
)

func (chatCompletionsProtocol) NewEventReader(body io.Reader) EventReader {
	return &chatEventReader{reader: bufio.NewReader(body)}
}

// chatEventReader reads Chat Completions server-sent events. Lines that are not
// data lines (event names, comments, blank separators) are collected and, once
// the stream ends, read as an error report — the way OpenAI-compatible servers
// report a failure after the 200 status went out.
type chatEventReader struct {
	reader   *bufio.Reader
	errLines bytes.Buffer
	finished bool
}

func (r *chatEventReader) Next() (StreamChunk, error) {
	if r.finished {
		return StreamChunk{}, io.EOF
	}
	emptyLines := 0
	hasErrorPrefix := false
	for {
		rawLine, readErr := r.reader.ReadBytes('\n')
		if readErr != nil || hasErrorPrefix {
			if perr := r.streamError(); perr != nil {
				return StreamChunk{}, perr
			}
			if readErr == nil {
				readErr = errors.New("llm: malformed stream error report")
			}
			return StreamChunk{}, readErr
		}
		line := bytes.TrimSpace(rawLine)
		if chatStreamErrorPrefix.Match(line) {
			hasErrorPrefix = true
		}
		if !chatStreamDataPrefix.Match(line) || hasErrorPrefix {
			if hasErrorPrefix {
				line = chatStreamDataPrefix.ReplaceAll(line, nil)
			}
			r.errLines.Write(line)
			emptyLines++
			if emptyLines > chatStreamEmptyLineLimit {
				return StreamChunk{}, errors.New("llm: stream has sent too many empty messages")
			}
			continue
		}
		data := chatStreamDataPrefix.ReplaceAll(line, nil)
		if string(data) == "[DONE]" {
			r.finished = true
			return StreamChunk{}, io.EOF
		}
		var chunk openai.ChatCompletionStreamResponse
		if err := json.Unmarshal(data, &chunk); err != nil {
			return StreamChunk{}, err
		}
		return chatStreamChunk(chunk), nil
	}
}

func (r *chatEventReader) streamError() *ProviderError {
	if r.errLines.Len() == 0 {
		return nil
	}
	var errResp openai.ErrorResponse
	if json.Unmarshal(r.errLines.Bytes(), &errResp) != nil || errResp.Error == nil {
		return nil
	}
	perr := &ProviderError{Message: errResp.Error.Message, Type: errResp.Error.Type}
	if errResp.Error.Param != nil {
		perr.Param = *errResp.Error.Param
	}
	if errResp.Error.Code != nil {
		perr.Code = fmt.Sprint(errResp.Error.Code)
	}
	return perr
}

func chatStreamChunk(chunk openai.ChatCompletionStreamResponse) StreamChunk {
	var out StreamChunk
	if chunk.Usage != nil {
		out.Usage = &model.TokenUsage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
	}
	for _, choice := range chunk.Choices {
		if choice.Index != 0 {
			continue
		}
		if choice.FinishReason != "" {
			out.FinishReason = FinishReason(choice.FinishReason)
		}
		if choice.Delta.ReasoningContent != "" {
			out.Reasoning += choice.Delta.ReasoningContent
			out.ReasoningKind = ReasoningRaw
		}
		out.Text += choice.Delta.Content
		for _, call := range choice.Delta.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, ToolCallDelta{
				Index:     call.Index,
				ID:        call.ID,
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			})
		}
	}
	return out
}

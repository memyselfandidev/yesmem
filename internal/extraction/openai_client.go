package extraction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIChatURL = "https://api.openai.com/v1/chat/completions"

// OpenAIClient is a minimal OpenAI Chat Completions API client used for extraction workloads.
type OpenAIClient struct {
	apiKey     string
	model      string
	endpoint   string
	name       string
	httpClient *http.Client
}

func NewOpenAIClient(apiKey, model, baseURL, name string) *OpenAIClient {
	if name == "" {
		name = "openai"
	}
	return &OpenAIClient{
		apiKey:   apiKey,
		model:    model,
		endpoint: normalizeOpenAIChatURL(baseURL),
		name:     name,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *OpenAIClient) Name() string  { return c.name }
func (c *OpenAIClient) Model() string { return c.model }

func (c *OpenAIClient) Complete(system, userMessage string, opts ...CallOption) (string, error) {
	return c.doRequest(system, userMessage, nil, opts...)
}

func (c *OpenAIClient) CompleteJSON(system, userMessage string, schema map[string]any, opts ...CallOption) (string, error) {
	// DeepSeek doesn't support "json_schema" response_format, fall back to "json_object"
	if strings.Contains(c.model, "deepseek") {
		return c.doRequest(system, userMessage, &openAIResponseFormat{
			Type: "json_object",
		}, opts...)
	}
	return c.doRequest(system, userMessage, &openAIResponseFormat{
		Type: "json_schema",
		JSONSchema: &openAIJSONSchema{
			Name:   "yesmem_output",
			Schema: schema,
			Strict: true,
		},
	}, opts...)
}

// --- Request types (Chat Completions API) ---

type openAIChatRequest struct {
	Model          string                `json:"model"`
	Messages       []openAIChatMessage   `json:"messages"`
	MaxTokens      int                   `json:"max_tokens,omitempty"`
	Store          *bool                 `json:"store,omitempty"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponseFormat matches Chat Completions API response_format structure.
// For json_schema: {type:"json_schema", json_schema:{name,schema,strict}}
type openAIResponseFormat struct {
	Type       string             `json:"type"`
	JSONSchema *openAIJSONSchema  `json:"json_schema,omitempty"`
}

type openAIJSONSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

// --- Response types (Chat Completions API) ---

type openAIChatResponse struct {
	Choices []openAIChatChoice  `json:"choices"`
	Usage   *openAIUsage        `json:"usage,omitempty"`
	Error   *openAIErrorEnvelope `json:"error,omitempty"`
}

type openAIChatChoice struct {
	Message openAIChatMessage `json:"message"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIErrorEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// --- Request execution ---

func (c *OpenAIClient) doRequest(system, userMessage string, respFmt *openAIResponseFormat, opts ...CallOption) (string, error) {
	o := applyOpts(opts)
	store := false

	messages := []openAIChatMessage{}
	if system != "" {
		messages = append(messages, openAIChatMessage{Role: "system", Content: system})
	}
	messages = append(messages, openAIChatMessage{Role: "user", Content: userMessage})

	body := openAIChatRequest{
		Model:          c.model,
		Messages:       messages,
		MaxTokens:      o.maxTokens,
		Store:          &store,
		ResponseFormat: respFmt,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var apiResp openAIChatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		if apiResp.Error != nil {
			return "", fmt.Errorf("api error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
		}
		return "", fmt.Errorf("api error: http %d", resp.StatusCode)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("api error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	result := ""
	for _, choice := range apiResp.Choices {
		if strings.TrimSpace(choice.Message.Content) != "" {
			result = choice.Message.Content
			break
		}
	}
	result = strings.TrimSpace(result)
	if result == "" {
		return "", fmt.Errorf("empty response")
	}

	if OnUsage != nil && apiResp.Usage != nil {
		OnUsage(c.model, apiResp.Usage.PromptTokens, apiResp.Usage.CompletionTokens)
	}

	return result, nil
}

func normalizeOpenAIChatURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultOpenAIChatURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(baseURL, "/chat/completions"):
		return baseURL
	case strings.HasSuffix(baseURL, "/v1"):
		return baseURL + "/chat/completions"
	default:
		return baseURL + "/v1/chat/completions"
	}
}

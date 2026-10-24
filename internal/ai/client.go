package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Message struct {
	Role    string
	Content string
}

type Result struct {
	Text      string
	Reasoning string
	Model     string
}

type Client struct {
	config     Config
	httpClient *http.Client
}

func NewClient(config Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{config: config, httpClient: httpClient}
}

type chatRequestBody struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	Temperature     *float64      `json:"temperature,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponseBody struct {
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type Error struct {
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("ai: provider returned %d: %s", e.StatusCode, e.Message)
}

func (c *Client) Chat(ctx context.Context, task Task, messages []Message) (Result, error) {
	resolved := c.config.Resolve(task)
	if resolved.Model == "" {
		return Result{}, fmt.Errorf("ai: no model configured for task %q (Config.Default.Model and the task override are both empty)", task)
	}

	var body chatRequestBody
	body.Model = resolved.Model
	body.Temperature = resolved.Temperature
	body.ReasoningEffort = resolved.ReasoningEffort
	if resolved.SystemPrompt != "" {
		body.Messages = append(body.Messages, chatMessage{Role: "system", Content: resolved.SystemPrompt})
	}
	for _, m := range messages {
		body.Messages = append(body.Messages, chatMessage{Role: m.Role, Content: m.Content})
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Result{}, fmt.Errorf("ai: encoding request: %w", err)
	}

	url := c.config.URL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("ai: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("ai: reading response: %w", err)
	}

	var parsed chatResponseBody

	_ = json.Unmarshal(respBytes, &parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := string(respBytes)
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return Result{}, &Error{StatusCode: resp.StatusCode, Message: msg}
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return Result{}, &Error{StatusCode: resp.StatusCode, Message: parsed.Error.Message}
	}
	if len(parsed.Choices) == 0 {
		return Result{}, fmt.Errorf("ai: provider response had no choices (status %d, body %q)", resp.StatusCode, truncate(respBytes, 500))
	}

	visible, reasoning := StripReasoning(parsed.Choices[0].Message.Content)
	if visible == "" {

		reasoningNote := ""
		if reasoning != "" {
			reasoningNote = fmt.Sprintf(" (%d chars of reasoning were produced but no final answer)", len(reasoning))
		}
		return Result{}, fmt.Errorf("ai: provider returned an empty response%s — consider a larger context window (num_ctx) or lower reasoning effort", reasoningNote)
	}
	return Result{Text: visible, Reasoning: reasoning, Model: parsed.Model}, nil
}

type modelsResponseBody struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	url := c.config.URL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ai: building request: %w", err)
	}
	if c.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{StatusCode: resp.StatusCode, Message: truncate(respBytes, 500)}
	}

	var parsed modelsResponseBody
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("ai: decoding models response: %w", err)
	}

	ids := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

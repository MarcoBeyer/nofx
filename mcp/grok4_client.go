package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	ProviderGrok4       = "grok4"
	DefaultGrok4BaseURL = "https://api.x.ai/v1"
	DefaultGrok4Model   = "grok-4.20-multi-agent-experimental-beta-0304"
)

type Grok4Client struct {
	*Client
}

// NewGrok4Client creates Grok 4.20 multi-agent client (backward compatible)
func NewGrok4Client() AIClient {
	return NewGrok4ClientWithOptions()
}

// NewGrok4ClientWithOptions creates Grok 4.20 multi-agent client (supports options pattern)
func NewGrok4ClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Grok4 preset options
	grok4Opts := []ClientOption{
		WithProvider(ProviderGrok4),
		WithModel(DefaultGrok4Model),
		WithBaseURL(DefaultGrok4BaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(grok4Opts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Grok4 client
	grok4Client := &Grok4Client{
		Client: baseClient,
	}

	// 5. Set hooks to point to Grok4Client (implement dynamic dispatch)
	baseClient.hooks = grok4Client

	return grok4Client
}

func (c *Grok4Client) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] Grok4 API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] Grok4 using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] Grok4 using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] Grok4 using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] Grok4 using default Model: %s", c.Model)
	}
}

// Grok4 uses standard OpenAI-compatible API with Bearer auth
func (c *Grok4Client) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

// buildUrl overrides to use Responses API endpoint instead of /chat/completions
func (c *Grok4Client) buildUrl() string {
	if c.UseFullURL {
		return c.BaseURL
	}
	return fmt.Sprintf("%s/responses", c.BaseURL)
}

// buildMCPRequestBody overrides to use Responses API format
// Uses "input" array instead of "messages", and sets reasoning.effort = "low" for 4 agents
func (c *Grok4Client) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	// Build input array (Responses API format)
	input := []map[string]string{}

	if systemPrompt != "" {
		input = append(input, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	input = append(input, map[string]string{
		"role":    "user",
		"content": userPrompt,
	})

	requestBody := map[string]any{
		"model": c.Model,
		"input": input,
		"reasoning": map[string]string{
			"effort": "low", // 4 agents (quick research)
		},
	}

	return requestBody
}

// buildRequestBodyFromRequest overrides to use Responses API format with Request builder
func (c *Grok4Client) buildRequestBodyFromRequest(req *Request) map[string]any {
	// Convert messages to Responses API "input" format
	input := make([]map[string]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		input = append(input, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	requestBody := map[string]any{
		"model": req.Model,
		"input": input,
		"reasoning": map[string]string{
			"effort": "low", // 4 agents
		},
	}

	return requestBody
}

// parseMCPResponse overrides to parse the Responses API output format
func (c *Grok4Client) parseMCPResponse(body []byte) (string, error) {
	// Responses API returns output as an array of items
	var result struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Grok4 response: %w", err)
	}

	// Prefer output_text (convenience field from Responses API)
	if result.OutputText != "" {
		// Report token usage if callback is set
		if TokenUsageCallback != nil && result.Usage.TotalTokens > 0 {
			TokenUsageCallback(TokenUsage{
				Provider:         c.Provider,
				Model:            c.Model,
				PromptTokens:     result.Usage.InputTokens,
				CompletionTokens: result.Usage.OutputTokens,
				TotalTokens:      result.Usage.TotalTokens,
			})
		}
		return result.OutputText, nil
	}

	// Fallback: extract text from output array
	for _, item := range result.Output {
		if item.Type == "message" {
			for _, content := range item.Content {
				if content.Type == "output_text" && content.Text != "" {
					if TokenUsageCallback != nil && result.Usage.TotalTokens > 0 {
						TokenUsageCallback(TokenUsage{
							Provider:         c.Provider,
							Model:            c.Model,
							PromptTokens:     result.Usage.InputTokens,
							CompletionTokens: result.Usage.OutputTokens,
							TotalTokens:      result.Usage.TotalTokens,
						})
					}
					return content.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Grok4 API returned empty response")
}

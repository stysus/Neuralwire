package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"neuralwire/backend/internal/config"
)

// resolveAIConfig returns the effective API key, base URL and model for a
// call. Unless the endpoint is a local/test provider, the latest values are
// re-read from the .env file on every call so configuration changes apply
// without a restart. Local endpoints and the "test-key" sentinel keep the
// explicitly configured credentials so unit tests and self-hosted providers
// (ollama etc.) never read .env.
func resolveAIConfig(configuredKey, configuredModel, endpoint string) (apiKey, baseURL, model string) {
	if strings.Contains(endpoint, "127.0.0.1") || strings.Contains(endpoint, "localhost") || configuredKey == "test-key" {
		return configuredKey, "", configuredModel
	}

	apiKey, baseURL, model = config.LoadAIConfig()
	if apiKey == "" {
		apiKey = configuredKey
	}
	if model == "" {
		model = configuredModel
	}
	return apiKey, baseURL, model
}

// chatCompletion posts one chat request to an OpenAI-compatible endpoint and
// returns the assistant text. ok is false on any failure (transport, HTTP
// status, empty body) so callers can decide how to fall back.
func chatCompletion(
	ctx context.Context,
	client *http.Client,
	logger *log.Logger,
	endpoint, apiKey, model, system, user string,
	maxTokens int,
) (string, bool) {
	reqBody, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	})
	if err != nil {
		logger.Printf("ai: marshal request: %v", err)
		return "", false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		logger.Printf("ai: build request: %v", err)
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("ai: request failed: %v", err)
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		logger.Printf("ai: upstream returned %s: %s", resp.Status, string(body))
		return "", false
	}

	var parsed chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		logger.Printf("ai: decode response: %v", err)
		return "", false
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		logger.Printf("ai: empty response from upstream")
		return "", false
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), true
}

// generateImage posts one DALL-E-style image generation request and returns
// the generated image URL. ok is false on any failure so callers can fall
// back to stock imagery.
func generateImage(
	ctx context.Context,
	client *http.Client,
	logger *log.Logger,
	endpoint, apiKey, prompt string,
) (string, bool) {
	reqBody, err := json.Marshal(imageReq{
		Prompt: prompt,
		N:      1,
		Size:   "1024x1024",
		Model:  "dall-e-2",
	})
	if err != nil {
		return "", false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		logger.Printf("ai: DALL-E image generation failed (status %s): %s", resp.Status, string(body))
		return "", false
	}

	var parsed imageResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return "", false
	}
	if len(parsed.Data) == 0 || parsed.Data[0].URL == "" {
		return "", false
	}
	return parsed.Data[0].URL, true
}

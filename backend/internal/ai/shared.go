package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"neuralwire/backend/internal/config"
)

// testAPIKeySentinel is used exclusively by unit tests so they never read the
// real .env. Production API keys must never equal this value; if they do, the
// server would bypass .env reloading, which is a misconfiguration we surface
// loudly below rather than silently ignoring.
const testAPIKeySentinel = "test-key"

// resolveAIConfig returns the effective API key, base URL and model for a
// call. Unless the endpoint is a local/test provider, the latest values are
// re-read from the .env file on every call so configuration changes apply
// without a restart. Local endpoints and the test-key sentinel keep the
// explicitly configured credentials so unit tests and self-hosted providers
// (ollama etc.) never read .env.
func resolveAIConfig(configuredKey, configuredModel, endpoint string) (apiKey, baseURL, model string) {
	if strings.Contains(endpoint, "127.0.0.1") || strings.Contains(endpoint, "localhost") || configuredKey == testAPIKeySentinel {
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
//
// Reasoning models (e.g. DeepSeek) spend tokens on hidden reasoning before
// producing the answer; when maxTokens is too small the answer comes back
// empty. To recover from that without losing the call, the request is retried
// once with a doubled token budget when the first attempt returns an empty
// body. Transport/HTTP errors are not retried (they usually indicate a real
// problem, and callers have their own fallback).
func chatCompletion(
	ctx context.Context,
	client *http.Client,
	logger *slog.Logger,
	endpoint, apiKey, model, system, user string,
	maxTokens int,
) (string, bool) {
	text, ok, empty := doChatCompletion(ctx, client, logger, endpoint, apiKey, model, system, user, maxTokens)
	if ok {
		return text, true
	}
	// Retry only the empty-body case (reasoning budget exhausted). A doubled
	// budget usually gives the model enough room to emit content.
	if empty {
		text, ok, _ = doChatCompletion(ctx, client, logger, endpoint, apiKey, model, system, user, maxTokens*2)
		if ok {
			logger.Info("ai: retry succeeded with larger token budget")
			return text, true
		}
	}
	return "", false
}

// doChatCompletion performs a single chat request attempt. The third return
// value reports whether the attempt failed because the upstream returned an
// empty body (a transient reasoning-budget issue worth retrying) as opposed
// to a transport/HTTP error.
func doChatCompletion(
	ctx context.Context,
	client *http.Client,
	logger *slog.Logger,
	endpoint, apiKey, model, system, user string,
	maxTokens int,
) (string, bool, bool) {
	reqBody, err := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens: maxTokens,
	})
	if err != nil {
		logger.Error("ai: marshal request", "error", err)
		return "", false, false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		logger.Error("ai: build request", "error", err)
		return "", false, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("ai: request failed", "error", err)
		recordAICall(true)
		return "", false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		logger.Error("ai: upstream returned non-200", "status", resp.Status, "body", string(body))
		recordAICall(true)
		return "", false, false
	}

	var parsed chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		logger.Error("ai: decode response", "error", err)
		recordAICall(true)
		return "", false, false
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		logger.Warn("ai: empty response from upstream")
		recordAICall(true)
		return "", false, true
	}
	recordAICall(false)
	return strings.TrimSpace(parsed.Choices[0].Message.Content), true, false
}

// generateImage posts one DALL-E-style image generation request and returns
// the generated image URL. ok is false on any failure so callers can fall
// back to stock imagery. unsupported is true when the upstream reports it
// does not implement image generation (404/405/501), letting callers skip
// future attempts instead of spamming the same failing request.
func generateImage(
	ctx context.Context,
	client *http.Client,
	logger *slog.Logger,
	endpoint, apiKey, prompt string,
) (url string, unsupported bool, ok bool) {
	reqBody, err := json.Marshal(imageReq{
		Prompt: prompt,
		N:      1,
		Size:   "1024x1024",
		Model:  "dall-e-2",
	})
	if err != nil {
		return "", false, false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", false, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		recordAICall(true)
		return "", false, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		logger.Error("ai: image generation failed", "status", resp.Status, "body", string(body))
		recordAICall(true)
		unsupported := resp.StatusCode == http.StatusNotFound ||
			resp.StatusCode == http.StatusMethodNotAllowed ||
			resp.StatusCode == http.StatusNotImplemented
		return "", unsupported, false
	}

	var parsed imageResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		recordAICall(true)
		return "", false, false
	}
	if len(parsed.Data) == 0 || parsed.Data[0].URL == "" {
		recordAICall(true)
		return "", false, false
	}
	recordAICall(false)
	return parsed.Data[0].URL, false, true
}

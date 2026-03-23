package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"

	"github.com/mikyk10/wisp/app/domain/ai"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

type descriptorClient struct {
	cfg    *config.GlobalConfig
	prompt *Prompt
}

// NewDescriptorClient constructs a DescriptorClient.
// If cfg.AI.DescriptorPromptPath is set, the prompt is loaded from that file;
// otherwise the built-in embedded prompt is used.
func NewDescriptorClient(cfg *config.GlobalConfig) (ai.DescriptorClient, error) {
	var prompt *Prompt
	if cfg.AI.DescriptorPromptPath != "" {
		var err error
		prompt, err = LoadPrompt(cfg.AI.DescriptorPromptPath)
		if err != nil {
			return nil, fmt.Errorf("descriptor client: %w", err)
		}
	} else {
		prompt = DefaultDescriptorPrompt()
	}
	return &descriptorClient{cfg: cfg, prompt: prompt}, nil
}

func (c *descriptorClient) Validate() error {
	if _, ok := c.cfg.AI.Providers[c.prompt.Config.Provider]; !ok {
		return fmt.Errorf("provider %q not found in ai.providers config", c.prompt.Config.Provider)
	}
	return nil
}

func (c *descriptorClient) PromptModel() string {
	return c.prompt.Config.Model
}

// WithPromptPath returns a new DescriptorClient that loads its prompt from path.
// The original client is unchanged.
func (c *descriptorClient) WithPromptPath(path string) (ai.DescriptorClient, error) {
	prompt, err := LoadPrompt(path)
	if err != nil {
		return nil, fmt.Errorf("descriptor: load prompt from %q: %w", path, err)
	}
	return &descriptorClient{cfg: c.cfg, prompt: prompt}, nil
}

func (c *descriptorClient) Describe(ctx context.Context, thumbJPEG []byte) (string, error) {
	provCfg, ok := c.cfg.AI.Providers[c.prompt.Config.Provider]
	if !ok {
		return "", fmt.Errorf("descriptor: provider %q not found in config", c.prompt.Config.Provider)
	}

	provider, err := newProvider(c.prompt.Config, provCfg, c.cfg.AI.RequestTimeoutSec)
	if err != nil {
		return "", fmt.Errorf("descriptor: create provider: %w", err)
	}

	imageB64 := base64.StdEncoding.EncodeToString(thumbJPEG)
	imageDataURL := "data:image/jpeg;base64," + imageB64

	params := anyllm.CompletionParams{
		Model:           c.prompt.Config.Model,
		MaxTokens:       c.prompt.Config.MaxTokens,
		ReasoningEffort: anyllm.ReasoningEffortNone,
		Messages: []anyllm.Message{
			{
				Role: anyllm.RoleUser,
				Content: []anyllm.ContentPart{
					{Type: "text", Text: c.prompt.Body},
					{Type: "image_url", ImageURL: &anyllm.ImageURL{URL: imageDataURL}},
				},
			},
		},
	}

	slog.Info("descriptor: request",
		"provider", c.prompt.Config.Provider,
		"model", c.prompt.Config.Model,
		"endpoint", provCfg.Endpoint,
		"max_tokens", c.prompt.Config.MaxTokens,
		"reasoning_effort", string(params.ReasoningEffort),
		"prompt_length", len(c.prompt.Body),
		"image_size_bytes", len(thumbJPEG),
		"image_b64_length", len(imageB64),
		"max_retries", c.cfg.AI.MaxRetries,
		"timeout_sec", c.cfg.AI.RequestTimeoutSec,
	)

	var result string
	var lastErr error
	maxRetries := c.cfg.AI.MaxRetries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := backoffDuration(attempt)
			slog.Debug("descriptor: retrying", "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		callCtx, callCancel := context.WithTimeout(ctx, time.Duration(c.cfg.AI.RequestTimeoutSec)*time.Second)
		start := time.Now()
		resp, err := provider.Completion(callCtx, params)
		elapsed := time.Since(start)
		callCancel()
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			lastErr = err
			logCompletionError("descriptor", attempt, elapsed, callCtx.Err() != nil, c.cfg.AI.RequestTimeoutSec, err)
			continue
		}
		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("descriptor: empty response choices")
			slog.Warn("descriptor: empty choices", "attempt", attempt, "elapsed", elapsed)
			continue
		}

		result = strings.TrimSpace(resp.Choices[0].Message.ContentString())
		attrs := logResponseAttrs(attempt, elapsed, resp.Choices[0], resp.Usage)
		attrs = append(attrs, "result_length", len(result))
		slog.Info("descriptor: response", attrs...)
		return result, nil
	}

	return "", fmt.Errorf("descriptor: all %d retries exhausted: %w", maxRetries, lastErr)
}

// providerAPIKey returns the API key or a placeholder if empty (some local servers don't require one).
func providerAPIKey(key string) string {
	if key == "" {
		return "none"
	}
	return key
}

// backoffDuration returns exponential backoff with jitter: base * 2^(attempt-1) + jitter.
func backoffDuration(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt-1)) * time.Second //nolint:gosec // attempt is always >= 1, safe to cast
	jitter := time.Duration(rand.Int64N(int64(500 * time.Millisecond)))
	return base + jitter
}

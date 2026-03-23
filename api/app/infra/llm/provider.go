package llm

import (
	"log/slog"
	"net/http"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers"
	openaicompat "github.com/mozilla-ai/any-llm-go/providers/openai"

	"github.com/mikyk10/wisp/app/domain/model/config"
)

// newProvider creates an OpenAI-compatible provider with the standard
// transport (request field injection + debug/error logging).
func newProvider(promptCfg PromptConfig, provCfg config.AIProviderConfig, timeoutSec int) (*openaicompat.CompatibleProvider, error) {
	return openaicompat.NewCompatible(
		openaicompat.CompatibleConfig{
			Name:           promptCfg.Provider,
			DefaultBaseURL: provCfg.Endpoint,
			RequireAPIKey:  false,
			DefaultAPIKey:  "none",
		},
		anyllm.WithBaseURL(provCfg.Endpoint),
		anyllm.WithAPIKey(providerAPIKey(provCfg.APIKey)),
		anyllm.WithHTTPClient(&http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
			Transport: &llmTransport{
				base:        http.DefaultTransport,
				extraFields: map[string]any{"reasoning_effort": "none"},
			},
		}),
	)
}

// logResponseAttrs builds common slog attributes from a completion response.
func logResponseAttrs(attempt int, elapsed time.Duration, choice providers.Choice, usage *providers.Usage) []any {
	attrs := []any{
		"attempt", attempt,
		"elapsed", elapsed,
		"finish_reason", choice.FinishReason,
	}
	if usage != nil {
		attrs = append(attrs,
			"prompt_tokens", usage.PromptTokens,
			"completion_tokens", usage.CompletionTokens,
			"total_tokens", usage.TotalTokens,
		)
	}
	return attrs
}

// logCompletionError logs a failed completion attempt with timeout info.
func logCompletionError(prefix string, attempt int, elapsed time.Duration, timedOut bool, timeoutSec int, err error) {
	slog.Warn(prefix+": completion failed",
		"attempt", attempt,
		"elapsed", elapsed,
		"timed_out", timedOut,
		"timeout_sec", timeoutSec,
		"err", err,
	)
}

package llm

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"

	anyllm "github.com/mozilla-ai/any-llm-go"
	openaicompat "github.com/mozilla-ai/any-llm-go/providers/openai"

	"github.com/mikyk10/wisp/app/domain/ai"
	"github.com/mikyk10/wisp/app/domain/model/config"
)

// bannedWords are stop-words and meta-terms that should never appear as tags.
var bannedWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "in": {}, "on": {}, "at": {},
	"of": {}, "to": {}, "with": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "has": {}, "have": {}, "had": {}, "do": {}, "does": {},
	"did": {}, "that": {}, "this": {}, "it": {}, "its": {}, "by": {}, "for": {},
	"as": {}, "image": {}, "photo": {}, "picture": {}, "photograph": {},
	"shows": {}, "features": {}, "depicts": {},
}

// validTagRe matches a single valid tag token: one or more lowercase ASCII letters.
var validTagRe = regexp.MustCompile(`^[a-z]+$`)

type taggerClient struct {
	cfg    *config.GlobalConfig
	prompt *Prompt
}

// NewTaggerClient constructs a TaggerClient.
// If cfg.AI.TaggerPromptPath is set, the prompt is loaded from that file;
// otherwise the built-in embedded prompt is used.
func NewTaggerClient(cfg *config.GlobalConfig) (ai.TaggerClient, error) {
	var prompt *Prompt
	if cfg.AI.TaggerPromptPath != "" {
		var err error
		prompt, err = LoadPrompt(cfg.AI.TaggerPromptPath)
		if err != nil {
			return nil, fmt.Errorf("tagger client: %w", err)
		}
	} else {
		prompt = DefaultTaggerPrompt()
	}
	return &taggerClient{cfg: cfg, prompt: prompt}, nil
}

type taggerTemplateVars struct {
	MaxTags     int
	Description string
}

func (c *taggerClient) Validate() error {
	if _, ok := c.cfg.AI.Providers[c.prompt.Config.Provider]; !ok {
		return fmt.Errorf("provider %q not found in ai.providers config", c.prompt.Config.Provider)
	}
	return nil
}

func (c *taggerClient) PromptModel() string {
	return c.prompt.Config.Model
}

// WithPromptPath returns a new TaggerClient that loads its prompt from path.
// The original client is unchanged.
func (c *taggerClient) WithPromptPath(path string) (ai.TaggerClient, error) {
	prompt, err := LoadPrompt(path)
	if err != nil {
		return nil, fmt.Errorf("tagger: load prompt from %q: %w", path, err)
	}
	return &taggerClient{cfg: c.cfg, prompt: prompt}, nil
}

func (c *taggerClient) Tag(ctx context.Context, description string) ([]string, error) {
	provCfg, ok := c.cfg.AI.Providers[c.prompt.Config.Provider]
	if !ok {
		return nil, fmt.Errorf("tagger: provider %q not found in config", c.prompt.Config.Provider)
	}

	// Expand template variables in the prompt body.
	tmpl, err := template.New("tagger").Parse(c.prompt.Body)
	if err != nil {
		return nil, fmt.Errorf("tagger: parse prompt template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, taggerTemplateVars{
		MaxTags:     c.cfg.AI.MaxTags,
		Description: description,
	}); err != nil {
		return nil, fmt.Errorf("tagger: execute prompt template: %w", err)
	}
	promptText := buf.String()

	provider, err := openaicompat.NewCompatible(
		openaicompat.CompatibleConfig{
			Name:           c.prompt.Config.Provider,
			DefaultBaseURL: provCfg.Endpoint,
			RequireAPIKey:  false,
			DefaultAPIKey:  "none",
		},
		anyllm.WithBaseURL(provCfg.Endpoint),
		anyllm.WithAPIKey(providerAPIKey(provCfg.APIKey)),
		anyllm.WithTimeout(time.Duration(c.cfg.AI.RequestTimeoutSec)*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("tagger: create provider: %w", err)
	}

	params := anyllm.CompletionParams{
		Model:     c.prompt.Config.Model,
		MaxTokens: c.prompt.Config.MaxTokens,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: promptText},
		},
	}

	maxRetries := c.cfg.AI.MaxRetries
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := backoffDuration(attempt)
			slog.Debug("tagger: retrying", "attempt", attempt, "backoff", backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := provider.Completion(ctx, params)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			slog.Warn("tagger: completion failed", "attempt", attempt, "err", err)
			continue
		}
		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("tagger: empty response choices")
			continue
		}

		raw := strings.TrimSpace(resp.Choices[0].Message.ContentString())
		tags, err := normalizeTags(raw, c.cfg.AI.MaxTags)
		if err != nil {
			lastErr = fmt.Errorf("tagger: format invalid on attempt %d: %w", attempt, err)
			slog.Warn("tagger: invalid tag format, retrying", "attempt", attempt, "raw", raw)
			continue
		}
		return tags, nil
	}

	return nil, fmt.Errorf("tagger: all %d retries exhausted: %w", maxRetries, lastErr)
}

// normalizeTags parses and normalizes a space-separated tag string.
// Returns an error if no valid tags could be extracted (format validation).
func normalizeTags(raw string, maxTags int) ([]string, error) {
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || r == ','
	})

	seen := make(map[string]struct{})
	var result []string
	for _, tok := range tokens {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		// Keep only lowercase a-z tokens.
		if !validTagRe.MatchString(tok) {
			continue
		}
		// Skip banned words.
		if _, banned := bannedWords[tok]; banned {
			continue
		}
		// Dedup.
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		result = append(result, tok)
		if len(result) >= maxTags {
			break
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid tags extracted from: %q", raw)
	}
	return result, nil
}

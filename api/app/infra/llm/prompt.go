package llm

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // sha1 used for prompt versioning, not cryptography
	"embed"
	"fmt"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

//go:embed prompts/descriptor_v1.md prompts/tagger_v1.md prompts/default_gen_meta.md prompts/default_gen_image.md
var embeddedPrompts embed.FS

// API type constants for prompt frontmatter.
const (
	ApiTypeChat            = "chat"             // chat completion (default)
	ApiTypeImageGeneration = "image_generation" // /v1/images/generations
	ApiTypeImageEdit       = "image_edit"       // /v1/images/edits (img2img)
	ApiTypeComfyUI         = "comfyui"          // ComfyUI (future)
)

// PromptMeta holds the YAML front-matter of a prompt file.
type PromptMeta struct {
	Version     string  `yaml:"version"`
	Stage       string  `yaml:"stage"`
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	ApiType     string  `yaml:"api_type"`    // "chat" (default), "image_generation", "comfyui"
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
	Size        string  `yaml:"size"`    // image_generation only
	Quality     string  `yaml:"quality"` // image_generation only
}

// Prompt represents a parsed prompt file.
type Prompt struct {
	Meta PromptMeta
	Body string // raw template body
	Hash string // first 12 chars of SHA1 of body
}

// LoadPrompt reads and parses a prompt from an external file path.
func LoadPrompt(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load prompt %s: %w", path, err)
	}
	return ParsePrompt(string(data))
}

// LoadEmbeddedPrompt reads a prompt from the embedded filesystem.
func LoadEmbeddedPrompt(name string) (*Prompt, error) {
	data, err := embeddedPrompts.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("load embedded prompt %s: %w", name, err)
	}
	return ParsePrompt(string(data))
}

// DefaultDescriptorPrompt returns the embedded default descriptor prompt.
func DefaultDescriptorPrompt() *Prompt {
	p, err := LoadEmbeddedPrompt("prompts/descriptor_v1.md")
	if err != nil {
		panic(fmt.Sprintf("missing embedded descriptor prompt: %v", err))
	}
	return p
}

// DefaultTaggerPrompt returns the embedded default tagger prompt.
func DefaultTaggerPrompt() *Prompt {
	p, err := LoadEmbeddedPrompt("prompts/tagger_v1.md")
	if err != nil {
		panic(fmt.Sprintf("missing embedded tagger prompt: %v", err))
	}
	return p
}

// ResolvePrompt loads a prompt from an external path if provided, otherwise from embedded.
// If path is empty, falls back to the embedded prompt with the given name.
func ResolvePrompt(path, embeddedName string) (*Prompt, error) {
	if path != "" {
		return LoadPrompt(path)
	}
	return LoadEmbeddedPrompt(embeddedName)
}

// ParsePrompt parses a prompt string with YAML front-matter.
func ParsePrompt(content string) (*Prompt, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return &Prompt{Body: content, Hash: hashBody(content)}, nil
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid prompt format: missing closing ---")
	}

	var meta PromptMeta
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, fmt.Errorf("parse prompt front-matter: %w", err)
	}

	body := strings.TrimSpace(parts[2])
	return &Prompt{
		Meta: meta,
		Body: body,
		Hash: hashBody(body),
	}, nil
}

// StageOutput holds the resolved outputs from pipeline stages.
type StageOutput struct {
	Text  string
	Image []byte
}

// TemplateData is the data available to prompt templates.
type TemplateData struct {
	Prev   StageOutput
	Stages map[string]StageOutput
	Config map[string]any
}

// RenderPrompt renders a prompt template with the given data.
func RenderPrompt(body string, data TemplateData) (string, error) {
	tmpl, err := template.New("prompt").Parse(body)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}

	ctx := map[string]any{
		"prev": map[string]any{
			"output": data.Prev.Text,
		},
		"config": data.Config,
	}

	stages := make(map[string]any)
	for name, out := range data.Stages {
		stages[name] = map[string]any{
			"output": out.Text,
		}
	}
	ctx["stages"] = stages

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}
	return buf.String(), nil
}

func hashBody(body string) string {
	h := sha1.Sum([]byte(body))
	return fmt.Sprintf("%x", h)[:12]
}

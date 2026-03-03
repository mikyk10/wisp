package llm

import (
	"bufio"
	"crypto/sha1" //nolint:gosec // sha1 used for prompt versioning, not cryptography
	"embed"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

//go:embed prompts/descriptor_v2.md prompts/tagger_v2.md
var embeddedPrompts embed.FS

// PromptConfig is the parsed YAML front-matter of a prompt file.
type PromptConfig struct {
	Version   string `yaml:"version"`
	Stage     string `yaml:"stage"`
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	MaxTokens *int   `yaml:"max_tokens"`
}

// Prompt holds a loaded prompt with its config and body.
type Prompt struct {
	Config PromptConfig
	Body   string // prompt text without the YAML front-matter
	Hash   string // SHA1(Body)[:12]
}

// LoadPrompt reads a prompt from an external file path.
func LoadPrompt(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load prompt %s: %w", path, err)
	}
	return parsePrompt(data)
}

// DefaultDescriptorPrompt returns the embedded default descriptor prompt.
func DefaultDescriptorPrompt() *Prompt {
	data, err := embeddedPrompts.ReadFile("prompts/descriptor_v2.md")
	if err != nil {
		panic(fmt.Sprintf("missing embedded descriptor prompt: %v", err))
	}
	p, err := parsePrompt(data)
	if err != nil {
		panic(fmt.Sprintf("invalid embedded descriptor prompt: %v", err))
	}
	return p
}

// DefaultTaggerPrompt returns the embedded default tagger prompt.
func DefaultTaggerPrompt() *Prompt {
	data, err := embeddedPrompts.ReadFile("prompts/tagger_v2.md")
	if err != nil {
		panic(fmt.Sprintf("missing embedded tagger prompt: %v", err))
	}
	p, err := parsePrompt(data)
	if err != nil {
		panic(fmt.Sprintf("invalid embedded tagger prompt: %v", err))
	}
	return p
}

// parsePrompt splits YAML front-matter from body and builds a Prompt.
func parsePrompt(data []byte) (*Prompt, error) {
	content := string(data)

	// Expect the file to start with "---\n".
	const delimiter = "---"
	if !strings.HasPrefix(strings.TrimSpace(content), delimiter) {
		return nil, fmt.Errorf("prompt file must start with YAML front-matter (---)")
	}

	// Find the closing "---" delimiter.
	scanner := bufio.NewScanner(strings.NewReader(content))
	var (
		frontLines []string
		bodyLines  []string
		inFront    bool
		closed     bool
		lineNum    int
	)
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		if lineNum == 1 && line == delimiter {
			inFront = true
			continue
		}
		if inFront && line == delimiter {
			inFront = false
			closed = true
			continue
		}
		if inFront {
			frontLines = append(frontLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	if !closed {
		return nil, fmt.Errorf("prompt file front-matter not closed with ---")
	}

	var cfg PromptConfig
	if err := yaml.Unmarshal([]byte(strings.Join(frontLines, "\n")), &cfg); err != nil {
		return nil, fmt.Errorf("parse prompt front-matter: %w", err)
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))

	//nolint:gosec // sha1 used for prompt versioning only
	sum := sha1.Sum([]byte(body))
	hash := fmt.Sprintf("%x", sum)[:12]

	return &Prompt{
		Config: cfg,
		Body:   body,
		Hash:   hash,
	}, nil
}

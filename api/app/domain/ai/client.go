package ai

import "context"

// DescriptorClient generates a textual description of a photo from its JPEG thumbnail.
type DescriptorClient interface {
	// Validate checks that the client is properly configured (e.g. the provider exists in config).
	// Call this before starting a batch to detect misconfiguration early.
	Validate() error
	Describe(ctx context.Context, thumbJPEG []byte) (string, error)
	// PromptModel returns the model identifier used for run records.
	PromptModel() string
	// WithPromptPath returns a new client that loads its prompt from the given file path.
	// The original client is unchanged.
	WithPromptPath(path string) (DescriptorClient, error)
}

// TaggerClient produces tags from a textual description of a photo.
type TaggerClient interface {
	// Validate checks that the client is properly configured (e.g. the provider exists in config).
	// Call this before starting a batch to detect misconfiguration early.
	Validate() error
	Tag(ctx context.Context, description string) ([]string, error)
	// PromptModel returns the model identifier used for run records.
	PromptModel() string
	// WithPromptPath returns a new client that loads its prompt from the given file path.
	// The original client is unchanged.
	WithPromptPath(path string) (TaggerClient, error)
}

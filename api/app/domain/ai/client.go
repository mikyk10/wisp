package ai

import "context"

// DescriptorClient generates a textual description of a photo from its JPEG thumbnail.
type DescriptorClient interface {
	Describe(ctx context.Context, thumbJPEG []byte) (string, error)
	// PromptModel returns the model identifier used for run records.
	PromptModel() string
}

// TaggerClient produces tags from a textual description of a photo.
type TaggerClient interface {
	Tag(ctx context.Context, description string) ([]string, error)
	// PromptModel returns the model identifier used for run records.
	PromptModel() string
}

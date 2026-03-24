package ai

import "context"

// StageExecutor executes a single pipeline stage.
// The implementation is chosen based on the combination of output type and api_type.
type StageExecutor interface {
	Execute(ctx context.Context, prompt string, images [][]byte) (*StageResult, error)
}

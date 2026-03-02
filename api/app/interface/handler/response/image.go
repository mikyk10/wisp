package response

import (
	"github.com/mikyk10/wisp/app/domain/model"
)

type Image struct {
	ID        model.PrimaryKey `json:"id"`
	Enabled   bool             `json:"enabled"`
	Timestamp string           `json:"timestamp"`
}

type Error struct {
	Message string `json:"message"`
}

type errorReponse struct {
	Error innerError `json:"error"`
}
type innerError struct {
	Message string `json:"description"`
	TraceID string `json:"trace_id"`
}

func NewErrorResponse(err error, traceID string) *errorReponse {
	return &errorReponse{Error: innerError{
		Message: err.Error(),
		TraceID: traceID,
	}}
}

package response

import (
	"github.com/mikyk10/wisp/app/domain/model"
)

type Image struct {
	ID        model.PrimaryKey `json:"id"`
	Enabled   bool             `json:"enabled"`
	Timestamp string           `json:"timestamp"`

	// Tags is always an array, never null: a photo with no tags has an empty
	// one, and a client should not have to tell that apart from a field the
	// server forgot to send.
	//
	// They ride along with the listing rather than being asked for per photo.
	// The grid shows hundreds of cards at once, so anything it needs per card
	// is either here or it is a request per card.
	Tags []string `json:"tags"`
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

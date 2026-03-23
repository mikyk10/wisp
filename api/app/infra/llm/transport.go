package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// llmTransport wraps an http.RoundTripper to:
//   - inject extra top-level JSON fields into POST request bodies
//   - log request bodies at debug level
//   - log error response bodies at warn level
type llmTransport struct {
	base       http.RoundTripper
	extraFields map[string]any
}

func (t *llmTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
	}

	// Inject extra fields into JSON body for POST requests.
	if req.Method == http.MethodPost && len(t.extraFields) > 0 && len(reqBody) > 0 {
		var body map[string]any
		if err := json.Unmarshal(reqBody, &body); err == nil {
			for k, v := range t.extraFields {
				body[k] = v
			}
			if patched, err := json.Marshal(body); err == nil {
				reqBody = patched
			}
		}
	}

	req.Body = io.NopCloser(bytes.NewReader(reqBody))
	req.ContentLength = int64(len(reqBody))

	slog.Debug("llm: http request",
		"method", req.Method,
		"url", req.URL.String(),
		"body_length", len(reqBody),
		"body", string(reqBody),
	)

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slog.Warn("llm: http transport error",
			"method", req.Method,
			"url", req.URL.String(),
			"err", err,
		)
		return resp, err
	}

	if resp.StatusCode >= 400 {
		var respBody []byte
		if resp.Body != nil {
			respBody, _ = io.ReadAll(resp.Body)
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
		}
		slog.Warn("llm: http error response",
			"method", req.Method,
			"url", req.URL.String(),
			"status", resp.StatusCode,
			"response_body", string(respBody),
		)
	}

	return resp, err
}

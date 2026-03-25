package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// reasoningTransport injects extra JSON fields into POST request bodies.
// Used to force reasoning_effort=none for models like Qwen3.5 that default
// to thinking mode when the field is absent.
//
// TODO: This is a blunt workaround — it intercepts all outgoing requests and
// mutates the body at the HTTP transport layer because AnyLLM does not expose
// a per-request option for reasoning_effort. If AnyLLM (or a replacement SDK)
// gains native support for this parameter, or if we move to per-provider
// client construction, this transport hack and its host-based skip list
// should be removed in favour of that cleaner approach.
type reasoningTransport struct {
	base        http.RoundTripper
	extraFields map[string]any
}

func newReasoningTransport(base http.RoundTripper, fields map[string]any) *reasoningTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &reasoningTransport{base: base, extraFields: fields}
}

func (t *reasoningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || req.Body == nil || len(t.extraFields) == 0 {
		return t.base.RoundTrip(req)
	}

	// Skip injection for OpenAI — it rejects unknown fields like reasoning_effort.
	if req.URL != nil && strings.Contains(req.URL.Host, "openai.com") {
		return t.base.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		// Not JSON, send as-is
		req.Body = io.NopCloser(bytes.NewReader(body))
		return t.base.RoundTrip(req)
	}

	for k, v := range t.extraFields {
		m[k] = v
	}

	modified, err := json.Marshal(m)
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return t.base.RoundTrip(req)
	}

	req.Body = io.NopCloser(bytes.NewReader(modified))
	req.ContentLength = int64(len(modified))
	return t.base.RoundTrip(req)
}

package provider

import "net/http"

// headersTransport adds a fixed set of headers to every outgoing request.
// The SDK wraps it with its own auth transport, so reserved headers are
// rejected at config validation rather than filtered here.
type headersTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *headersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// The RoundTripper contract forbids mutating the caller's request.
	req = req.Clone(req.Context())
	for name, value := range t.headers {
		req.Header.Set(name, value)
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

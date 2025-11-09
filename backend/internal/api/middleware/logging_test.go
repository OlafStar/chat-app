package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type hijackableRecorder struct {
	http.ResponseWriter
	hijacked bool
	err      error
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, h.err
}

func (h *hijackableRecorder) Flush() {
	if f, ok := h.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func TestLoggingMiddlewarePreservesHijacker(t *testing.T) {
	expectedErr := errors.New("hijack invoked")
	recorder := &hijackableRecorder{
		ResponseWriter: httptest.NewRecorder(),
		err:            expectedErr,
	}

	handlerCalled := false
	handler := Logging()(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("response writer should implement http.Hijacker")
		}
		if _, _, err := hj.Hijack(); !errors.Is(err, expectedErr) {
			t.Fatalf("unexpected hijack error: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(recorder, req)

	if !handlerCalled {
		t.Fatal("inner handler was not invoked")
	}
	if !recorder.hijacked {
		t.Fatal("underlying Hijack was not called")
	}
}

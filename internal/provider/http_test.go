package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGetJSONRetriesTransientFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(writer, "busy", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := getJSON(server.Client(), request, &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || calls.Load() != 2 {
		t.Fatalf("unexpected retry result: ok=%v calls=%d", result.OK, calls.Load())
	}
}

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func getJSON(client *http.Client, request *http.Request, target any) error {
	for attempt := 0; attempt < 2; attempt++ {
		response, err := client.Do(request.Clone(request.Context()))
		if err != nil {
			return err
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			defer response.Body.Close()
			decoder := json.NewDecoder(io.LimitReader(response.Body, 8<<20))
			if err := decoder.Decode(target); err != nil {
				return fmt.Errorf("decode upstream response: %w", err)
			}
			return nil
		}
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		response.Body.Close()
		if attempt == 0 && retryable(response.StatusCode) {
			delay := retryDelay(response.Header.Get("Retry-After"))
			timer := time.NewTimer(delay)
			select {
			case <-request.Context().Done():
				timer.Stop()
				return request.Context().Err()
			case <-timer.C:
				continue
			}
		}
		return fmt.Errorf("upstream returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("upstream request failed after retry")
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func retryDelay(header string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(header))
	if err == nil && seconds > 0 {
		delay := time.Duration(seconds) * time.Second
		if delay <= 2*time.Second {
			return delay
		}
	}
	return 300 * time.Millisecond
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		return ""
	}
}

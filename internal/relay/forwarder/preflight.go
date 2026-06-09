package forwarder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func Preflight(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode >= 500 {
		return fmt.Errorf("upstream server error: %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return errors.New("rate limit exceeded")
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New("authentication failed")
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			var errorBody map[string]any
			if json.Unmarshal(body, &errorBody) == nil {
				if msg, ok := errorBody["error"].(map[string]any); ok {
					if message, ok := msg["message"].(string); ok {
						return fmt.Errorf("upstream error: %s", message)
					}
				}
				if msg, ok := errorBody["error"].(string); ok {
					return fmt.Errorf("upstream error: %s", msg)
				}
			}
		}
	}

	return fmt.Errorf("upstream returned status %d", resp.StatusCode)
}

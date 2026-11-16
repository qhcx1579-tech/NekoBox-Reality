package libcore

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const exitIPURL = "https://my.ippure.com/v1/info"

// Expose only fixed error codes across IPC; never return remote bodies or node details.
func fetchExitIP(client *http.Client) (string, error) {
	client.Timeout = 10 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("exit_ip_service_error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exitIPURL, nil)
	if err != nil {
		return "", errors.New("exit_ip_service_error")
	}
	req.Header.Set("User-Agent", "NekoBox-Reality")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	response, err := client.Do(req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "reality verification failed") {
			return "", errors.New("exit_ip_reality_failed")
		}
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return "", errors.New("exit_ip_timeout")
		}
		return "", errors.New("exit_ip_service_error")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New("exit_ip_service_error")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 {
		return "", errors.New("exit_ip_invalid_response")
	}
	return parseExitIP(string(body))
}

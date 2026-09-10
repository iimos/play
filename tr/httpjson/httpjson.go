// Package httpjson предоставляет общий HTTP-клиент для получения JSON с MOEX ISS:
// ретраи транзиентных ошибок и вспомогательные функции форматирования.
package httpjson

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GetJSON выполняет GET-запрос и возвращает тело ответа. Транзиентные ошибки
// (сеть, 5xx, 429, 408) ретраятся с нарастающей паузой, остальные 4xx — нет.
func GetJSON(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return body, nil
		}

		lastErr = fmt.Errorf("%s: http %d", u, resp.StatusCode)
		if !retriableStatus(resp.StatusCode) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

// retriableStatus возвращает true для транзиентных HTTP-статусов.
func retriableStatus(code int) bool {
	return code >= 500 || code == http.StatusTooManyRequests || code == http.StatusRequestTimeout
}

// Truncate обрезает строку до n символов, добавляя «...».
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}

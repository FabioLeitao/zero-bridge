package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func healthcheck(raw string, stderr io.Writer) int {
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Get(raw)
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: healthcheck: %v\n", err)
		return 1
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1024))
	if err != nil {
		fmt.Fprintf(stderr, "zero-bridge: healthcheck: %v\n", err)
		return 1
	}
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"ok":true`) {
		fmt.Fprintf(stderr, "zero-bridge: healthcheck: status %d\n", res.StatusCode)
		return 1
	}
	return 0
}

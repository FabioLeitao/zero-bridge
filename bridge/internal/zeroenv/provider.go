package zeroenv

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const graniteTag = "granite4.2:3b-cpu-ctx16k"

const defaultTagsURL = "http://127.0.0.1:11434/api/tags"

// ProviderEnv returns extra environment entries for a zero acp child (e.g. ZERO_PROVIDER).
func ProviderEnv() []string {
	return ProviderEnvWith(Config{Client: http.DefaultClient})
}

// Config controls how ProviderEnvWith resolves the active local model tag.
type Config struct {
	TagsURL    string
	Client     *http.Client
	GraniteTag string
}

// ProviderEnvWith is like ProviderEnv but allows injecting the HTTP client (tests).
func ProviderEnvWith(cfg Config) []string {
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}
	if cfg.TagsURL == "" {
		cfg.TagsURL = defaultTagsURL
	}
	if cfg.GraniteTag == "" {
		cfg.GraniteTag = graniteTag
	}
	body, err := fetchTagsBody(cfg.Client, cfg.TagsURL)
	if err == nil && strings.Contains(body, cfg.GraniteTag) {
		return []string{"ZERO_PROVIDER=granite-local"}
	}
	return nil
}

func fetchTagsBody(client *http.Client, url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama status %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

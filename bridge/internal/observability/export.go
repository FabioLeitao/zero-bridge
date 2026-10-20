package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// InsecureExportEnabled is the explicit opt-in for plaintext OTLP.
// The values "1" and "true" turn it on. Anything else, including empty, stays on TLS.
func InsecureExportEnabled() bool {
	v := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"))
	return v == "1" || strings.EqualFold(v, "true")
}

var (
	allowedRoutes = map[string]struct{}{
		"/health":                  {},
		"/v1/sessions/{id}/prompt": {},
	}
	allowedHTTPMethods = map[string]struct{}{
		"GET": {}, "POST": {}, "PUT": {}, "DELETE": {}, "HEAD": {},
	}
	allowedMetrics = map[string]struct{}{
		"http.request.duration_ms": {},
		"http.request.count":       {},
		"acp.call.duration_ms":     {},
		"acp.call.count":           {},
	}
	acpMethod = regexp.MustCompile(`^[a-z][a-z0-9./_-]{0,40}$`)
)

// UseEndpoint installs an exporter for raw. https uses the default TLS
// stack, at least TLS 1.2. http is refused unless InsecureExportEnabled.
// The payload keeps route, method, status, ACP method, and the error flag.
// A prompt, a path, a token, and a session id are not fields of that payload.
func UseEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return fmt.Errorf("otlp endpoint rejected")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !InsecureExportEnabled() {
			return fmt.Errorf("plaintext otlp needs OTEL_EXPORTER_OTLP_INSECURE=true")
		}
	default:
		return fmt.Errorf("otlp endpoint rejected")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	exp := &exporter{client: client, url: u.String(), ch: make(chan []byte, 32)}
	go exp.loop()
	SetGlobal(exp)
	return nil
}

type exporter struct {
	client *http.Client
	url    string
	ch     chan []byte
}

type exportRecord struct {
	Name       string            `json:"name,omitempty"`
	Value      *float64          `json:"value,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func (e *exporter) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, func()) {
	cfg := &SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	attrs := keepAttrs(cfg.Attributes)
	return ctx, func() {
		if len(attrs) == 0 {
			return
		}
		e.send(exportRecord{Attributes: attrs})
	}
}

func (e *exporter) RecordMetric(name string, value float64, opts ...MetricOption) {
	if _, ok := allowedMetrics[name]; !ok {
		return
	}
	cfg := &MetricConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	v := value
	e.send(exportRecord{Name: name, Value: &v, Attributes: keepAttrs(cfg.Attributes)})
}

func (e *exporter) Log(context.Context, string, string, ...any) {}

func (e *exporter) Shutdown(context.Context) error { return nil }

func (e *exporter) send(rec exportRecord) {
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	select {
	case e.ch <- b:
	default:
	}
}

func (e *exporter) loop() {
	for b := range e.ch {
		req, err := http.NewRequest(http.MethodPost, e.url, bytes.NewReader(b))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := e.client.Do(req)
		if err != nil {
			continue
		}
		res.Body.Close()
	}
}

func keepAttrs(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if !keepAttr(k, v) {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func keepAttr(key, val string) bool {
	switch key {
	case "http.route":
		_, ok := allowedRoutes[val]
		return ok
	case "http.method":
		_, ok := allowedHTTPMethods[val]
		return ok
	case "http.status":
		if len(val) == 0 || len(val) > 3 {
			return false
		}
		n, err := strconv.Atoi(val)
		return err == nil && n >= 100 && n <= 599
	case "acp.method":
		return acpMethod.MatchString(val)
	case "error":
		return val == "true"
	default:
		return false
	}
}

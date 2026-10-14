package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCapabilityGate(t *testing.T) {
	cases := []struct {
		name   string
		env    []string
		method string
		path   string
		body   string
		want   int
	}{
		{"list unsupported", []string{"ZB_FAKE_NO_LIST=1"}, http.MethodGet, "/v1/sessions", "", http.StatusNotImplemented},
		{"load unsupported", []string{"ZB_FAKE_NO_LOAD=1"}, http.MethodPost, "/v1/sessions/missing/load", `{"workspace":"/tmp"}`, http.StatusNotImplemented},
		{"resume unsupported", []string{"ZB_FAKE_NO_RESUME=1"}, http.MethodPost, "/v1/sessions/missing/resume", `{}`, http.StatusNotImplemented},
		{"list supported", nil, http.MethodGet, "/v1/sessions", "", http.StatusOK},
		{"load still missing", []string{"ZB_FAKE_NO_RESUME=1"}, http.MethodPost, "/v1/sessions/missing/load", `{}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, base, cleanup := startTestServer(t, tc.env)
			defer cleanup()
			req, err := http.NewRequest(tc.method, base+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			b, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.want {
				t.Fatalf("status %d body %s", res.StatusCode, b)
			}
			if tc.want == http.StatusNotImplemented && !strings.Contains(string(b), "not supported by agent") {
				t.Fatalf("body %s", b)
			}
		})
	}
}

func TestModeUnsupportedBeforeAgentCall(t *testing.T) {
	_, base, cleanup := startConfigured(t, Config{
		Profile:  "generic",
		ExtraEnv: []string{"ZB_FAKE_NO_MODES=1"},
		Logger:   nil,
	})
	defer cleanup()
	dir := t.TempDir()
	res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(`{"workspace":"`+dir+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var created struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.SessionID == "" {
		t.Fatal("empty session")
	}
	modeReq, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+created.SessionID+"/mode", strings.NewReader(`{"mode":"auto"}`))
	if err != nil {
		t.Fatal(err)
	}
	modeReq.Header.Set("Content-Type", "application/json")
	modeRes, err := http.DefaultClient.Do(modeReq)
	if err != nil {
		t.Fatal(err)
	}
	defer modeRes.Body.Close()
	b, _ := io.ReadAll(modeRes.Body)
	if modeRes.StatusCode != http.StatusNotImplemented || !strings.Contains(string(b), "not supported by agent") {
		t.Fatalf("status %d body %s", modeRes.StatusCode, b)
	}
}

func TestModesFollowAgentList(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		mode    string
		want    int
		snippet string
	}{
		{
			name:    "zero fallback allows ask",
			cfg:     Config{ExtraEnv: []string{"ZB_FAKE_NO_MODES=1"}},
			mode:    "ask",
			want:    http.StatusOK,
			snippet: `"ok":true`,
		},
		{
			name:    "zero fallback rejects other ids",
			cfg:     Config{ExtraEnv: []string{"ZB_FAKE_NO_MODES=1"}},
			mode:    "build",
			want:    http.StatusBadRequest,
			snippet: "auto, ask, or plan",
		},
		{
			name:    "announced id is allowed",
			cfg:     Config{Profile: "generic", ExtraEnv: []string{"ZB_FAKE_MODE_IDS=build,chat"}},
			mode:    "build",
			want:    http.StatusOK,
			snippet: `"ok":true`,
		},
		{
			name:    "fixed id is rejected when not announced",
			cfg:     Config{Profile: "generic", ExtraEnv: []string{"ZB_FAKE_MODE_IDS=build,chat"}},
			mode:    "auto",
			want:    http.StatusBadRequest,
			snippet: "not offered by the agent",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, base, cleanup := startConfigured(t, tc.cfg)
			defer cleanup()
			dir := t.TempDir()
			res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(`{"workspace":"`+dir+`"}`))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			var created struct {
				SessionID string `json:"sessionId"`
			}
			if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+created.SessionID+"/mode", strings.NewReader(`{"mode":"`+tc.mode+`"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			got, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer got.Body.Close()
			b, _ := io.ReadAll(got.Body)
			if got.StatusCode != tc.want || !strings.Contains(string(b), tc.snippet) {
				t.Fatalf("status %d body %s", got.StatusCode, b)
			}
		})
	}
}

func TestCapabilitiesEndpoint(t *testing.T) {
	_, base, cleanup := startConfigured(t, Config{Token: bearerToken, LoginFailureDelay: -1, Profile: "generic"})
	defer cleanup()
	bare, err := http.Get(base + "/v1/capabilities")
	if err != nil {
		t.Fatal(err)
	}
	bare.Body.Close()
	if bare.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth %d", bare.StatusCode)
	}
	req, err := http.NewRequest(http.MethodGet, base+"/v1/capabilities", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "evil.example"
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	denied, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("host %d", denied.StatusCode)
	}
	okReq, err := http.NewRequest(http.MethodGet, base+"/v1/capabilities", nil)
	if err != nil {
		t.Fatal(err)
	}
	okReq.Header.Set("Authorization", "Bearer "+bearerToken)
	okRes, err := http.DefaultClient.Do(okReq)
	if err != nil {
		t.Fatal(err)
	}
	defer okRes.Body.Close()
	b, _ := io.ReadAll(okRes.Body)
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", okRes.StatusCode, b)
	}
	if strings.Contains(string(b), "sess-") || strings.Contains(string(b), bearerToken) {
		t.Fatalf("leaked %s", b)
	}
	var body struct {
		Profile       string `json:"profile"`
		LoadSession   bool   `json:"loadSession"`
		SessionList   bool   `json:"sessionList"`
		SessionResume bool   `json:"sessionResume"`
		Agent         struct {
			Name string `json:"name"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if body.Profile != "generic" || !body.LoadSession || !body.SessionList || !body.SessionResume || body.Agent.Name != "acptest" {
		t.Fatalf("%s", b)
	}
}

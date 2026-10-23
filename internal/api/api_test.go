package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	rh "repoharbor"
	"repoharbor/internal/config"
	"repoharbor/internal/events"
	"repoharbor/internal/gitrepo"
	"repoharbor/internal/scheduler"
	"repoharbor/internal/store"
)

func testServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Addr: ":0", DataDir: dir, DBPath: filepath.Join(dir, "test.db"), ReposDir: filepath.Join(dir, "repos"), LogsDir: filepath.Join(dir, "logs"), SessionTTL: time.Hour, MaxRetries: 3}
	for _, p := range []string{cfg.ReposDir, cfg.LogsDir} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(context.Background(), cfg.DBPath, rh.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	git := gitrepo.New(cfg.ReposDir)
	broker := events.NewBroker()
	sched := scheduler.New(cfg, st, git, broker)
	srv := httptest.NewServer(New(cfg, st, git, sched, broker, nil).Routes())
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	t.Cleanup(func() { srv.Close(); _ = st.Close() })
	return srv, client
}

func requestJSON(t *testing.T, client *http.Client, method, url string, body any, csrf string) (int, map[string]any, *http.Response) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	return resp.StatusCode, out, resp
}

func TestSetupLoginAndCSRF(t *testing.T) {
	srv, client := testServer(t)
	base := srv.URL

	status, body, _ := requestJSON(t, client, http.MethodGet, base+"/api/v1/setup/status", nil, "")
	if status != 200 || body["needs_setup"] != true {
		t.Fatalf("setup status: %d %#v", status, body)
	}
	status, _, _ = requestJSON(t, client, http.MethodPost, base+"/api/v1/setup", map[string]string{"password": "short"}, "")
	if status != http.StatusBadRequest {
		t.Fatalf("weak password status=%d", status)
	}
	status, body, _ = requestJSON(t, client, http.MethodPost, base+"/api/v1/setup", map[string]string{"password": "testpassword123"}, "")
	if status != http.StatusCreated {
		t.Fatalf("setup status=%d body=%#v", status, body)
	}
	csrf, _ := body["csrf_token"].(string)
	if csrf == "" {
		t.Fatal("missing csrf token")
	}
	status, body, _ = requestJSON(t, client, http.MethodGet, base+"/api/v1/auth/me", nil, "")
	if status != 200 || body["username"] != "admin" {
		t.Fatalf("me status=%d body=%#v", status, body)
	}
	status, _, _ = requestJSON(t, client, http.MethodPost, base+"/api/v1/repositories", map[string]any{"url": "bad"}, "")
	if status != http.StatusForbidden {
		t.Fatalf("expected CSRF rejection, got %d", status)
	}
	status, body, _ = requestJSON(t, client, http.MethodPost, base+"/api/v1/repositories", map[string]any{"url": "bad"}, csrf)
	if status != http.StatusBadRequest || body["code"] != "INVALID_URL" {
		t.Fatalf("invalid url status=%d body=%#v", status, body)
	}
	status, body, _ = requestJSON(t, client, http.MethodPost, base+"/api/v1/repositories", map[string]any{
		"url": "https://github.com/octocat/Hello-World", "cron_expression": "not-a-cron",
	}, csrf)
	if status != http.StatusBadRequest || body["code"] != "INVALID_CRON" {
		t.Fatalf("invalid cron status=%d body=%#v", status, body)
	}
	status, _, _ = requestJSON(t, client, http.MethodPost, base+"/api/v1/auth/logout", nil, csrf)
	if status != http.StatusNoContent {
		t.Fatalf("logout status=%d", status)
	}
	status, _, _ = requestJSON(t, client, http.MethodGet, base+"/api/v1/auth/me", nil, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %d", status)
	}
}

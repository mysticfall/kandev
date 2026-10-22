package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kandev/kandev/internal/agentctl/types"
)

func TestOpenCodeCredentialImportUsesSessionHomeAndStdin(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() { _ = server.procMgr.StopForTeardown(context.Background()) })
	home := t.TempDir()
	args := []string{os.Args[0], "-test.run=^TestOpenCodeCredentialImportHelper$", "--", "npx", "--yes", "--prefer-offline", "--prefix", "~/.kandev/managed-npm-runtime", "@opencode/cli@2.1.2", "acp", "--print-logs"}
	if err := server.procMgr.Configure("", args, true, map[string]string{"HOME": home, "KANDEV_TEST_CREDENTIAL_IMPORT": "1"}, "", nil, false); err != nil {
		t.Fatal(err)
	}
	const credentials = `[{"id":"cred_test","integrationID":"openai","label":"test","active":true,"value":{"type":"key","key":"synthetic-secret"}}]`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/opencode/credentials", strings.NewReader(credentials))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("credential import status = %d, want 200: %s", resp.Code, resp.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(home, "imported.json"))
	if err != nil || string(got) != credentials {
		t.Fatalf("session import did not receive exact credentials: %v", err)
	}
}

func TestOpenCodeCredentialImportHelper(t *testing.T) {
	if os.Getenv("KANDEV_TEST_CREDENTIAL_IMPORT") != "1" {
		return
	}
	if !slices.Equal(os.Args[len(os.Args)-3:], []string{"auth", "import", "--standalone"}) {
		os.Exit(2)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || !json.Valid(data) {
		os.Exit(3)
	}
	if os.Getenv("KANDEV_TEST_IMPORT_FAIL") == "1" {
		_, _ = os.Stdout.Write(data)
		_, _ = os.Stderr.Write(data)
		os.Exit(6)
	}
	if barrier := os.Getenv("KANDEV_TEST_IMPORT_BARRIER"); barrier != "" {
		resp, err := http.Get(barrier)
		if err != nil {
			os.Exit(5)
		}
		_ = resp.Body.Close()
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), "imported.json"), data, 0o600); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestOpenCodeCredentialImportSerializesConfigurationAndStart(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() { _ = server.procMgr.StopForTeardown(context.Background()) })
	arrived, release := make(chan struct{}), make(chan struct{})
	barrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-release
	}))
	defer barrier.Close()
	releaseImport := sync.OnceFunc(func() { close(release) })
	defer releaseImport()
	args := []string{os.Args[0], "-test.run=^TestOpenCodeCredentialImportHelper$", "--", "npx", "--yes", "--prefer-offline", "--prefix", "~/.kandev/managed-npm-runtime", "@opencode/cli@2.1.2", "acp"}
	env := map[string]string{"HOME": t.TempDir(), "KANDEV_TEST_CREDENTIAL_IMPORT": "1", "KANDEV_TEST_IMPORT_BARRIER": barrier.URL}
	if err := server.procMgr.Configure("", args, true, env, "", nil, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.procMgr.ImportOpenCodeCredentials(ctx, []byte(`[]`)) }()
	select {
	case <-arrived:
	case err := <-done:
		t.Fatalf("import exited before barrier: %v", err)
	case <-ctx.Done():
		t.Fatal("import did not reach barrier")
	}
	if err := server.procMgr.Configure("", args, true, env, "", nil, false); err == nil {
		t.Fatal("configuration changed during credential import")
	}
	if _, err := server.procMgr.StartWithGeneration(ctx); err == nil || !strings.Contains(err.Error(), "credential import") {
		t.Fatalf("startup was not blocked by credential import: %v", err)
	}
	if err := server.procMgr.ImportOpenCodeCredentials(ctx, []byte(`[]`)); err == nil {
		t.Fatal("concurrent credential import was accepted")
	}
	if _, err := server.procMgr.Passthrough().Start(ctx, types.PassthroughStartRequest{
		SessionID: server.procMgr.GetSessionID(), OperationID: "during-import", Command: args,
	}); err == nil || !strings.Contains(err.Error(), "credential import") {
		t.Fatalf("passthrough startup was not blocked by credential import: %v", err)
	}
	releaseImport()
	if err := <-done; err != nil {
		t.Fatalf("credential import failed: %v", err)
	}
	if err := server.procMgr.Configure("", args, true, env, "", nil, false); err != nil {
		t.Fatalf("configuration remained blocked after import: %v", err)
	}
}

func TestOpenCodeCredentialImportRejectsUnconfiguredAgent(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/opencode/credentials", strings.NewReader(`[{"id":"cred_test"}]`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("credential import status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func TestOpenCodeCredentialImportFailureDoesNotExposeOutput(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() { _ = server.procMgr.StopForTeardown(context.Background()) })
	args := []string{os.Args[0], "-test.run=^TestOpenCodeCredentialImportHelper$", "--", "npx", "--yes", "--prefer-offline", "--prefix", "~/.kandev/managed-npm-runtime", "@opencode/cli@2.1.2", "acp"}
	if err := server.procMgr.Configure("", args, true, map[string]string{"HOME": t.TempDir(), "KANDEV_TEST_CREDENTIAL_IMPORT": "1", "KANDEV_TEST_IMPORT_FAIL": "1"}, "", nil, false); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/agent/opencode/credentials", strings.NewReader(`[{"secret":"synthetic-secret"}]`)))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "synthetic-secret") {
		t.Fatalf("failed credential command exposed output or succeeded: status %d", response.Code)
	}
}

func TestOpenCodeCredentialImportCancellationReleasesSession(t *testing.T) {
	server := newTestServer(t)
	t.Cleanup(func() { _ = server.procMgr.StopForTeardown(context.Background()) })
	arrived, disconnected := make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	barrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		select {
		case <-r.Context().Done():
			close(disconnected)
		case <-release:
		}
	}))
	defer barrier.Close()
	defer close(release)
	args := []string{os.Args[0], "-test.run=^TestOpenCodeCredentialImportHelper$", "--", "npx", "--yes", "--prefer-offline", "--prefix", "~/.kandev/managed-npm-runtime", "@opencode/cli@2.1.2", "acp"}
	env := map[string]string{"HOME": t.TempDir(), "KANDEV_TEST_CREDENTIAL_IMPORT": "1", "KANDEV_TEST_IMPORT_BARRIER": barrier.URL}
	if err := server.procMgr.Configure("", args, true, env, "", nil, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.procMgr.ImportOpenCodeCredentials(ctx, []byte(`[]`)) }()
	select {
	case <-arrived:
	case <-ctx.Done():
		t.Fatal("import did not reach barrier")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled import succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled import did not finish")
	}
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("import subprocess survived cancellation")
	}
	if err := server.procMgr.Configure("", args, true, env, "", nil, false); err != nil {
		t.Fatalf("canceled import left configuration blocked: %v", err)
	}
}

func TestOpenCodeCredentialImportRejectsInvalidPayloadBeforeLaunching(t *testing.T) {
	for _, payload := range []string{`null`, `{}`, `[]`, `{"secret":"synthetic-secret"}`, strings.Repeat("x", (1<<20)+1)} {
		t.Run(fmt.Sprintf("bytes-%d", len(payload)), func(t *testing.T) {
			server := newTestServer(t)
			t.Cleanup(func() { _ = server.procMgr.StopForTeardown(context.Background()) })
			home := t.TempDir()
			args := []string{os.Args[0], "-test.run=^TestOpenCodeCredentialImportHelper$", "--", "npx", "--yes", "--prefer-offline", "--prefix", "~/.kandev/managed-npm-runtime", "@opencode/cli@2.1.2", "acp"}
			if err := server.procMgr.Configure("", args, true, map[string]string{"HOME": home, "KANDEV_TEST_CREDENTIAL_IMPORT": "1"}, "", nil, false); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			server.router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/agent/opencode/credentials", strings.NewReader(payload)))
			if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "synthetic-secret") {
				t.Fatalf("invalid credential payload response: %d", response.Code)
			}
			if _, err := os.Stat(filepath.Join(home, "imported.json")); !os.IsNotExist(err) {
				t.Fatal("invalid payload reached import process")
			}
		})
	}
}

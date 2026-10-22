package lifecycle

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kandev/kandev/internal/agent/managedruntime"
	"github.com/kandev/kandev/internal/agentruntime"
)

const syntheticOpenCodeCredentials = `[{"id":"cred_test","integrationID":"openai","label":"test","active":true,"value":{"type":"key","key":"synthetic-secret"}}]`

func TestOpenCodeCredentialProvisioningLeavesV1Unchanged(t *testing.T) {
	execution := &AgentExecution{
		AgentID: "opencode-acp", RuntimeName: agentruntime.RuntimeKubernetes,
		AgentArgs: []string{"npx", "--yes", "--prefer-offline", "--prefix", managedruntime.NPMProjectPrefix, "opencode-ai@1.18.32", "acp"},
		metadata:  map[string]interface{}{"remote_credentials": "invalid legacy metadata"},
	}
	if err := provisionOpenCodeCredentials(context.Background(), execution, nil); err != nil {
		t.Fatalf("V2 provisioning changed V1 launch behavior: %v", err)
	}
}

func runOpenCodeExportFixture() {
	if endpoint := os.Getenv("KANDEV_TEST_EXPORT_CHILD"); endpoint != "" {
		runOpenCodeExportChild(endpoint)
		os.Exit(0)
	}
	if len(os.Args) < 4 || !slices.Equal(os.Args[len(os.Args)-3:], []string{"auth", "export", "--standalone"}) {
		os.Exit(2)
	}
	fmt.Print(syntheticOpenCodeCredentials)
	os.Exit(0)
}

func runOpenCodeExportChild(endpoint string) {
	if os.Getenv("KANDEV_TEST_EXPORT_GRANDCHILD") != "1" {
		child := exec.Command(os.Args[0], os.Args[1:]...)
		child.Env = append(os.Environ(), "KANDEV_TEST_EXPORT_GRANDCHILD=1")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		_ = child.Run()
		return
	}
	if os.Getenv("KANDEV_TEST_EXPORT_OVERSIZE") == "1" {
		fmt.Print(strings.Repeat("x", (1<<20)+1))
	}
	resp, err := http.Get(endpoint)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func TestOpenCodeCredentialExportBoundsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("host CLI fixture uses an executable symlink")
	}
	release := make(chan struct{})
	barrier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer barrier.Close()
	defer close(release)
	bin := t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "npx")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KANDEV_TEST_OPENCODE_EXPORT", "1")
	t.Setenv("KANDEV_TEST_EXPORT_CHILD", barrier.URL)
	t.Setenv("KANDEV_TEST_EXPORT_OVERSIZE", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := exportOpenCodeCredentials(ctx, "@opencode/cli@2.1.2"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("oversized export succeeded")
		}
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("oversized export was not stopped at the output limit")
	}
}

func TestOpenCodeCredentialExportCancellationStopsDescendants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("host CLI fixture uses an executable symlink")
	}
	arrived, disconnected, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
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
	bin := t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "npx")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KANDEV_TEST_OPENCODE_EXPORT", "1")
	t.Setenv("KANDEV_TEST_EXPORT_CHILD", barrier.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := exportOpenCodeCredentials(ctx, "@opencode/cli@2.1.2"); done <- err }()
	select {
	case <-arrived:
	case <-ctx.Done():
		t.Fatal("export child did not start")
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled export succeeded")
	}
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("export descendant survived cancellation")
	}
}

func TestConfigureAndStartAgentProvisionsOpenCodeBeforeStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("host CLI fixture uses an executable symlink")
	}
	bin := t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "npx")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KANDEV_TEST_OPENCODE_EXPORT", "1")
	var calls []string
	var imported string
	var rejectImport bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.URL.Path == "/api/v1/agent/opencode/credentials" {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			imported = string(data)
			if rejectImport {
				http.Error(w, "synthetic-secret", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	t.Cleanup(srv.Close)
	client := newTestAgentctlClient(t, srv.URL, newTestLogger())
	execution := &AgentExecution{
		ID: "exec-opencode", AgentID: "opencode-acp", RuntimeName: agentruntime.RuntimeKubernetes,
		AgentArgs:     []string{"npx", "--yes", "--prefer-offline", "--prefix", managedruntime.NPMProjectPrefix, "@opencode/cli@2.1.2", "acp"},
		WorkspacePath: t.TempDir(), agentctl: client,
		metadata: map[string]interface{}{"remote_credentials": `["agent:opencode-acp:files:0"]`},
	}
	if _, err := newTestManager(t).configureAndStartAgent(context.Background(), execution); err != nil {
		t.Fatal(err)
	}
	if imported != syntheticOpenCodeCredentials || !slices.Equal(calls, []string{"/api/v1/agent/configure", "/api/v1/agent/opencode/credentials", "/api/v1/start"}) {
		t.Fatalf("credential transfer or ordering incorrect: calls=%v", calls)
	}
	calls = nil
	imported = ""
	execution.AgentCommand = "npx"
	refresh := &RemoteInstanceRefresh{Instance: &ExecutorInstance{Client: client}}
	if _, err := newTestManager(t).prepareRestartedKubernetesAgentctl(context.Background(), execution, refresh); err != nil {
		t.Fatal(err)
	}
	if imported != syntheticOpenCodeCredentials || !slices.Equal(calls, []string{"/api/v1/agent/configure", "/api/v1/agent/opencode/credentials", "/api/v1/start"}) {
		t.Fatalf("restart credential transfer or ordering incorrect: calls=%v", calls)
	}
	calls = nil
	rejectImport = true
	if _, err := newTestManager(t).configureAndStartAgent(context.Background(), execution); err == nil || strings.Contains(err.Error(), "synthetic-secret") {
		t.Fatalf("import failure was not sanitized and propagated: %v", err)
	}
	if !slices.Equal(calls, []string{"/api/v1/agent/configure", "/api/v1/agent/opencode/credentials"}) {
		t.Fatalf("failed credential import admitted ACP: %v", calls)
	}
	for _, selection := range []string{"", `[]`, `["agent:other:files:0"]`} {
		calls = nil
		execution.metadata["remote_credentials"] = selection
		if _, err := newTestManager(t).configureAndStartAgent(context.Background(), execution); err != nil {
			t.Fatalf("unselected provisioning failed launch: %v", err)
		}
		if !slices.Equal(calls, []string{"/api/v1/agent/configure", "/api/v1/start"}) {
			t.Fatalf("unselected credentials were transferred: %v", calls)
		}
	}
}

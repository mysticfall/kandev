package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/kandev/kandev/internal/agent/managedruntime"
	agentctl "github.com/kandev/kandev/internal/agent/runtime/agentctl"
	"github.com/kandev/kandev/internal/agentruntime"
)

const openCodeCredentialMethodID = "agent:opencode-acp:files:0"
const openCodeCredentialAgentID = "opencode-acp"

func provisionOpenCodeCredentials(ctx context.Context, execution *AgentExecution, client *agentctl.Client) error {
	if execution.AgentID != openCodeCredentialAgentID || execution.RuntimeName == agentruntime.RuntimeStandalone {
		return nil
	}
	packageIndex := slices.IndexFunc(execution.AgentArgs, func(arg string) bool {
		return strings.HasPrefix(arg, managedruntime.OpenCodeV2Package+"@")
	})
	if packageIndex < 0 {
		return nil
	}
	selected := execution.RuntimeName == agentruntime.RuntimeDocker
	var ids []string
	if raw := getMetadataString(execution.MetadataSnapshot(), "remote_credentials"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			return errors.New("invalid remote credential selection")
		}
		selected = slices.Contains(ids, openCodeCredentialMethodID)
	}
	if !selected {
		return nil
	}
	if _, err := managedruntime.OpenCodeCredentialCommand(execution.AgentArgs, "import"); err != nil {
		return err
	}
	data, err := exportOpenCodeCredentials(ctx, execution.AgentArgs[packageIndex])
	if err != nil {
		return err
	}
	return client.ImportOpenCodeCredentials(ctx, data)
}

func exportOpenCodeCredentials(parent context.Context, packageSpec string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	cmd := exec.Command("npx", "--yes", "--prefer-offline", "--prefix", managedruntime.NPMProjectPrefix, packageSpec, "auth", "export", "--standalone")
	if err := managedruntime.PrepareNPMProjectPrefix(cmd.Args); err != nil {
		return nil, errors.New("could not prepare OpenCode credential export")
	}
	cmd.WaitDelay = 5 * time.Second
	output := credentialExportBuffer{cancel: cancel}
	cmd.Stdout = &output
	err := runOpenCodeExport(ctx, cmd)
	if err != nil {
		return nil, errors.New("OpenCode credential export failed; sign in on the Kandev host with a supported V2 CLI")
	}
	data := output.buffer.Bytes()
	var entries []json.RawMessage
	if len(data) > 1<<20 || json.Unmarshal(data, &entries) != nil || len(entries) == 0 {
		return nil, errors.New("OpenCode credential export did not contain accounts")
	}
	return data, nil
}

type credentialExportBuffer struct {
	buffer bytes.Buffer
	cancel context.CancelFunc
}

func (b *credentialExportBuffer) Write(data []byte) (int, error) {
	if len(data) > (1<<20)-b.buffer.Len() {
		b.cancel()
		return 0, errors.New("OpenCode credential export exceeded its limit")
	}
	return b.buffer.Write(data)
}

func runOpenCodeExport(ctx context.Context, cmd *exec.Cmd) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configureCredentialExportProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	stop, err := captureCredentialExportProcess(cmd)
	if err != nil {
		_ = cmd.Wait()
		return err
	}
	defer stop()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		stop()
		<-done
		return ctx.Err()
	}
}

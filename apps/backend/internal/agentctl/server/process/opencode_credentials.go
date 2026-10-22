package process

import (
	"context"
	"errors"

	"github.com/kandev/kandev/internal/agent/managedruntime"
	tools "github.com/kandev/kandev/internal/tools/installer"
)

// ImportOpenCodeCredentials provisions the configured session through the CLI's
// merge contract. Credential values are carried only through stdin.
func (m *Manager) ImportOpenCodeCredentials(ctx context.Context, data []byte) error {
	m.startMu.Lock()
	if m.credentialImport || m.Status() == StatusRunning || m.Status() == StatusStarting || m.passthrough.Live() {
		m.startMu.Unlock()
		return errors.New("OpenCode credential import requires an idle agent")
	}
	args, err := managedruntime.OpenCodeCredentialCommand(m.cfg.AgentArgs, "import")
	dir := m.cfg.WorkDir
	if err == nil {
		m.credentialImport = true
	}
	m.startMu.Unlock()
	if err != nil {
		return err
	}
	defer func() {
		m.startMu.Lock()
		m.credentialImport = false
		m.startMu.Unlock()
	}()
	_, _, err = m.runManagedCommandInput(ctx, tools.CommandSpec{Path: args[0], Args: args[1:], Dir: dir}, data)
	if err != nil {
		return errors.New("OpenCode credential import failed")
	}
	return nil
}

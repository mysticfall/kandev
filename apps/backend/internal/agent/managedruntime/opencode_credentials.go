package managedruntime

import (
	"errors"
	"strings"
)

var ErrOpenCodeCredentialRuntime = errors.New("OpenCode credential provisioning requires a managed V2 runtime version 2.1.2 or newer")

// OpenCodeCredentialCommand retains the configured launcher prefix and exact
// runtime version, replacing ACP arguments with the standalone auth operation.
func OpenCodeCredentialCommand(args []string, operation string) ([]string, error) {
	for i, arg := range args {
		if arg != "npx" || i+6 >= len(args) {
			continue
		}
		if args[i+1] != "--yes" || (args[i+2] != "--prefer-offline" && args[i+2] != "--prefer-online") ||
			args[i+3] != "--prefix" || args[i+4] != NPMProjectPrefix || args[i+6] != "acp" {
			return nil, ErrOpenCodeCredentialRuntime
		}
		if !strings.HasPrefix(args[i+5], OpenCodeV2Package+"@") {
			return nil, ErrOpenCodeCredentialRuntime
		}
		if !supportsOpenCodeCredentials(strings.TrimPrefix(args[i+5], OpenCodeV2Package+"@")) {
			return nil, ErrOpenCodeCredentialRuntime
		}
		return append(append([]string(nil), args[:i+6]...), "auth", operation, "--standalone"), nil
	}
	return nil, ErrOpenCodeCredentialRuntime
}

func supportsOpenCodeCredentials(raw string) bool {
	version, err := ParseStableVersion(raw)
	return err == nil && version.Major() == 2 && (version.Minor() > 1 || (version.Minor() == 1 && version.Patch() >= 2))
}

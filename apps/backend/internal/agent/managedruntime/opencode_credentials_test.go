package managedruntime

import (
	"slices"
	"testing"
)

func TestOpenCodeCredentialCommandRejectsUnsupportedRuntime(t *testing.T) {
	for _, pkg := range []string{"opencode-ai@1.2.3", "@opencode/cli@2.0.18", "@opencode/cli@2.1.1", "@opencode/cli@latest", "2.1.2"} {
		t.Run(pkg, func(t *testing.T) {
			args := []string{"npx", "--yes", "--prefer-offline", "--prefix", NPMProjectPrefix, pkg, "acp"}
			if _, err := OpenCodeCredentialCommand(args, "import"); err == nil {
				t.Fatal("unsupported package accepted for credential import")
			}
		})
	}
}

func TestOpenCodeCredentialCommandPreservesPinnedLauncher(t *testing.T) {
	args := []string{"wrapper", "--", "npx", "--yes", "--prefer-online", "--prefix", NPMProjectPrefix, "@opencode/cli@2.1.2", "acp", "--print-logs"}
	got, err := OpenCodeCredentialCommand(args, "import")
	want := []string{"wrapper", "--", "npx", "--yes", "--prefer-online", "--prefix", NPMProjectPrefix, "@opencode/cli@2.1.2", "auth", "import", "--standalone"}
	if err != nil || !slices.Equal(got, want) || args[8] != "acp" {
		t.Fatalf("credential command = %v, %v; want %v without mutating launch argv", got, err, want)
	}
}

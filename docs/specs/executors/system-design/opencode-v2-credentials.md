---
status: current
system: executors
created: 2026-10-22
owners:
  - tbd
requirements:
  - REQ-EXECUTORS-OPENCODE-AUTH-001
---

# OpenCode V2 Credential Provisioning Design

## Ownership and selection

The lifecycle package owns provisioning between agentctl configuration and ACP
startup. Both normal launch and Kubernetes controller refresh use this boundary.
The stable method ID remains `agent:opencode-acp:files:0`. For V2 the agent
descriptor marks it for launch provisioning and supplies no legacy source files.
Catalog availability means provisioning can be attempted; accounts are checked
at launch rather than exporting secrets during settings polling.

Remote executors honor `remote_credentials`; local Docker retains automatic
credential seeding unless the metadata explicitly supplies a selection.
Standalone uses its existing host authentication. V1 retains `auth.json` copying.

## Export and import

The recorded managed command supplies the exact V2 package version. Supported
versions start at 2.1.2, which is also the bundled V2 default. Export runs the same
package on the controller with `auth export --standalone`, using its host
HOME/XDG environment. Export has a two-minute deadline, a 1 MiB output cap, and
platform-owned process-tree cleanup.

The authenticated `POST /api/v1/agent/opencode/credentials` endpoint accepts the
export array as its request body. It uses existing agentctl auth and instance
middleware. It rejects empty, malformed, or oversized payloads. The configured
command prefix and exact runtime pin are retained while ACP arguments are
replaced with `auth import --standalone`. The owned subprocess receives the
configured session environment and credentials on stdin; neither output stream
is exposed to callers. The response acknowledges success explicitly.

The manager excludes configuration, ACP and passthrough startup, and another
import while provisioning is active. Cancellation uses owned-process teardown.
Import uses the CLI merge contract: an existing credential ID is skipped, keeping
refreshed values. Additional accounts for an existing integration remain inactive,
preserving destination account selection. Kandev does not copy SQLite databases.

## Failure and recovery

Selected export/import errors abort launch before ACP. A missing endpoint on an
older worker also fails launch; deploy matching controller and agentctl builds.
Reconfiguration after a stopped session runs the same provisioning path. This
does not restore a missing Kubernetes bootstrap environment or provider history.

## Requirement mapping

`AC-EXECUTORS-OPENCODE-AUTH-001.1` and `.4` map to ownership and selection;
`.2`, `.5`, and `.6` map to export and import; `.3` maps to failure and recovery.

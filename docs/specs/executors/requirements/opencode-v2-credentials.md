---
status: active
system: executors
created: 2026-10-22
owners:
  - tbd
---

# OpenCode V2 Credential Provisioning

## Overview

Operators can provision OpenCode accounts into isolated executor sessions without
copying provider databases or installing a credential-bootstrap wrapper.

## Requirements

### REQ-EXECUTORS-OPENCODE-AUTH-001: Session credential provisioning

#### Acceptance criteria

- **AC-EXECUTORS-OPENCODE-AUTH-001.1:** When the OpenCode credential method is selected for a managed V2 session, the system shall provision host accounts before starting ACP, including subsequent sessions and stopped-session starts.
- **AC-EXECUTORS-OPENCODE-AUTH-001.2:** Repeated provisioning shall preserve existing destination credential values and active-account selections, including refreshed OAuth credentials.
- **AC-EXECUTORS-OPENCODE-AUTH-001.3:** When selected provisioning fails or the selected V2 version is older than 2.1.2, launch shall fail without starting ACP or exposing credential values in errors.
- **AC-EXECUTORS-OPENCODE-AUTH-001.4:** The system shall retain V1 credential-file behavior and the existing remote credential selection. Unselected remote methods and standalone sessions shall not export host accounts.
- **AC-EXECUTORS-OPENCODE-AUTH-001.5:** Cancellation shall terminate provisioning subprocesses and release the session for a later attempt. Concurrent configuration, agent startup, and credential imports shall not change a session during import.
- **AC-EXECUTORS-OPENCODE-AUTH-001.6:** Credential exports shall be bounded to 1 MiB and shall not be transported through subprocess arguments, environment variables, or temporary export files.

## Exclusions

Volatile Kubernetes bootstrap recovery, provider-history durability, runtime cache
lifecycle, and deployment-specific workaround retirement are separate work.

## System design

See [OpenCode V2 credentials](../system-design/opencode-v2-credentials.md).

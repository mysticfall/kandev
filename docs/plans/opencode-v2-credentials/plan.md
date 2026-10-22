---
created: 2026-10-22
status: completed
requirements:
  - REQ-EXECUTORS-OPENCODE-AUTH-001
system_design:
  - ../../specs/executors/system-design/opencode-v2-credentials.md
legacy_specs: []
---

# Native OpenCode V2 Credentials

## Overview

Implement the approved credential-provisioning scope through the common lifecycle
launch boundary, preserving V1 and remote profile selections. Target V2 2.1.2 or
newer. Build and exercise the engine in a disposable image before committing.

## Work orders

- [x] [Task 01: Provision V2 session credentials](task-01-provision-credentials.md)

## Risks and boundaries

Credential transfer must not overwrite refreshed destination accounts or leak
CLI output. Controller and worker must be upgraded together. Existing Kubernetes
bootstrap recovery and runtime-cache capacity issues are separate follow-ups.
Demo configuration and workaround retirement are excluded.

## Verification

The work order owns regression, race, lint, documentation, and isolated-image
checks. Image checks use synthetic accounts and no provider requests.

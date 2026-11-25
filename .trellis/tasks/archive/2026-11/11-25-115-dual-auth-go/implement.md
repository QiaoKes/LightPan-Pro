# Execution plan

- [x] Complete W0 source/version fingerprint and exact protocol matrix, including SDK wrapper/cipher dependencies.
- [x] Freeze contracts.md: QR purpose/session, credential persistence, FileRef, complete snapshot, events, errors and scheduling ownership.
- [x] Review design and matrix with user; execution explicitly authorized on 2026-11-25.
- [x] Materialize independent child tasks from roadmap rows, record dependencies/owners, curate each context manifest.
- [x] Execute implementation in M1–M4; integrate shared files through one owner. Live M4 acceptance is deferred under user decision D10 below.
- [x] Remove obsolete 115_Open registration/config and unused code after the capability inventory and retained-interface review.
- [x] Record deliberate deviations from reference and verify their offline behavior; see acceptance.md and runbook.md.
- [x] Validate pure Go Linux build, docs and retained third-party notices. Container/live execution and cipher distribution licensing remain unverified.
- [x] Record D10: L1–L3 real-account/player acceptance is explicitly deferred to follow-up; no live pass or zero-risk claim.
- [x] Complete D11/W9 exact charset detection and differential validation; raw export-name ambiguity is inherited, monitor path escaping already implemented.
- [x] Implement D9 trash-only deletion: remove permanent option and interim partial-error branch; exact-ID single-call fixtures and independent review passed.
- [x] User explicitly authorized committing the full115 changes and push on 2026-11-25; implementation committed as89fc7c4.
- [x] User-requested Docker-only GHCR publishing passed on GitHub: run45644010263, public sha-9b65537 image. Full115 code and CI pushed to feature branch; main merge/push authorized.

## Validation
Targeted Go package tests first; then `go test ./...` and `go vet ./...`. Use `go test -race` on changed auth, scheduler, driver, playback and STRM packages. In web: `npm run type-check`, `npm run build` using repository lockfile/tooling. Run deterministic httptest fixtures for protocol bytes, auth states and retry counts; tree parser fixtures cover incomplete scans and path scope. Tests must verify observable behavior, not mirror implementation.
Real-account acceptance: dual QR login, save/restart/refresh, several real players and UA, initial play/seek/repeat/concurrent play, root/subtree export, attachments and move/rename/delete events. Obtain credentials through product flows; do not log or commit them. Record network request counts and failures; never infer zero risk from synthetic fixtures.

## Merge / rollback gates
W0 evidence before protocol implementation; W1 vectors before Cookie download integration; W2 persistence before mount UI enables save; W6 complete-snapshot proof before destructive local reconciliation; W8 all required checks before completion. Keep changes scoped by deliverable so failures can be reverted without dropping other agents' work. No automatic commit or deployment.

## PRD alignment — 2026-11-25
The user approved the PRD and subsequently explicitly authorized implementation with gpt-6.1-sol subagents. `prd.md` R1–R15 and A1–A13 are authoritative. W0 evidence/contracts are frozen; implementation and coordinator acceptance are in progress. D10 defers live-account evidence to follow-up; this round delivers code and offline acceptance.

## Reference fidelity gate
Read `reference-contract.md` before dispatch, implementation or review. Every package must inventory required reference modules and deliver source-to-Go mapping plus ordered request traces. Do not optimize waiting/retries/cache/cleanup or omit helpers without an approved behavior difference. Copy this constraint into every child task.

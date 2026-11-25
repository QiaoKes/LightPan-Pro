# Technical design — draft for review

## Decisions
Pure Go runtime; first installation; no legacy migration or old 115 driver alias. Cookie and Open access/refresh credentials are mandatory and must identify the same user. Preserve unrelated providers. Existing generic interfaces are extended only when a reusable capability is needed.

## Boundaries and proposed layout
```text
drivers/115/                 # replaces drivers/115_Open after integration
  driver.go config.go        # metadata, mandatory credential validation, lifecycle
  auth.go                    # adapt shared auth guard/persistence
  ops.go full_list.go         # generic driver capabilities
  tree.go events.go          # optional scan/event capabilities
  client/
    transport.go errors.go scheduler.go
    cookie_auth.go open_auth.go
    cookie_files.go open_files.go download.go
    tree.go events.go subtitles.go
    cipher/                  # only verified protocol primitives
internal/driver/              # generic QR purpose / tree snapshot contracts
internal/domain/              # credentials and scan identity, if necessary
internal/api/                 # existing account/QR routes adapted
internal/store/               # persistence of credentials and event checkpoints
internal/playback/            # 302, request UA, shared cache/request coalescing
internal/strm/                # tree consumer, writer, attachments, incremental sync
web/src/components/admin/    # dual authorization and mount validation
```
Client package knows HTTP/protocol DTOs only; no database, web handler or STRM writes. Driver maps DTOs into domain types. Persistence enters through existing callbacks. STRM owns local output and deletion policy. Playback owns HTTP responses. No shared mutable UA headers.

## Contracts to freeze in work package W0
- CredentialBundle: cookie, access_token, refresh_token, expiry and verified UID; no token duplicate source of truth in Addition versus auth state. Existing domain/account.go already holds Cookie and token fields: extend behavior before inventing another table.
- QR purpose: cookie/open, device/app selection, opaque session token, expiry and explicit state; reuse internal/driver/qrlogin.go + internal/api/qr.go. Successful Cookie authorization must not erase Open tokens, nor vice versa. Verifier/sign remain server-side. Cancel/expiry removes session state.
- Mount: both credentials validated independently, same UID checked, then persist complete bundle; failed probe does not create partially usable account.
- FileRef: account identity plus stable ID or explicit remote path; optional pickcode; never pretend a tree path is a stable file ID. Freeze representation before tree/playback workers start.
- TreeSnapshot: root, entries, completeness and operation identity. Stream entries where practical; deletion eligibility only after complete validated snapshot.
- Client errors: auth-expired(cookie/open), verification-required, rate-limited, not-found, transient, malformed; include retry information without credentials.
- EventBatch: events, checkpoint, completion; consumer commits checkpoint only after durable processing. Overlap-window deduplication required.

## Request behavior
Open is preferred for media downurl; Cookie is used for export and Cookie-specific capabilities. Do not silently switch identities or retry every failure via Cookie. Match verified reference request headers, multipart and PKCE bytes, wire parameters and call order. Preserve reference request cadence and selection semantics. Proposed fixes to repeated calls, retry loops, shared UA state or session storage require evidence of a defect and explicit deviation tests; do not remove apparently redundant requests without checking their purpose.
Reference Open bucket is process-shared 1 request/sec, burst 1. Preserve this conservative baseline initially; distinguish authentication polling from business API calls. Queue priorities and wait semantics must be traced per reference branch; do not introduce a new maximum wait or fairness algorithm without approved evidence. Go context cancellation must not silently change normal reference request ordering. Do not stack another delay gate accidentally.
Link cache lifetime baseline is reference 1200 seconds; expiry clamping beyond reference behavior requires an explicit approved difference. Key includes account/credential generation, file identity and exact UA. Coalesce concurrent misses. Invalidate on credential changes and relevant file mutations. Do not probe CDN on each play. 302 is eligible only when downstream can satisfy required headers; validate Cookie-download header requirements before exposing such URLs.

## Directory and event flow
Cookie root/name resolution -> export -> bounded poll (reference timeout 3000s, empty poll 2s) -> download exported file (reference up to 6 attempts) -> parse -> normalize scoped paths -> existing STRM filter/writer -> commit complete snapshot -> reconcile removals. Track exported object ID and delete only that object. Export cleanup failure is distinct from scan completeness. Handle cancellation and temporary-file cleanup. Investigate root pagination completeness; do not change reference pagination until a demonstrated defect and proposed correction are reviewed.
Attachments reuse STRM metadata pipeline; classify subtitles versus images/NFO and preserve reference request routing. Life events use overlap window, persisted dedup/checkpoint, rename/move invalidation, and bounded reconciliation after missing history. No separate duplicate scheduler inside protocol client.

## Compatibility / rollout
No existing-user migration. Remove obsolete 115 Open-only metadata/forms and references when replacement is integrated. Shared auth/QR changes require regression checks for other providers. Until integration is complete, do not expose a half-implemented replacement driver. Revert by work-package commits, not mixed cross-owner edits.

## Research limits
Exact bundled SDK compatibility and cipher vectors remain gates W0/W1; current SDK defaults are not assumed identical. Real service verification remains separate from fixture tests. This draft is not implementation approval.

## Reference behavior follow-up
See decisions.md for STRM versus Emby reverse-proxy selection, independent credential behavior and automatic local deletion. Investigate temporary-copy handling for simultaneous playback of the same media; request coalescing is not an equivalent replacement.

## Consolidated architecture proposal (2026-11-25)
User delegated remaining design choices to architecture-based synthesis. The following resolves proposed D3/D4/D7 behavior for final design review; it does not authorize implementation.

### Playback and failure routing
Media direct links use Open; no automatic Cookie fallback after Open failure. Expired tokens integrate with the existing refresh guard while preserving reference refresh conditions; retries follow the applicable reference branch. Cookie export-file downloads remain Cookie operations, not media fallback. No background CDN probe on each play. Maintain separate reference STRM and Emby behavior.
Emby session/device orchestration belongs in internal/embyproxy, which already delegates to playback.Service (service.go:836,864,1300). Extend playback request context with optional caller/session/device intent through a generic contract; do not query Emby inside a driver. Driver provides optional temporary-copy resolution capability. Same-media/different-device temporary-copy behavior is enabled only when the Emby session evidence matches the reference condition; ordinary STRM must not infer device identity from UA. Keep temporary object IDs in a cleanup ledger; align reference cleanup timing after verifying its lifecycle. Missing session metadata takes ordinary reference STRM behavior, not speculative copies. Same UID single credential set remains mandatory; do not implement cross-account balance pools.

### Credential health and shared auth integration
Initial mount verifies Cookie and Open identities, then persists one complete bundle. Runtime health is split by credential capability. Open failure does not trigger Cookie media fallback; Cookie failure does not disable valid Open playback. Preserve account IsActive as user enable/disable intent.
Current domain/account.go supports both secrets, but AuthState.Status and auth.GateChecker.Check are account-wide. W2 must extend auth handling with optional capability-aware checks and typed auth errors, rather than mark the whole account token_expired on Cookie failure. Existing single-credential providers retain their behavior. Suggested check categories are OpenAPI and CookieAPI; public interface naming is frozen in W0. Cookie reconnection preserves Open state and vice versa. No proactive per-play health request. UI shows each credential status plus aggregate healthy/degraded/unavailable derived from capabilities; reconnect only failed side. Existing MergeAuthCredentials preserves empty secret fields and must remain atomic under refresh/relogin races.

### STRM ownership and deletion
Use existing TaskRelDir/TaskOutputDir (internal/strm/writer.go:70,146) for a dedicated managed subtree per new task. Reject equal or ancestor/descendant output overlaps, filesystem root targets and nonempty unmanaged destinations at task creation. This is first-install behavior, no migration needed. Track generated STRM and downloaded attachments in task-owned inventory; do not recursively remove arbitrary discovered files or follow symlinks outside root.
Automatically remove obsolete task STRM on complete validated snapshots or verified delete events, matching reference synchronization intent. Rename/move stages new output before retiring old output where destination belongs to task. Moving outside scope removes owned old output. Directory delete removes owned descendants and empty directories only; unknown/manual files are retained and reported, never silently adopted.
LitePan metadata reconciliation already has cloud_primary/local_primary/bidirectional modes (metadata_reconcile.go:16,140). Keep this user-visible contract: downloaded attachments in cloud_primary follow cloud deletion; local_primary/bidirectional use existing reconciliation semantics instead of blanket reference rmtree. New 115 reference-style tasks default to cloud_primary; alternative modes remain explicit task options. No second independent attachment deletion mechanism. A failed or partial scan cannot infer missing entries. Scan and event reconciliation share task serialization/checkpoints so older snapshots cannot undo newer events.

### Required roadmap adjustments
W0: freeze capability-aware auth, caller playback intent, temporary-copy lifecycle, output ownership and snapshot/event ordering contracts.
W2: owns auth capability/error integration and individual health persistence; coordinator integrates shared schema/gates.
W3: dual credential health and reconnect UX.
W5: includes internal/embyproxy session mapping and temporary-copy lifecycle; generic playback and driver adapter boundaries remain explicit.
W6: output overlap checks, managed inventory and complete-snapshot reconciliation.
W7: event/snapshot ordering and metadata-mode-aware attachment reconciliation.
W8: test cross-provider gate regressions, Open works with failed Cookie, no fallback on Open failure, same-media different-device branch, output overlap/manual-file protection and cloud-primary attachment deletion.

## PRD alignment — 2026-11-25
The user approved the PRD and subsequently explicitly authorized implementation with gpt-6.1-sol subagents. `prd.md` R1–R15 and A1–A13 are authoritative. W0 evidence/contracts are frozen; implementation and coordinator acceptance are in progress. Live-account evidence remains a separate gate.

## Playback parity clarification
Use research/playback-parity.md as W5/W8 baseline. Emby direct-link resolution failure uses original-Emby bypass where available (not Cookie fallback); map onto existing proxy forwarding, prevent redirect loops. Preserve reference cache-before-upstream order, per-device identity and temporary-copy timing. Existing generic cache is not proof that current playback matches this flow.

## Mandatory source fidelity
`reference-contract.md` overrides any earlier unverified optimization proposal in this design. User requires strict reference logic; code organization alone is adaptable. New behavior differences need evidence and user confirmation.

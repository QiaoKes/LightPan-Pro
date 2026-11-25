# Reference fidelity contract

Status: mandatory planning/implementation/check constraint, confirmed by user on 2026-11-25. This governs W0–W8 and future child tasks. The user subsequently authorized implementation; this contract remains mandatory during implementation and acceptance.

## Source authority
1. User-confirmed LitePan product constraints (pure Go, mandatory dual credentials, first deployment, one credential set, agreed output ownership and metadata modes).
2. Local EmbyNginx business logic at `/Users/saki/Desktop/code/ebn/restored-python`, including every module in the relevant invocation chain.
3. The reference bundled SDK behavior; pinned upstream source at `/Users/saki/Desktop/code/ebn/115-reference/p115client` is supporting evidence until version/default differences are established.
4. Existing LitePan architecture determines code ownership and integration, not alternative 115 request behavior.

## Required fidelity
- Directory export: root resolution, request payloads, polling, waits, download UA, retry count, export deletion order, parsing, attachment scheduling and final reconciliation.
- Playback: real entry routing, original UA semantics, cache lookup location and lifetime, Emby device/session predicate, temporary copy creation and cleanup, direct-link extraction and failure path.
- 302: preserve caller-specific return/fallback behavior; Emby original-service bypass differs from ordinary STRM failure. Do not collapse them into a generic response policy.
- Scheduling: queue workers, priority, bucket sharing scope, waiting and retry behavior must be individually traced; do not infer one global policy from a single module.
- Authentication: QR app/device, multipart encoding, polling, PKCE bytes, refresh trigger and persistence semantics must be verified from the applicable implementation.
- Files/attachments/events: every helper in a required call chain must be inventoried; a module is not removable just because its reason is unclear.

## Prohibited unilateral changes
No guessed optimizations or generic replacements: removing duplicate lookups, changing default app/UA, adding health/CDN probes, changing rate scope or retry budgets, changing queue fairness, reordering export cleanup, shortening temporary-copy lifetime, merging distinct device sessions, replacing original-service fallback, or dropping helpers without call-path evidence.
Previously discussed bounded retries, coalescing, upstream-expiry clamping, extra cleanup recovery and root-pagination corrections are proposals, not blanket authorization to change reference behavior. Verify each against reference and approved product requirements. Preserve unexplained behavior while investigating.

## Differences and evidence
Go package structure, dependency injection and type safety may adapt freely while preserving observable behavior. Already confirmed product differences must be explicitly mapped to PRD requirements. Any further behavior difference requires source anchors, trigger, reference trace, proposed trace, justification and tests, and user confirmation before implementation. If a protocol detail is unknown, record the evidence gate; do not fill it with an invented implementation.

## Worker deliverables and quality gate
Each work package supplies a source-to-Go module map and scenario traces listing ordered HTTP method/endpoint, redacted parameter/header structure, cache decisions, waits, retries, created/deleted objects and outcome. Compare both normal and failure cases. Trace differences must be attributable to approved requirements. Unknowns and actual service verification are reported separately.
Reviewer rejects unexplained behavioral deviations, dropped modules, missing source anchors and claims of parity based solely on similar endpoint names or passing unit tests. Full request parity remains the target; successful tests do not establish a guarantee about upstream risk control.

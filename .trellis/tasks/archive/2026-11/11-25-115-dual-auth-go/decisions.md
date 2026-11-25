# Confirmed product decisions

2026-11-25: User approved the consolidated behavior for inclusion in the PRD. This is planning approval, not implementation approval. PRD is the authoritative requirements document.

| ID | Confirmed behavior | Requirements |
|---|---|---|
| D1 | Reference wire behavior is baseline; evidence-backed defect fixes require recorded differences/tests | R5 |
| D2 | One Cookie and one Open credential set per mount; no rotation pool | R3 |
| D3 | Open media errors do not automatically fall back to Cookie | R8 |
| D4 | Initial dual validation; independent runtime credential health and capability checks | R3/R9 |
| D5 | Full scope including attachments/events and retained file/upload/transfer/offline capabilities | R6 |
| D6 | Per-task opt-in monitoring after complete initial snapshot, with reconciliation | R13 |
| D7 | Automatic scoped managed-output deletion; dedicated non-overlapping output; attachment sync modes respected | R11/R12 |
| D8 | Reference Emby temporary-copy semantics in session-aware adapter, ordinary STRM unchanged | R14 |
| D9 | 115 deletion only moves exact requested IDs to trash, matching EmbyNginx; remove the permanent-delete option and its fallback branch | R5/R6 |
| D10 | Deliver code and offline acceptance in this round; real-account QR/refresh/player/Emby/tree/event acceptance is explicitly deferred to a follow-up | Acceptance scope |
| D11 | Complete pure Go charset detection following bundled charset_normalizer3.5.1; offline differential validation before delivery; current three-encoding approximation is not accepted as final | R5/R10/R15 |

2026-11-25 execution follow-up: D9 and D10 were explicitly selected by the user in the batched clarification. D9 supersedes the earlier partial-outcome permanent-delete behavior. D10 does not authorize claims of live-service validation or account risk-control immunity.

Historical evidence below records how the decisions were derived; provisional wording is superseded by the table and PRD.

## User answers and reference follow-up
Confirmed: D1=A, D2=A, D5=A, D6=A. User asks to derive D3/D4/D7 from EmbyNginx and prioritize its production request behavior. Do not label unexplained delays/repeated requests as bugs without evidence.

D3 evidence: STRM router selects Open if present; unsuccessful Open result does not call Cookie. Emby reverse-proxy resolver differs: configured CookieType selects Cookie or Open branch (router/emby_nginx/emby_nginx.py:189,320). Open failure returns empty/logs notification; no automatic Cookie retry. Concurrent media-session handling can copy a file then resolve that temporary pickcode; load balancing is a separate configured path, not error fallback. Proposed decision: no automatic Open-to-Cookie fallback.

D4 evidence: separate clients/pools in config/cookie_init.py:24; Open branch at emby_nginx.py:320 can use Open client without checking Cookie. No global dual-credential health gate. Cookie client construction is not proof of health (util_115.py:54). Restored source explicitly fixes Open-only pool aliases, whereas original recovery notes describe a limitation; preserve this distinction. Proposed decision: independently track credential health, continue usable Open playback and suspend failed Cookie operations with notification; this is an explicit Go state model, not an existing Python degraded-state feature.

D7 evidence: util_115_tree.py:287 unconditionally reconciles missing STRM after tree processing; empty_folder setting at 289 controls directory cleanup only. util_115_monitor.py:202 deletes mapped directories recursively, media STRM variants and mapped non-media files; event type 22 consults recycle bin, 20/24 delete old path then create new. Proposed decision: automatic deletion synchronization as reference, but use scoped task-owned output; recursive deletion of mixed/manual output is a separate boundary needing review.

Further planning evidence required: map Emby duplicate-media temporary-copy workflow to existing LitePan Emby integration before deciding scope; do not silently replace it with URL cache coalescing. Reference request habits are baseline; any changes require observed defect and test evidence. Production experience informs baseline but does not establish immunity from upstream risk control.

## Consolidated disposition
D3: Open media resolution without Cookie error fallback. D4: capability-level degraded operation. D7: automatic managed STRM deletion, dedicated non-overlapping task output, metadata follows existing synchronization modes. User requested assistant to synthesize these choices; see design.md for proposed final behavior and evidence. These replace the earlier binary recommendations for design review.

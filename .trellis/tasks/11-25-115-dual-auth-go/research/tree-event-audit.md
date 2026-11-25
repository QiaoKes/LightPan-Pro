# Independent tree/event audit (2026-11-25, implementation in progress)

Read-only audit of current shared worktree; W6/W7 still implementing. No live account testing.

Final coordinator status: the sections below preserve historical findings. Production attachment/event wiring, absolute roots, scan-start baseline, bounded receipts, reconciliation outside locks, task-scope resets, same-target rename, pending output intent, raw JSON truthiness and subtitle exclusions have since been implemented and regression-tested. See tree-implementation.md, events-implementation.md and ../acceptance.md for final evidence. Live acceptance and disclosed path/encoding/purge limits remain open; the historical observations below must not be used as the current implementation checklist.

## Verified
- `drivers/115/client/tree_export.go:22` root files payload matches pinned SDK `p115client/client.py:12257-12264`: default limit32/offset0, and custom_order=2 is **conditional** on asc/fc_mix/o; reference `util_115_tree.py:134` calls fs_files() without these. Missing custom_order is not a defect.
- Export cleanup scalar fid is correct: reference `util_115_tree.py:177` calls fs_delete(status_info['file_id']); SDK `client.py:11100-11104` scalar maps to fid, iterable maps to fid[i]. fid[0] is not required here.
- Reference tree download `util_115_tree.py:166-177`: empty UA for link and body request, six attempts (initial retry=5), delete after successful body download before parsing. Go tree_export matches order/attempt count and empty UA.
- `internal/strm/tree.go:24-31` rejects incomplete snapshots before output publication or missing-output reconciliation. Root truncation produces incomplete flag. No per-media metadata lookup exists in generation loop: path URL writes only (`tree.go:94`). This prevents generation-time upstream lookup storms.
- Path references are explicit, no fake ID: tree emits Path only; BuildPathPlayURL embeds ParentID and relative path; `internal/api/strm_play.go:71` resolves through file service at playback. Live resolution/playability remains unverified.
- Ownership uses task inventory and content hashes (`tree_ownership.go`), rejects existing symlink traversal, preserves modified/unknown outputs. Scan/event serialize through WithTaskSync.

## In progress / follow-up gates sent to owners
- Current tree scan selects media and bypasses generic attachment pipeline (`tree.go:57-60`): subtitles/images/NFO and cloud/local/bidirectional policies not yet present in this path. W7 notified; do not mark A9 complete.
- Current EventSync has persistence and complete snapshot gate, overlap receipts, checkpoint after Apply; at inspection there is no production Poll caller or driver ReadLifeEvents adapter. Startup/opt-in and snapshot checkpoint initialization must be verified after W7 integration. Do not mark A10 complete.
- `event_sync.go:44-59` invokes Reconcile while holding task lock. If callback uses scanTreeTask (same lock), it deadlocks. W7 notified to keep reconciliation outside/reuse already-held lock safely.
- Cleanup API error currently makes export fail even after body obtained; failure accounting/owned temporary ledger and cancelled/download-failed exports remain to be checked against task requirements.
- Fixture tests do not prove real Chinese/special-name path playback or actual upstream download header sufficiency.

## Verification
`go test ./drivers/115/client ./internal/strm` passed (2.961s and 5.069s). Current tests cover only implemented state, not missing production event/attachment wiring.

## Production event follow-up audit
Inspected drivers/115/events.go and internal/strm/service_events.go after production integration landed. The earlier statement of absent wiring is superseded; correctness remains in progress. `go test ./drivers/115 ./drivers/115/client ./internal/strm` passes (cached), but these gaps are not covered by tests.

### Confirmed integration defects (W7 and coordinator notified)
1. service_events.go relative closure (~95) scopes against task.Path, which is mount-relative; W6 uses absolute snapshot.RootPath. Persist the verified absolute root and root/account identity in the baseline, then consume it for events. Generated event URL (~191) uses task.Path as root instead of ParentID, including empty root. Use the same path reference identity as full scan.
2. treeBaselineComplete:36 sets initial SinceUnix at scan END. Changes during a scan longer than30sec can disappear before first event overlap. Carry scan START into baseline commit; preserve old durable watermark across calibration until events covering scan interval are replayed.
3. EventSync Poll copies every receipt forever. Retain receipts within durable replay/overlap horizon and prune older receipts only after watermark safely committed.
4. service_events.go directory create/rename returns error (~182), while Poll Apply errors do not set NeedsReconcile. Same directory event blocks all subsequent checkpoints forever. Request task reconciliation outside held task lock and replay/mark only after full verified reconciliation. Reference monitor actually traverses directory creates; do not silently discard.
5. UpdateTask service.go:412 persists edited task without resetting prior event baseline/checkpoint on root/account changes. Serialize update with synchronization and invalidate scope-bound state before enabling new monitor.
6. Same-output rename: movie.mkv→movie.mp4 can map both to movie.strm. Current code accepts old owned bytes unchanged then remove(OldPath) deletes that same STRM. Compare canonical managed targets, atomically update changed reference, and never retire the newly published target.
7. Existing attachment destination returns early before retiring old rename output; checkpoints success despite stale old output. Ownership is saved before attachment O_EXCL publication, leaving false ownership on failure/crash. STRM publication saves after write, leaving an orphan on crash before ownership save. Use recoverable publication/inventory intent and collision-specific tests, without adopting unknown files.

### Fidelity questions cleared / requiring explicit evidence
- Reference util_115_monitor.py:285–289 treats BOTH type20 and24 as old/new filename in the current parent. Current Go mirrors that; assuming type24 must be a cross-parent move would be an unsupported deviation. Actual move support remains an acceptance gap: use persistent prior file→path identity/reconciliation when old parent is not in event payload, rather than guessing it.
- Reference :249 determines directory by Python file_size truthiness. Numeric0 is classified directory in reference too; Go includes string0 as directory, while Python nonempty string0 is truthy. Preserve raw JSON type semantics or verify actual wire type. Empty media must not indefinitely block synchronization; source ambiguity should trigger safe reconciliation.
- Reference recycle :276 matches name plus signed dtime-update_time<=3, without lower bound or ID match. Go duplicates this. A much older recycled same-name object can therefore match. This is inherited behavior, not silently fixable under fidelity contract: document evidence and approved safety requirement before tightening to absolute/nonnegative delta/identity. Meanwhile do not claim strong deletion verification solely from this predicate.
- Go skips all /subtitle- paths, reference :260 skips subtitle files specifically in create-file branch; non-subtitle data under that prefix may diverge. Needs explicit scope decision/source trace.

Suggested regression coverage: scan-start replay across long export, mounted absolute baseline, root/account edits, numeric/string zero size, same basename extension rename, existing destination rename, directory followed by file event, bounded receipt horizon, crash between publication and inventory commit.

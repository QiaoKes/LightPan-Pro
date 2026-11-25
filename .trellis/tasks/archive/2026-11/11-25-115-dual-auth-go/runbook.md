# 115 first-deployment and acceptance runbook

## Mount

Choose driver `115`. Supply Cookie and an Open access/refresh token pair belonging to the same 115 account, either manually or with the two separate QR flows. Cookie QR defaults to qandroid; Open QR uses the reference client ID. The save operation validates both identities online. No old `115_Open` configuration or STRM migration is provided for this new fork.

Keep the download mode at its default `redirect` for 302 playback. Proxy mode intentionally transfers media through LitePan and is a different user-selected mode. Configure the mount root explicitly when needed. Cookie and Open health are displayed independently; reconnect the failed side without replacing the healthy side. Disabling a mount blocks cached cloud playback too.

## Tree task

Create a dedicated, empty task output directory. Overlapping task outputs are rejected. Choose the remote root, recursion/media options and attachment policy. New 115 tasks default to cloud-primary. Local-primary preserves local metadata; bidirectional additionally uploads eligible local-only metadata using the existing conflict policy. Unknown or locally modified files are retained.

First run completes the tree export and publishes an owned inventory. Only a complete snapshot permits missing-file cleanup. Generated STRMs contain explicit path references; they do not pretend the export supplied stable file IDs. A nonzero mount root is mapped to its absolute cloud path during the scan, not on every seek.

Monitoring is opt-in and defaults off. Enable it after the first successful scan. Tasks sharing one account share a 30-second upstream fetch. A failed or unverifiable batch requires a complete reconciliation before continuing. Changes to account/root reset the baseline. Cookie failures stop Cookie work while valid Open playback remains available.

## Playback acceptance

Record the account, entrypoint, file, UA and effective device for each scenario, with secrets redacted. Keep control API requests separate from player-to-CDN Range traffic.

| Scenario | Observe |
|---|---|
| Initial ordinary STRM | Reference path double lookup, one successful downurl, final302 |
| 20 GET/HEAD seeks with same identity | No additional metadata/downurl while the1200s cache is valid |
| Changed UA / expired cache | Separate resolution, no Cookie media fallback |
| Concurrent same-key cold requests | One coalesced resolution |
| Open401 | Failure/original Emby bypass as applicable, no arbitrary refresh or Cookie retry |
| Cookie invalid, Open valid | Open media still works; tree/monitor asks for Cookie reconnect |
| Same-media Emby devices | Exact session predicate; temporary copy only when required |
| Copy success/failure/restart | Known copied ID remains tracked; ten-second post-link cleanup; original untouched |
| Rename/delete followed by play | Changed generation prevents stale cached cloud link |
| Disabled mount | No cached direct link and no new remote request |

Inspect initial play, repeated play and seeks in more than one actual player. A successful run cannot guarantee future upstream risk-control behavior. Do not repeatedly poll the115 service merely to produce a health dashboard.

## Tree/event acceptance

Use a small test directory containing nested media, Unicode names, ordinary metadata and srt/ass/ssa subtitles. Verify root and mounted-subtree paths. Exercise create, same-basename extension rename, directory rename/move, removal and restart during synchronization. Confirm unknown local files and modified owned files survive. Cancel/timeout/incomplete exports must not remove existing outputs. Ensure a cloud-primary attachment removal does not override a local-primary task.

## Known limits

- D10 explicitly defers real-account QR, refresh, CDN/player/Emby, tree and event tests to follow-up. This round delivers code and offline acceptance; no live credentials were used.
- Reference crypto, required request shapes and default character detection are verified offline against the restored artifact. D11/W9 ports charset-normalizer3.5.1 in pure Go, with2,690 independent detector/text-reader cases and component vectors; this is finite coverage, not exhaustive equivalence or every configurable Python API.
- Required monitor component escaping and Cookie path decoding are implemented from bundled vectors. Ordinary reference tree exports join raw names, so slash-name boundaries lost by that producer cannot be reconstructed unambiguously. Special subtitles intentionally retain raw names. Unused generic SDK path modes are outside this business port.
- A remote copy may succeed before its returned ID is discovered and persisted. This unknown-ID crash window cannot safely be cleaned by guessing or deleting similarly named files. Known-ID recovery is implemented.
- D9 adopts trash-only deletion, matching the reference. There is no permanent-delete option or partial-error fallback. Ordinary deletion and temporary-copy cleanup move only their exact IDs to trash; no recycle-list or purge calls are issued.
- A move without a known old identity/path may require full reconciliation. No old path or stable ID is fabricated.
- The bundled cipher source and package metadata disagree on licensing. Original notices and license are retained; public distribution remains pending resolution of the applicable license and compatibility.

## Maintenance evidence

See `research/protocol-baseline.md`, `research/playback-implementation.md`, `research/tree-implementation.md`, `research/charset-implementation.md`, `research/events-implementation.md` and `research/integration-audit.md`. The executable maintenance contract is `.trellis/spec/backend/115-reference-contract.md`. Keep the source hashes, cipher attribution/license and request fixtures with future changes.

# Playback parity baseline — EmbyNginx

Goal: port the production-used reference playback path and verify equivalence at HTTP/request boundaries. Existing LitePan cache alone is not evidence of parity. Source root: /Users/saki/Desktop/code/ebn/restored-python.

| Stage | Reference behavior | Source | Go acceptance |
|---|---|---|---|
| Emby request identity | MediaSourceId may replace video_id; DeviceId and PlaySessionId read; absent device uses IP inside resolver | router/emby_nginx/emby_nginx.py:109,516 | Preserve explicit session/device context; represent missing identity instead of guessing from UA |
| Cache before upstream work | server_name/video_id/(UA+device) lookup before PlaybackInfo and cloud operations; retain 1200 seconds | same:109; cache helpers:39 | Repeat seek same identity hits cache before media/session/cloud resolution; avoid raw concat collisions with structured key |
| Normal resolve | Branch selected by configured or path-derived cookie_type; Open branch is not universally selected | same:138,189,320 | Dual-credential design deliberately chooses Open media path, while preserving branch request semantics; record this difference |
| Same-media concurrent devices | Session predicate chooses original or temporary copied file; not every Range request | same:338 | Same-device seek does not create copies; distinct-device predicate covered with recorded session fixtures |
| Temporary cleanup | Wait 10 seconds then delete exact temporary ID | same:67 | Keep source timing as baseline, trace cleanup and retry only against owned object IDs; extra durability is documented improvement |
| STRM route | Cache keyed cookie/path/UA before selecting Open-first client; Lavf goes through queue | router/strm/strm.py:211 | Keep STRM semantics distinct from Emby device cache |
| Queue cadence | Cookie Lavf work waits to space calls 3s; Open priority queue has 2 workers; Open API decorator shares rate=1/capacity=1 | router/strm/strm.py:181; config/constant.py:16; util/wangpan/util_115_open.py:176 | Record queue versus actual API rate; do not mistake worker count for request allowance |
| Response | Valid direct link returns HTTP 302 | router/emby_nginx/emby_nginx.py:516 | Direct response exits before generic proxy streaming/range-splitting |
| Emby resolution failure | Redirects through original Emby /embydirect endpoint; no direct Cookie retry in failed Open branch | same:559 | Preserve original-Emby fallback in Emby integration if bypass is available; prevent self-redirect recursion. Ordinary STRM has no such fallback |

## Evidence still required before implementation
- Full PlaybackInfo response rewrite and nginx bypass config contract; ensure original-Emby fallback is wired through existing Go proxy rather than inventing a deployment URL.
- SDK bundled-version wire equivalence, per-player UA behavior, current Go proxy pre-resolve calls and exact metadata/session request counts.
- Differential fixtures for start, same-device seek, concurrent misses, expiry, distinct devices, resolution failure and fallback loop prevention.

PRD seek tests are necessary but not sufficient: the acceptance baseline is the full reference behavior including session selection, fallback and request cadence. Do not assert all workflow parity is finished or that 20 seeks alone prove no upstream risk control.

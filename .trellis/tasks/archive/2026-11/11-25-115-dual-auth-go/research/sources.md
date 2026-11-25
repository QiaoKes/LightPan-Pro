# Source inventory and evidence

- Business reference: `/Users/saki/Desktop/code/ebn/restored-python` (existing local source).
- SDK repository: `https://github.com/ChenyangGao/p115client`
- Local checkout: `/Users/saki/Desktop/code/ebn/115-reference/p115client`
- Examined commit: `b649be77a6c77681e26d6eb1d1d013418b49d5e8`; pyproject version `0.0.9.7.7`.
- SDK checkout is a shallow clone. Preserve commit when obtaining more history.
- Target repository: `/Users/saki/Desktop/code/ebn/LitePan`.
- Reference bundled SDK is bytecode; equivalence with the cloned commit is NOT established.
- SDK MIT notice must accompany derived implementation where applicable. Cipher dependencies require their own source/version/license inventory.

## Verified source anchors
Paths below are relative to the corresponding source root.

| Flow | Business reference | Observed behavior / SDK evidence |
|---|---|---|
| Cookie QR | util/wangpan/util_115.py:306 | GET qrcodeapi `/api/1.0/{app}/1.0/token/`; returns uid/time/sign |
| QR image | util/wangpan/util_115.py:322 | `/api/1.0/web/1.0/qrcode?uid=...` |
| Cookie poll/exchange | util/wangpan/util_115.py:368 | GET `/get/status/`, uid/time/sign; status=2; POST passportapi `/app/1.0/{app}/1.0/login/qrcode/`, multipart app/account; collect response cookies; up to 5 polls with 7 second sleeps |
| Open QR | util/wangpan/util_115_open.py:59 | POST passportapi `/open/authDeviceCode`, client_id, code_challenge, code_challenge_method=sha256; verifier length up to 128; standard Base64 SHA256, not assumed Base64URL |
| Open exchange/refresh | util/wangpan/util_115_open.py:115 | POST `/open/deviceCodeToToken` uid/code_verifier; `/open/refreshToken` refresh_token |
| Token lifetime | util/wangpan/util_115_open.py:141 | lock, refresh before expiry, update persistent token and Bearer header |
| API scheduling | util/wangpan/util_115_open.py:176; util/queue/rate_limit.py | shared decorator bucket rate=1 capacity=1; acquisition retries 15, 1 second spacing; HTTP 429 waits reset+5 then recursively calls |
| Direct link | util/wangpan/util_115_open.py:233 | POST proapi `/open/ufile/downurl`, pick_code; request-specific UA |
| Path lookup | util/wangpan/util_115_open.py:198,241 | POST `/open/folder/get_info` path or GET file_id; current path download repeats lookup twice |
| Selection/cache | router/strm/strm.py:211; config/cookie_init.py:24 | Open-first same-user pool; Cookie alternative; Lavf queued |
| Cache lifetime | router/emby_nginx/emby_nginx.py:39 | server/video/UA identity; 1200 second retention |
| Directory export | util/wangpan/util_115_tree.py:118 | root fs_files / named fs_dir_getid; export; poll; download with empty UA; delete own exported object; parse tree; produce STRM |
| Tree SDK | p115client/client.py:11808,11910 (SDK root) | POST webapi `/files/export_dir`, file_ids,target=U_1_0; GET same endpoint export_id |
| Cookie download SDK | p115client/client.py:8762,9030 (SDK root) | current high-level default app=os_windows; branch-specific downurl endpoints; RSA request/response processing near 9095; MUST compare bundled default before selecting wire behavior |
| Change events | util/wangpan/util_115_monitor.py:50 | life_list GET, limit=1000,show_type=0,start_time minus 30 seconds; 10 second timeout; flatten items and sort update_time |

## Remaining evidence gates
G1: fingerprint bundled client/cipher, recover or locate matching source, document differences from cloned SDK.
G2: trace request wrapper, headers, serialization, cipher keys/padding and response decoding; create fixed request/response vectors without real secrets.
G3: enumerate event types, cursor semantics, subtitle endpoints, root pagination and malformed-tree cases.
G4: complete current account persistence/auth scheduling and STRM reference call paths before shared contracts are frozen.
These are explicit research deliverables, not verified implementation details or user questions. No real-account tests have run.

# W9 目录树字符集探测严格移植

Status: implementation offline verified; workflow commit confirmation pending. User D11 explicitly authorizes full pure Go detection following reference algorithm. Parent PRD R5/R10/R15 and A3/A5/A13 apply.

Reference: /Users/saki/Desktop/code/ebn/115-reference/charset_normalizer, official tag3.5.1 commit e239bdc5cc1eb1f0db08d4046ad531f805dbea71. Compiled source trees for api/legacy/utils/models/constants match bundled Python3.12 bytecode. See parent research/protocol-vectors/charset-source-comparison.json and charset-framework.json.

Deliver default legacy.detect behavior (from_bytes defaults), candidate order, strict candidate decoding, mess/coherence scoring/ranking, BOM/signature handling and final Python text-open errors=ignore/universal-newline semantics in Go. Preserve default output when detection returns no encoding (reference Linux UTF8 default). No Python runtime/build dependency. No replacement detector with unrelated heuristics. No new115 requests.

Acceptance: pinned Python oracle component and whole-detector differential fixtures, 99-codec candidate coverage/aliases, ASCII/BOM/UTF/CJK/legacy/stateful/malformed/binary/small/large text, tree leaf path comparison, cancellation/request traces unchanged, race and pure Go Linux build. Native vs pure3.5.1 scoring equivalence must be evidenced. Verify tests cover behavior, not merely implementation.

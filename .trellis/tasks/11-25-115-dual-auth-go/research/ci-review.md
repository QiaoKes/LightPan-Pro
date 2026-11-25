# CI / image review — 2026-11-25

Scope: workflow, Dockerfile, Docker ignore, README and Compose deployment configuration. No application changes or commits.

PASS: actionlint (go-run invocation), both Compose configs with LIGHTPAN_TAG=sha-abcdef0, frontend type-check, and upload-dispatcher tests. PRs receive contents-read permission and cannot run publish; package-write is isolated to publish. Checkout credentials are not persisted. Image tags derive from GITHUB_SHA, not interpolated PR text. SHA tag naming and amd64 architecture match README and both Compose files.

Docker uses Node 22, Go 1.26.6 and CGO disabled. Build regenerates frontend assets. Production charset/cipher embedded data survive ignore patterns; testdata, Python reference assets and tests are excluded. LICENSE and extensionless NOTICE files survive the context filters. Runtime installs certificates/timezone/FUSE and copies the Go binary; runtime license-file distribution remains inherited from existing Dockerfile and was not broadened in this review.

No mechanical issues required changes. Limits: Docker daemon unavailable, so actual image build/run and registry publication require the GitHub run. Full Go suite/race checks already passed in coordinator acceptance and were not duplicated. Official action SHA existence/version verification belongs to coordinator's upstream verification; actionlint establishes syntax only. Runner memory/time suitability remains subject to first hosted execution.

Follow-up fixed: runtime now includes root LICENSE, THIRD_PARTY_NOTICES.md, charset upstream/CPython licenses and NOTICE, and cipher license/NOTICE under /app/licenses with separate subdirectories. Added a Docker-ignore exception for THIRD_PARTY_NOTICES.md. All COPY sources exist. Independently spot-checked actions/checkout v7.0.1 via GitHub git-ref API: matches pinned 3d3c42e5aac5ba805825da76410c181273ba90b1. Coordinator reports all six action pins independently resolved by implementer and Linux fuse build passed.

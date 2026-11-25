# CI and container publishing contract

## 1. Scope / trigger

Applies to `.github/workflows/ci.yml`, Dockerfile and deployment examples. Full115 runtime is included in the ordinary Go binary; CI does not invoke Python, real115 services or production credentials. The image is `ghcr.io/qiaokes/lightpan-pro` for linux/amd64.

## 2. Signatures

Events: push, pull_request, workflow_dispatch. Jobs: validate, then publish with needs:validate. Image reference: `ghcr.io/qiaokes/lightpan-pro:sha-${GITHUB_SHA:0:7}`. Compose requires `LIGHTPAN_TAG=sha-<7hex>`; no implicit latest or upstream image. Actions are pinned to verified release commit SHAs.

## 3. Contracts

Default token permission is contents:read, checkout does not persist credentials. Only publish receives packages:write. Pull requests cannot publish. Registry login uses GITHUB_TOKEN, without an external registry secret. A failed validation prevents publication. Publication builds the same checked-out workflow commit, not an arbitrary remote branch.

Validate frontend with lockfile npm ci, type-check, test:upload and build before Go embedding. Run full Go tests/vet plus affected auth/playback/Emby/STRM/115 race suites, including charset. Read Go version from go.mod; Docker Go builder must match. Node22 satisfies the frontend tooling requirements. Linux binary build includes fuse with CGO disabled. Docker uses official npm/Go registries and rebuilds frontend assets.

Docker context excludes Trellis research/interpreters, agent files, node_modules and test-only assets while preserving immutable codec tables, source notices and licenses. Runtime license copies accompany the binary. Runtime paths/ports/FUSE remain compatible with existing deployment behavior. No CI test may add periodic115 probes or supply account credentials.

## 4. Validation / error matrix

| Condition | Behavior |
|---|---|
| Pull request, including forks | Validate; no registry login or publish |
| Failed frontend/Go/race check | Publish skipped |
| Successful push/manual | Build and publish only sha-<first7hex> |
| Missing Compose LIGHTPAN_TAG | Configuration fails explicitly |
| No local Docker daemon | Record limitation; actual hosted Docker build required |
| Registry permission failure | Fail job; do not substitute another registry/image |

## 5. Good / base / bad cases

Good: image revision label equals the workflow commit and its tag uses that commit's first7hex. Base: a new push passes validation then publishes its own tag. Bad: latest tag hides which code was built, or a PR receives package-write permission.

## 6. Required tests

Run actionlint and diff whitespace checks; resolve pinned actions to official release refs. Verify both Compose files with LIGHTPAN_TAG and verify the missing-tag error. Confirm Linux CGO-disabled fuse build, frontend type/upload tests and the hosted full workflow. Inspect publish job success and registry manifest for the exact SHA tag before calling image delivery complete.

## 7. Wrong versus correct

Wrong: publish from pull_request with write-all permissions and an unversioned latest tag.

Correct: contents:read by default; publish needs a successful validate job, excludes pull_request, receives only packages:write plus contents:read, and tags the exact checked-out workflow SHA.

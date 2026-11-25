# Docker publishing contract

## 1. Scope / trigger

User explicitly requested a minimal Docker-only pipeline on2026-11-25, superseding the initial separate validation job. Applies to .github/workflows/ci.yml and Docker/deployment files.

## 2. Signatures

Events: push and workflow_dispatch. One publish job. Image ghcr.io/qiaokes/lightpan-pro:sha-${GITHUB_SHA:0:7}, linux/amd64. Compose requires LIGHTPAN_TAG. No latest tag.

## 3. Contracts

Checkout exact workflow commit, set up Buildx, log in using GITHUB_TOKEN, build and push. Use contents:read and packages:write only; pinned official action SHAs. No separate Go/frontend tests, vet, race or type-check in CI. Docker compiles frontend assets and the CGO-disabled Go binary including fuse. Keep original runtime ports/paths and distributed licenses. Exclude local research/Python environments; include runtime codec tables.

## 4. Validation / error matrix

Failed Docker build: job fails and cannot publish a completed image. Successful build: publish exact SHA tag. Missing Compose LIGHTPAN_TAG: explicit configuration error. No local Docker daemon: actual hosted build establishes container build result.

## 5. Good / base / bad

Good: one job builds and publishes a SHA-tagged image. Base: push produces its own image. Bad: reintroducing separate lengthy validation jobs after the explicit Docker-only instruction.

## 6. Required verification

Workflow syntax review and hosted Docker build/publish result. Avoid redundant test suites. No real115 account credentials or calls.

## 7. Wrong versus correct

Wrong: validate-all job gates the user's Docker-only pipeline. Correct: build-push directly compiles source and publishes its SHA image.

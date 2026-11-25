package auth

import (
	"context"
	"litepan/internal/domain"
)

// GateChecker 是文件层等调用方依赖的最小认证闸门接口。
type GateChecker interface {
	Check(ctx context.Context, accountID int64) error
	HandlePassiveError(ctx context.Context, accountID int64) error
}

// CapabilityGateChecker is optional for callers that know the upstream stage.
type CapabilityGateChecker interface {
	CheckCapability(context.Context, int64, domain.AuthCapability) error
	HandleCapabilityError(context.Context, int64, domain.AuthCapability, error) error
}

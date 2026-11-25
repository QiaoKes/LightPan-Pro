package auth

import (
	"context"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

// Gate 是被动刷新闸门：请求前检查状态，401 时触发刷新。
type Gate struct {
	svc *Service
}

var _ GateChecker = (*Gate)(nil)

// Check 请求前闸门；failed/token_expired 阻断，冷却未到期阻断，冷却到期尝试被动刷新。
func (g *Gate) Check(ctx context.Context, accountID int64) error {
	if g == nil || g.svc == nil {
		return nil
	}
	if g.svc.accounts != nil {
		account, err := g.svc.accounts.Get(ctx, accountID)
		if err != nil {
			return err
		}
		if !account.IsActive {
			return domain.Errorf(domain.CodePermissionDenied, "account is disabled")
		}
	}
	st, err := g.svc.loadState(ctx, accountID)
	if err != nil {
		return err
	}
	if st.CookieAuth.Status != "" || st.OpenAuth.Status != "" {
		if st.CookieAuth.Status == domain.AuthActive || st.OpenAuth.Status == domain.AuthActive {
			return nil
		}
		return domain.Errf(domain.CodeAuthExpired)
	}
	now := g.svc.now()
	switch st.Status {
	case domain.AuthFailed, domain.AuthTokenExpired, domain.AuthCooldown:
		if err := authBlocked(st, now); err != nil {
			return err
		}
		return g.HandlePassiveError(ctx, accountID)
	default:
		return nil
	}
}

// HandlePassiveError 被动刷新入口（请求遇到认证错误时调用）。
func (g *Gate) HandlePassiveError(ctx context.Context, accountID int64) error {
	if g == nil || g.svc == nil {
		return nil
	}
	outcome, rerr := g.svc.Refresh(ctx, accountID, driver.CallerPassive)
	if outcome == driver.RefreshSuccess {
		return nil
	}
	if rerr != nil {
		return rerr
	}
	return domain.Errf(domain.CodeAuthExpired)
}

// HandleRetryFailure 重试后仍认证失败，将账号标记为异常；并发失败只登记首个。
func (g *Gate) HandleRetryFailure(ctx context.Context, accountID int64, cause error) {
	if g == nil || g.svc == nil || !IsAuthError(cause) {
		return
	}
	ctx, unlock, err := g.svc.lockAccount(ctx, accountID)
	if err != nil {
		return
	}
	defer unlock()
	st, err := g.svc.loadState(ctx, accountID)
	if err != nil || st.Status != domain.AuthActive {
		return
	}
	g.svc.recordRefreshFailure(ctx, accountID, driver.RefreshRetryable, driver.CallerPassive, cause)
}

func (g *Gate) CheckCapability(ctx context.Context, accountID int64, capability domain.AuthCapability) error {
	if g == nil || g.svc == nil {
		return nil
	}
	if g.svc.accounts != nil {
		account, err := g.svc.accounts.Get(ctx, accountID)
		if err != nil {
			return err
		}
		if !account.IsActive {
			return domain.Errorf(domain.CodePermissionDenied, "account is disabled")
		}
	}
	st, err := g.svc.loadState(ctx, accountID)
	if err != nil {
		return err
	}
	if st.CookieAuth.Status == "" && st.OpenAuth.Status == "" {
		return g.Check(ctx, accountID)
	}
	health := st.CookieAuth
	switch capability {
	case domain.AuthCapabilityCookieAPI:
	case domain.AuthCapabilityOpenAPI:
		health = st.OpenAuth
	default:
		return domain.Errorf(domain.CodeValidation, "unknown credential capability")
	}
	if health.Status != domain.AuthActive {
		return &driver.CapabilityAuthError{Capability: capability, Cause: domain.Errf(domain.CodeAuthExpired)}
	}
	return nil
}
func (g *Gate) HandleCapabilityError(ctx context.Context, accountID int64, capability domain.AuthCapability, cause error) error {
	if g == nil || g.svc == nil || !IsAuthError(cause) {
		return cause
	}
	ctx, unlock, err := g.svc.lockAccount(ctx, accountID)
	if err != nil {
		return err
	}
	defer unlock()
	st, err := g.svc.loadState(ctx, accountID)
	if err != nil {
		return err
	}
	var health *domain.CredentialHealth
	switch capability {
	case domain.AuthCapabilityCookieAPI:
		health = &st.CookieAuth
	case domain.AuthCapabilityOpenAPI:
		health = &st.OpenAuth
	default:
		return domain.Errorf(domain.CodeValidation, "unknown credential capability")
	}
	health.Status = domain.AuthTokenExpired
	// Persist a bounded, generic error; upstream messages may contain credentials.
	health.LastError = "credential expired; reconnect required"
	var persistErr error
	if patcher, ok := g.svc.authStates.(domain.CredentialHealthPatcher); ok {
		var cookie, open *domain.CredentialHealth
		if capability == domain.AuthCapabilityCookieAPI {
			cookie = health
		} else {
			open = health
		}
		persistErr = patcher.PatchCredentialHealth(ctx, accountID, st.CredentialGeneration, cookie, open)
	} else {
		persistErr = g.svc.authStates.Upsert(ctx, st)
	}
	if persistErr != nil {
		return persistErr
	}
	if capability == domain.AuthCapabilityOpenAPI && g.svc.credentialInvalidator != nil {
		g.svc.credentialInvalidator(accountID)
	}
	return &driver.CapabilityAuthError{Capability: capability, Cause: cause}
}

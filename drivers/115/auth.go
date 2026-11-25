package pan115

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

var refreshFatalCodes = map[int64]struct{}{
	40140115: {},
	40140116: {},
	40140119: {},
	40140120: {},
}

var refreshRetryableCodes = map[int64]struct{}{
	40140117: {},
	40140121: {},
}

func classifyRefreshError(err error) driver.RefreshOutcome {
	code := errorCode(err)
	if _, ok := refreshFatalCodes[code]; ok {
		return driver.RefreshFatal
	}
	if _, ok := refreshRetryableCodes[code]; ok {
		return driver.RefreshRetryable
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "refresh_token") && strings.Contains(msg, "empty") ||
		strings.Contains(msg, "不能都为空") {
		return driver.RefreshFatal
	}
	return driver.ClassifyOAuthRefreshError(err)
}

func errorCode(err error) int64 {
	if ae, ok := domain.AsAppError(err); ok {
		msg := ae.Message
		if i := strings.Index(msg, "("); i >= 0 {
			if j := strings.Index(msg[i+1:], ")"); j >= 0 {
				codeStr := msg[i+1 : i+1+j]
				if c, err := strconv.ParseInt(strings.TrimSpace(codeStr), 10, 64); err == nil {
					return c
				}
			}
		}
	}
	msg := err.Error()
	const prefix = "code: "
	if idx := strings.Index(msg, prefix); idx >= 0 {
		rest := msg[idx+len(prefix):]
		end := strings.Index(rest, ",")
		if end < 0 {
			end = len(rest)
		}
		if c, err := strconv.ParseInt(strings.TrimSpace(rest[:end]), 10, 64); err == nil {
			return c
		}
	}
	if i := strings.Index(msg, "115 刷新失败("); i >= 0 {
		rest := msg[i+len("115 刷新失败("):]
		if j := strings.Index(rest, ")"); j >= 0 {
			if c, err := strconv.ParseInt(strings.TrimSpace(rest[:j]), 10, 64); err == nil {
				return c
			}
		}
	}
	return 0
}

func (d *Driver) RefreshAuth(ctx context.Context, _ driver.RefreshCaller) (driver.RefreshOutcome, error) {
	d.mu.Lock()
	expiry := d.tokenExpires
	d.mu.Unlock()
	if !expiry.IsZero() && !expiry.Before(time.Now().Add(time.Second)) {
		return driver.RefreshSuccess, nil
	}
	_, err := d.doRefresh(ctx)
	if err != nil {
		return classifyRefreshError(err), err
	}
	return driver.RefreshSuccess, nil
}

type refreshData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (d *Driver) doRefresh(ctx context.Context) (string, error) {
	token, err := d.RefreshToken(ctx, d.currentToken, d.exchangeToken, classifyRefreshError)
	if ae, ok := domain.AsAppError(err); ok && ae.Code == domain.CodeAuthExpired {
		return token, &driver.CapabilityAuthError{Capability: domain.AuthCapabilityOpenAPI, Cause: err}
	}
	return token, err
}

func (d *Driver) exchangeToken(ctx context.Context) (string, error) {
	d.mu.Lock()
	refresh, oldToken := d.refresh, d.token
	generation := cloneGeneration(d.generation)
	oldExpiry := d.tokenExpires
	health := d.openHealth
	d.mu.Unlock()
	if refresh == "" {
		return "", domain.Errorf(domain.CodeAuthExpired, "缺少 refresh_token，无法刷新访问令牌")
	}
	var data refreshData
	if err := d.postPassport(ctx, refreshURL, urlValues(map[string]string{"refresh_token": refresh}), &data); err != nil {
		return "", err
	}
	if data.AccessToken == "" {
		return "", domain.Errorf(domain.CodeDriverError, "115 刷新响应缺少 access_token")
	}
	refreshTok := data.RefreshToken
	if refreshTok == "" {
		refreshTok = refresh
	}
	expiry := oldExpiry
	if data.ExpiresIn > 0 {
		expiry = time.Now().Add(time.Duration(data.ExpiresIn) * time.Second)
	}
	d.mu.Lock()
	stale := d.refresh != refresh || d.token != oldToken || !sameGeneration(d.generation, generation)
	current := d.token
	d.mu.Unlock()
	if stale {
		return current, nil
	}
	health.Status = domain.AuthActive
	health.LastError = ""
	if d.persist != nil {
		if err := d.persist(ctx, domain.AuthCredentials{AccessToken: data.AccessToken, RefreshToken: refreshTok, TokenExpires: expiry, OpenAuth: &health, ExpectedGeneration: generation}); err != nil {
			return "", domain.Wrap(domain.CodeInternal, err)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.refresh != refresh || d.token != oldToken || !sameGeneration(d.generation, generation) {
		return d.token, nil
	}
	d.token = data.AccessToken
	d.refresh = refreshTok
	d.tokenExpires = expiry
	d.openHealth = health
	if generation != nil && (oldToken != data.AccessToken || refresh != refreshTok) {
		next := *generation + 1
		d.generation = &next
	}
	return d.token, nil
}

func urlValues(values map[string]string) url.Values {
	out := url.Values{}
	for k, v := range values {
		out.Set(k, v)
	}
	return out
}

// CredentialHealth returns a snapshot without exposing either credential.
func (d *Driver) CredentialHealth() map[domain.AuthCapability]domain.CredentialHealth {
	d.mu.Lock()
	defer d.mu.Unlock()
	return map[domain.AuthCapability]domain.CredentialHealth{domain.AuthCapabilityCookieAPI: d.cookieHealth, domain.AuthCapabilityOpenAPI: d.openHealth}
}
func (d *Driver) requireCapability(cap domain.AuthCapability) error {
	h := d.CredentialHealth()[cap]
	if h.Status != "" && h.Status != domain.AuthActive {
		return &driver.CapabilityAuthError{Capability: cap, Cause: domain.Errf(domain.CodeAuthExpired)}
	}
	return nil
}

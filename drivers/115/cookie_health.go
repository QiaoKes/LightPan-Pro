package pan115

import (
	"context"
	pan115client "litepan/drivers/115/client"
	"litepan/internal/domain"
	"litepan/internal/driver"
)

// cookieErrorHandler captures the credentials before I/O so a late failure
// cannot invalidate a newer Cookie login. Use its result at Cookie boundaries.
func (d *Driver) cookieErrorHandler() func(context.Context, error) error {
	d.mu.Lock()
	cookie := d.cookie
	generation := cloneGeneration(d.generation)
	d.mu.Unlock()
	return func(ctx context.Context, err error) error { return d.cookieErrorAt(ctx, err, cookie, generation) }
}
func (d *Driver) wrapCookieError(ctx context.Context, err error) error {
	return d.cookieErrorHandler()(ctx, err)
}
func (d *Driver) cookieErrorAt(ctx context.Context, err error, cookie string, generation *uint64) error {
	kind := pan115client.KindOf(err)
	if kind != pan115client.AuthExpired && kind != pan115client.VerificationRequired {
		return err
	}
	d.mu.Lock()
	if d.cookie != cookie || !sameGeneration(d.generation, generation) {
		d.mu.Unlock()
		return &driver.CapabilityAuthError{Capability: domain.AuthCapabilityCookieAPI, Cause: err}
	}
	health := d.cookieHealth
	if kind == pan115client.AuthExpired {
		health.Status = domain.AuthTokenExpired
		health.LastError = "115 Cookie 已失效，请重新登录"
	} else {
		health.Status = domain.AuthFailed
		health.LastError = "115 Cookie 需要验证，请完成验证或重新登录"
	}
	d.cookieHealth = health
	d.mu.Unlock()
	if d.persist != nil {
		if e := d.persist(ctx, domain.AuthCredentials{CookieAuth: &health, ExpectedGeneration: generation}); e != nil {
			return &driver.CapabilityAuthError{Capability: domain.AuthCapabilityCookieAPI, Cause: e}
		}
	}
	return &driver.CapabilityAuthError{Capability: domain.AuthCapabilityCookieAPI, Cause: err}
}

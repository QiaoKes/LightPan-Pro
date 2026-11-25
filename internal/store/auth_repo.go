package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"litepan/internal/domain"
)

type authStateRepo struct{ db *DB }

func (r *authStateRepo) Get(ctx context.Context, accountID int64) (*domain.AuthState, error) {
	row := r.db.read.QueryRowContext(ctx,
		`SELECT account_id,status,access_token,refresh_token,token_expires,cookie,cookie_expires,
		        active_attempts,passive_attempts,last_error,last_failure_kind,next_retry_at,last_refresh_at,last_notified_at,capabilities_json,credential_generation
		 FROM account_auth_states WHERE account_id=?`, accountID)

	var (
		s            domain.AuthState
		status       string
		tokenExp     sql.NullString
		cookieExp    sql.NullString
		nextRetry    sql.NullString
		lastRefresh  sql.NullString
		lastNotified sql.NullString
		failureKind  sql.NullString
		healthJSON   string
	)
	err := row.Scan(&s.AccountID, &status, &s.AccessToken, &s.RefreshToken, &tokenExp,
		&s.Cookie, &cookieExp, &s.ActiveAttempts, &s.PassiveAttempts, &s.LastError, &failureKind,
		&nextRetry, &lastRefresh, &lastNotified, &healthJSON, &s.CredentialGeneration)
	if err != nil {
		return nil, wrapDB(err)
	}
	var health map[domain.AuthCapability]domain.CredentialHealth
	if err := json.Unmarshal([]byte(healthJSON), &health); err != nil {
		return nil, err
	}
	s.CookieAuth = health[domain.AuthCapabilityCookieAPI]
	s.OpenAuth = health[domain.AuthCapabilityOpenAPI]
	s.Status = domain.AuthStatus(status)
	s.LastFailureKind = domain.AuthFailureKind(failureKind.String)
	s.TokenExpires = parseTS(tokenExp)
	s.CookieExpires = parseTS(cookieExp)
	s.NextRetryAt = parseTS(nextRetry)
	s.LastRefreshAt = parseTS(lastRefresh)
	s.LastNotifiedAt = parseTS(lastNotified)
	return &s, nil
}

func (r *authStateRepo) Upsert(ctx context.Context, s *domain.AuthState) error {
	return r.upsert(ctx, s, nil, false)
}
func (r *authStateRepo) UpsertGeneration(ctx context.Context, s *domain.AuthState, expected uint64) error {
	return r.upsert(ctx, s, &expected, false)
}
func (r *authStateRepo) upsert(ctx context.Context, s *domain.AuthState, expected *uint64, runtimeOnly bool) error {
	status := s.Status
	if status == "" {
		status = domain.AuthActive
	}
	healthJSON, err := json.Marshal(map[domain.AuthCapability]domain.CredentialHealth{domain.AuthCapabilityCookieAPI: s.CookieAuth, domain.AuthCapabilityOpenAPI: s.OpenAuth})
	if err != nil {
		return err
	}
	condition := ""
	if expected != nil {
		condition = " WHERE account_auth_states.credential_generation=" + strconv.FormatUint(*expected, 10)
	}
	assignments := `
		    status=excluded.status, access_token=excluded.access_token, refresh_token=excluded.refresh_token,
		    token_expires=excluded.token_expires, cookie=excluded.cookie, cookie_expires=excluded.cookie_expires,
		    active_attempts=excluded.active_attempts, passive_attempts=excluded.passive_attempts,
		    last_error=excluded.last_error, last_failure_kind=excluded.last_failure_kind,
		    next_retry_at=excluded.next_retry_at,
		    last_refresh_at=excluded.last_refresh_at, last_notified_at=excluded.last_notified_at, capabilities_json=excluded.capabilities_json, credential_generation=excluded.credential_generation`
	if runtimeOnly {
		// Dual-provider expiries belong to credential persistence. Legacy providers
		// still use generic inferred scheduling through runtime bookkeeping.
		assignments = `status=excluded.status,
 token_expires=CASE WHEN COALESCE(json_extract(capabilities_json,'$.cookie_api.status'),'')<>'' OR COALESCE(json_extract(capabilities_json,'$.open_api.status'),'')<>'' THEN token_expires ELSE excluded.token_expires END,
 cookie_expires=CASE WHEN COALESCE(json_extract(capabilities_json,'$.cookie_api.status'),'')<>'' OR COALESCE(json_extract(capabilities_json,'$.open_api.status'),'')<>'' THEN cookie_expires ELSE excluded.cookie_expires END,
 active_attempts=excluded.active_attempts, passive_attempts=excluded.passive_attempts,
 last_error=excluded.last_error, last_failure_kind=excluded.last_failure_kind,
 next_retry_at=excluded.next_retry_at,last_refresh_at=excluded.last_refresh_at,last_notified_at=excluded.last_notified_at`
	}
	result, err := r.db.write.ExecContext(ctx,
		`INSERT INTO account_auth_states
		   (account_id,status,access_token,refresh_token,token_expires,cookie,cookie_expires,
		    active_attempts,passive_attempts,last_error,last_failure_kind,next_retry_at,last_refresh_at,last_notified_at,capabilities_json,credential_generation)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(account_id) DO UPDATE SET `+assignments+condition,
		s.AccountID, string(status), s.AccessToken, s.RefreshToken, tsValue(s.TokenExpires),
		s.Cookie, tsValue(s.CookieExpires), s.ActiveAttempts, s.PassiveAttempts, s.LastError,
		string(s.LastFailureKind), tsValue(s.NextRetryAt), tsValue(s.LastRefreshAt), tsValue(s.LastNotifiedAt), string(healthJSON), s.CredentialGeneration)
	if err == nil && expected != nil {
		if n, e := result.RowsAffected(); e != nil {
			return e
		} else if n == 0 {
			return domain.Errorf(domain.CodeValidation, "stale credential generation")
		}
	}
	return wrapDB(err)
}

func (r *authStateRepo) Delete(ctx context.Context, accountID int64) error {
	_, err := r.db.write.ExecContext(ctx, `DELETE FROM account_auth_states WHERE account_id=?`, accountID)
	return wrapDB(err)
}

// PatchCredentialHealth performs one SQL statement so independent observations cannot overwrite each other.
func (r *authStateRepo) PatchCredentialHealth(ctx context.Context, id int64, expected uint64, cookie, open *domain.CredentialHealth) error {
	expression := "capabilities_json"
	args := []any{}
	for _, side := range []struct {
		path   string
		health *domain.CredentialHealth
	}{{"$.cookie_api", cookie}, {"$.open_api", open}} {
		if side.health == nil {
			continue
		}
		encoded, err := json.Marshal(side.health)
		if err != nil {
			return err
		}
		expression = "json_set(" + expression + ", ?, json(?))"
		args = append(args, side.path, string(encoded))
	}
	if len(args) == 0 {
		return nil
	}
	args = append(args, id, expected)
	result, err := r.db.write.ExecContext(ctx, "UPDATE account_auth_states SET capabilities_json="+expression+" WHERE account_id=? AND credential_generation=?", args...)
	if err != nil {
		return wrapDB(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.Errorf(domain.CodeValidation, "stale credential generation")
	}
	return nil
}

func (r *authStateRepo) UpsertAuthRuntime(ctx context.Context, s *domain.AuthState) error {
	return r.upsert(ctx, s, &s.CredentialGeneration, true)
}

// PatchAuthCredentials merges a driver result against current SQL state, not a stale Go snapshot.
func (r *authStateRepo) PatchAuthCredentials(ctx context.Context, id int64, expected uint64, c domain.AuthCredentials) error {
	assignments := []string{}
	args := []any{}
	changed := []string{}
	changedArgs := []any{}
	for _, field := range []struct{ name, value string }{{"access_token", c.AccessToken}, {"refresh_token", c.RefreshToken}, {"cookie", c.Cookie}} {
		if field.value == "" {
			continue
		}
		assignments = append(assignments, field.name+"=?")
		args = append(args, field.value)
		changed = append(changed, field.name+"<>?")
		changedArgs = append(changedArgs, field.value)
	}
	for _, field := range []struct {
		name  string
		value time.Time
	}{{"token_expires", c.TokenExpires}, {"cookie_expires", c.CookieExpires}} {
		if field.value.IsZero() {
			continue
		}
		assignments = append(assignments, field.name+"=?")
		args = append(args, tsValue(field.value))
	}
	healthExpression := "capabilities_json"
	for _, field := range []struct {
		path   string
		health *domain.CredentialHealth
	}{{"$.cookie_api", c.CookieAuth}, {"$.open_api", c.OpenAuth}} {
		if field.health == nil {
			continue
		}
		encoded, err := json.Marshal(field.health)
		if err != nil {
			return err
		}
		healthExpression = "json_set(" + healthExpression + ", ?, json(?))"
		args = append(args, field.path, string(encoded))
	}
	if healthExpression != "capabilities_json" {
		assignments = append(assignments, "capabilities_json="+healthExpression)
	}
	if len(changed) > 0 {
		assignments = append(assignments, "credential_generation=credential_generation+CASE WHEN "+strings.Join(changed, " OR ")+" THEN 1 ELSE 0 END")
		args = append(args, changedArgs...)
	}
	if len(assignments) == 0 {
		return nil
	}
	args = append(args, id, expected)
	result, err := r.db.write.ExecContext(ctx, "UPDATE account_auth_states SET "+strings.Join(assignments, ",")+" WHERE account_id=? AND credential_generation=?", args...)
	if err != nil {
		return wrapDB(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.Errorf(domain.CodeValidation, "stale credential generation")
	}
	return nil
}

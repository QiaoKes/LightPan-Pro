package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/settings"
)

type qrStartReq struct {
	Purpose    string `json:"purpose"`
	App        string `json:"app"`
	ClientID   string `json:"client_id"`
	DriverType string `json:"driver_type"`
	Config     string `json:"config"`
}

type qrStartResp struct {
	Purpose       string `json:"purpose,omitempty"`
	Token         string `json:"token"`
	QRImageBase64 string `json:"qr_image_base64"`
	QRURL         string `json:"qr_url"`
	ExpiresIn     int    `json:"expires_in"`
	Title         string `json:"title,omitempty"`
	Hint          string `json:"hint,omitempty"`
}

type qrPollReq struct {
	Purpose    string `json:"purpose"`
	DriverType string `json:"driver_type"`
	Token      string `json:"token"`
}

type qrPollResp struct {
	Purpose       string            `json:"purpose,omitempty"`
	TokenExpires  string            `json:"token_expires,omitempty"`
	CookieExpires string            `json:"cookie_expires,omitempty"`
	Status        string            `json:"status"`
	Cookie        string            `json:"cookie,omitempty"`
	AccessToken   string            `json:"access_token,omitempty"`
	RefreshToken  string            `json:"refresh_token,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
	Message       string            `json:"message,omitempty"`
}

func (h *Handler) qrEphemeralConfig() driver.EphemeralConfig {
	return driver.EphemeralConfig{
		OAuthServerURL: func(ctx context.Context) string {
			if h.settings == nil {
				return domain.NormalizeOAuthServerURL("")
			}
			return domain.NormalizeOAuthServerURL(h.settings.String(settings.KeyOAuthServerURL))
		},
	}
}

func qrProvider(ctx context.Context, driverType, config string, cfg driver.EphemeralConfig) (driver.QRLoginProvider, func(context.Context), error) {
	dt := strings.TrimSpace(driverType)
	if dt == "" {
		return nil, nil, domain.Errorf(domain.CodeValidation, "缺少 driver_type")
	}
	drv, release, err := driver.OpenEphemeral(ctx, dt, config, cfg)
	if err != nil {
		return nil, nil, err
	}
	p, ok := drv.(driver.QRLoginProvider)
	if !ok {
		if purpose, supported := drv.(driver.PurposeQRLoginProvider); supported {
			return purposeQRAdapter{purpose}, release, nil
		}
	}
	if !ok {
		release(ctx)
		return nil, nil, domain.Errorf(domain.CodeValidation, "该驱动不支持扫码登录")
	}
	return p, release, nil
}

func (h *Handler) startQRLogin(w http.ResponseWriter, r *http.Request) {
	var in qrStartReq
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	p, release, err := qrProvider(r.Context(), in.DriverType, in.Config, h.qrEphemeralConfig())
	if err != nil {
		writeErr(w, err)
		return
	}
	defer release(r.Context())

	var res *driver.QRStartResult
	if pp, ok := p.(driver.PurposeQRLoginProvider); ok {
		res, err = pp.StartQRLoginFor(r.Context(), driver.QRLoginOptions{Purpose: in.Purpose, App: in.App, ClientID: in.ClientID})
	} else if in.Purpose != "" {
		err = domain.Errorf(domain.CodeValidation, "unsupported QR purpose")
	} else {
		res, err = p.StartQRLogin(r.Context())
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, qrStartResp{
		Purpose:       in.Purpose,
		Token:         res.Token,
		QRImageBase64: res.QRImageBase64,
		QRURL:         res.QRURL,
		ExpiresIn:     res.ExpiresIn,
		Title:         res.Title,
		Hint:          res.Hint,
	})
}

func (h *Handler) pollQRLogin(w http.ResponseWriter, r *http.Request) {
	var in qrPollReq
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(in.Token) == "" {
		writeErr(w, domain.Errorf(domain.CodeValidation, "缺少扫码会话 token"))
		return
	}
	p, release, err := qrProvider(r.Context(), in.DriverType, "", h.qrEphemeralConfig())
	if err != nil {
		writeErr(w, err)
		return
	}
	defer release(r.Context())

	var res *driver.QRPollResult
	if pp, ok := p.(driver.PurposeQRLoginProvider); ok {
		res, err = pp.PollQRLoginFor(r.Context(), in.Token, in.Purpose)
	} else if in.Purpose != "" {
		err = domain.Errorf(domain.CodeValidation, "unsupported QR purpose")
	} else {
		res, err = p.PollQRLogin(r.Context(), in.Token)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, qrPollResp{
		Purpose:       in.Purpose,
		TokenExpires:  qrExpiry(res.Credentials.TokenExpires),
		CookieExpires: qrExpiry(res.Credentials.CookieExpires),
		Status:        string(res.Status),
		Cookie:        res.Credentials.Cookie,
		AccessToken:   res.Credentials.AccessToken,
		RefreshToken:  res.Credentials.RefreshToken,
		Fields:        res.Fields,
		Message:       res.Message,
	})
}

func qrExpiry(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
func (h *Handler) cancelQRLogin(w http.ResponseWriter, r *http.Request) {
	var in qrPollReq
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if strings.TrimSpace(in.Token) == "" {
		writeErr(w, domain.Errorf(domain.CodeValidation, "missing QR token"))
		return
	}
	p, release, err := qrProvider(r.Context(), in.DriverType, "", h.qrEphemeralConfig())
	if err != nil {
		writeErr(w, err)
		return
	}
	defer release(r.Context())
	pp, ok := p.(driver.PurposeQRLoginProvider)
	if !ok {
		writeErr(w, domain.Errorf(domain.CodeValidation, "QR cancellation unsupported"))
		return
	}
	if err := pp.CancelQRLogin(r.Context(), in.Token, in.Purpose); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]bool{"cancelled": true})
}

type purposeQRAdapter struct{ driver.PurposeQRLoginProvider }

func (p purposeQRAdapter) StartQRLogin(ctx context.Context) (*driver.QRStartResult, error) {
	return p.StartQRLoginFor(ctx, driver.QRLoginOptions{})
}
func (p purposeQRAdapter) PollQRLogin(ctx context.Context, token string) (*driver.QRPollResult, error) {
	return p.PollQRLoginFor(ctx, token, "")
}

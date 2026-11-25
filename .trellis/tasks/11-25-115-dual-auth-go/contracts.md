# Frozen integration contracts v1

2026-11-25. Coordinator adopts this minimal contract to permit independent work. Reference-contract.md governs behavior. Changes require coordinator coordination before cross-owner files are edited.

## Generic authentication (W2-framework owner)
- Add driver.QRLoginOptions {Purpose, App, ClientID string} and optional PurposeQRLoginProvider with StartQRLoginFor(ctx,options), PollQRLoginFor(ctx,token,purpose), CancelQRLogin(ctx,token,purpose). Existing QRLoginProvider remains supported. API start/poll add purpose; start app/client_id optional; cancel endpoint uses driver_type/token/purpose. Poll response expiry fields RFC3339. Sessions opaque/server-owned; never trust client UID.
- Add domain.AuthCapability constants AuthCapabilityCookieAPI="cookie_api", AuthCapabilityOpenAPI="open_api". CredentialHealth {Status AuthStatus, VerifiedUID string, LastError string}. AuthState contains CookieAuth, OpenAuth plus CredentialGeneration uint64. Credentials contain optional health updates and ExpectedGeneration for conditional persistence; exact pointer field definitions W2 owns and must communicate.
- Optional driver capability-auth provider identifies dual accounts and health. Generic Gate.Check admits dual accounts if any capability usable; typed capability errors bypass generic whole-account refresh. Required capability checked by driver operation immediately before relevant stage. Public auth DTO credential_health and credential_statuses with cookie_api/open_api. W2 owns schema, gates, account persistence and API DTO. Driver provider implementation W4/W2-driver coordinates. All old providers retain existing behavior.

## Protocol transport (W1 owner)
Package `litepan/drivers/115/client`, package name `pan115client`.
`func New(httpClient *http.Client, cookie string) *Client`
`func (c *Client) SetCookie(cookie string)`
`func (c *Client) Request(ctx context.Context, method, rawURL string, query, form url.Values, headers http.Header) (json.RawMessage,error)` — Cookie-aware request, no implicit Open business limiter; preserve response envelope. Uses supplied HTTP client's transport so fixtures can intercept fixed reference hosts. No source-level global DNS overrides.
`func (c *Client) CookieDownload(ctx context.Context, pickcode, ua string) (*Download,error)` where Download {URL string; Headers http.Header; FileID, PickCode, Name string; Size int64}. Required cipher exact bundled defaults; no guessed protocol.
Cookie/Open QR protocol DTO/methods belong W1; communicate their concrete signatures immediately to driver-auth worker before implementing callers. Driver owns session lifecycle; protocol client performs wire operations.
W4 owns ordinary Open business transport migrated from existing driver, aligned to verified reference (shared rate=1/capacity=1 across instances, acquisition retry15/1s, 429 reset+5 loop, context cancellation). W1 does not create second Open business limiter. Do not alter reference call count/waits.

## Tree/events (W6/W7 owners)
New optional interfaces go in separate internal/driver/tree_snapshot.go and change_events.go; W6 owns tree contract, W7 owns events contract. Path-based entries do not invent stable IDs. Concrete method signatures must be messaged before adapters are written. Tree client file uses W1 Request/CookieDownload. Cookie capability and UID must be checked before scan/event I/O.
W6 owns driver tree.go and internal/strm tree integration/ownership. W7 owns driver events.go, client/events.go, client/subtitles.go and STRM new event coordinator; W6/W7 agree wiring in existing scanner/service files before edits. W7 waits for W6 snapshot shape but may implement protocol from local verified evidence independently.

## Playback (W5 owner)
W5 owns internal/playback and internal/embyproxy changes; optional driver copy interface goes in NEW internal/driver/temporary_playback.go. Publish exact signatures to W4. W4 owns adapter implementation in drivers/115/temporary_playback.go if requested, or delegates that NEW file explicitly to W5. W5 may own that adapter file after agreement; never concurrent edit ops.go. Emby identity/device/session state stays outside driver. Ordinary path-based STRM reuses existing path references. Source behavior differences only approved R3/R9/R11/R12/R14, no arbitrary proxy-to-stream behavior changes.

## File ownership
W4 owns rename drivers/115_Open to drivers/115 and registration imports; preserve existing file/upload/offline/profile capabilities. W2-driver (separate worker) owns drivers/115/qr.go and auth_sessions.go, NOT auth.go/driver.go/config.go; submit adapter requirements to W4. W1 owns client transport/cipher/auth wire files. W3 owns frontend. Coordinator owns task artifacts and final integration. No agent rewrites another owner's work; share required changes by messages.

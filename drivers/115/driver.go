package pan115

import (
	"context"
	"encoding/json"
	pan115client "litepan/drivers/115/client"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/httpx"
)

type Driver struct {
	driver.AuthRefreshControl
	add          Addition
	client       *http.Client
	uploadClient *http.Client
	oauthBase    string

	intervalGate driver.RequestIntervalGate
	persist      driver.AuthPersistFunc

	mu            sync.Mutex
	token         string
	refresh       string
	cookie        string
	cookieHealth  domain.CredentialHealth
	openHealth    domain.CredentialHealth
	openAdmission func(context.Context) error
	openSessionUA string
	openWait      func(context.Context, time.Duration) error

	tokenExpires time.Time
	generation   *uint64
	pickMu       sync.RWMutex
	pickBy       map[string]string
}

var config = driver.Config{
	Name:          "115",
	QRDeviceField: "app",
	QRDevices: []driver.FieldOption{
		{Value: "qandroid", Label: "115管理安卓端"}, {Value: "tv", Label: "安卓电视端"}, {Value: "web", Label: "网页端"}, {Value: "android", Label: "115生活安卓端"}, {Value: "115android", Label: "115安卓端"}, {Value: "ios", Label: "115生活ios端"}, {Value: "115ipad", Label: "iPad端"}, {Value: "alipaymini", Label: "支付宝小程序"}, {Value: "115ios", Label: "115ios端"},
	},
	DisplayName:            "115网盘",
	Description:            "115网盘双凭据接入，支持目录树、文件管理与 OpenAPI 播放",
	CardTags:               []string{"Cookie + OpenAPI", "双凭据", "支持302", "SHA1"},
	SortOrder:              2,
	AuthLabel:              "Cookie + OpenAPI",
	CardColor:              "#22A7F0",
	CardLogo:               "/logos/115.png",
	DefaultRoot:            "0",
	AuthType:               driver.AuthToken,
	SupportsAccountProfile: true,
	OAuthName:              "115网盘",
	TokenLifetime:          2 * time.Hour,
	RefreshAtExpiry:        true,
	ProvideHashes:          []string{"sha1"},
}

func New() driver.Driver { return &Driver{} }

func init() { driver.Register(New) }

func (d *Driver) Config() driver.Config { return config }

func (d *Driver) GetAddition() any { return &d.add }

func (d *Driver) SetAuthCredentials(creds domain.AuthCredentials) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if creds.ExpectedGeneration != nil && d.generation != nil && *creds.ExpectedGeneration < *d.generation {
		return
	}
	d.token = strings.TrimSpace(creds.AccessToken)
	d.refresh = strings.TrimSpace(creds.RefreshToken)
	d.cookie = strings.TrimSpace(creds.Cookie)
	d.tokenExpires = creds.TokenExpires
	if creds.ExpectedGeneration != nil {
		n := *creds.ExpectedGeneration
		d.generation = &n
	}
	if creds.CookieAuth != nil {
		d.cookieHealth = *creds.CookieAuth
	}
	if creds.OpenAuth != nil {
		d.openHealth = *creds.OpenAuth
	}
}

func (d *Driver) SetAuthPersister(fn driver.AuthPersistFunc) { d.persist = fn }

func (d *Driver) SetOAuthServer(baseURL string) { d.oauthBase = baseURL }

func (d *Driver) SetRequestIntervalGate(gate driver.RequestIntervalGate) { d.intervalGate = gate }

func (d *Driver) Init(ctx context.Context) error {
	if d.client == nil {
		d.client = httpx.NewClient(httpx.ClientOptions{Timeout: 30 * time.Second})
	}
	if d.uploadClient == nil {
		d.uploadClient = httpx.NewUploadClient(d.client, 60*time.Second, config.UploadUseHTTP2)
	}
	d.mu.Lock()
	token := d.token
	refresh := d.refresh
	d.mu.Unlock()
	if token == "" && refresh == "" {
		token = strings.TrimSpace(d.add.AccessToken)
		refresh = strings.TrimSpace(d.add.RefreshToken)
	}
	if token == "" && refresh == "" {
		return domain.Errorf(domain.CodeValidation, "access_token 与 refresh_token 不能都为空")
	}
	d.mu.Lock()
	d.token = token
	d.refresh = refresh
	d.mu.Unlock()
	if strings.TrimSpace(token) == "" {
		if _, err := d.doRefresh(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) Drop(context.Context) error {
	httpx.CloseClient(d.client)
	httpx.CloseClient(d.uploadClient)
	return nil
}

func (d *Driver) validateCookie(ctx context.Context, cookie string) (string, error) {
	raw, err := pan115client.New(d.client, cookie).Request(ctx, http.MethodGet, "https://my.115.com/", urlValues(map[string]string{"ct": "guide", "ac": "status"}), nil, nil)
	if err != nil {
		return "", err
	}
	var status struct {
		State json.RawMessage `json:"state"`
	}
	if err = json.Unmarshal(raw, &status); err != nil {
		return "", domain.Wrap(domain.CodeDriverError, err)
	}
	if len(status.State) == 0 || !isSuccessState(status.State) {
		return "", domain.Errorf(domain.CodeAuthExpired, "Cookie 已失效")
	}
	uid := ""
	for _, part := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == "UID" {
			uid = strings.SplitN(kv[1], "_", 2)[0]
		}
	}
	if uid == "" {
		return "", domain.Errorf(domain.CodeValidation, "Cookie 缺少 UID")
	}
	if n, err := strconv.ParseUint(uid, 10, 64); err != nil || n == 0 {
		return "", domain.Errorf(domain.CodeValidation, "Cookie UID 无效")
	}
	n, _ := strconv.ParseUint(uid, 10, 64)
	return strconv.FormatUint(n, 10), nil
}
func (d *Driver) validateOpen(ctx context.Context, token string) (string, error) {
	var info struct {
		UID flexString `json:"user_id"`
	}
	if err := d.beforeCall(ctx); err != nil {
		return "", err
	}
	if err := d.rawRequest(ctx, http.MethodGet, pathUserInfo, token, nil, nil, nil, &info, false); err != nil {
		return "", err
	}
	uid := strings.TrimSpace(string(info.UID))
	n, err := strconv.ParseUint(uid, 10, 64)
	if err != nil || n == 0 {
		return "", domain.Errorf(domain.CodeValidation, "Open UID 无效")
	}
	return strconv.FormatUint(n, 10), nil
}
func (d *Driver) Ping(ctx context.Context) error {
	return d.ValidateCredentialUpdate(ctx, domain.AuthCredentials{})
}

// ValidateCredentialUpdate probes changed credentials only. The retained side's
// verified identity remains authoritative even when its credential has expired.
func (d *Driver) ValidateCredentialUpdate(ctx context.Context, previous domain.AuthCredentials) error {
	d.mu.Lock()
	cookie := d.cookie
	originalCookie := cookie
	if cookie == "" {
		cookie = strings.TrimSpace(d.add.Cookie)
	}
	token, refresh := d.token, d.refresh
	generation := cloneGeneration(d.generation)
	d.mu.Unlock()
	if cookie == "" || token == "" || refresh == "" {
		return domain.Errorf(domain.CodeValidation, "Cookie、access_token 和 refresh_token 均为必填项")
	}
	cookieChanged := cookie != previous.Cookie
	openChanged := token != previous.AccessToken || refresh != previous.RefreshToken
	ch, oh := domain.CredentialHealth{}, domain.CredentialHealth{}
	if previous.CookieAuth != nil {
		ch = *previous.CookieAuth
	}
	if previous.OpenAuth != nil {
		oh = *previous.OpenAuth
	}
	var err error
	if cookieChanged || ch.VerifiedUID == "" {
		var uid string
		uid, err = d.validateCookie(ctx, cookie)
		if err != nil {
			return err
		}
		ch = domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: uid}
	}
	if openChanged || oh.VerifiedUID == "" {
		var uid string
		uid, err = d.validateOpen(ctx, token)
		if err != nil {
			return err
		}
		oh = domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: uid}
	}
	if ch.VerifiedUID == "" || ch.VerifiedUID != oh.VerifiedUID {
		return domain.Errorf(domain.CodeValidation, "Cookie 与 Open 凭据不属于同一用户")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cookie != originalCookie || d.token != token || d.refresh != refresh || !sameGeneration(d.generation, generation) {
		return domain.Errorf(domain.CodeValidation, "凭据在验证期间已更新，请重试")
	}
	d.cookie = cookie
	d.cookieHealth = ch
	d.openHealth = oh
	return nil
}
func cloneGeneration(g *uint64) *uint64 {
	if g == nil {
		return nil
	}
	n := *g
	return &n
}
func sameGeneration(a, b *uint64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func listPageLimit(pageNum int, remaining int64) int {
	if remaining <= 0 {
		return 0
	}
	cap := listPageFollow
	switch pageNum {
	case 1:
		cap = listPageFirst
	case 2:
		cap = listPageSecond
	}
	if int64(cap) > remaining {
		return int(remaining)
	}
	return cap
}

func (d *Driver) ListFiles(ctx context.Context, parentID string) ([]domain.FileItem, error) {
	parent := d.normalizeParent(parentID)
	var items []domain.FileItem
	offset := 0
	fetched := 0
	totalCount := int64(0)
	pageNum := 0

	for {
		pageNum++
		limit := listPageFirst
		if pageNum > 1 {
			remaining := totalCount - int64(fetched)
			if totalCount > 0 && remaining <= 0 {
				break
			}
			if totalCount > 0 {
				limit = listPageLimit(pageNum, remaining)
			} else {
				limit = listPageLimit(pageNum, int64(listPageFollow))
			}
			if limit <= 0 {
				break
			}
		}

		query := urlValues(map[string]string{
			"cid":      parent,
			"limit":    strconv.Itoa(limit),
			"offset":   strconv.Itoa(offset),
			"show_dir": "1",
		})
		var page listPageResp
		if err := d.apiCallFull(ctx, http.MethodGet, pathList, query, nil, &page); err != nil {
			return nil, err
		}
		if pageNum == 1 {
			totalCount = page.Count
		}
		if len(page.Data) == 0 {
			break
		}
		for _, f := range page.Data {
			if isTrashed(f) {
				continue
			}
			d.rememberPickCode(f)
			items = append(items, fileToItem(f))
		}
		fetched += len(page.Data)
		if totalCount > 0 && int64(fetched) >= totalCount {
			break
		}
		if len(page.Data) < limit {
			break
		}
		offset += len(page.Data)
	}
	return items, nil
}

func (d *Driver) GetFileInfo(ctx context.Context, fileID string) (*domain.FileItem, error) {
	id := strings.TrimSpace(fileID)
	root := d.rootID()
	if id == "" || id == "0" || id == root {
		return &domain.FileItem{
			ID:     root,
			Name:   "根目录",
			IsDir:  true,
			IDKind: domain.IDStable,
		}, nil
	}
	query := urlValues(map[string]string{"file_id": id})
	var info fileEntry
	if err := d.apiCall(ctx, http.MethodGet, pathFileInfo, query, nil, &info); err != nil {
		return nil, err
	}
	if info.entryID() == "" {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	d.rememberPickCode(info)
	item := fileToItem(info)
	return &item, nil
}

var (
	_ driver.Driver                    = (*Driver)(nil)
	_ driver.CapabilityAuthProvider    = (*Driver)(nil)
	_ driver.CredentialUpdateValidator = (*Driver)(nil)
	_ driver.InfoGetter                = (*Driver)(nil)
	_ driver.Downloader                = (*Driver)(nil)
	_ driver.Deleter                   = (*Driver)(nil)
	_ driver.Mover                     = (*Driver)(nil)
	_ driver.Copier                    = (*Driver)(nil)
	_ driver.Renamer                   = (*Driver)(nil)
	_ driver.FolderCreator             = (*Driver)(nil)
	_ driver.AuthRefresher             = (*Driver)(nil)
	_ driver.AuthCredentialConsumer    = (*Driver)(nil)
	_ driver.AuthPersistConsumer       = (*Driver)(nil)
	_ driver.OAuthConsumer             = (*Driver)(nil)
	_ driver.RequestIntervalConsumer   = (*Driver)(nil)
	_ driver.LocalUploader             = (*Driver)(nil)
	_ driver.OfflineDownloadProvider   = (*Driver)(nil)
	_ driver.OfflineURLDownloader      = (*Driver)(nil)
	_ driver.OfflineTaskRefresher      = (*Driver)(nil)
	_ driver.OfflineTaskDeleter        = (*Driver)(nil)
	_ driver.OfflineTorrentDownloader  = (*Driver)(nil)
)

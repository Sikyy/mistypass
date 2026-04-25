package talenta

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
)

var ErrPullCredentialRequired = errors.New("talenta pull credential is required")
var ErrInvalidPullCredential = errors.New("invalid talenta pull credential")
var ErrPullClientIDRequired = errors.New("talenta pull client_id is required")
var ErrPullClientSecretRequired = errors.New("talenta pull client_secret is required")
var ErrPullHTTPClientRequired = errors.New("talenta pull http client is required")
var ErrPullUnexpectedStatus = errors.New("unexpected talenta pull response status")
var ErrPullInvalidResponse = errors.New("invalid talenta pull response")

const (
	defaultPullBaseURL            = "https://api.mekari.com"
	defaultPullEmployeePath       = "/v2/talenta/v2/employee"
	defaultPullPageLimit          = 20
	defaultPullUpdatedAfterParam  = "updated_after"
	defaultPullUpdatedBeforeParam = "updated_before"
	defaultPullTimestampFormat    = "rfc3339"
	maxPullPageCount              = 500
)

type PullAdapter struct{}

type PullCredential struct {
	ClientID           string `json:"client_id"`
	ClientSecret       string `json:"client_secret"`
	BaseURL            string `json:"base_url,omitempty"`
	EmployeePath       string `json:"employee_path,omitempty"`
	PageLimit          int    `json:"page_limit,omitempty"`
	UpdatedAfterParam  string `json:"updated_after_param,omitempty"`
	UpdatedBeforeParam string `json:"updated_before_param,omitempty"`
	TimestampFormat    string `json:"timestamp_format,omitempty"`
}

func NewPullAdapter() *PullAdapter {
	return &PullAdapter{}
}

func (a *PullAdapter) Vendor() string {
	return "talenta"
}

func (a *PullAdapter) Pull(ctx context.Context, input hris.PullInput) (hris.PullResult, error) {
	credential, err := ParsePullCredential(input.CredentialValue)
	if err != nil {
		return hris.PullResult{}, err
	}
	if err := credential.ValidateForPull(); err != nil {
		return hris.PullResult{}, err
	}
	if input.HTTPClient == nil {
		return hris.PullResult{}, ErrPullHTTPClientRequired
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	limit := credential.NormalizedPageLimit()
	mode := hris.NormalizePullMode(input.Mode)
	items := make([]enterprise.EmployeeSyncInput, 0, limit)
	page := 1
	for {
		if page > maxPullPageCount {
			return hris.PullResult{}, fmt.Errorf("%w: page limit exceeded", ErrPullInvalidResponse)
		}
		endpoint, err := credential.EmployeeListURL(page, mode, input.LastSuccessAt, now)
		if err != nil {
			return hris.PullResult{}, err
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return hris.PullResult{}, err
		}
		if err := SignPullRequest(request, credential, now); err != nil {
			return hris.PullResult{}, err
		}

		response, err := input.HTTPClient.Do(request)
		if err != nil {
			return hris.PullResult{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		closeErr := response.Body.Close()
		if readErr != nil {
			return hris.PullResult{}, readErr
		}
		if closeErr != nil {
			return hris.PullResult{}, closeErr
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return hris.PullResult{}, fmt.Errorf("%w: status=%d body=%s", ErrPullUnexpectedStatus, response.StatusCode, strings.TrimSpace(string(body)))
		}

		pageItems, hasMore, err := decodePullEmployees(body, limit)
		if err != nil {
			return hris.PullResult{}, err
		}
		items = append(items, pageItems...)
		if !hasMore {
			break
		}
		page++
	}

	result := hris.NormalizePullResult(hris.PullResult{
		TenantID:    strings.TrimSpace(input.Connector.TenantID),
		Source:      hris.SyncSourceForVendor(input.Connector.Vendor),
		Actor:       hris.SyncActor,
		RequestID:   buildPullRequestID(input.Connector, now),
		Mode:        mode,
		ConnectorID: strings.TrimSpace(input.Connector.ID),
		Employees:   items,
		PulledAt:    now,
	})
	return result, nil
}

func (a *PullAdapter) SupportsIncremental(input hris.PullInput) bool {
	credential, err := ParsePullCredential(input.CredentialValue)
	if err != nil {
		return false
	}
	return credential.SupportsIncremental(input.LastSuccessAt)
}

func ParsePullCredential(raw string) (PullCredential, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return PullCredential{}, ErrPullCredentialRequired
	}

	if !strings.HasPrefix(value, "{") {
		return PullCredential{
			ClientID: value,
		}, nil
	}

	var credential PullCredential
	if err := json.Unmarshal([]byte(value), &credential); err != nil {
		return PullCredential{}, fmt.Errorf("%w: %v", ErrInvalidPullCredential, err)
	}
	credential.ClientID = strings.TrimSpace(credential.ClientID)
	credential.ClientSecret = strings.TrimSpace(credential.ClientSecret)
	credential.BaseURL = strings.TrimSpace(credential.BaseURL)
	credential.EmployeePath = strings.TrimSpace(credential.EmployeePath)
	credential.UpdatedAfterParam = strings.TrimSpace(credential.UpdatedAfterParam)
	credential.UpdatedBeforeParam = strings.TrimSpace(credential.UpdatedBeforeParam)
	credential.TimestampFormat = strings.TrimSpace(credential.TimestampFormat)
	if credential.PageLimit < 0 {
		credential.PageLimit = 0
	}
	return credential, nil
}

func (c PullCredential) ValidateForPull() error {
	if strings.TrimSpace(c.ClientID) == "" {
		return ErrPullClientIDRequired
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return ErrPullClientSecretRequired
	}
	return nil
}

func (c PullCredential) WebhookClientID() string {
	return strings.TrimSpace(c.ClientID)
}

func (c PullCredential) NormalizedBaseURL() string {
	baseURL := strings.TrimSpace(c.BaseURL)
	if baseURL == "" {
		return defaultPullBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func (c PullCredential) NormalizedEmployeePath() string {
	path := strings.TrimSpace(c.EmployeePath)
	if path == "" {
		return defaultPullEmployeePath
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func (c PullCredential) NormalizedPageLimit() int {
	limit := c.PageLimit
	if limit <= 0 {
		return defaultPullPageLimit
	}
	return limit
}

func (c PullCredential) NormalizedUpdatedAfterParam() string {
	param := strings.TrimSpace(c.UpdatedAfterParam)
	if param == "" {
		return defaultPullUpdatedAfterParam
	}
	return param
}

func (c PullCredential) NormalizedUpdatedBeforeParam() string {
	param := strings.TrimSpace(c.UpdatedBeforeParam)
	if param == "" {
		return defaultPullUpdatedBeforeParam
	}
	return param
}

func (c PullCredential) NormalizedTimestampFormat() string {
	format := strings.TrimSpace(c.TimestampFormat)
	if format == "" {
		return defaultPullTimestampFormat
	}
	return format
}

func (c PullCredential) SupportsIncremental(lastSuccessAt *time.Time) bool {
	return strings.TrimSpace(c.NormalizedUpdatedAfterParam()) != "" && lastSuccessAt != nil
}

func (c PullCredential) EmployeeListURL(
	page int,
	mode string,
	lastSuccessAt *time.Time,
	now time.Time,
) (string, error) {
	baseURL := c.NormalizedBaseURL()
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base_url", ErrInvalidPullCredential)
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/") + c.NormalizedEmployeePath()
	query := parsedURL.Query()
	query.Set("limit", strconv.Itoa(c.NormalizedPageLimit()))
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if hris.NormalizePullMode(mode) == hris.PullModeIncremental && c.SupportsIncremental(lastSuccessAt) {
		query.Set(c.NormalizedUpdatedAfterParam(), c.FormatTimestamp(*lastSuccessAt))
		if nextParam := c.NormalizedUpdatedBeforeParam(); nextParam != "" {
			query.Set(nextParam, c.FormatTimestamp(now))
		}
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func (c PullCredential) FormatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	format := c.NormalizedTimestampFormat()
	switch strings.ToLower(format) {
	case "rfc3339":
		return value.UTC().Format(time.RFC3339)
	case "rfc3339nano":
		return value.UTC().Format(time.RFC3339Nano)
	case "datetime", "date_time", "talenta":
		return value.UTC().Format("2006-01-02 15:04:05")
	case "date":
		return value.UTC().Format("2006-01-02")
	default:
		return value.UTC().Format(format)
	}
}

func SignPullRequest(req *http.Request, credential PullCredential, now time.Time) error {
	if req == nil {
		return ErrPullInvalidResponse
	}
	if err := credential.ValidateForPull(); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	dateHeader := now.Format(http.TimeFormat)
	requestURI := "/"
	if req.URL != nil {
		requestURI = req.URL.RequestURI()
	}
	proto := strings.TrimSpace(req.Proto)
	if proto == "" {
		proto = defaultWebhookHTTPVersion
	}

	signature := pullSignature(credential.ClientSecret, dateHeader, req.Method, requestURI, proto)
	req.Header.Set("Date", dateHeader)
	req.Header.Set("Accept", "application/json")
	req.Header.Set(
		"Authorization",
		fmt.Sprintf(
			`hmac username="%s", algorithm="%s", headers="%s", signature="%s"`,
			strings.TrimSpace(credential.ClientID),
			hmacAuthorizationAlgo,
			hmacSignedHeadersValue,
			signature,
		),
	)
	return nil
}

func pullSignature(clientSecret, dateHeader, method, requestURI, proto string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(clientSecret)))
	mac.Write([]byte(webhookStringToSign(dateHeader, method, requestURI, proto)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildPullRequestID(connector enterprise.HRISConnector, pulledAt time.Time) string {
	nextVendor := hris.NormalizeVendor(connector.Vendor)
	if nextVendor == "" {
		nextVendor = "hris"
	}
	timestamp := pulledAt.UTC().Format("20060102t150405z")
	if strings.TrimSpace(connector.ID) == "" {
		return nextVendor + ":pull:" + timestamp
	}
	return nextVendor + ":" + strings.TrimSpace(connector.ID) + ":pull:" + timestamp
}

func decodePullEmployees(body []byte, limit int) ([]enterprise.EmployeeSyncInput, bool, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrPullInvalidResponse, err)
	}

	records := resolvePullEmployeeRecords(root)
	output := make([]enterprise.EmployeeSyncInput, 0, len(records))
	for i := range records {
		record, ok := records[i].(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("%w: record[%d] is not an object", ErrPullInvalidResponse, i)
		}
		employee, _, _, err := normalizeEmployeeInput(EventEmployeeDetailCreated, record, record)
		if err != nil {
			return nil, false, fmt.Errorf("%w: record[%d]: %v", ErrPullInvalidResponse, i, err)
		}
		output = append(output, employee)
	}
	return output, resolvePullHasMore(root, len(records), limit), nil
}

func resolvePullEmployeeRecords(root map[string]any) []any {
	candidates := [][]any{
		sliceAt(root, "data"),
		sliceAt(root, "employee"),
		sliceAt(root, "employees"),
		sliceAt(mapAt(root, "data"), "data"),
		sliceAt(mapAt(root, "data"), "items"),
		sliceAt(mapAt(root, "data"), "employee"),
		sliceAt(mapAt(root, "data"), "employees"),
		sliceAt(mapAt(root, "response"), "data"),
		sliceAt(mapAt(root, "response"), "items"),
		sliceAt(mapAt(mapAt(root, "response"), "data"), "items"),
		sliceAt(mapAt(root, "result"), "data"),
		sliceAt(mapAt(root, "result"), "items"),
	}
	for i := range candidates {
		if len(candidates[i]) > 0 {
			return candidates[i]
		}
	}
	return nil
}

func resolvePullHasMore(root map[string]any, itemCount int, limit int) bool {
	for _, candidate := range []map[string]any{
		root,
		mapAt(root, "data"),
		mapAt(root, "meta"),
		mapAt(root, "pagination"),
		mapAt(mapAt(root, "data"), "meta"),
		mapAt(mapAt(root, "data"), "pagination"),
		mapAt(root, "response"),
		mapAt(mapAt(root, "response"), "meta"),
		mapAt(mapAt(root, "response"), "pagination"),
	} {
		if nextPageURL := firstNonEmptyString(stringAt(candidate, "next_page_url"), stringAt(candidate, "next")); nextPageURL != "" {
			return true
		}
		currentPage := intAt(candidate, "current_page")
		lastPage := intAt(candidate, "last_page")
		if currentPage > 0 && lastPage > 0 {
			return currentPage < lastPage
		}
		total := intAt(candidate, "total")
		perPage := intAt(candidate, "per_page")
		if total > 0 && perPage > 0 && currentPage > 0 {
			return currentPage*perPage < total
		}
	}
	return limit > 0 && itemCount >= limit
}

func sliceAt(input map[string]any, key string) []any {
	if len(input) == 0 {
		return nil
	}
	value, ok := input[key]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if ok {
		return items
	}
	return nil
}

func intAt(input map[string]any, key string) int {
	if len(input) == 0 {
		return 0
	}
	value, ok := input[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

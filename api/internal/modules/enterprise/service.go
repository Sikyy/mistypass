package enterprise

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrDomainRequired = errors.New("domain is required")
var ErrInvalidDomain = errors.New("invalid domain")
var ErrDomainAlreadyMapped = errors.New("domain already mapped")
var ErrDomainMappingNotFound = errors.New("domain mapping not found")
var ErrInvalidDomainMappingStatus = errors.New("invalid domain mapping status")
var ErrEmailRequired = errors.New("email is required")
var ErrDomainNotMapped = errors.New("domain is not mapped")
var ErrIDPConfigNotFound = errors.New("enterprise idp config not found")
var ErrInvalidIDPProvider = errors.New("invalid idp provider")
var ErrIssuerURLRequired = errors.New("issuer_url is required")
var ErrClientIDRequired = errors.New("client_id is required")
var ErrInvalidIDPStatus = errors.New("invalid idp status")
var ErrSAMLACSURLRequired = errors.New("saml_acs_url is required for saml provider")
var ErrInvalidSAMLACSURL = errors.New("saml_acs_url must use https://")
var ErrSAMLX509CertRequired = errors.New("saml_x509_cert is required for saml provider")
var ErrEmployeesRequired = errors.New("employees is required")
var ErrInvalidReconcileLimit = errors.New("reconcile limit must be >= 1")
var ErrSyncRequestIDRequired = errors.New("request_id is required")
var ErrSyncRequestNotFound = errors.New("sync request not found")
var ErrEmployeeEmailDomainMismatch = errors.New("employee email domain does not match tenant domain mapping")
var ErrEmployeeNotFound = errors.New("enterprise employee not found")
var ErrEmployeeInactive = errors.New("enterprise employee is inactive")
var ErrEmployeeExternalIDConflict = errors.New("enterprise employee external_id conflict")
var ErrJITProvisionApprovalRequired = errors.New("enterprise jit provisioning requires approval")
var ErrJITProvisionApprovalNotFound = errors.New("enterprise jit provision approval not found")
var ErrInvalidJITProvisionApprovalDecision = errors.New("invalid jit provision approval decision")
var ErrInvalidJITProvisionApprovalExternalSyncStatus = errors.New("invalid jit provision approval external sync status")
var ErrAuthStateTokenRequired = errors.New("state_token is required")
var ErrAuthStateTokenNotFound = errors.New("state_token is invalid or expired")
var ErrAuthStateProviderMismatch = errors.New("state_token provider mismatch")
var ErrRedirectURIRequired = errors.New("redirect_uri is required")
var ErrInvalidRedirectURI = errors.New("redirect_uri must use https:// or http://localhost")
var ErrAccessSyncApplierRequired = errors.New("access sync applier is required")

type DomainMapping struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Domain    string    `json:"domain"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TenantResolution struct {
	Email    string `json:"email"`
	Domain   string `json:"domain"`
	TenantID string `json:"tenant_id"`
	Matched  bool   `json:"matched"`
}

type IDPConfig struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Provider     string    `json:"provider"`
	IssuerURL    string    `json:"issuer_url"`
	ClientID     string    `json:"client_id"`
	AuthURL      string    `json:"auth_url,omitempty"`
	TokenURL     string    `json:"token_url,omitempty"`
	JWKSURL      string    `json:"jwks_url,omitempty"`
	UserInfoURL  string    `json:"user_info_url,omitempty"`
	SAMLACSURL   string    `json:"saml_acs_url,omitempty"`
	SAMLX509Cert string    `json:"saml_x509_cert,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	Status       string    `json:"status"`
	SyncMode     string    `json:"sync_mode"`
	UpdatedBy    string    `json:"updated_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type IDPConfigValidationItem struct {
	Field   string `json:"field"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type IDPConfigValidation struct {
	TenantID  string                    `json:"tenant_id"`
	Provider  string                    `json:"provider"`
	Valid     bool                      `json:"valid"`
	Items     []IDPConfigValidationItem `json:"items"`
	CheckedAt time.Time                 `json:"checked_at"`
}

type EmployeeSyncInput struct {
	ExternalID        string `json:"external_id"`
	Email             string `json:"email"`
	FullName          string `json:"full_name"`
	Department        string `json:"department"`
	JobTitle          string `json:"job_title"`
	Location          string `json:"location"`
	Phone             string `json:"phone,omitempty"`
	ManagerExternalID string `json:"manager_external_id,omitempty"`
	EmploymentStatus  string `json:"employment_status,omitempty"`
	Status            string `json:"status"`
}

type EnterpriseEmployee struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	ExternalID        string    `json:"external_id"`
	Email             string    `json:"email"`
	FullName          string    `json:"full_name"`
	Department        string    `json:"department"`
	JobTitle          string    `json:"job_title"`
	Location          string    `json:"location"`
	Phone             string    `json:"phone,omitempty"`
	ManagerExternalID string    `json:"manager_external_id,omitempty"`
	EmploymentStatus  string    `json:"employment_status,omitempty"`
	AccessRole        string    `json:"access_role"`
	BuildingID        string    `json:"building_id"`
	GroupIDs          []string  `json:"group_ids,omitempty"`
	Status            string    `json:"status"`
	Source            string    `json:"source"`
	LastSyncedAt      time.Time `json:"last_synced_at"`
}

type JITProvisionProfile struct {
	FullName          string `json:"full_name"`
	Department        string `json:"department"`
	JobTitle          string `json:"job_title"`
	Location          string `json:"location"`
	Phone             string `json:"phone"`
	ManagerExternalID string `json:"manager_external_id"`
	EmploymentStatus  string `json:"employment_status"`
}

type JITProvisionApproval struct {
	ID                       string     `json:"id"`
	TenantID                 string     `json:"tenant_id"`
	Email                    string     `json:"email"`
	ExternalID               string     `json:"external_id,omitempty"`
	Provider                 string     `json:"provider,omitempty"`
	EmploymentStatus         string     `json:"employment_status,omitempty"`
	Status                   string     `json:"status"`
	Reason                   string     `json:"reason,omitempty"`
	ExternalSyncStatus       string     `json:"external_sync_status,omitempty"`
	ExternalSyncRef          string     `json:"external_sync_ref,omitempty"`
	ExternalSyncAttemptCount int        `json:"external_sync_attempt_count,omitempty"`
	ExternalSyncLastError    string     `json:"external_sync_last_error,omitempty"`
	ExternalSyncUpdatedAt    *time.Time `json:"external_sync_updated_at,omitempty"`
	ReviewedBy               string     `json:"reviewed_by,omitempty"`
	ReviewedAt               *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type SyncJob struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	Total       int       `json:"total"`
	Created     int       `json:"created"`
	Updated     int       `json:"updated"`
	Deactivated int       `json:"deactivated"`
	Rejected    int       `json:"rejected"`
	Actor       string    `json:"actor"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
}

type SyncResult struct {
	Job   SyncJob              `json:"job"`
	Items []EnterpriseEmployee `json:"items"`
}

type AccessSyncApplier func(items []EnterpriseEmployee) (created, updated, rejected int, err error)

type SyncRequestRecord struct {
	RequestID           string     `json:"request_id"`
	TenantID            string     `json:"tenant_id"`
	Result              SyncResult `json:"result"`
	AccessApplied       bool       `json:"access_applied"`
	AccessCreated       int        `json:"access_created"`
	AccessUpdated       int        `json:"access_updated"`
	AccessRejected      int        `json:"access_rejected"`
	AccessAttemptCount  int        `json:"access_attempt_count"`
	LastAccessError     string     `json:"last_access_error,omitempty"`
	LastAccessAttemptAt *time.Time `json:"last_access_attempt_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

type PendingSyncReconcileResult struct {
	RequestID      string     `json:"request_id"`
	JobID          string     `json:"job_id"`
	AccessApplied  bool       `json:"access_applied"`
	AccessCreated  int        `json:"access_created"`
	AccessUpdated  int        `json:"access_updated"`
	AccessRejected int        `json:"access_rejected"`
	AttemptCount   int        `json:"access_attempt_count"`
	LastError      string     `json:"last_access_error,omitempty"`
	AttemptedAt    *time.Time `json:"last_access_attempt_at,omitempty"`
}

type BatchPendingSyncReconcileResult struct {
	Processed             int                          `json:"processed"`
	Applied               int                          `json:"applied"`
	Failed                int                          `json:"failed"`
	SkippedByAttemptLimit int                          `json:"skipped_by_attempt_limit,omitempty"`
	SkippedByCooldown     int                          `json:"skipped_by_cooldown,omitempty"`
	Items                 []PendingSyncReconcileResult `json:"items"`
}

type AuthStateToken struct {
	Token       string    `json:"state_token"`
	TenantID    string    `json:"tenant_id"`
	Provider    string    `json:"provider"`
	Email       string    `json:"email,omitempty"`
	RedirectURI string    `json:"redirect_uri"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_enterprise"

const (
	defaultReconcileLimit    = 20
	maxReconcileLimit        = 200
	defaultAuthStateTokenTTL = 5 * time.Minute
)

type stateSnapshot struct {
	DomainMappings        []DomainMapping              `json:"domain_mappings"`
	IDPConfigs            map[string]IDPConfig         `json:"idp_configs"`
	Employees             []EnterpriseEmployee         `json:"employees"`
	SyncJobs              []SyncJob                    `json:"sync_jobs"`
	SyncRequestRecords    map[string]SyncRequestRecord `json:"sync_request_records,omitempty"`
	JITProvisionApprovals []JITProvisionApproval       `json:"jit_provision_approvals,omitempty"`
}

type Service struct {
	mu                    sync.RWMutex
	domainMappings        []DomainMapping
	idpConfigs            map[string]IDPConfig
	employees             []EnterpriseEmployee
	syncJobs              []SyncJob
	syncRequestRecords    map[string]SyncRequestRecord
	jitProvisionApprovals []JITProvisionApproval
	authStateTokens       map[string]AuthStateToken
	stateStore            StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		domainMappings: []DomainMapping{
			{
				ID:        "dm_001",
				TenantID:  "tenant_demo_jakarta",
				Domain:    "sudirman.co",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "dm_002",
				TenantID:  "tenant_demo_factory",
				Domain:    "factory.local",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		idpConfigs: map[string]IDPConfig{
			"tenant_demo_jakarta": {
				ID:          "idp_001",
				TenantID:    "tenant_demo_jakarta",
				Provider:    "oidc",
				IssuerURL:   "https://id.sudirman.co",
				ClientID:    "mistypass-web-admin",
				AuthURL:     "https://id.sudirman.co/oauth2/auth",
				TokenURL:    "https://id.sudirman.co/oauth2/token",
				JWKSURL:     "https://id.sudirman.co/.well-known/jwks.json",
				UserInfoURL: "https://id.sudirman.co/oauth2/userinfo",
				Scopes:      []string{"openid", "profile", "email"},
				Status:      "active",
				SyncMode:    "jit",
				UpdatedBy:   "system",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		employees: []EnterpriseEmployee{
			{
				ID:               "emp_001",
				TenantID:         "tenant_demo_jakarta",
				ExternalID:       "hris-jkt-1001",
				Email:            "arief.putra@sudirman.co",
				FullName:         "Arief Putra",
				Department:       "Finance",
				JobTitle:         "Finance Manager",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				AccessRole:       "resident",
				BuildingID:       "building_demo_001",
				GroupIDs:         []string{"ug_1001"},
				Status:           "active",
				Source:           "seed",
				LastSyncedAt:     now,
			},
		},
		syncJobs:              []SyncJob{},
		syncRequestRecords:    map[string]SyncRequestRecord{},
		jitProvisionApprovals: []JITProvisionApproval{},
		authStateTokens:       map[string]AuthStateToken{},
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	svc := NewService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) ListDomainMappings(tenantID string) []DomainMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]DomainMapping, 0, len(s.domainMappings))
	for i := range s.domainMappings {
		if filterTenantID != "" && s.domainMappings[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.domainMappings[i])
	}
	return items
}

func (s *Service) CreateDomainMapping(tenantID, domain, status string) (DomainMapping, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return DomainMapping{}, ErrTenantIDRequired
	}

	nextDomain, err := normalizeDomain(domain)
	if err != nil {
		return DomainMapping{}, err
	}

	nextStatus, err := normalizeDomainMappingStatus(status)
	if err != nil {
		return DomainMapping{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.domainMappings {
		if s.domainMappings[i].Domain == nextDomain {
			return DomainMapping{}, ErrDomainAlreadyMapped
		}
	}

	id, err := randomID("dm_")
	if err != nil {
		return DomainMapping{}, err
	}

	now := time.Now().UTC()
	record := DomainMapping{
		ID:        id,
		TenantID:  nextTenantID,
		Domain:    nextDomain,
		Status:    nextStatus,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.domainMappings = append([]DomainMapping{record}, s.domainMappings...)
	if err := s.persistLocked(); err != nil {
		return DomainMapping{}, err
	}

	return record, nil
}

func (s *Service) UpdateDomainMappingStatus(tenantID, mappingID, status string) (DomainMapping, error) {
	nextMappingID := strings.TrimSpace(mappingID)
	if nextMappingID == "" {
		return DomainMapping{}, ErrDomainMappingNotFound
	}

	nextStatus, err := normalizeDomainMappingStatus(status)
	if err != nil {
		return DomainMapping{}, err
	}

	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.domainMappings {
		if s.domainMappings[i].ID != nextMappingID {
			continue
		}
		if filterTenantID != "" && s.domainMappings[i].TenantID != filterTenantID {
			return DomainMapping{}, ErrDomainMappingNotFound
		}
		s.domainMappings[i].Status = nextStatus
		s.domainMappings[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return DomainMapping{}, err
		}
		return s.domainMappings[i], nil
	}

	return DomainMapping{}, ErrDomainMappingNotFound
}

func (s *Service) ResolveTenantByEmail(email string) (TenantResolution, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return TenantResolution{}, ErrEmailRequired
	}

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return TenantResolution{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	bestTenantID := ""
	bestDomain := ""
	for i := range s.domainMappings {
		if s.domainMappings[i].Status != "active" {
			continue
		}
		mappedDomain := s.domainMappings[i].Domain
		if !domainMatches(domain, mappedDomain) {
			continue
		}
		if len(mappedDomain) <= len(bestDomain) {
			continue
		}
		bestDomain = mappedDomain
		bestTenantID = s.domainMappings[i].TenantID
	}

	if bestTenantID != "" {
		return TenantResolution{
			Email:    nextEmail,
			Domain:   domain,
			TenantID: bestTenantID,
			Matched:  true,
		}, nil
	}

	return TenantResolution{}, ErrDomainNotMapped
}

func (s *Service) GetIDPConfig(tenantID string) (IDPConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return IDPConfig{}, ErrTenantIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.idpConfigs[nextTenantID]
	if !exists {
		return IDPConfig{}, ErrIDPConfigNotFound
	}
	config.Scopes = append([]string(nil), config.Scopes...)
	return config, nil
}

func (s *Service) UpsertIDPConfig(
	tenantID, provider, issuerURL, clientID, authURL, tokenURL, jwksURL, userInfoURL, samlACSURL, samlX509Cert, status, syncMode, actor string,
	scopes []string,
) (IDPConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return IDPConfig{}, ErrTenantIDRequired
	}

	nextProvider, err := normalizeProvider(provider)
	if err != nil {
		return IDPConfig{}, err
	}

	nextIssuerURL := strings.TrimSpace(issuerURL)
	if nextIssuerURL == "" {
		return IDPConfig{}, ErrIssuerURLRequired
	}

	nextClientID := strings.TrimSpace(clientID)
	if nextClientID == "" {
		return IDPConfig{}, ErrClientIDRequired
	}

	nextSAMLACSURL := strings.TrimSpace(samlACSURL)
	if nextSAMLACSURL == "" {
		// Keep backward compatibility: if existing clients filled auth_url for SAML ACS endpoint.
		nextSAMLACSURL = strings.TrimSpace(authURL)
	}
	nextSAMLX509Cert := strings.TrimSpace(samlX509Cert)
	if nextProvider == "saml" {
		if nextSAMLACSURL == "" {
			return IDPConfig{}, ErrSAMLACSURLRequired
		}
		if !looksLikeHTTPSURL(nextSAMLACSURL) {
			return IDPConfig{}, ErrInvalidSAMLACSURL
		}
		if nextSAMLX509Cert == "" {
			return IDPConfig{}, ErrSAMLX509CertRequired
		}
	}

	nextStatus, err := normalizeIDPStatus(status)
	if err != nil {
		return IDPConfig{}, err
	}

	nextSyncMode := normalizeSyncMode(syncMode)
	nextActor := strings.TrimSpace(actor)
	if nextActor == "" {
		nextActor = "system"
	}
	nextScopes := normalizeScopes(scopes)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	createdAt := now
	configID := ""
	if current, exists := s.idpConfigs[nextTenantID]; exists {
		createdAt = current.CreatedAt
		configID = current.ID
	}
	if configID == "" {
		configID, err = randomID("idp_")
		if err != nil {
			return IDPConfig{}, err
		}
	}

	record := IDPConfig{
		ID:           configID,
		TenantID:     nextTenantID,
		Provider:     nextProvider,
		IssuerURL:    nextIssuerURL,
		ClientID:     nextClientID,
		AuthURL:      strings.TrimSpace(authURL),
		TokenURL:     strings.TrimSpace(tokenURL),
		JWKSURL:      strings.TrimSpace(jwksURL),
		UserInfoURL:  strings.TrimSpace(userInfoURL),
		SAMLACSURL:   nextSAMLACSURL,
		SAMLX509Cert: nextSAMLX509Cert,
		Scopes:       nextScopes,
		Status:       nextStatus,
		SyncMode:     nextSyncMode,
		UpdatedBy:    nextActor,
		CreatedAt:    createdAt,
		UpdatedAt:    now,
	}

	s.idpConfigs[nextTenantID] = record
	if err := s.persistLocked(); err != nil {
		return IDPConfig{}, err
	}
	return record, nil
}

func (s *Service) ValidateIDPConfig(
	tenantID, provider, issuerURL, clientID, authURL, tokenURL, jwksURL, userInfoURL, samlACSURL, samlX509Cert string,
) IDPConfigValidation {
	nextTenantID := strings.TrimSpace(tenantID)
	nextProvider := strings.ToLower(strings.TrimSpace(provider))

	result := IDPConfigValidation{
		TenantID:  nextTenantID,
		Provider:  nextProvider,
		Valid:     true,
		Items:     make([]IDPConfigValidationItem, 0, 8),
		CheckedAt: time.Now().UTC(),
	}

	push := func(field, status, message string) {
		result.Items = append(result.Items, IDPConfigValidationItem{
			Field:   field,
			Status:  status,
			Message: message,
		})
		if status == "error" {
			result.Valid = false
		}
	}

	if nextTenantID == "" {
		push("tenant_id", "error", ErrTenantIDRequired.Error())
	} else {
		push("tenant_id", "ok", "tenant_id looks good")
	}

	if _, err := normalizeProvider(nextProvider); err != nil {
		push("provider", "error", err.Error())
	} else {
		push("provider", "ok", "provider is supported")
	}

	if strings.TrimSpace(issuerURL) == "" {
		push("issuer_url", "error", ErrIssuerURLRequired.Error())
	} else if !looksLikeHTTPSURL(issuerURL) {
		push("issuer_url", "error", "issuer_url must use https://")
	} else {
		push("issuer_url", "ok", "issuer_url format is valid")
	}

	if strings.TrimSpace(clientID) == "" {
		push("client_id", "error", ErrClientIDRequired.Error())
	} else {
		push("client_id", "ok", "client_id looks good")
	}

	if strings.TrimSpace(authURL) != "" && !looksLikeHTTPSURL(authURL) {
		push("auth_url", "error", "auth_url must use https://")
	} else if strings.TrimSpace(authURL) != "" {
		push("auth_url", "ok", "auth_url format is valid")
	}
	if strings.TrimSpace(tokenURL) != "" && !looksLikeHTTPSURL(tokenURL) {
		push("token_url", "error", "token_url must use https://")
	} else if strings.TrimSpace(tokenURL) != "" {
		push("token_url", "ok", "token_url format is valid")
	}
	if strings.TrimSpace(jwksURL) != "" && !looksLikeHTTPSURL(jwksURL) {
		push("jwks_url", "error", "jwks_url must use https://")
	} else if strings.TrimSpace(jwksURL) != "" {
		push("jwks_url", "ok", "jwks_url format is valid")
	}
	if strings.TrimSpace(userInfoURL) != "" && !looksLikeHTTPSURL(userInfoURL) {
		push("user_info_url", "error", "user_info_url must use https://")
	} else if strings.TrimSpace(userInfoURL) != "" {
		push("user_info_url", "ok", "user_info_url format is valid")
	}

	nextSAMLACSURL := strings.TrimSpace(samlACSURL)
	if nextSAMLACSURL == "" && strings.TrimSpace(authURL) != "" {
		nextSAMLACSURL = strings.TrimSpace(authURL)
		push("saml_acs_url", "warn", "fallback to auth_url for saml_acs_url")
	}

	if nextProvider == "saml" {
		if nextSAMLACSURL == "" {
			push("saml_acs_url", "error", ErrSAMLACSURLRequired.Error())
		} else if !looksLikeHTTPSURL(nextSAMLACSURL) {
			push("saml_acs_url", "error", ErrInvalidSAMLACSURL.Error())
		} else {
			push("saml_acs_url", "ok", "saml_acs_url format is valid")
		}

		if strings.TrimSpace(samlX509Cert) == "" {
			push("saml_x509_cert", "error", ErrSAMLX509CertRequired.Error())
		} else if _, err := normalizeSAMLX509Certificate(strings.TrimSpace(samlX509Cert)); err != nil {
			push("saml_x509_cert", "error", err.Error())
		} else {
			push("saml_x509_cert", "ok", "saml_x509_cert is valid")
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if hasActiveDomainForTenant(s.domainMappings, nextTenantID) {
		push("domain_mapping", "ok", "active domain mapping exists")
	} else {
		push("domain_mapping", "warn", "no active domain mapping for tenant")
	}

	return result
}

func (s *Service) ListEmployees(tenantID string) []EnterpriseEmployee {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]EnterpriseEmployee, 0, len(s.employees))
	for i := range s.employees {
		if filterTenantID != "" && s.employees[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneEmployee(s.employees[i]))
	}
	return items
}

func (s *Service) GetEmployeeByEmail(tenantID, email string) (EnterpriseEmployee, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return EnterpriseEmployee{}, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return EnterpriseEmployee{}, ErrEmailRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	found := false
	candidate := EnterpriseEmployee{}
	for i := range s.employees {
		item := s.employees[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if normalizeEmail(item.Email) != nextEmail {
			continue
		}
		if normalizeEmployeeStatus(item.Status) != "active" {
			continue
		}
		if !found || item.LastSyncedAt.After(candidate.LastSyncedAt) {
			candidate = cloneEmployee(item)
			found = true
		}
	}
	if !found {
		return EnterpriseEmployee{}, ErrEmployeeNotFound
	}

	return candidate, nil
}

func (s *Service) HasActiveJITEmployeeIdentity(tenantID, email, externalID string) (bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return false, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return false, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	if !domainMatchesAny(domain, tenantDomains) {
		return false, ErrEmployeeEmailDomainMismatch
	}

	targetIndex := findJITEmployeeMatchIndexLocked(s.employees, nextTenantID, nextEmail, nextExternalID)
	if targetIndex < 0 {
		return false, nil
	}

	current := s.employees[targetIndex]
	if normalizeEmployeeStatus(current.Status) != "active" || EmploymentStatusBlocksSession(current.EmploymentStatus) {
		return false, ErrEmployeeInactive
	}
	currentExternalID := strings.TrimSpace(current.ExternalID)
	if nextExternalID != "" && currentExternalID != "" && currentExternalID != nextExternalID {
		return false, ErrEmployeeExternalIDConflict
	}
	return true, nil
}

func (s *Service) UpsertJITProvisionApprovalRequest(
	tenantID string,
	email string,
	externalID string,
	provider string,
	employmentStatus string,
) (JITProvisionApproval, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return JITProvisionApproval{}, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return JITProvisionApproval{}, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)
	nextProvider := strings.ToLower(strings.TrimSpace(provider))
	nextEmploymentStatus := NormalizeEmploymentStatus(employmentStatus)

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return JITProvisionApproval{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	if !domainMatchesAny(domain, tenantDomains) {
		return JITProvisionApproval{}, ErrEmployeeEmailDomainMismatch
	}

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if normalizeEmail(item.Email) != nextEmail {
			continue
		}
		if strings.TrimSpace(item.Status) != "pending" {
			continue
		}
		if nextExternalID != "" && strings.TrimSpace(item.ExternalID) != "" && strings.TrimSpace(item.ExternalID) != nextExternalID {
			continue
		}

		item.ExternalID = chooseNonEmpty(nextExternalID, strings.TrimSpace(item.ExternalID))
		item.Provider = chooseNonEmpty(nextProvider, strings.TrimSpace(item.Provider))
		item.EmploymentStatus = chooseNonEmpty(nextEmploymentStatus, strings.TrimSpace(item.EmploymentStatus))
		item.UpdatedAt = time.Now().UTC()
		s.jitProvisionApprovals[i] = item
		if err := s.persistLocked(); err != nil {
			return JITProvisionApproval{}, err
		}
		return cloneJITProvisionApproval(item), nil
	}

	approvalID, err := randomID("jap_")
	if err != nil {
		return JITProvisionApproval{}, err
	}
	now := time.Now().UTC()
	record := JITProvisionApproval{
		ID:               approvalID,
		TenantID:         nextTenantID,
		Email:            nextEmail,
		ExternalID:       nextExternalID,
		Provider:         nextProvider,
		EmploymentStatus: nextEmploymentStatus,
		Status:           "pending",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.jitProvisionApprovals = append([]JITProvisionApproval{record}, s.jitProvisionApprovals...)
	if err := s.persistLocked(); err != nil {
		return JITProvisionApproval{}, err
	}
	return cloneJITProvisionApproval(record), nil
}

func (s *Service) HasApprovedJITProvisionApproval(tenantID, email, externalID string) (bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return false, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return false, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if normalizeEmail(item.Email) != nextEmail {
			continue
		}
		if strings.TrimSpace(item.Status) != "approved" {
			continue
		}
		itemExternalID := strings.TrimSpace(item.ExternalID)
		if nextExternalID != "" && itemExternalID != "" && itemExternalID != nextExternalID {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (s *Service) ListJITProvisionApprovals(tenantID, status string, limit int) []JITProvisionApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	filterStatus := strings.ToLower(strings.TrimSpace(status))
	items := make([]JITProvisionApproval, 0, len(s.jitProvisionApprovals))
	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if nextTenantID != "" && strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if filterStatus != "" && strings.ToLower(strings.TrimSpace(item.Status)) != filterStatus {
			continue
		}
		items = append(items, cloneJITProvisionApproval(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ReviewJITProvisionApproval(
	tenantID string,
	approvalID string,
	decision string,
	reviewedBy string,
	reason string,
) (JITProvisionApproval, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return JITProvisionApproval{}, ErrTenantIDRequired
	}
	nextApprovalID := strings.TrimSpace(approvalID)
	if nextApprovalID == "" {
		return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
	}
	nextDecision, err := normalizeJITProvisionApprovalDecision(decision)
	if err != nil {
		return JITProvisionApproval{}, err
	}
	nextReviewedBy := strings.TrimSpace(reviewedBy)
	if nextReviewedBy == "" {
		nextReviewedBy = "system"
	}
	nextReason := strings.TrimSpace(reason)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.ID) != nextApprovalID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
		}
		now := time.Now().UTC()
		item.Status = nextDecision
		item.ReviewedBy = nextReviewedBy
		item.Reason = chooseNonEmpty(nextReason, item.Reason)
		item.ExternalSyncStatus = "pending"
		item.ExternalSyncLastError = ""
		item.ExternalSyncRef = ""
		item.ExternalSyncUpdatedAt = nil
		item.UpdatedAt = now
		item.ReviewedAt = &now
		s.jitProvisionApprovals[i] = item
		if err := s.persistLocked(); err != nil {
			return JITProvisionApproval{}, err
		}
		return cloneJITProvisionApproval(item), nil
	}
	return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
}

func (s *Service) ListPendingJITProvisionApprovalExternalSync(tenantID string, limit int) []JITProvisionApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	items := make([]JITProvisionApproval, 0, len(s.jitProvisionApprovals))
	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if nextTenantID != "" && strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status != "approved" && status != "rejected" {
			continue
		}
		if strings.TrimSpace(item.ExternalSyncStatus) != "pending" {
			continue
		}
		items = append(items, cloneJITProvisionApproval(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) UpdateJITProvisionApprovalExternalSync(
	tenantID string,
	approvalID string,
	status string,
	externalSyncRef string,
	lastError string,
) (JITProvisionApproval, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return JITProvisionApproval{}, ErrTenantIDRequired
	}
	nextApprovalID := strings.TrimSpace(approvalID)
	if nextApprovalID == "" {
		return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
	}
	nextStatus, err := normalizeJITProvisionApprovalExternalSyncStatus(status)
	if err != nil {
		return JITProvisionApproval{}, err
	}
	nextRef := strings.TrimSpace(externalSyncRef)
	nextLastError := strings.TrimSpace(lastError)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.ID) != nextApprovalID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
		}

		now := time.Now().UTC()
		item.ExternalSyncStatus = nextStatus
		item.ExternalSyncRef = nextRef
		item.ExternalSyncUpdatedAt = &now
		item.UpdatedAt = now
		switch nextStatus {
		case "synced":
			item.ExternalSyncLastError = ""
		case "failed":
			item.ExternalSyncAttemptCount++
			item.ExternalSyncLastError = nextLastError
		}

		s.jitProvisionApprovals[i] = item
		if err := s.persistLocked(); err != nil {
			return JITProvisionApproval{}, err
		}
		return cloneJITProvisionApproval(item), nil
	}
	return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
}

func (s *Service) ResolveOrProvisionJITEmployee(
	tenantID string,
	email string,
	externalID string,
	fullName string,
	department string,
	jobTitle string,
	location string,
) (EnterpriseEmployee, bool, error) {
	return s.ResolveOrProvisionJITEmployeeWithProfile(
		tenantID,
		email,
		externalID,
		JITProvisionProfile{
			FullName:   fullName,
			Department: department,
			JobTitle:   jobTitle,
			Location:   location,
		},
	)
}

func (s *Service) ResolveOrProvisionJITEmployeeWithProfile(
	tenantID string,
	email string,
	externalID string,
	profile JITProvisionProfile,
) (EnterpriseEmployee, bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return EnterpriseEmployee{}, false, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return EnterpriseEmployee{}, false, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)
	nextFullName := strings.TrimSpace(profile.FullName)
	nextDepartment := strings.TrimSpace(profile.Department)
	nextJobTitle := strings.TrimSpace(profile.JobTitle)
	nextLocation := strings.TrimSpace(profile.Location)
	nextPhone := normalizeEmployeePhone(profile.Phone)
	nextManagerExternalID := strings.TrimSpace(profile.ManagerExternalID)
	nextEmploymentStatus := NormalizeEmploymentStatus(profile.EmploymentStatus)
	if nextEmploymentStatus == "" {
		nextEmploymentStatus = "active"
	}
	if EmploymentStatusBlocksSession(nextEmploymentStatus) {
		return EnterpriseEmployee{}, false, ErrEmployeeInactive
	}

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return EnterpriseEmployee{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	if !domainMatchesAny(domain, tenantDomains) {
		return EnterpriseEmployee{}, false, ErrEmployeeEmailDomainMismatch
	}

	targetIndex := findJITEmployeeMatchIndexLocked(s.employees, nextTenantID, nextEmail, nextExternalID)
	if targetIndex >= 0 {
		current := s.employees[targetIndex]
		if normalizeEmployeeStatus(current.Status) != "active" || EmploymentStatusBlocksSession(current.EmploymentStatus) {
			return EnterpriseEmployee{}, false, ErrEmployeeInactive
		}

		currentExternalID := strings.TrimSpace(current.ExternalID)
		if nextExternalID != "" && currentExternalID != "" && currentExternalID != nextExternalID {
			return EnterpriseEmployee{}, false, ErrEmployeeExternalIDConflict
		}
		if nextExternalID != "" {
			current.ExternalID = nextExternalID
		}
		current.Email = nextEmail

		preferDirectorySnapshot := hasDirectorySnapshotPrioritySource(current.Source)
		if nextFullName != "" {
			if !preferDirectorySnapshot || strings.TrimSpace(current.FullName) == "" {
				current.FullName = nextFullName
			}
		} else if strings.TrimSpace(current.FullName) == "" {
			current.FullName = nextEmail
		}

		profileChanged := false
		if nextDepartment != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.Department) == "") {
			if current.Department != nextDepartment {
				current.Department = nextDepartment
				profileChanged = true
			}
		}
		if nextJobTitle != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.JobTitle) == "") {
			if current.JobTitle != nextJobTitle {
				current.JobTitle = nextJobTitle
				profileChanged = true
			}
		}
		if nextLocation != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.Location) == "") {
			if current.Location != nextLocation {
				current.Location = nextLocation
				profileChanged = true
			}
		}
		if nextPhone != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.Phone) == "") {
			current.Phone = nextPhone
		}
		if nextManagerExternalID != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.ManagerExternalID) == "") {
			current.ManagerExternalID = nextManagerExternalID
		}
		if nextEmploymentStatus != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.EmploymentStatus) == "") {
			current.EmploymentStatus = nextEmploymentStatus
		} else if strings.TrimSpace(current.EmploymentStatus) == "" {
			current.EmploymentStatus = "active"
		}
		if strings.TrimSpace(current.AccessRole) == "" || profileChanged {
			role, buildingID, groupIDs := assignAccessTemplate(
				nextTenantID,
				current.Department,
				current.JobTitle,
				current.Location,
			)
			current.AccessRole = role
			current.BuildingID = buildingID
			current.GroupIDs = append([]string(nil), groupIDs...)
		}
		if strings.TrimSpace(current.Source) == "" || strings.TrimSpace(current.Source) == "jit_provision" {
			current.Source = "jit_provision"
		}
		current.Status = normalizeEmployeeStatus(current.EmploymentStatus)
		if normalizeEmployeeStatus(current.Status) != "active" {
			return EnterpriseEmployee{}, false, ErrEmployeeInactive
		}
		current.LastSyncedAt = time.Now().UTC()
		s.employees[targetIndex] = current
		if err := s.persistLocked(); err != nil {
			return EnterpriseEmployee{}, false, err
		}
		return cloneEmployee(current), false, nil
	}

	role, buildingID, groupIDs := assignAccessTemplate(nextTenantID, nextDepartment, nextJobTitle, nextLocation)
	employeeID, err := randomID("emp_")
	if err != nil {
		return EnterpriseEmployee{}, false, err
	}
	nextExternalForCreate := nextExternalID
	if nextExternalForCreate == "" {
		nextExternalForCreate = "jit_email:" + nextEmail
	}
	nextFullNameForCreate := nextFullName
	if nextFullNameForCreate == "" {
		nextFullNameForCreate = nextEmail
	}
	now := time.Now().UTC()
	record := EnterpriseEmployee{
		ID:                employeeID,
		TenantID:          nextTenantID,
		ExternalID:        nextExternalForCreate,
		Email:             nextEmail,
		FullName:          nextFullNameForCreate,
		Department:        nextDepartment,
		JobTitle:          nextJobTitle,
		Location:          nextLocation,
		Phone:             nextPhone,
		ManagerExternalID: nextManagerExternalID,
		EmploymentStatus:  nextEmploymentStatus,
		AccessRole:        role,
		BuildingID:        buildingID,
		GroupIDs:          append([]string(nil), groupIDs...),
		Status:            normalizeEmployeeStatus(nextEmploymentStatus),
		Source:            "jit_provision",
		LastSyncedAt:      now,
	}
	s.employees = append([]EnterpriseEmployee{record}, s.employees...)
	if err := s.persistLocked(); err != nil {
		return EnterpriseEmployee{}, false, err
	}
	return cloneEmployee(record), true, nil
}

func (s *Service) StartAuthStateToken(
	tenantID string,
	provider string,
	email string,
	redirectURI string,
	ttl time.Duration,
) (AuthStateToken, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AuthStateToken{}, ErrTenantIDRequired
	}
	nextProvider, err := normalizeProvider(provider)
	if err != nil {
		return AuthStateToken{}, err
	}
	nextRedirectURI := strings.TrimSpace(redirectURI)
	if nextRedirectURI == "" {
		return AuthStateToken{}, ErrRedirectURIRequired
	}
	if !looksLikeAllowedRedirectURI(nextRedirectURI) {
		return AuthStateToken{}, ErrInvalidRedirectURI
	}
	nextEmail := normalizeEmail(email)
	if ttl <= 0 {
		ttl = defaultAuthStateTokenTTL
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredAuthStateTokensLocked(time.Now().UTC())

	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}

	tokenValue, err := randomID("st_")
	if err != nil {
		return AuthStateToken{}, err
	}
	now := time.Now().UTC()
	record := AuthStateToken{
		Token:       tokenValue,
		TenantID:    nextTenantID,
		Provider:    nextProvider,
		Email:       nextEmail,
		RedirectURI: nextRedirectURI,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	s.authStateTokens[tokenValue] = record
	return record, nil
}

func (s *Service) ConsumeAuthStateToken(token string, expectedProvider string) (AuthStateToken, error) {
	nextToken := strings.TrimSpace(token)
	if nextToken == "" {
		return AuthStateToken{}, ErrAuthStateTokenRequired
	}
	nextExpectedProvider := strings.ToLower(strings.TrimSpace(expectedProvider))
	if nextExpectedProvider != "" {
		if _, err := normalizeProvider(nextExpectedProvider); err != nil {
			return AuthStateToken{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.cleanupExpiredAuthStateTokensLocked(now)

	record, exists := s.authStateTokens[nextToken]
	if !exists {
		return AuthStateToken{}, ErrAuthStateTokenNotFound
	}
	if nextExpectedProvider != "" && record.Provider != nextExpectedProvider {
		delete(s.authStateTokens, nextToken)
		return AuthStateToken{}, ErrAuthStateProviderMismatch
	}
	if !record.ExpiresAt.After(now) {
		delete(s.authStateTokens, nextToken)
		return AuthStateToken{}, ErrAuthStateTokenNotFound
	}

	delete(s.authStateTokens, nextToken)
	return record, nil
}

func (s *Service) SyncEmployees(tenantID, source, actor string, inputs []EmployeeSyncInput) (SyncResult, error) {
	nextTenantID, nextSource, nextActor, _, err := normalizeSyncEmployeesRequest(tenantID, source, actor, "", inputs)
	if err != nil {
		return SyncResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.syncEmployeesLocked(nextTenantID, nextSource, nextActor, inputs)
	if err != nil {
		return SyncResult{}, err
	}
	if err := s.persistLocked(); err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func (s *Service) SyncEmployeesWithAccessUpsert(
	tenantID, source, actor string,
	requestID string,
	inputs []EmployeeSyncInput,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	nextTenantID, nextSource, nextActor, nextRequestID, err := normalizeSyncEmployeesRequest(tenantID, source, actor, requestID, inputs)
	if err != nil {
		return SyncResult{}, 0, 0, 0, err
	}
	if applier == nil {
		return SyncResult{}, 0, 0, 0, ErrAccessSyncApplierRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	recordKey := syncRequestRecordKey(nextTenantID, nextRequestID)
	if recordKey != "" {
		if record, exists := s.syncRequestRecords[recordKey]; exists {
			return s.applyAccessForSyncRecordLocked(recordKey, nextRequestID, record, applier)
		}
	}

	previousEmployees := cloneEmployees(s.employees)
	previousSyncJobs := cloneSyncJobs(s.syncJobs)
	previousSyncRequestRecords := cloneSyncRequestRecords(s.syncRequestRecords)

	result, err := s.syncEmployeesLocked(nextTenantID, nextSource, nextActor, inputs)
	if err != nil {
		return SyncResult{}, 0, 0, 0, err
	}
	if recordKey != "" {
		if s.syncRequestRecords == nil {
			s.syncRequestRecords = make(map[string]SyncRequestRecord)
		}
		s.syncRequestRecords[recordKey] = SyncRequestRecord{
			RequestID:     nextRequestID,
			TenantID:      nextTenantID,
			Result:        cloneSyncResult(result),
			AccessApplied: false,
			CreatedAt:     time.Now().UTC(),
		}
	}
	if err := s.persistLocked(); err != nil {
		return SyncResult{}, 0, 0, 0, err
	}

	if recordKey != "" {
		record := s.syncRequestRecords[recordKey]
		reconciledResult, created, updated, rejected, applyErr := s.applyAccessForSyncRecordLocked(recordKey, nextRequestID, record, applier)
		if applyErr == nil {
			return reconciledResult, created, updated, rejected, nil
		}
		s.employees = previousEmployees
		s.syncJobs = previousSyncJobs
		s.syncRequestRecords = previousSyncRequestRecords
		if rollbackErr := s.persistLocked(); rollbackErr != nil {
			return SyncResult{}, 0, 0, 0, fmt.Errorf(
				"access sync failed: %v; enterprise rollback failed: %w",
				applyErr,
				rollbackErr,
			)
		}
		return SyncResult{}, 0, 0, 0, fmt.Errorf("access sync failed, enterprise sync rolled back: %w", applyErr)
	}

	created, updated, rejected, applyErr := applier(result.Items)
	if applyErr == nil {
		return cloneSyncResult(result), created, updated, rejected, nil
	}

	s.employees = previousEmployees
	s.syncJobs = previousSyncJobs
	s.syncRequestRecords = previousSyncRequestRecords
	if rollbackErr := s.persistLocked(); rollbackErr != nil {
		return SyncResult{}, 0, 0, 0, fmt.Errorf(
			"access sync failed: %v; enterprise rollback failed: %w",
			applyErr,
			rollbackErr,
		)
	}
	return SyncResult{}, 0, 0, 0, fmt.Errorf("access sync failed, enterprise sync rolled back: %w", applyErr)
}

func (s *Service) ReconcileSyncRequestAccess(
	tenantID string,
	requestID string,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SyncResult{}, 0, 0, 0, ErrTenantIDRequired
	}
	nextRequestID := strings.TrimSpace(requestID)
	if nextRequestID == "" {
		return SyncResult{}, 0, 0, 0, ErrSyncRequestIDRequired
	}
	if applier == nil {
		return SyncResult{}, 0, 0, 0, ErrAccessSyncApplierRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	recordKey := syncRequestRecordKey(nextTenantID, nextRequestID)
	record, exists := s.syncRequestRecords[recordKey]
	if !exists {
		return SyncResult{}, 0, 0, 0, ErrSyncRequestNotFound
	}
	return s.applyAccessForSyncRecordLocked(recordKey, nextRequestID, record, applier)
}

func (s *Service) ListSyncRequestRecords(tenantID string, limit int) []SyncRequestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]SyncRequestRecord, 0, len(s.syncRequestRecords))
	for _, record := range s.syncRequestRecords {
		if filterTenantID != "" && strings.TrimSpace(record.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneSyncRequestRecord(record))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].RequestID > items[j].RequestID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) GetSyncRequestRecord(tenantID string, requestID string) (SyncRequestRecord, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SyncRequestRecord{}, ErrTenantIDRequired
	}
	nextRequestID := strings.TrimSpace(requestID)
	if nextRequestID == "" {
		return SyncRequestRecord{}, ErrSyncRequestIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	recordKey := syncRequestRecordKey(nextTenantID, nextRequestID)
	record, exists := s.syncRequestRecords[recordKey]
	if !exists {
		return SyncRequestRecord{}, ErrSyncRequestNotFound
	}
	return cloneSyncRequestRecord(record), nil
}

func (s *Service) ReconcilePendingSyncRequests(
	tenantID string,
	limit int,
	applier AccessSyncApplier,
) (BatchPendingSyncReconcileResult, error) {
	return s.ReconcilePendingSyncRequestsWithPolicy(tenantID, limit, 0, 0, applier)
}

func (s *Service) ReconcilePendingSyncRequestsWithPolicy(
	tenantID string,
	limit int,
	maxAttempts int,
	retryCooldown time.Duration,
	applier AccessSyncApplier,
) (BatchPendingSyncReconcileResult, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return BatchPendingSyncReconcileResult{}, ErrTenantIDRequired
	}
	nextLimit, err := normalizeReconcileLimit(limit)
	if err != nil {
		return BatchPendingSyncReconcileResult{}, err
	}
	if applier == nil {
		return BatchPendingSyncReconcileResult{}, ErrAccessSyncApplierRequired
	}
	if maxAttempts < 0 {
		maxAttempts = 0
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	type pendingRecord struct {
		key    string
		record SyncRequestRecord
	}
	now := time.Now().UTC()
	result := BatchPendingSyncReconcileResult{
		Items: make([]PendingSyncReconcileResult, 0, len(s.syncRequestRecords)),
	}
	pendingRecords := make([]pendingRecord, 0, len(s.syncRequestRecords))
	for key, record := range s.syncRequestRecords {
		if strings.TrimSpace(record.TenantID) != nextTenantID {
			continue
		}
		if record.AccessApplied {
			continue
		}
		if maxAttempts > 0 && record.AccessAttemptCount >= maxAttempts {
			result.SkippedByAttemptLimit++
			continue
		}
		if retryCooldown > 0 && record.LastAccessAttemptAt != nil {
			retryReadyAt := record.LastAccessAttemptAt.Add(retryCooldown)
			if retryReadyAt.After(now) {
				result.SkippedByCooldown++
				continue
			}
		}
		pendingRecords = append(pendingRecords, pendingRecord{
			key:    key,
			record: record,
		})
	}

	sort.Slice(pendingRecords, func(i, j int) bool {
		if pendingRecords[i].record.CreatedAt.Equal(pendingRecords[j].record.CreatedAt) {
			return pendingRecords[i].record.RequestID < pendingRecords[j].record.RequestID
		}
		return pendingRecords[i].record.CreatedAt.Before(pendingRecords[j].record.CreatedAt)
	})
	if len(pendingRecords) > nextLimit {
		pendingRecords = pendingRecords[:nextLimit]
	}
	result.Items = make([]PendingSyncReconcileResult, 0, len(pendingRecords))
	for i := range pendingRecords {
		pending := pendingRecords[i]
		_, _, _, _, applyErr := s.applyAccessForSyncRecordLocked(
			pending.key,
			pending.record.RequestID,
			pending.record,
			applier,
		)

		latestRecord := s.syncRequestRecords[pending.key]
		item := PendingSyncReconcileResult{
			RequestID:      latestRecord.RequestID,
			JobID:          latestRecord.Result.Job.ID,
			AccessApplied:  latestRecord.AccessApplied,
			AccessCreated:  latestRecord.AccessCreated,
			AccessUpdated:  latestRecord.AccessUpdated,
			AccessRejected: latestRecord.AccessRejected,
			AttemptCount:   latestRecord.AccessAttemptCount,
			LastError:      latestRecord.LastAccessError,
		}
		if latestRecord.LastAccessAttemptAt != nil {
			attemptedAt := *latestRecord.LastAccessAttemptAt
			item.AttemptedAt = &attemptedAt
		}
		result.Items = append(result.Items, item)
		result.Processed++

		if applyErr != nil {
			result.Failed++
			continue
		}
		result.Applied++
	}

	return result, nil
}

func (s *Service) applyAccessForSyncRecordLocked(
	recordKey string,
	requestID string,
	record SyncRequestRecord,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	if record.AccessApplied {
		return cloneSyncResult(record.Result), record.AccessCreated, record.AccessUpdated, record.AccessRejected, nil
	}

	attemptAt := time.Now().UTC()
	record.AccessAttemptCount++
	record.LastAccessAttemptAt = &attemptAt

	created, updated, rejected, applyErr := applier(record.Result.Items)
	if applyErr != nil {
		record.LastAccessError = strings.TrimSpace(applyErr.Error())
		s.syncRequestRecords[recordKey] = record
		// Best-effort persistence for compensation audit trail.
		_ = s.persistLocked()
		return SyncResult{}, 0, 0, 0, fmt.Errorf("access sync retry failed for request_id %s: %w", requestID, applyErr)
	}
	record.AccessApplied = true
	record.AccessCreated = created
	record.AccessUpdated = updated
	record.AccessRejected = rejected
	record.LastAccessError = ""
	s.syncRequestRecords[recordKey] = record
	// Idempotency cache persistence should not fail the successful sync path.
	_ = s.persistLocked()
	return cloneSyncResult(record.Result), created, updated, rejected, nil
}

func (s *Service) syncEmployeesLocked(
	nextTenantID, nextSource, nextActor string,
	inputs []EmployeeSyncInput,
) (SyncResult, error) {
	startedAt := time.Now().UTC()
	jobID, err := randomID("syn_")
	if err != nil {
		return SyncResult{}, err
	}

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	existingByExternalID := make(map[string]int)
	existingByEmail := make(map[string]int)
	for i := range s.employees {
		if s.employees[i].TenantID != nextTenantID {
			continue
		}
		externalID := strings.TrimSpace(s.employees[i].ExternalID)
		if externalID != "" {
			existingByExternalID[externalID] = i
		}
		existingByEmail[normalizeEmail(s.employees[i].Email)] = i
	}
	createdRecords := make([]EnterpriseEmployee, 0, len(inputs))
	createdByExternalID := make(map[string]int)
	createdByEmail := make(map[string]int)

	resultItems := make([]EnterpriseEmployee, 0, len(inputs))
	created := 0
	updated := 0
	deactivated := 0
	rejected := 0

	for i := range inputs {
		externalID := strings.TrimSpace(inputs[i].ExternalID)
		email := normalizeEmail(inputs[i].Email)
		if email == "" {
			rejected++
			continue
		}

		domain, err := emailDomain(email)
		if err != nil {
			rejected++
			continue
		}
		if !domainMatchesAny(domain, tenantDomains) {
			rejected++
			continue
		}

		employmentStatus := NormalizeEmploymentStatus(inputs[i].EmploymentStatus)
		if employmentStatus == "" {
			employmentStatus = NormalizeEmploymentStatus(inputs[i].Status)
		}
		if employmentStatus == "" {
			employmentStatus = "active"
		}
		status := normalizeEmployeeStatus(employmentStatus)
		phone := normalizeEmployeePhone(inputs[i].Phone)
		managerExternalID := strings.TrimSpace(inputs[i].ManagerExternalID)
		role, buildingID, groupIDs := assignAccessTemplate(nextTenantID, inputs[i].Department, inputs[i].JobTitle, inputs[i].Location)
		now := time.Now().UTC()

		existingExternalIndex, hasExistingExternalMatch := -1, false
		if externalID != "" {
			existingExternalIndex, hasExistingExternalMatch = existingByExternalID[externalID]
		}
		createdExternalIndex, hasCreatedExternalMatch := -1, false
		if externalID != "" {
			createdExternalIndex, hasCreatedExternalMatch = createdByExternalID[externalID]
		}
		existingEmailIndex, hasExistingEmailMatch := existingByEmail[email]
		createdEmailIndex, hasCreatedEmailMatch := createdByEmail[email]

		hasExternalMatch := hasExistingExternalMatch || hasCreatedExternalMatch
		hasEmailMatch := hasExistingEmailMatch || hasCreatedEmailMatch

		targetIsCreated := false
		targetIndex := -1
		switch {
		case hasExternalMatch && hasEmailMatch:
			switch {
			case hasExistingExternalMatch && hasExistingEmailMatch && existingExternalIndex == existingEmailIndex:
				targetIndex = existingExternalIndex
			case hasCreatedExternalMatch && hasCreatedEmailMatch && createdExternalIndex == createdEmailIndex:
				targetIndex = createdExternalIndex
				targetIsCreated = true
			default:
				rejected++
				continue
			}
		case hasExternalMatch:
			if hasExistingExternalMatch {
				targetIndex = existingExternalIndex
			} else {
				targetIndex = createdExternalIndex
				targetIsCreated = true
			}
		case hasEmailMatch:
			if hasExistingEmailMatch {
				targetIndex = existingEmailIndex
			} else {
				targetIndex = createdEmailIndex
				targetIsCreated = true
			}
		}

		if targetIsCreated {
			currentExternalID := strings.TrimSpace(createdRecords[targetIndex].ExternalID)
			if externalID != "" && currentExternalID != "" && currentExternalID != externalID {
				rejected++
				continue
			}

			previousEmail := normalizeEmail(createdRecords[targetIndex].Email)
			previousExternalID := strings.TrimSpace(createdRecords[targetIndex].ExternalID)
			createdRecords[targetIndex].ExternalID = externalID
			createdRecords[targetIndex].Email = email
			createdRecords[targetIndex].FullName = strings.TrimSpace(inputs[i].FullName)
			createdRecords[targetIndex].Department = strings.TrimSpace(inputs[i].Department)
			createdRecords[targetIndex].JobTitle = strings.TrimSpace(inputs[i].JobTitle)
			createdRecords[targetIndex].Location = strings.TrimSpace(inputs[i].Location)
			createdRecords[targetIndex].Phone = phone
			createdRecords[targetIndex].ManagerExternalID = managerExternalID
			createdRecords[targetIndex].EmploymentStatus = employmentStatus
			createdRecords[targetIndex].AccessRole = role
			createdRecords[targetIndex].BuildingID = buildingID
			createdRecords[targetIndex].GroupIDs = append([]string(nil), groupIDs...)
			createdRecords[targetIndex].Status = status
			createdRecords[targetIndex].Source = nextSource
			createdRecords[targetIndex].LastSyncedAt = now

			if previousEmail != email {
				if previousIndex, exists := createdByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(createdByEmail, previousEmail)
				}
			}
			createdByEmail[email] = targetIndex
			if previousExternalID != "" && previousExternalID != externalID {
				if previousIndex, exists := createdByExternalID[previousExternalID]; exists && previousIndex == targetIndex {
					delete(createdByExternalID, previousExternalID)
				}
			}
			if externalID != "" {
				createdByExternalID[externalID] = targetIndex
			}
			continue
		}

		if targetIndex >= 0 {
			currentExternalID := strings.TrimSpace(s.employees[targetIndex].ExternalID)
			if externalID != "" && currentExternalID != "" && currentExternalID != externalID {
				rejected++
				continue
			}

			previousEmail := normalizeEmail(s.employees[targetIndex].Email)
			previousExternalID := strings.TrimSpace(s.employees[targetIndex].ExternalID)
			s.employees[targetIndex].ExternalID = externalID
			s.employees[targetIndex].Email = email
			s.employees[targetIndex].FullName = strings.TrimSpace(inputs[i].FullName)
			s.employees[targetIndex].Department = strings.TrimSpace(inputs[i].Department)
			s.employees[targetIndex].JobTitle = strings.TrimSpace(inputs[i].JobTitle)
			s.employees[targetIndex].Location = strings.TrimSpace(inputs[i].Location)
			s.employees[targetIndex].Phone = phone
			s.employees[targetIndex].ManagerExternalID = managerExternalID
			s.employees[targetIndex].EmploymentStatus = employmentStatus
			s.employees[targetIndex].AccessRole = role
			s.employees[targetIndex].BuildingID = buildingID
			s.employees[targetIndex].GroupIDs = append([]string(nil), groupIDs...)
			s.employees[targetIndex].Status = status
			s.employees[targetIndex].Source = nextSource
			s.employees[targetIndex].LastSyncedAt = now

			if previousEmail != email {
				if previousIndex, exists := existingByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(existingByEmail, previousEmail)
				}
			}
			existingByEmail[email] = targetIndex
			if previousExternalID != "" && previousExternalID != externalID {
				if previousIndex, exists := existingByExternalID[previousExternalID]; exists && previousIndex == targetIndex {
					delete(existingByExternalID, previousExternalID)
				}
			}
			if externalID != "" {
				existingByExternalID[externalID] = targetIndex
			}

			updated++
			if status != "active" {
				deactivated++
			}
			resultItems = append(resultItems, cloneEmployee(s.employees[targetIndex]))
			continue
		}

		employeeID, err := randomID("emp_")
		if err != nil {
			return SyncResult{}, err
		}

		record := EnterpriseEmployee{
			ID:                employeeID,
			TenantID:          nextTenantID,
			ExternalID:        externalID,
			Email:             email,
			FullName:          strings.TrimSpace(inputs[i].FullName),
			Department:        strings.TrimSpace(inputs[i].Department),
			JobTitle:          strings.TrimSpace(inputs[i].JobTitle),
			Location:          strings.TrimSpace(inputs[i].Location),
			Phone:             phone,
			ManagerExternalID: managerExternalID,
			EmploymentStatus:  employmentStatus,
			AccessRole:        role,
			BuildingID:        buildingID,
			GroupIDs:          append([]string(nil), groupIDs...),
			Status:            status,
			Source:            nextSource,
			LastSyncedAt:      now,
		}
		createdIndex := len(createdRecords)
		createdByEmail[email] = createdIndex
		if externalID != "" {
			createdByExternalID[externalID] = createdIndex
		}
		createdRecords = append(createdRecords, record)
		created++
		resultItems = append(resultItems, cloneEmployee(record))
	}
	if len(createdRecords) > 0 {
		s.employees = append(createdRecords, s.employees...)
	}

	job := SyncJob{
		ID:          jobID,
		TenantID:    nextTenantID,
		Source:      nextSource,
		Status:      "completed",
		Total:       len(inputs),
		Created:     created,
		Updated:     updated,
		Deactivated: deactivated,
		Rejected:    rejected,
		Actor:       nextActor,
		StartedAt:   startedAt,
		EndedAt:     time.Now().UTC(),
	}
	s.syncJobs = append([]SyncJob{job}, s.syncJobs...)

	return SyncResult{
		Job:   job,
		Items: resultItems,
	}, nil
}

func normalizeSyncEmployeesRequest(
	tenantID, source, actor, requestID string,
	inputs []EmployeeSyncInput,
) (nextTenantID, nextSource, nextActor, nextRequestID string, err error) {
	nextTenantID = strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return "", "", "", "", ErrTenantIDRequired
	}
	if len(inputs) == 0 {
		return "", "", "", "", ErrEmployeesRequired
	}

	nextSource = strings.TrimSpace(source)
	if nextSource == "" {
		nextSource = "manual_sync"
	}

	nextActor = strings.TrimSpace(actor)
	if nextActor == "" {
		nextActor = "system"
	}

	nextRequestID = strings.TrimSpace(requestID)
	return nextTenantID, nextSource, nextActor, nextRequestID, nil
}

func normalizeReconcileLimit(input int) (int, error) {
	if input < 0 {
		return 0, ErrInvalidReconcileLimit
	}
	if input == 0 {
		return defaultReconcileLimit, nil
	}
	if input > maxReconcileLimit {
		return maxReconcileLimit, nil
	}
	return input, nil
}

func (s *Service) ListSyncJobs(tenantID string) []SyncJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]SyncJob, 0, len(s.syncJobs))
	for i := range s.syncJobs {
		if filterTenantID != "" && s.syncJobs[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.syncJobs[i])
	}
	return items
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		return s.stateStore.Save(stateKey, stateSnapshot{
			DomainMappings:        cloneDomainMappings(s.domainMappings),
			IDPConfigs:            cloneIDPConfigs(s.idpConfigs),
			Employees:             cloneEmployees(s.employees),
			SyncJobs:              cloneSyncJobs(s.syncJobs),
			SyncRequestRecords:    cloneSyncRequestRecords(s.syncRequestRecords),
			JITProvisionApprovals: cloneJITProvisionApprovals(s.jitProvisionApprovals),
		})
	}

	s.mu.Lock()
	s.domainMappings = cloneDomainMappings(snapshot.DomainMappings)
	s.idpConfigs = cloneIDPConfigs(snapshot.IDPConfigs)
	s.employees = cloneEmployees(snapshot.Employees)
	s.syncJobs = cloneSyncJobs(snapshot.SyncJobs)
	s.syncRequestRecords = cloneSyncRequestRecords(snapshot.SyncRequestRecords)
	s.jitProvisionApprovals = cloneJITProvisionApprovals(snapshot.JITProvisionApprovals)
	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}
	s.mu.Unlock()

	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		DomainMappings:        cloneDomainMappings(s.domainMappings),
		IDPConfigs:            cloneIDPConfigs(s.idpConfigs),
		Employees:             cloneEmployees(s.employees),
		SyncJobs:              cloneSyncJobs(s.syncJobs),
		SyncRequestRecords:    cloneSyncRequestRecords(s.syncRequestRecords),
		JITProvisionApprovals: cloneJITProvisionApprovals(s.jitProvisionApprovals),
	})
}

func (s *Service) cleanupExpiredAuthStateTokensLocked(now time.Time) {
	if len(s.authStateTokens) == 0 {
		return
	}
	for token, item := range s.authStateTokens {
		if !item.ExpiresAt.After(now) {
			delete(s.authStateTokens, token)
		}
	}
}

func hasActiveDomainForTenant(items []DomainMapping, tenantID string) bool {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return false
	}
	for i := range items {
		if items[i].TenantID == nextTenantID && items[i].Status == "active" {
			return true
		}
	}
	return false
}

func activeDomainsForTenant(items []DomainMapping, tenantID string) map[string]struct{} {
	nextTenantID := strings.TrimSpace(tenantID)
	output := map[string]struct{}{}
	if nextTenantID == "" {
		return output
	}
	for i := range items {
		if items[i].TenantID != nextTenantID || items[i].Status != "active" {
			continue
		}
		output[items[i].Domain] = struct{}{}
	}
	return output
}

func domainMatchesAny(domain string, patterns map[string]struct{}) bool {
	for pattern := range patterns {
		if domainMatches(domain, pattern) {
			return true
		}
	}
	return false
}

func assignAccessTemplate(tenantID, department, jobTitle, location string) (string, string, []string) {
	dept := strings.ToLower(strings.TrimSpace(department))
	title := strings.ToLower(strings.TrimSpace(jobTitle))
	loc := strings.ToLower(strings.TrimSpace(location))
	tags := strings.Join([]string{dept, title}, " ")

	role := "resident"
	switch {
	case containsAny(tags, "security", "satpam", "guard", "hse", "safety"):
		role = "operator"
	case containsAny(tags, "facility", "engineering", "building", "maintenance", "teknisi"):
		role = "building_admin"
	case containsAny(tags, "it", "identity", "admin", "hris", "iam"):
		role = "tenant_admin"
	}

	buildingID := "building_demo_001"
	groupIDs := templateGroupIDs(strings.TrimSpace(tenantID), role)
	if strings.TrimSpace(tenantID) == "tenant_demo_factory" {
		buildingID = "building_demo_003"
	}

	if strings.Contains(loc, "factory") || strings.Contains(loc, "pabrik") || strings.Contains(loc, "bandung") || strings.Contains(loc, "bekasi") {
		buildingID = "building_demo_003"
	}
	if strings.Contains(loc, "jakarta") || strings.Contains(loc, "jkt") || strings.Contains(loc, "sudirman") {
		buildingID = "building_demo_001"
	}

	return role, buildingID, groupIDs
}

func templateGroupIDs(tenantID, role string) []string {
	nextTenantID := strings.TrimSpace(tenantID)
	nextRole := strings.TrimSpace(role)
	switch nextTenantID {
	case "tenant_demo_factory":
		switch nextRole {
		case "operator":
			return []string{"ug_security_fct"}
		case "building_admin":
			return []string{"ug_building_ops_fct"}
		case "tenant_admin":
			return []string{"ug_tenant_admin_fct"}
		default:
			return []string{"ug_common_office_fct"}
		}
	default:
		switch nextRole {
		case "operator":
			return []string{"ug_security_jkt"}
		case "building_admin":
			return []string{"ug_building_ops_jkt"}
		case "tenant_admin":
			return []string{"ug_tenant_admin_jkt"}
		default:
			return []string{"ug_common_office_jkt"}
		}
	}
}

func containsAny(input string, terms ...string) bool {
	for i := range terms {
		if strings.Contains(input, strings.ToLower(strings.TrimSpace(terms[i]))) {
			return true
		}
	}
	return false
}

func findJITEmployeeMatchIndexLocked(items []EnterpriseEmployee, tenantID, email, externalID string) int {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := normalizeEmail(email)
	nextExternalID := strings.TrimSpace(externalID)

	externalMatchIndex := -1
	emailMatchIndex := -1
	for i := range items {
		item := items[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		itemExternalID := strings.TrimSpace(item.ExternalID)
		itemEmail := normalizeEmail(item.Email)
		if nextExternalID != "" && itemExternalID == nextExternalID {
			externalMatchIndex = i
			break
		}
		if emailMatchIndex < 0 && itemEmail == nextEmail {
			emailMatchIndex = i
		}
	}
	if externalMatchIndex >= 0 {
		return externalMatchIndex
	}
	return emailMatchIndex
}

func hasDirectorySnapshotPrioritySource(source string) bool {
	next := strings.ToLower(strings.TrimSpace(source))
	return strings.Contains(next, "scim") || strings.Contains(next, "hris")
}

func normalizeDomain(input string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(input))
	domain = strings.TrimPrefix(domain, "@")
	if domain == "" {
		return "", ErrDomainRequired
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") || strings.Contains(domain, " ") {
		return "", ErrInvalidDomain
	}
	if strings.Count(domain, ".") < 1 {
		return "", ErrInvalidDomain
	}
	return domain, nil
}

func domainMatches(actualDomain, mappedDomain string) bool {
	nextActual := strings.ToLower(strings.TrimSpace(actualDomain))
	nextMapped := strings.ToLower(strings.TrimSpace(mappedDomain))
	if nextActual == "" || nextMapped == "" {
		return false
	}
	if nextActual == nextMapped {
		return true
	}
	return strings.HasSuffix(nextActual, "."+nextMapped)
}

func normalizeDomainMappingStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidDomainMappingStatus
	}
}

func normalizeProvider(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "oidc":
		return "oidc", nil
	case "saml":
		return "saml", nil
	default:
		return "", ErrInvalidIDPProvider
	}
}

func normalizeIDPStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidIDPStatus
	}
}

func NormalizeEmploymentStatus(input string) string {
	next := strings.ToLower(strings.TrimSpace(input))
	switch next {
	case "true", "1", "yes", "y":
		return "active"
	case "false", "0", "no", "n":
		return "inactive"
	case "inactive", "terminated", "disabled", "suspended", "deprovisioned":
		return next
	case "active":
		return "active"
	default:
		return next
	}
}

func EmploymentStatusBlocksSession(status string) bool {
	switch NormalizeEmploymentStatus(status) {
	case "inactive", "terminated", "disabled", "suspended", "deprovisioned":
		return true
	default:
		return false
	}
}

func normalizeSyncMode(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "scheduled":
		return "scheduled"
	case "manual":
		return "manual"
	default:
		return "jit"
	}
}

func normalizeJITProvisionApprovalDecision(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "approved":
		return "approved", nil
	case "rejected":
		return "rejected", nil
	default:
		return "", ErrInvalidJITProvisionApprovalDecision
	}
}

func normalizeJITProvisionApprovalExternalSyncStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "synced":
		return "synced", nil
	case "failed":
		return "failed", nil
	default:
		return "", ErrInvalidJITProvisionApprovalExternalSyncStatus
	}
}

func normalizeScopes(input []string) []string {
	items := make([]string, 0, len(input))
	set := make(map[string]struct{}, len(input))
	for i := range input {
		next := strings.ToLower(strings.TrimSpace(input[i]))
		if next == "" {
			continue
		}
		if _, exists := set[next]; exists {
			continue
		}
		set[next] = struct{}{}
		items = append(items, next)
	}
	return items
}

func normalizeEmployeeStatus(input string) string {
	if EmploymentStatusBlocksSession(input) {
		return "inactive"
	}
	return "active"
}

func normalizeEmployeePhone(input string) string {
	next := strings.TrimSpace(input)
	if next == "" {
		return ""
	}

	builder := strings.Builder{}
	for i := 0; i < len(next); i++ {
		ch := next[i]
		if ch >= '0' && ch <= '9' {
			builder.WriteByte(ch)
			continue
		}
		if ch == '+' && builder.Len() == 0 {
			builder.WriteByte(ch)
		}
	}
	normalized := builder.String()
	if normalized == "" || normalized == "+" {
		return next
	}
	if strings.HasPrefix(normalized, "00") {
		return "+" + strings.TrimPrefix(normalized, "00")
	}
	if strings.HasPrefix(normalized, "+") {
		return normalized
	}
	if strings.HasPrefix(normalized, "62") && len(normalized) >= 10 {
		return "+" + normalized
	}
	return next
}

func chooseNonEmpty(primary, fallback string) string {
	nextPrimary := strings.TrimSpace(primary)
	if nextPrimary != "" {
		return nextPrimary
	}
	return strings.TrimSpace(fallback)
}

func normalizeEmail(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func syncRequestRecordKey(tenantID, requestID string) string {
	nextTenantID := strings.TrimSpace(tenantID)
	nextRequestID := strings.TrimSpace(requestID)
	if nextTenantID == "" || nextRequestID == "" {
		return ""
	}
	return nextTenantID + ":" + nextRequestID
}

func emailDomain(email string) (string, error) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", ErrInvalidDomain
	}
	return normalizeDomain(parts[1])
}

func looksLikeHTTPSURL(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "https://")
}

func looksLikeAllowedRedirectURI(input string) bool {
	next := strings.ToLower(strings.TrimSpace(input))
	if strings.HasPrefix(next, "https://") {
		return true
	}
	return strings.HasPrefix(next, "http://localhost")
}

func randomID(prefix string) (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func cloneEmployee(input EnterpriseEmployee) EnterpriseEmployee {
	output := input
	output.GroupIDs = append([]string(nil), input.GroupIDs...)
	return output
}

func cloneDomainMappings(items []DomainMapping) []DomainMapping {
	output := make([]DomainMapping, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneIDPConfigs(items map[string]IDPConfig) map[string]IDPConfig {
	output := make(map[string]IDPConfig, len(items))
	for tenantID, record := range items {
		record.Scopes = append([]string(nil), record.Scopes...)
		output[tenantID] = record
	}
	return output
}

func cloneEmployees(items []EnterpriseEmployee) []EnterpriseEmployee {
	output := make([]EnterpriseEmployee, 0, len(items))
	for i := range items {
		output = append(output, cloneEmployee(items[i]))
	}
	return output
}

func cloneSyncJobs(items []SyncJob) []SyncJob {
	output := make([]SyncJob, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneSyncResult(input SyncResult) SyncResult {
	output := SyncResult{
		Job:   input.Job,
		Items: make([]EnterpriseEmployee, 0, len(input.Items)),
	}
	for i := range input.Items {
		output.Items = append(output.Items, cloneEmployee(input.Items[i]))
	}
	return output
}

func cloneSyncRequestRecord(input SyncRequestRecord) SyncRequestRecord {
	output := input
	output.Result = cloneSyncResult(input.Result)
	if input.LastAccessAttemptAt != nil {
		attemptAt := *input.LastAccessAttemptAt
		output.LastAccessAttemptAt = &attemptAt
	}
	return output
}

func cloneSyncRequestRecords(items map[string]SyncRequestRecord) map[string]SyncRequestRecord {
	if len(items) == 0 {
		return map[string]SyncRequestRecord{}
	}
	output := make(map[string]SyncRequestRecord, len(items))
	for key, record := range items {
		output[key] = cloneSyncRequestRecord(record)
	}
	return output
}

func cloneJITProvisionApproval(input JITProvisionApproval) JITProvisionApproval {
	output := input
	if input.ReviewedAt != nil {
		reviewedAt := *input.ReviewedAt
		output.ReviewedAt = &reviewedAt
	}
	return output
}

func cloneJITProvisionApprovals(items []JITProvisionApproval) []JITProvisionApproval {
	output := make([]JITProvisionApproval, 0, len(items))
	for i := range items {
		output = append(output, cloneJITProvisionApproval(items[i]))
	}
	return output
}

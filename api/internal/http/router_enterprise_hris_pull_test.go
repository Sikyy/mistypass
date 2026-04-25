package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
)

type failNTimesPullAdapter struct {
	failUntil int
	calls     int
}

func (a *failNTimesPullAdapter) Vendor() string {
	return "talenta"
}

func (a *failNTimesPullAdapter) Pull(ctx context.Context, input hris.PullInput) (hris.PullResult, error) {
	_ = ctx
	a.calls++
	if a.calls <= a.failUntil {
		return hris.PullResult{}, errors.New("forced pull adapter failure")
	}
	return hris.NormalizePullResult(hris.PullResult{
		TenantID:    input.Connector.TenantID,
		Source:      hris.SyncSourceForVendor(input.Connector.Vendor),
		Actor:       hris.SyncActor,
		RequestID:   "pull-fail-n-times-001",
		Mode:        input.Mode,
		ConnectorID: input.Connector.ID,
		Employees: []enterprise.EmployeeSyncInput{
			{
				ExternalID:       "EMP-PULL-BACKOFF-001",
				Email:            "pull.backoff@pull.local",
				FullName:         "Pull Backoff",
				Department:       "IT",
				JobTitle:         "Engineer",
				Location:         "Jakarta HQ",
				EmploymentStatus: "active",
				Status:           "active",
			},
		},
		PulledAt: input.Now,
	}), nil
}

func TestRunEnterpriseHRISPullWorkerTick(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v2/talenta/v2/employee" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("unexpected page query: %s", got)
		}
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-001",
						"employee_number":   "E-001",
						"organization_name": "IT",
						"job_position":      "Engineer",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
						"join_date":         "2026-04-20",
					},
					"personal": map[string]any{
						"first_name":   "Pull",
						"last_name":    "Worker",
						"email":        "pull.worker@pull.local",
						"mobile_phone": "+628133333333",
						"avatar":       "https://cdn.example.com/photos/pull-worker.jpg",
					},
					"leave_info": map[string]any{
						"status": "approved",
						"type":   "Sick Leave",
					},
					"payroll_info": map[string]any{
						"cost_center_name": "CC-PULL-01",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"`+apiServer.URL+`"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg:                    config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hrisVaultSvc,
		hrisPullStateSvc:       hris.NewPullStateService(),
		hrisPullRegistry:       hris.NewPullRegistry(talenta.NewPullAdapter()),
		hrisHTTPClient:         apiServer.Client(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "pull.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	_, err = s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       "EMP-LEGACY",
				EmployeeNumber:   "E-LEGACY",
				Email:            "legacy.user@pull.local",
				FullName:         "Legacy User",
				Department:       "Ops",
				JobTitle:         "Operator",
				Location:         "Jakarta HQ",
				EmploymentStatus: "active",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected legacy seed sync success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 1)

	connectors := s.enterpriseSvc.ListHRISConnectors("tenant_demo_jakarta")
	if len(connectors) != 1 || connectors[0].LastSyncAt == nil {
		t.Fatalf("expected connector last_sync_at to be updated: %+v", connectors)
	}

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success: %v", err)
	}
	if state.Status != "succeeded" {
		t.Fatalf("expected pull state succeeded, got %s", state.Status)
	}
	if state.LastRequestID == "" {
		t.Fatalf("expected pull state request id to be set")
	}
	if state.LastMode != hris.PullModeFull {
		t.Fatalf("expected full pull mode to be tracked, got %s", state.LastMode)
	}
	if state.LastFullSuccessAt == nil {
		t.Fatalf("expected last_full_success_at to be set after full snapshot")
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundPulled := false
	foundPulledLeaveStatus := false
	foundPulledExtendedFields := false
	foundLegacyInactive := false
	for i := range employees {
		switch employees[i].ExternalID {
		case "EMP-001":
			foundPulled = employees[i].Status == "active"
			foundPulledLeaveStatus = employees[i].LeaveStatus == "sick_leave"
			foundPulledExtendedFields = employees[i].JoinDate == "2026-04-20" &&
				employees[i].CostCenter == "CC-PULL-01" &&
				employees[i].PhotoURL == "https://cdn.example.com/photos/pull-worker.jpg"
		case "EMP-LEGACY":
			foundLegacyInactive = employees[i].Status == "inactive" && strings.TrimSpace(employees[i].EmploymentStatus) == "inactive"
		}
	}
	if !foundPulled {
		t.Fatalf("expected pulled employee to be active")
	}
	if !foundPulledLeaveStatus {
		t.Fatalf("expected pulled employee leave_status to be normalized from pull payload")
	}
	if !foundPulledExtendedFields {
		t.Fatalf("expected pulled employee extended fields to be normalized from pull payload")
	}
	if !foundLegacyInactive {
		t.Fatalf("expected legacy employee to be deactivated by reconcile")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "pull.worker@pull.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected pulled employee access user to be upserted")
	}

	requests := s.enterpriseSvc.ListSyncRequestRecords("tenant_demo_jakarta", 10)
	if len(requests) == 0 {
		t.Fatalf("expected sync request record to be created")
	}
	if requests[0].ConnectorID != connector.ID {
		t.Fatalf("unexpected connector_id on sync request: %s", requests[0].ConnectorID)
	}
	if !strings.HasPrefix(requests[0].RawPayloadRef, "hris_pull_run:") {
		t.Fatalf("unexpected raw_payload_ref on pull sync request: %s", requests[0].RawPayloadRef)
	}
}

func TestRunEnterpriseHRISPullWorkerTickIncrementalUsesTalentaDefaultContractAndDoesNotDeactivateMissingEmployees(t *testing.T) {
	requestCount := 0
	var lastUpdatedAfter string
	var lastUpdatedBefore string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		lastUpdatedAfter = r.URL.Query().Get("updated_after")
		lastUpdatedBefore = r.URL.Query().Get("updated_before")
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-001",
						"employee_number":   "E-001",
						"organization_name": "IT",
						"job_position":      "Senior Engineer",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Pull",
						"last_name":  "Worker",
						"email":      "pull.worker@pull.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"`+apiServer.URL+`"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg:                    config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hrisVaultSvc,
		hrisPullStateSvc:       hris.NewPullStateService(),
		hrisPullRegistry:       hris.NewPullRegistry(talenta.NewPullAdapter()),
		hrisHTTPClient:         apiServer.Client(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "pull.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	_, err = s.enterpriseSvc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       "EMP-001",
				EmployeeNumber:   "E-001",
				Email:            "pull.worker@pull.local",
				FullName:         "Pull Worker",
				Department:       "IT",
				JobTitle:         "Engineer",
				Location:         "Jakarta HQ",
				EmploymentStatus: "active",
				Status:           "active",
			},
			{
				ExternalID:       "EMP-LEGACY",
				EmployeeNumber:   "E-LEGACY",
				Email:            "legacy.user@pull.local",
				FullName:         "Legacy User",
				Department:       "Ops",
				JobTitle:         "Operator",
				Location:         "Jakarta HQ",
				EmploymentStatus: "active",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected seed sync success: %v", err)
	}

	lastFullSuccessAt := time.Now().UTC().Add(-1 * time.Hour)
	if _, err := s.hrisPullStateSvc.MarkSucceeded(
		connector.TenantID,
		connector.ID,
		connector.Vendor,
		hris.PullModeFull,
		"talenta:full:seed",
		lastFullSuccessAt,
	); err != nil {
		t.Fatalf("expected seed pull state success: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISConnectorSynced(connector.TenantID, connector.ID, lastFullSuccessAt); err != nil {
		t.Fatalf("expected connector synced mark success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 24*time.Hour, 1)

	if requestCount != 1 {
		t.Fatalf("expected one incremental request, got %d", requestCount)
	}
	if lastUpdatedAfter == "" {
		t.Fatalf("expected incremental updated_after query to be set")
	}
	if lastUpdatedAfter != lastFullSuccessAt.UTC().Format(time.RFC3339) {
		t.Fatalf("expected default updated_after to use rfc3339 last_full_success_at, got %s", lastUpdatedAfter)
	}
	if lastUpdatedBefore == "" {
		t.Fatalf("expected incremental updated_before query to be set")
	}
	if _, err := time.Parse(time.RFC3339, lastUpdatedBefore); err != nil {
		t.Fatalf("expected default updated_before to use rfc3339, got %s err=%v", lastUpdatedBefore, err)
	}

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success: %v", err)
	}
	if state.LastMode != hris.PullModeIncremental {
		t.Fatalf("expected incremental mode to be tracked, got %s", state.LastMode)
	}
	if state.LastFullSuccessAt == nil || !state.LastFullSuccessAt.Equal(lastFullSuccessAt) {
		t.Fatalf("expected last_full_success_at to stay on last full snapshot, got %+v", state.LastFullSuccessAt)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundUpdated := false
	foundLegacyStillActive := false
	for i := range employees {
		switch employees[i].ExternalID {
		case "EMP-001":
			foundUpdated = employees[i].Status == "active" && employees[i].JobTitle == "Senior Engineer"
		case "EMP-LEGACY":
			foundLegacyStillActive = employees[i].Status == "active"
		}
	}
	if !foundUpdated {
		t.Fatalf("expected incremental employee update to be applied")
	}
	if !foundLegacyStillActive {
		t.Fatalf("expected missing employee to remain active during incremental pull")
	}
}

func TestRunEnterpriseHRISPullWorkerTickRecoversStaleRunningState(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-STALE-001",
						"employee_number":   "E-STALE-001",
						"organization_name": "IT",
						"job_position":      "Engineer",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Stale",
						"last_name":  "Running",
						"email":      "stale.running@pull.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"`+apiServer.URL+`"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg:              config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:    enterprise.NewService(),
		accessSvc:        access.NewService(),
		auditSvc:         audit.NewService(),
		hrisVaultSvc:     hrisVaultSvc,
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(talenta.NewPullAdapter()),
		hrisHTTPClient:   apiServer.Client(),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "pull.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	if _, err := s.hrisPullStateSvc.MarkStarted(
		connector.TenantID,
		connector.ID,
		connector.Vendor,
		hris.PullModeFull,
		time.Now().UTC().Add(-45*time.Minute),
	); err != nil {
		t.Fatalf("expected stale running seed state success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTickWithProcessingTimeout(10, 5, 0, 0, 30*time.Minute, 1)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after stale recovery: %v", err)
	}
	if state.Status != "succeeded" {
		t.Fatalf("expected stale running state to be recovered to succeeded, got %s", state.Status)
	}
	if state.LastRequestID == "" {
		t.Fatalf("expected stale recovery to produce request id")
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].Email == "stale.running@pull.local" && employees[i].Status == "active" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected stale running recovery to sync employee")
	}
}

func TestRunEnterpriseHRISPullWorkerTickSkipsFreshRunningState(t *testing.T) {
	s := &server{
		cfg:              config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:    enterprise.NewService(),
		auditSvc:         audit.NewService(),
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(talenta.NewPullAdapter()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	if _, err := s.hrisPullStateSvc.MarkStarted(
		connector.TenantID,
		connector.ID,
		connector.Vendor,
		hris.PullModeFull,
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("expected fresh running seed state success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTickWithProcessingTimeout(10, 5, 0, 0, 30*time.Minute, 1)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after fresh running skip: %v", err)
	}
	if state.Status != "running" {
		t.Fatalf("expected fresh running state to stay running, got %s", state.Status)
	}
	if state.LastError != "" {
		t.Fatalf("expected fresh running skip to avoid new failure state, got %s", state.LastError)
	}

	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 0 {
		t.Fatalf("expected no pull worker alert audit when only skipped_in_flight, got %d", len(alertLogs))
	}
}

func TestRunEnterpriseHRISPullWorkerTickWithLeaseRunsWhenAcquired(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-LEASE-001",
						"employee_number":   "E-LEASE-001",
						"organization_name": "IT",
						"job_position":      "Engineer",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Lease",
						"last_name":  "Pull",
						"email":      "lease.pull@pull.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	leaseStore := &stubWorkerLeaseStore{acquireOK: true}
	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"`+apiServer.URL+`"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg: config.Config{
			ExternalAuthTimeout:             3 * time.Second,
			EnterpriseHRISPullWorkerEnabled: true,
		},
		enterpriseSvc:    enterprise.NewService(),
		accessSvc:        access.NewService(),
		auditSvc:         audit.NewService(),
		hrisVaultSvc:     hrisVaultSvc,
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(talenta.NewPullAdapter()),
		hrisHTTPClient:   apiServer.Client(),
		workerLeaseStore: leaseStore,
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "pull.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTickWithLease(10, 5, 0, 0, 1, 10*time.Minute)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success: %v", err)
	}
	if state.Status != "succeeded" {
		t.Fatalf("expected leased pull worker to succeed, got %s", state.Status)
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 1 {
		t.Fatalf("expected one lease acquire/release, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseHRISPullLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	if leaseStore.lastTTL != 10*time.Minute {
		t.Fatalf("unexpected lease ttl: %s", leaseStore.lastTTL)
	}
}

func TestRunEnterpriseHRISPullWorkerTickWithLeaseSkipsWhenUnavailable(t *testing.T) {
	leaseStore := &stubWorkerLeaseStore{acquireOK: false}
	s := &server{
		cfg: config.Config{
			ExternalAuthTimeout:             3 * time.Second,
			EnterpriseHRISPullWorkerEnabled: true,
		},
		enterpriseSvc:    enterprise.NewService(),
		auditSvc:         audit.NewService(),
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(talenta.NewPullAdapter()),
		workerLeaseStore: leaseStore,
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTickWithLease(10, 5, 0, 0, 1, 10*time.Minute)

	_, err = s.hrisPullStateSvc.GetState(connector.ID)
	if !errors.Is(err, hris.ErrPullStateNotFound) {
		t.Fatalf("expected no pull state when lease unavailable, got err=%v", err)
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 0 {
		t.Fatalf("expected one acquire and no release, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseHRISPullLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	if leaseStore.lastTTL != 10*time.Minute {
		t.Fatalf("unexpected lease ttl: %s", leaseStore.lastTTL)
	}
}

func TestRunEnterpriseHRISPullWorkerTickProcessesHybridConnectorAfterWebhookSync(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v2/talenta/v2/employee" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("unexpected page query: %s", got)
		}
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-001",
						"employee_number":   "E-001",
						"organization_name": "IT",
						"job_position":      "Senior Engineer",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
						"join_date":         "2026-04-21",
					},
					"personal": map[string]any{
						"first_name": "Hybrid",
						"last_name":  "One",
						"email":      "hybrid.one@hybrid.local",
						"avatar":     "https://cdn.example.com/photos/hybrid-one-pull.jpg",
					},
					"leave_info": map[string]any{
						"status": "approved",
						"type":   "Annual Leave",
					},
					"payroll_info": map[string]any{
						"cost_center_name": "CC-HYBRID-PULL-01",
					},
				},
				{
					"employment": map[string]any{
						"employee_id":       "EMP-002",
						"employee_number":   "E-002",
						"organization_name": "Security",
						"job_position":      "Supervisor",
						"branch":            "Jakarta HQ",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Hybrid",
						"last_name":  "Two",
						"email":      "hybrid.two@hybrid.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"`+apiServer.URL+`"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}
	webhookSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/webhook_secret",
		"webhook_secret",
		"talenta-webhook-secret-001",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected webhook secret upsert success: %v", err)
	}

	s := &server{
		cfg:                    config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hrisVaultSvc,
		hrisPullStateSvc:       hris.NewPullStateService(),
		hrisPullRegistry:       hris.NewPullRegistry(talenta.NewPullAdapter()),
		hrisHTTPClient:         apiServer.Client(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "hybrid.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		credentialSecret.Ref,
		webhookSecret.Ref,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
				"employment":{
					"employee_id":"EMP-001",
					"employee_number":"E-001",
					"organization_name":"IT",
					"job_position":"Engineer",
					"branch":"Jakarta HQ",
					"employment_status":"active",
					"join_date":"2026-04-20"
				},
				"personal":{
					"first_name":"Hybrid",
					"last_name":"One",
					"email":"hybrid.one@hybrid.local",
					"avatar":"https://cdn.example.com/photos/hybrid-one-webhook.jpg"
				},
				"payroll_info":{
					"cost_center_name":"CC-HYBRID-WEBHOOK-01"
				}
			}
		}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-hybrid-webhook-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(
		request,
		body,
		"talenta-client-001",
		"talenta-webhook-secret-001",
		time.Now().UTC(),
	)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from hybrid webhook, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	webhookEmployee, err := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", "hybrid.one@hybrid.local")
	if err != nil {
		t.Fatalf("expected webhook-synced employee lookup success: %v", err)
	}
	if webhookEmployee.JobTitle != "Engineer" {
		t.Fatalf("expected webhook seed job title Engineer, got %s", webhookEmployee.JobTitle)
	}
	if webhookEmployee.CostCenter != "CC-HYBRID-WEBHOOK-01" {
		t.Fatalf("expected webhook seed cost_center CC-HYBRID-WEBHOOK-01, got %s", webhookEmployee.CostCenter)
	}
	if webhookEmployee.PhotoURL != "https://cdn.example.com/photos/hybrid-one-webhook.jpg" {
		t.Fatalf("expected webhook seed photo_url, got %s", webhookEmployee.PhotoURL)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 1)

	connectors := s.enterpriseSvc.ListHRISConnectors("tenant_demo_jakarta")
	if len(connectors) != 1 || connectors[0].LastSyncAt == nil {
		t.Fatalf("expected hybrid connector last_sync_at to be updated: %+v", connectors)
	}

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success: %v", err)
	}
	if state.Status != "succeeded" {
		t.Fatalf("expected pull state succeeded, got %s", state.Status)
	}
	if state.LastMode != hris.PullModeFull {
		t.Fatalf("expected hybrid connector to run full pull first, got %s", state.LastMode)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundUpdatedWebhookEmployee := false
	foundPulledEmployee := false
	for i := range employees {
		switch employees[i].ExternalID {
		case "EMP-001":
			foundUpdatedWebhookEmployee = employees[i].Status == "active" &&
				employees[i].JobTitle == "Senior Engineer" &&
				employees[i].LeaveStatus == "annual_leave" &&
				employees[i].JoinDate == "2026-04-21" &&
				employees[i].CostCenter == "CC-HYBRID-PULL-01" &&
				employees[i].PhotoURL == "https://cdn.example.com/photos/hybrid-one-pull.jpg"
		case "EMP-002":
			foundPulledEmployee = employees[i].Status == "active" && employees[i].Email == "hybrid.two@hybrid.local"
		}
	}
	if !foundUpdatedWebhookEmployee {
		t.Fatalf("expected pull worker to update webhook employee, including extended fields")
	}
	if !foundPulledEmployee {
		t.Fatalf("expected pull worker to create second hybrid employee")
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	foundAccessOne := false
	foundAccessTwo := false
	for i := range accessUsers {
		switch accessUsers[i].Email {
		case "hybrid.one@hybrid.local":
			foundAccessOne = true
		case "hybrid.two@hybrid.local":
			foundAccessTwo = true
		}
	}
	if !foundAccessOne || !foundAccessTwo {
		t.Fatalf("expected hybrid access users to exist, found one=%t two=%t", foundAccessOne, foundAccessTwo)
	}
}

func TestRunEnterpriseHRISPullWorkerTickPreservesTalentaResignationCancelledHybridState(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v2/talenta/v2/employee" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("unexpected page query: %s", got)
		}
		response := map[string]any{
			"data": []map[string]any{
				{
					"employment": map[string]any{
						"employee_id":       "EMP-RESIGN-CANCELLED-001",
						"employee_number":   "TAL-EMP-RESIGN-CANCELLED-001",
						"organization_name": "Operations",
						"job_position":      "Operator",
						"branch":            "Jakarta",
						"employment_status": "active",
						"status":            "active",
						"join_date":         "2024-01-15",
						"resign_date":       "",
					},
					"personal": map[string]any{
						"first_name":   "Resign",
						"last_name":    "Cancelled User",
						"email":        "resign.cancelled@talenta-sync.local",
						"avatar":       "https://cdn.example.com/photos/resign-cancelled-pull.jpg",
						"mobile_phone": "+628155555555",
					},
					"leave_info": map[string]any{
						"status": "approved",
						"type":   "Annual Leave",
					},
					"payroll_info": map[string]any{
						"cost_center_name": "CC-HYBRID-PULL-RESIGN-01",
					},
				},
				{
					"employment": map[string]any{
						"employee_id":       "EMP-HYBRID-NEW-001",
						"employee_number":   "TAL-HYBRID-NEW-001",
						"organization_name": "Finance",
						"job_position":      "Analyst",
						"branch":            "Jakarta",
						"employment_status": "active",
					},
					"personal": map[string]any{
						"first_name": "Hybrid",
						"last_name":  "New",
						"email":      "hybrid.new@talenta-sync.local",
					},
				},
			},
			"pagination": map[string]any{
				"current_page": 1,
				"last_page":    1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer apiServer.Close()

	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"`+apiServer.URL+`"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}
	webhookSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/webhook_secret",
		"webhook_secret",
		"talenta-webhook-secret-001",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected webhook secret upsert success: %v", err)
	}

	s := &server{
		cfg:                    config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hrisVaultSvc,
		hrisPullStateSvc:       hris.NewPullStateService(),
		hrisPullRegistry:       hris.NewPullRegistry(talenta.NewPullAdapter()),
		hrisHTTPClient:         apiServer.Client(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}
	seedTalentaResignedEmployee(t, s.enterpriseSvc)

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"hybrid",
		credentialSecret.Ref,
		webhookSecret.Ref,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.resignation.cancelled",
		"employment":{
			"employee_id":"EMP-RESIGN-CANCELLED-001",
			"organization_name":"Operations",
			"job_position":"Operator",
			"branch":"Jakarta",
			"resign_date":"",
			"status":"active"
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-hybrid-resignation-cancelled-001")
	request.Header.Set("X-Event-Type", "talenta.employee.resignation.cancelled")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(
		request,
		body,
		"talenta-client-001",
		"talenta-webhook-secret-001",
		time.Now().UTC(),
	)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from hybrid webhook, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	webhookEmployee, err := s.enterpriseSvc.GetEmployeeByEmail("tenant_demo_jakarta", "resign.cancelled@talenta-sync.local")
	if err != nil {
		t.Fatalf("expected webhook-synced employee lookup success: %v", err)
	}
	if webhookEmployee.EmploymentStatus != "active" {
		t.Fatalf("expected webhook resignation cancelled to restore active employment_status, got %s", webhookEmployee.EmploymentStatus)
	}
	if webhookEmployee.Status != "active" {
		t.Fatalf("expected webhook resignation cancelled to restore active status, got %s", webhookEmployee.Status)
	}
	if webhookEmployee.ResignDate != "" {
		t.Fatalf("expected webhook resignation cancelled to clear resign_date, got %s", webhookEmployee.ResignDate)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 1)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success: %v", err)
	}
	if state.Status != "succeeded" {
		t.Fatalf("expected pull state succeeded, got %s", state.Status)
	}
	if state.LastMode != hris.PullModeFull {
		t.Fatalf("expected hybrid connector to run full pull first, got %s", state.LastMode)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundRecoveredEmployee := false
	foundPulledEmployee := false
	for i := range employees {
		switch employees[i].ExternalID {
		case "EMP-RESIGN-CANCELLED-001":
			foundRecoveredEmployee = employees[i].Status == "active" &&
				employees[i].EmploymentStatus == "active" &&
				employees[i].ResignDate == "" &&
				employees[i].CostCenter == "CC-HYBRID-PULL-RESIGN-01" &&
				employees[i].PhotoURL == "https://cdn.example.com/photos/resign-cancelled-pull.jpg"
		case "EMP-HYBRID-NEW-001":
			foundPulledEmployee = employees[i].Status == "active" && employees[i].Email == "hybrid.new@talenta-sync.local"
		}
	}
	if !foundRecoveredEmployee {
		t.Fatalf("expected pull worker to preserve resignation cancelled state while refreshing extended fields")
	}
	if !foundPulledEmployee {
		t.Fatalf("expected pull worker to create second hybrid employee after resignation cancelled webhook")
	}
}

func TestRunEnterpriseHRISPullWorkerTickFailureAlertAndSkipControls(t *testing.T) {
	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		"not-json",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg:              config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:    enterprise.NewService(),
		auditSvc:         audit.NewService(),
		hrisVaultSvc:     hrisVaultSvc,
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(talenta.NewPullAdapter()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 1)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after failed tick: %v", err)
	}
	if state.Status != "failed" {
		t.Fatalf("expected failed pull state, got %s", state.Status)
	}
	if state.ConsecutiveFailures != 1 {
		t.Fatalf("expected consecutive_failures=1, got %d", state.ConsecutiveFailures)
	}
	if !strings.Contains(state.LastError, "client_secret is required") {
		t.Fatalf("unexpected last_error: %s", state.LastError)
	}

	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected one pull worker alert log, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=1") ||
		!strings.Contains(alertLogs[0].Target, "threshold=1") ||
		!strings.Contains(alertLogs[0].Target, "consecutive_failures=1") {
		t.Fatalf("unexpected pull worker alert payload: %s", alertLogs[0].Target)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 1*time.Hour, 0, 1)
	afterCooldown, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after cooldown skip: %v", err)
	}
	if afterCooldown.ConsecutiveFailures != 1 {
		t.Fatalf("expected cooldown skip to preserve consecutive_failures=1, got %d", afterCooldown.ConsecutiveFailures)
	}
	alertLogs = s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 2 {
		t.Fatalf("expected cooldown skip to append one extra pull worker alert, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=0") ||
		!strings.Contains(alertLogs[0].Target, "skipped_cooldown=1") ||
		!strings.Contains(alertLogs[0].Target, "consecutive_failures=1") {
		t.Fatalf("unexpected cooldown skip alert payload: %s", alertLogs[0].Target)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 1, 0, 0, 1)
	afterAttemptLimit, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after attempt-limit skip: %v", err)
	}
	if afterAttemptLimit.ConsecutiveFailures != 1 {
		t.Fatalf("expected attempt-limit skip to preserve consecutive_failures=1, got %d", afterAttemptLimit.ConsecutiveFailures)
	}
	alertLogs = s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 3 {
		t.Fatalf("expected attempt-limit skip to append one extra pull worker alert, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=0") ||
		!strings.Contains(alertLogs[0].Target, "skipped_attempt_limit=1") ||
		!strings.Contains(alertLogs[0].Target, "consecutive_failures=1") {
		t.Fatalf("unexpected attempt-limit skip alert payload: %s", alertLogs[0].Target)
	}
}

func TestRunEnterpriseHRISPullWorkerTickWithRetryBackoffHonorsMaxBackoff(t *testing.T) {
	adapter := &failNTimesPullAdapter{failUntil: 2}
	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		"opaque-credential",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg:              config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:    enterprise.NewService(),
		accessSvc:        access.NewService(),
		auditSvc:         audit.NewService(),
		hrisVaultSvc:     hrisVaultSvc,
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(adapter),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "pull.local", "active")
	if err != nil {
		t.Fatalf("expected domain mapping create success: %v", err)
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	baseBackoff := 30 * time.Millisecond
	maxBackoff := 50 * time.Millisecond
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, 0, 30*time.Minute, 1)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after first failure: %v", err)
	}
	if state.Status != "failed" || state.ConsecutiveFailures != 1 {
		t.Fatalf("unexpected pull state after first failure: %+v", state)
	}
	if adapter.calls != 1 {
		t.Fatalf("expected one pull adapter call after first attempt, got %d", adapter.calls)
	}

	time.Sleep(baseBackoff + 10*time.Millisecond)
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, 0, 30*time.Minute, 1)

	state, err = s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after second failure: %v", err)
	}
	if state.Status != "failed" || state.ConsecutiveFailures != 2 {
		t.Fatalf("unexpected pull state after second failure: %+v", state)
	}
	if adapter.calls != 2 {
		t.Fatalf("expected two pull adapter calls after second attempt, got %d", adapter.calls)
	}

	time.Sleep(baseBackoff + 10*time.Millisecond)
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, 0, 30*time.Minute, 1)

	state, err = s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup during exponential cooldown: %v", err)
	}
	if state.ConsecutiveFailures != 2 {
		t.Fatalf("expected exponential cooldown to preserve consecutive_failures=2, got %d", state.ConsecutiveFailures)
	}
	if adapter.calls != 2 {
		t.Fatalf("expected exponential cooldown to skip third attempt, calls=%d", adapter.calls)
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 3 {
		t.Fatalf("expected cooldown skip to append third pull worker alert, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=0") ||
		!strings.Contains(alertLogs[0].Target, "skipped_cooldown=1") ||
		!strings.Contains(alertLogs[0].Target, "consecutive_failures=2") {
		t.Fatalf("unexpected exponential cooldown alert payload: %s", alertLogs[0].Target)
	}

	time.Sleep(15 * time.Millisecond)
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, baseBackoff, maxBackoff, 0, 30*time.Minute, 1)

	state, err = s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after exponential cooldown expiry: %v", err)
	}
	if state.Status != "succeeded" || state.ConsecutiveFailures != 0 {
		t.Fatalf("unexpected pull state after exponential cooldown expiry: %+v", state)
	}
	if adapter.calls != 3 {
		t.Fatalf("expected third pull adapter call after exponential cooldown expiry, got %d", adapter.calls)
	}

	foundEmployee := false
	for _, employee := range s.enterpriseSvc.ListEmployees("tenant_demo_jakarta") {
		if employee.Email == "pull.backoff@pull.local" && employee.Status == "active" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected pull backoff recovery to sync pull.backoff@pull.local")
	}
	foundAccessUser := false
	for _, user := range s.accessSvc.ListUsers("tenant_demo_jakarta") {
		if user.Email == "pull.backoff@pull.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected pull backoff recovery to sync access user pull.backoff@pull.local")
	}
}

func TestRunEnterpriseHRISPullWorkerTickStatefulThresholdUsesConsecutiveFailures(t *testing.T) {
	hrisVaultSvc := hris.NewVaultService("vault-master-key-001")
	credentialSecret, err := hrisVaultSvc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		"not-json",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected credential secret upsert success: %v", err)
	}

	s := &server{
		cfg:              config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:    enterprise.NewService(),
		auditSvc:         audit.NewService(),
		hrisVaultSvc:     hrisVaultSvc,
		hrisPullStateSvc: hris.NewPullStateService(),
		hrisPullRegistry: hris.NewPullRegistry(talenta.NewPullAdapter()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected connector create success: %v", err)
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 3)
	if logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10); len(logs) != 0 {
		t.Fatalf("expected first failed tick to stay below stateful threshold, got %d logs", len(logs))
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 3)
	if logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10); len(logs) != 0 {
		t.Fatalf("expected second failed tick to stay below stateful threshold, got %d logs", len(logs))
	}

	s.runEnterpriseHRISPullWorkerTick(10, 5, 0, 0, 3)

	state, err := s.hrisPullStateSvc.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull state lookup success after third failed tick: %v", err)
	}
	if state.ConsecutiveFailures != 3 {
		t.Fatalf("expected consecutive_failures=3 after third failed tick, got %d", state.ConsecutiveFailures)
	}

	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_pull_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected third failed tick to append one stateful pull alert, got %d", len(alertLogs))
	}
	if !strings.Contains(alertLogs[0].Target, "failed=1") ||
		!strings.Contains(alertLogs[0].Target, "threshold=3") ||
		!strings.Contains(alertLogs[0].Target, "consecutive_failures=3") {
		t.Fatalf("unexpected stateful pull alert payload: %s", alertLogs[0].Target)
	}
}

func TestListEnterpriseHRISPullStatesRoute(t *testing.T) {
	pullStateSvc := hris.NewPullStateService()
	startedAt := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)
	succeededAt := time.Date(2026, 4, 22, 9, 5, 0, 0, time.UTC)
	failedAt := time.Date(2026, 4, 22, 9, 30, 0, 0, time.UTC)

	if _, err := pullStateSvc.MarkStarted("tenant_demo_jakarta", "connector-talenta", "talenta", hris.PullModeIncremental, startedAt); err != nil {
		t.Fatalf("mark started should succeed: %v", err)
	}
	if _, err := pullStateSvc.MarkSucceeded(
		"tenant_demo_jakarta",
		"connector-talenta",
		"talenta",
		hris.PullModeIncremental,
		"pull-req-001",
		succeededAt,
	); err != nil {
		t.Fatalf("mark succeeded should succeed: %v", err)
	}
	if _, err := pullStateSvc.MarkFailed("tenant_demo_jakarta", "connector-talenta", "talenta", failedAt, errors.New("429 throttled")); err != nil {
		t.Fatalf("mark failed should succeed: %v", err)
	}
	if _, err := pullStateSvc.MarkStarted("tenant_other", "connector-gadjian", "gadjian", hris.PullModeFull, startedAt); err != nil {
		t.Fatalf("mark other tenant should succeed: %v", err)
	}

	s := &server{hrisPullStateSvc: pullStateSvc}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/enterprise/hris-pull-states?tenant_id=tenant_demo_jakarta", nil)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISPullStates(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []hris.ConnectorPullState `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid pull state payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one tenant-scoped pull state, got %d", len(payload.Items))
	}
	if payload.Items[0].ConnectorID != "connector-talenta" ||
		payload.Items[0].Vendor != "talenta" ||
		payload.Items[0].LastRequestID != "pull-req-001" ||
		payload.Items[0].LastMode != hris.PullModeIncremental ||
		payload.Items[0].Status != "failed" {
		t.Fatalf("unexpected pull state payload: %+v", payload.Items[0])
	}
}

func TestListEnterpriseHRISPullStatesRouteRefreshesSharedState(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstSvc, err := hris.NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected first pull state service to initialize: %v", err)
	}
	secondSvc, err := hris.NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected second pull state service to initialize: %v", err)
	}

	startedAt := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
	if _, err := firstSvc.MarkStarted(
		"tenant_demo_jakarta",
		"connector-talenta-shared",
		"talenta",
		hris.PullModeIncremental,
		startedAt,
	); err != nil {
		t.Fatalf("expected shared pull state seed to succeed: %v", err)
	}

	s := &server{hrisPullStateSvc: secondSvc}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/enterprise/hris-pull-states?tenant_id=tenant_demo_jakarta", nil)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISPullStates(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 after shared-state refresh, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []hris.ConnectorPullState `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid pull state payload after refresh: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 || payload.Items[0].ConnectorID != "connector-talenta-shared" {
		t.Fatalf("expected refreshed pull state from shared store, got %+v", payload.Items)
	}
}

func TestRunEnterpriseHRISPullWorkerTickRefreshesSharedState(t *testing.T) {
	store := &httpMemoryStateStore{}
	enterpriseSvcA, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected first enterprise service with state store to initialize: %v", err)
	}
	enterpriseSvcB, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected second enterprise service with state store to initialize: %v", err)
	}
	pullStateSvcA, err := hris.NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected first pull state service with state store to initialize: %v", err)
	}
	_ = pullStateSvcA
	pullStateSvcB, err := hris.NewPullStateServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("expected second pull state service with state store to initialize: %v", err)
	}
	vaultSvcA, err := hris.NewVaultServiceWithStateStore("vault-master-key-001", store)
	if err != nil {
		t.Fatalf("expected first vault service with state store to initialize: %v", err)
	}

	credentialSecret, err := vaultSvcA.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001","client_secret":"talenta-secret-001","base_url":"https://pull-refresh.local"}`,
		"qa",
	)
	if err != nil {
		t.Fatalf("expected shared credential secret create success: %v", err)
	}
	if _, err := enterpriseSvcA.CreateDomainMapping("tenant_demo_jakarta", "pull.local", "active"); err != nil {
		t.Fatalf("expected shared domain mapping create success: %v", err)
	}
	connector, err := enterpriseSvcA.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"pull",
		credentialSecret.Ref,
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("expected shared connector create success: %v", err)
	}
	vaultSvcB, err := hris.NewVaultServiceWithStateStore("vault-master-key-001", store)
	if err != nil {
		t.Fatalf("expected refreshed second vault service with state store to initialize: %v", err)
	}

	s := &server{
		cfg:              config.Config{ExternalAuthTimeout: 3 * time.Second},
		enterpriseSvc:    enterpriseSvcB,
		accessSvc:        access.NewService(),
		auditSvc:         audit.NewService(),
		hrisVaultSvc:     vaultSvcB,
		hrisPullStateSvc: pullStateSvcB,
		hrisPullRegistry: hris.NewPullRegistry(&failNTimesPullAdapter{}),
	}

	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(10, 5, 0, 0, 0, 30*time.Minute, 1)

	state, err := pullStateSvcB.GetState(connector.ID)
	if err != nil {
		t.Fatalf("expected pull worker to observe shared connector state without restart: %v", err)
	}
	if state.Status != "succeeded" || state.LastRequestID == "" {
		t.Fatalf("unexpected refreshed pull worker state: %+v", state)
	}

	foundEmployee := false
	for _, employee := range enterpriseSvcB.ListEmployees("tenant_demo_jakarta") {
		if employee.Email == "pull.backoff@pull.local" && employee.Status == "active" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected pull worker to sync shared connector employee after refresh")
	}
}

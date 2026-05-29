package enterprise

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

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
	nonceValue, err := randomID("non_")
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
		Nonce:       nonceValue,
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
	return s.SyncEmployeesWithAccessUpsertMetadata(
		tenantID,
		source,
		actor,
		requestID,
		"",
		"",
		inputs,
		applier,
	)
}

func (s *Service) SyncEmployeesWithAccessUpsertMetadata(
	tenantID, source, actor string,
	requestID string,
	connectorID string,
	rawPayloadRef string,
	inputs []EmployeeSyncInput,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	nextTenantID, nextSource, nextActor, nextRequestID, err := normalizeSyncEmployeesRequest(tenantID, source, actor, requestID, inputs)
	if err != nil {
		return SyncResult{}, 0, 0, 0, err
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	nextRawPayloadRef := strings.TrimSpace(rawPayloadRef)
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
			ConnectorID:   nextConnectorID,
			RawPayloadRef: nextRawPayloadRef,
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
		employeeNumber := strings.TrimSpace(inputs[i].EmployeeNumber)
		joinDate := strings.TrimSpace(inputs[i].JoinDate)
		resignDate := strings.TrimSpace(inputs[i].ResignDate)
		shiftCode := strings.TrimSpace(inputs[i].ShiftCode)
		scheduleWindow := strings.TrimSpace(inputs[i].ScheduleWindow)
		leaveStatus := strings.TrimSpace(inputs[i].LeaveStatus)
		costCenter := strings.TrimSpace(inputs[i].CostCenter)
		photoURL := strings.TrimSpace(inputs[i].PhotoURL)
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
			createdRecords[targetIndex].EmployeeNumber = employeeNumber
			createdRecords[targetIndex].Email = email
			createdRecords[targetIndex].FullName = strings.TrimSpace(inputs[i].FullName)
			createdRecords[targetIndex].Department = strings.TrimSpace(inputs[i].Department)
			createdRecords[targetIndex].JobTitle = strings.TrimSpace(inputs[i].JobTitle)
			createdRecords[targetIndex].Location = strings.TrimSpace(inputs[i].Location)
			createdRecords[targetIndex].Phone = phone
			createdRecords[targetIndex].ManagerExternalID = managerExternalID
			createdRecords[targetIndex].EmploymentStatus = employmentStatus
			createdRecords[targetIndex].JoinDate = joinDate
			createdRecords[targetIndex].ResignDate = resignDate
			createdRecords[targetIndex].ShiftCode = shiftCode
			createdRecords[targetIndex].ScheduleWindow = scheduleWindow
			createdRecords[targetIndex].LeaveStatus = leaveStatus
			createdRecords[targetIndex].CostCenter = costCenter
			createdRecords[targetIndex].PhotoURL = photoURL
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
			s.employees[targetIndex].EmployeeNumber = employeeNumber
			s.employees[targetIndex].Email = email
			s.employees[targetIndex].FullName = strings.TrimSpace(inputs[i].FullName)
			s.employees[targetIndex].Department = strings.TrimSpace(inputs[i].Department)
			s.employees[targetIndex].JobTitle = strings.TrimSpace(inputs[i].JobTitle)
			s.employees[targetIndex].Location = strings.TrimSpace(inputs[i].Location)
			s.employees[targetIndex].Phone = phone
			s.employees[targetIndex].ManagerExternalID = managerExternalID
			s.employees[targetIndex].EmploymentStatus = employmentStatus
			s.employees[targetIndex].JoinDate = joinDate
			s.employees[targetIndex].ResignDate = resignDate
			s.employees[targetIndex].ShiftCode = shiftCode
			s.employees[targetIndex].ScheduleWindow = scheduleWindow
			s.employees[targetIndex].LeaveStatus = leaveStatus
			s.employees[targetIndex].CostCenter = costCenter
			s.employees[targetIndex].PhotoURL = photoURL
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
			EmployeeNumber:    employeeNumber,
			Email:             email,
			FullName:          strings.TrimSpace(inputs[i].FullName),
			Department:        strings.TrimSpace(inputs[i].Department),
			JobTitle:          strings.TrimSpace(inputs[i].JobTitle),
			Location:          strings.TrimSpace(inputs[i].Location),
			Phone:             phone,
			ManagerExternalID: managerExternalID,
			EmploymentStatus:  employmentStatus,
			JoinDate:          joinDate,
			ResignDate:        resignDate,
			ShiftCode:         shiftCode,
			ScheduleWindow:    scheduleWindow,
			LeaveStatus:       leaveStatus,
			CostCenter:        costCenter,
			PhotoURL:          photoURL,
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

	nextSource, err = normalizeSyncSource(source)
	if err != nil {
		return "", "", "", "", err
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

func (s *Service) GetSyncWorkerAlertSubscription(tenantID string) (SyncWorkerAlertSubscription, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertSubscription{}, false
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SyncWorkerAlertSubscription{}, false
	}
	for i := range s.syncWorkerAlertSubscriptions {
		if s.syncWorkerAlertSubscriptions[i].TenantID != nextTenantID {
			continue
		}
		return cloneSyncWorkerAlertSubscription(s.syncWorkerAlertSubscriptions[i]), true
	}
	return SyncWorkerAlertSubscription{}, false
}

func (s *Service) UpsertSyncWorkerAlertSubscription(
	input SyncWorkerAlertSubscriptionUpsertOptions,
) (SyncWorkerAlertSubscription, error) {
	resolved, err := resolveSyncWorkerAlertSubscriptionUpsertOptions(input)
	if err != nil {
		return SyncWorkerAlertSubscription{}, err
	}

	record := SyncWorkerAlertSubscription{
		TenantID:             resolved.TenantID,
		Enabled:              resolved.Enabled,
		WorkerAlertThreshold: resolved.WorkerAlertThreshold,
		WindowSeconds:        int64(resolved.Window.Seconds()),
		CooldownSeconds:      int64(resolved.Cooldown.Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email:    resolved.EmailEnabled,
			WhatsApp: resolved.WhatsAppEnabled,
		},
		ReceiverGroups: normalizeSyncWorkerAlertSubscriptionReceiverGroups(resolved.ReceiverGroups),
		UpdatedAt:      time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateSyncWorkerAlertStateLocked(func() error {
		upserted := false
		for i := range s.syncWorkerAlertSubscriptions {
			if s.syncWorkerAlertSubscriptions[i].TenantID != resolved.TenantID {
				continue
			}
			s.syncWorkerAlertSubscriptions[i] = cloneSyncWorkerAlertSubscription(record)
			upserted = true
			break
		}
		if !upserted {
			s.syncWorkerAlertSubscriptions = append(
				[]SyncWorkerAlertSubscription{cloneSyncWorkerAlertSubscription(record)},
				s.syncWorkerAlertSubscriptions...,
			)
		}
		return nil
	}); err != nil {
		return SyncWorkerAlertSubscription{}, err
	}
	return cloneSyncWorkerAlertSubscription(record), nil
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
		s.mu.Lock()
		initialSnapshot := s.coreStateSnapshotLocked()
		s.mu.Unlock()
		return s.stateStore.Save(stateKey, initialSnapshot)
	}

	var alertSnapshot syncWorkerAlertStateSnapshot
	alertFound, err := s.stateStore.Load(syncWorkerAlertStateKey, &alertSnapshot)
	if err != nil {
		return err
	}

	var hrisSnapshot hrisWebhookStateSnapshot
	hrisFound, err := s.stateStore.Load(hrisWebhookStateKey, &hrisSnapshot)
	if err != nil {
		return err
	}
	if !hrisFound {
		hrisSnapshot = hrisWebhookStateSnapshotFromLegacyStateSnapshot(snapshot)
		if hasHRISWebhookStateSnapshot(hrisSnapshot) {
			if err := s.stateStore.Save(hrisWebhookStateKey, hrisSnapshot); err != nil {
				return err
			}
			if err := s.stateStore.Save(stateKey, coreStateSnapshotFromSnapshot(snapshot)); err != nil {
				return err
			}
		}
	}
	if !alertFound {
		alertSnapshot = syncWorkerAlertStateSnapshotFromLegacyStateSnapshot(snapshot)
		if hasSyncWorkerAlertStateSnapshot(alertSnapshot) {
			if err := s.stateStore.Save(syncWorkerAlertStateKey, alertSnapshot); err != nil {
				return err
			}
			if err := s.stateStore.Save(stateKey, coreStateSnapshotFromSnapshot(snapshot)); err != nil {
				return err
			}
		}
	}

	s.mu.Lock()
	s.restoreCoreStateLocked(snapshot)
	s.restoreHRISWebhookStateLocked(hrisSnapshot)
	s.restoreSyncWorkerAlertStateLocked(alertSnapshot)
	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}
	s.mu.Unlock()

	return nil
}

func (s *Service) RefreshCoreState() error {
	if s == nil || s.stateStore == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCoreStateLocked()
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, s.coreStateSnapshotLocked())
}

func (s *Service) loadCoreStateLocked() (stateSnapshot, bool, error) {
	if s.stateStore == nil {
		return stateSnapshot{}, false, nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return stateSnapshot{}, false, err
	}
	if !found {
		return stateSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *Service) refreshCoreStateLocked() error {
	snapshot, found, err := s.loadCoreStateLocked()
	if err != nil {
		return err
	}
	if found {
		s.restoreCoreStateLocked(snapshot)
	}

	hrisSnapshot, hrisFound, err := s.loadHRISWebhookStateLocked()
	if err != nil {
		return err
	}
	if hrisFound {
		s.restoreHRISWebhookStateLocked(hrisSnapshot)
	} else {
		s.restoreHRISWebhookStateLocked(hrisWebhookStateSnapshot{})
	}
	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}
	return nil
}

func (s *Service) loadHRISWebhookStateLocked() (hrisWebhookStateSnapshot, bool, error) {
	if s.stateStore == nil {
		return hrisWebhookStateSnapshot{}, false, nil
	}

	var snapshot hrisWebhookStateSnapshot
	found, err := s.stateStore.Load(hrisWebhookStateKey, &snapshot)
	if err != nil {
		return hrisWebhookStateSnapshot{}, false, err
	}
	if !found {
		return hrisWebhookStateSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *Service) persistHRISWebhookStateLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(hrisWebhookStateKey, s.hrisWebhookStateSnapshotLocked())
}

func (s *Service) mutateHRISWebhookStateLocked(mutator func() (bool, error)) error {
	if s.stateStore == nil {
		changed, err := mutator()
		if err != nil {
			return err
		}
		if changed {
			s.normalizeHRISWebhookReceiptDueIndexLocked()
			s.syncQueuedHRISWebhookExecutionIndicesLocked()
		}
		return nil
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	if !hasCAS {
		if err := s.refreshCoreStateLocked(); err != nil {
			return err
		}
		changed, err := mutator()
		if err != nil || !changed {
			return err
		}
		s.normalizeHRISWebhookReceiptDueIndexLocked()
		s.syncQueuedHRISWebhookExecutionIndicesLocked()
		return s.persistHRISWebhookStateLocked()
	}

	baseSnapshot := s.hrisWebhookStateSnapshotLocked()
	for attempt := 0; attempt < maxEnterpriseHRISWebhookCASRetries; attempt++ {
		snapshot, found, err := s.loadHRISWebhookStateLocked()
		if err != nil {
			return err
		}
		if found {
			s.restoreHRISWebhookStateLocked(snapshot)
		} else {
			s.restoreHRISWebhookStateLocked(baseSnapshot)
		}

		changed, err := mutator()
		if err != nil {
			if found {
				s.restoreHRISWebhookStateLocked(snapshot)
			} else {
				s.restoreHRISWebhookStateLocked(baseSnapshot)
			}
			return err
		}
		if !changed {
			if found {
				s.restoreHRISWebhookStateLocked(snapshot)
			} else {
				s.restoreHRISWebhookStateLocked(baseSnapshot)
			}
			return err
		}
		s.normalizeHRISWebhookReceiptDueIndexLocked()
		s.syncQueuedHRISWebhookExecutionIndicesLocked()

		persisted, err := casStore.CompareAndSwap(
			hrisWebhookStateKey,
			found,
			snapshot,
			s.hrisWebhookStateSnapshotLocked(),
		)
		if err != nil {
			if found {
				s.restoreHRISWebhookStateLocked(snapshot)
			} else {
				s.restoreHRISWebhookStateLocked(baseSnapshot)
			}
			return err
		}
		if persisted {
			return nil
		}
	}
	if snapshot, found, err := s.loadHRISWebhookStateLocked(); err == nil {
		if found {
			s.restoreHRISWebhookStateLocked(snapshot)
		} else {
			s.restoreHRISWebhookStateLocked(baseSnapshot)
		}
	} else {
		s.restoreHRISWebhookStateLocked(baseSnapshot)
	}
	return ErrEnterpriseHRISWebhookStateConflict
}

func (s *Service) persistSyncWorkerAlertStateLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(syncWorkerAlertStateKey, s.syncWorkerAlertStateSnapshotLocked())
}

func (s *Service) loadSyncWorkerAlertStateLocked() (syncWorkerAlertStateSnapshot, bool, error) {
	if s.stateStore == nil {
		return syncWorkerAlertStateSnapshot{}, false, nil
	}

	var snapshot syncWorkerAlertStateSnapshot
	found, err := s.stateStore.Load(syncWorkerAlertStateKey, &snapshot)
	if err != nil {
		return syncWorkerAlertStateSnapshot{}, false, err
	}
	if !found {
		return syncWorkerAlertStateSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *Service) refreshSyncWorkerAlertStateLocked() error {
	if s.stateStore == nil {
		return nil
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	now := time.Now().UTC()
	for attempt := 0; attempt < maxSyncWorkerAlertCASRetries; attempt++ {
		snapshot, found, err := s.loadSyncWorkerAlertStateLocked()
		if err != nil {
			return err
		}
		if !found {
			s.restoreSyncWorkerAlertStateLocked(syncWorkerAlertStateSnapshot{})
			return nil
		}

		s.restoreSyncWorkerAlertStateLocked(snapshot)
		recoveredNotifications, recoveredCooldowns := s.recoverExpiredSyncWorkerAlertInFlightsLocked(now)
		flightCount := len(s.syncWorkerAlertInFlights)
		s.pruneExpiredSyncWorkerAlertInFlightsLocked(now)
		if len(recoveredNotifications) == 0 && len(recoveredCooldowns) == 0 && len(s.syncWorkerAlertInFlights) == flightCount {
			return nil
		}

		cleaned := s.syncWorkerAlertStateSnapshotLocked()
		if !hasCAS {
			return s.stateStore.Save(syncWorkerAlertStateKey, cleaned)
		}

		persisted, err := casStore.CompareAndSwap(
			syncWorkerAlertStateKey,
			true,
			snapshot,
			cleaned,
		)
		if err != nil {
			return err
		}
		if persisted {
			return nil
		}
	}
	return ErrSyncWorkerAlertStateConflict
}

func (s *Service) mutateSyncWorkerAlertStateLocked(mutator func() error) error {
	if s.stateStore == nil {
		return mutator()
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	if !hasCAS {
		if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
			return err
		}
		if err := mutator(); err != nil {
			return err
		}
		return s.persistSyncWorkerAlertStateLocked()
	}

	for attempt := 0; attempt < maxSyncWorkerAlertCASRetries; attempt++ {
		snapshot, found, err := s.loadSyncWorkerAlertStateLocked()
		if err != nil {
			return err
		}
		if found {
			s.restoreSyncWorkerAlertStateLocked(snapshot)
		} else {
			s.restoreSyncWorkerAlertStateLocked(syncWorkerAlertStateSnapshot{})
		}
		if err := mutator(); err != nil {
			return err
		}
		persisted, err := casStore.CompareAndSwap(
			syncWorkerAlertStateKey,
			found,
			snapshot,
			s.syncWorkerAlertStateSnapshotLocked(),
		)
		if err != nil {
			return err
		}
		if persisted {
			return nil
		}
	}
	return ErrSyncWorkerAlertStateConflict
}

func (s *Service) appendSyncWorkerAlertStateDeltaLocked(
	notifications []SyncWorkerAlertNotification,
	cooldowns []SyncWorkerAlertCooldown,
) error {
	if len(notifications) == 0 && len(cooldowns) == 0 {
		return nil
	}
	return s.mutateSyncWorkerAlertStateLocked(func() error {
		s.prependSyncWorkerAlertNotificationsLocked(notifications)
		s.applySyncWorkerAlertCooldownUpdatesLocked(cooldowns)
		return nil
	})
}

func (s *Service) prependSyncWorkerAlertNotificationsLocked(items []SyncWorkerAlertNotification) {
	if len(items) == 0 {
		return
	}
	cloned := cloneSyncWorkerAlertNotifications(items)
	s.syncWorkerAlertNotifications = append(cloned, s.syncWorkerAlertNotifications...)
	if len(s.syncWorkerAlertNotifications) > maxSyncWorkerAlertNotificationLimit {
		s.syncWorkerAlertNotifications = s.syncWorkerAlertNotifications[:maxSyncWorkerAlertNotificationLimit]
	}
}

func (s *Service) applySyncWorkerAlertCooldownUpdatesLocked(items []SyncWorkerAlertCooldown) {
	for i := range items {
		next := items[i]
		tenantID := strings.TrimSpace(next.TenantID)
		fingerprint := strings.TrimSpace(next.Fingerprint)
		if tenantID == "" || fingerprint == "" {
			continue
		}
		s.upsertSyncWorkerAlertCooldownLocked(tenantID, fingerprint, next.LastSentAt)
	}
}

func (s *Service) coreStateSnapshotLocked() stateSnapshot {
	return stateSnapshot{
		DomainMappings:        cloneDomainMappings(s.domainMappings),
		HRISConnectors:        cloneHRISConnectors(s.hrisConnectors),
		IDPConfigs:            cloneIDPConfigs(s.idpConfigs),
		Employees:             cloneEmployees(s.employees),
		SyncJobs:              cloneSyncJobs(s.syncJobs),
		SyncRequestRecords:    cloneSyncRequestRecords(s.syncRequestRecords),
		JITProvisionApprovals: cloneJITProvisionApprovals(s.jitProvisionApprovals),
	}
}

func (s *Service) hrisWebhookStateSnapshotLocked() hrisWebhookStateSnapshot {
	return hrisWebhookStateSnapshot{
		HRISWebhookReceipts:         cloneHRISWebhookReceipts(s.hrisWebhookReceipts),
		HRISWebhookExecutions:       cloneHRISWebhookExecutions(s.hrisWebhookExecutions),
		DueReceiptIDs:               cloneHRISWebhookReceiptDueIndexEntries(s.dueReceiptIDs),
		QueuedReceiptExecutionIDs:   append([]string(nil), s.queuedReceiptExecutionIDs...),
		QueuedDLQReplayExecutionIDs: append([]string(nil), s.queuedDLQReplayExecutionIDs...),
	}
}

func (s *Service) syncWorkerAlertStateSnapshotLocked() syncWorkerAlertStateSnapshot {
	return syncWorkerAlertStateSnapshot{
		SyncWorkerAlertSubscriptions: cloneSyncWorkerAlertSubscriptions(s.syncWorkerAlertSubscriptions),
		SyncWorkerAlertNotifications: cloneSyncWorkerAlertNotifications(s.syncWorkerAlertNotifications),
		SyncWorkerAlertCooldowns:     cloneSyncWorkerAlertCooldowns(s.syncWorkerAlertCooldowns),
		SyncWorkerAlertInFlights:     cloneSyncWorkerAlertInFlights(s.syncWorkerAlertInFlights),
	}
}

func (s *Service) restoreSyncWorkerAlertStateLocked(snapshot syncWorkerAlertStateSnapshot) {
	s.syncWorkerAlertSubscriptions = cloneSyncWorkerAlertSubscriptions(snapshot.SyncWorkerAlertSubscriptions)
	s.syncWorkerAlertNotifications = cloneSyncWorkerAlertNotifications(snapshot.SyncWorkerAlertNotifications)
	s.syncWorkerAlertCooldowns = cloneSyncWorkerAlertCooldowns(snapshot.SyncWorkerAlertCooldowns)
	s.syncWorkerAlertInFlights = cloneSyncWorkerAlertInFlights(snapshot.SyncWorkerAlertInFlights)
}

func (s *Service) restoreCoreStateLocked(snapshot stateSnapshot) {
	s.domainMappings = cloneDomainMappings(snapshot.DomainMappings)
	s.hrisConnectors = cloneHRISConnectors(snapshot.HRISConnectors)
	s.idpConfigs = cloneIDPConfigs(snapshot.IDPConfigs)
	s.employees = cloneEmployees(snapshot.Employees)
	s.syncJobs = cloneSyncJobs(snapshot.SyncJobs)
	s.syncRequestRecords = cloneSyncRequestRecords(snapshot.SyncRequestRecords)
	s.jitProvisionApprovals = cloneJITProvisionApprovals(snapshot.JITProvisionApprovals)
}

func (s *Service) restoreHRISWebhookStateLocked(snapshot hrisWebhookStateSnapshot) {
	s.hrisWebhookReceipts = cloneHRISWebhookReceipts(snapshot.HRISWebhookReceipts)
	s.hrisWebhookExecutions = cloneHRISWebhookExecutions(snapshot.HRISWebhookExecutions)
	s.dueReceiptIDs = cloneHRISWebhookReceiptDueIndexEntries(snapshot.DueReceiptIDs)
	s.queuedReceiptExecutionIDs = append([]string(nil), snapshot.QueuedReceiptExecutionIDs...)
	s.queuedDLQReplayExecutionIDs = append([]string(nil), snapshot.QueuedDLQReplayExecutionIDs...)
	s.normalizeHRISWebhookReceiptDueIndexLocked()
	s.syncQueuedHRISWebhookExecutionIndicesLocked()
}

func coreStateSnapshotFromSnapshot(snapshot stateSnapshot) stateSnapshot {
	return stateSnapshot{
		DomainMappings:        cloneDomainMappings(snapshot.DomainMappings),
		HRISConnectors:        cloneHRISConnectors(snapshot.HRISConnectors),
		IDPConfigs:            cloneIDPConfigs(snapshot.IDPConfigs),
		Employees:             cloneEmployees(snapshot.Employees),
		SyncJobs:              cloneSyncJobs(snapshot.SyncJobs),
		SyncRequestRecords:    cloneSyncRequestRecords(snapshot.SyncRequestRecords),
		JITProvisionApprovals: cloneJITProvisionApprovals(snapshot.JITProvisionApprovals),
	}
}

func hrisWebhookStateSnapshotFromLegacyStateSnapshot(snapshot stateSnapshot) hrisWebhookStateSnapshot {
	state := hrisWebhookStateSnapshot{
		HRISWebhookReceipts:   cloneHRISWebhookReceipts(snapshot.HRISWebhookReceipts),
		HRISWebhookExecutions: cloneHRISWebhookExecutions(snapshot.HRISWebhookExecutions),
	}
	state.DueReceiptIDs = buildHRISWebhookReceiptDueIndexEntries(state.HRISWebhookReceipts)
	state.QueuedReceiptExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		state.HRISWebhookExecutions,
		HRISWebhookExecutionKindReceiptProcess,
	)
	state.QueuedDLQReplayExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		state.HRISWebhookExecutions,
		HRISWebhookExecutionKindDLQReplay,
	)
	return state
}

func syncWorkerAlertStateSnapshotFromLegacyStateSnapshot(snapshot stateSnapshot) syncWorkerAlertStateSnapshot {
	return syncWorkerAlertStateSnapshot{
		SyncWorkerAlertSubscriptions: cloneSyncWorkerAlertSubscriptions(snapshot.SyncWorkerAlertSubscriptions),
		SyncWorkerAlertNotifications: cloneSyncWorkerAlertNotifications(snapshot.SyncWorkerAlertNotifications),
		SyncWorkerAlertCooldowns:     cloneSyncWorkerAlertCooldowns(snapshot.SyncWorkerAlertCooldowns),
	}
}

func hasHRISWebhookStateSnapshot(snapshot hrisWebhookStateSnapshot) bool {
	return len(snapshot.HRISWebhookReceipts) > 0 ||
		len(snapshot.HRISWebhookExecutions) > 0 ||
		len(snapshot.DueReceiptIDs) > 0 ||
		len(snapshot.QueuedReceiptExecutionIDs) > 0 ||
		len(snapshot.QueuedDLQReplayExecutionIDs) > 0
}

func (s *Service) syncQueuedHRISWebhookExecutionIndicesLocked() {
	s.queuedReceiptExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		s.hrisWebhookExecutions,
		HRISWebhookExecutionKindReceiptProcess,
	)
	s.queuedDLQReplayExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		s.hrisWebhookExecutions,
		HRISWebhookExecutionKindDLQReplay,
	)
}

func hasSyncWorkerAlertStateSnapshot(snapshot syncWorkerAlertStateSnapshot) bool {
	return len(snapshot.SyncWorkerAlertSubscriptions) > 0 ||
		len(snapshot.SyncWorkerAlertNotifications) > 0 ||
		len(snapshot.SyncWorkerAlertCooldowns) > 0 ||
		len(snapshot.SyncWorkerAlertInFlights) > 0
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


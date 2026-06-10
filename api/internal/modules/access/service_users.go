package access

import (
	"errors"
	"strings"
	"time"
)

func (s *Service) ListUsers(tenantID string) []AccessUser {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]AccessUser, 0, len(s.users))
	for i := range s.users {
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.users[i])
	}
	return items
}

func (s *Service) GetUser(tenantID, userID string) (AccessUser, error) {
	nextID := strings.TrimSpace(userID)
	if nextID == "" {
		return AccessUser{}, ErrUserNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.users {
		if s.users[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return AccessUser{}, ErrUserNotFound
		}
		return cloneAccessUser(s.users[i]), nil
	}
	return AccessUser{}, ErrUserNotFound
}

func (s *Service) FindUserByEmail(tenantID, email string) (AccessUser, bool) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, false
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.users {
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			continue
		}
		if normalizeEmail(s.users[i].Email) == nextEmail {
			return cloneAccessUser(s.users[i]), true
		}
	}
	return AccessUser{}, false
}

func (s *Service) CreateUser(tenantID, buildingID, name, email, role, status string, groupIDs []string) (AccessUser, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AccessUser{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return AccessUser{}, ErrUserNameRequired
	}

	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, ErrUserEmailRequired
	}

	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return AccessUser{}, err
	}

	nextRole := strings.ToLower(strings.TrimSpace(role))
	if nextRole == "" {
		nextRole = "employee"
	}

	id, err := accessID("usr_")
	if err != nil {
		return AccessUser{}, err
	}

	record := AccessUser{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: strings.TrimSpace(buildingID),
		Name:       nextName,
		Email:      nextEmail,
		Role:       nextRole,
		Status:     nextStatus,
		GroupIDs:   uniqueIDs(groupIDs),
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.users = append([]AccessUser{record}, s.users...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return AccessUser{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateUser(tenantID, userID, buildingID, name, email, role, status string, groupIDs []string) (AccessUser, error) {
	nextID := strings.TrimSpace(userID)
	if nextID == "" {
		return AccessUser{}, ErrUserNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return AccessUser{}, ErrUserNameRequired
	}

	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, ErrUserEmailRequired
	}

	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return AccessUser{}, err
	}

	nextRole := strings.ToLower(strings.TrimSpace(role))
	if nextRole == "" {
		nextRole = "employee"
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	nextGroupIDs := uniqueIDs(groupIDs)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.users {
		if s.users[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return AccessUser{}, ErrUserNotFound
		}

		s.users[i].BuildingID = nextBuildingID
		s.users[i].Name = nextName
		s.users[i].Email = nextEmail
		s.users[i].Role = nextRole
		s.users[i].Status = nextStatus
		s.users[i].GroupIDs = nextGroupIDs
		if err := s.persistLocked(); err != nil {
			return AccessUser{}, err
		}
		return cloneAccessUser(s.users[i]), nil
	}

	return AccessUser{}, ErrUserNotFound
}

func (s *Service) DeleteUser(tenantID, userID string) (AccessUser, error) {
	nextID := strings.TrimSpace(userID)
	if nextID == "" {
		return AccessUser{}, ErrUserNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.users {
		if s.users[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return AccessUser{}, ErrUserNotFound
		}
		foundIndex = i
		break
	}
	if foundIndex == -1 {
		return AccessUser{}, ErrUserNotFound
	}

	originalUsers := cloneAccessUsers(s.users)
	originalGroups := cloneUserGroups(s.userGroups)
	originalRoleAssignments := cloneRoleAssignments(s.roleAssignments)
	originalTeamMemberships := cloneTeamMemberships(s.teamMemberships)

	removed := cloneAccessUser(s.users[foundIndex])
	s.users = append(s.users[:foundIndex], s.users[foundIndex+1:]...)
	for i := range s.userGroups {
		s.userGroups[i].Members = removeID(s.userGroups[i].Members, nextID)
	}

	nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if strings.EqualFold(s.roleAssignments[i].AssigneeType, "User") && s.roleAssignments[i].AssigneeID == nextID {
			continue
		}
		nextAssignments = append(nextAssignments, s.roleAssignments[i])
	}
	s.roleAssignments = nextAssignments

	nextMemberships := make([]TeamMembership, 0, len(s.teamMemberships))
	for i := range s.teamMemberships {
		if strings.EqualFold(s.teamMemberships[i].MemberType, "User") && s.teamMemberships[i].MemberID == nextID {
			continue
		}
		nextMemberships = append(nextMemberships, s.teamMemberships[i])
	}
	s.teamMemberships = nextMemberships

	if err := s.persistLocked(); err != nil {
		s.users = originalUsers
		s.userGroups = originalGroups
		s.roleAssignments = originalRoleAssignments
		s.teamMemberships = originalTeamMemberships
		return AccessUser{}, err
	}
	return removed, nil
}

func (s *Service) BatchUpdateUserStatus(tenantID string, userIDs []string, status string) (BatchUserStatusResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return BatchUserStatusResult{}, ErrTenantIDRequired
	}
	if len(userIDs) == 0 {
		return BatchUserStatusResult{}, ErrUserIDsRequired
	}
	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return BatchUserStatusResult{}, err
	}

	targetSet := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if v := strings.TrimSpace(id); v != "" {
			targetSet[v] = struct{}{}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated, skipped, notFound int
	updatedIDs := make([]string, 0, len(targetSet))
	found := make(map[string]struct{}, len(targetSet))

	for i := range s.users {
		if s.users[i].TenantID != filterTenantID {
			continue
		}
		if _, ok := targetSet[s.users[i].ID]; !ok {
			continue
		}
		found[s.users[i].ID] = struct{}{}
		if strings.EqualFold(s.users[i].Status, nextStatus) {
			skipped++
			continue
		}
		s.users[i].Status = nextStatus
		updated++
		updatedIDs = append(updatedIDs, s.users[i].ID)
	}
	notFound = len(targetSet) - len(found)

	if updated > 0 {
		if err := s.persistLocked(); err != nil {
			return BatchUserStatusResult{}, err
		}
	}

	return BatchUserStatusResult{
		TenantID: filterTenantID,
		Status:   nextStatus,
		Updated:  updated,
		Skipped:  skipped,
		NotFound: notFound,
		UserIDs:  updatedIDs,
	}, nil
}

func (s *Service) BatchDeleteUsers(tenantID string, userIDs []string) (BatchUserDeleteResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return BatchUserDeleteResult{}, ErrTenantIDRequired
	}
	if len(userIDs) == 0 {
		return BatchUserDeleteResult{}, ErrUserIDsRequired
	}

	targetSet := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if v := strings.TrimSpace(id); v != "" {
			targetSet[v] = struct{}{}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted, notFound int
	deletedIDs := make([]string, 0, len(targetSet))
	nextUsers := make([]AccessUser, 0, len(s.users))
	removedIDs := make(map[string]struct{}, len(targetSet))

	for i := range s.users {
		if s.users[i].TenantID != filterTenantID {
			nextUsers = append(nextUsers, s.users[i])
			continue
		}
		if _, ok := targetSet[s.users[i].ID]; !ok {
			nextUsers = append(nextUsers, s.users[i])
			continue
		}
		removedIDs[s.users[i].ID] = struct{}{}
		deleted++
		deletedIDs = append(deletedIDs, s.users[i].ID)
	}
	notFound = len(targetSet) - deleted

	if deleted > 0 {
		s.users = nextUsers
		// clean up group memberships
		for i := range s.userGroups {
			for rid := range removedIDs {
				s.userGroups[i].Members = removeID(s.userGroups[i].Members, rid)
			}
		}
		// clean up role assignments
		nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
		for i := range s.roleAssignments {
			if strings.EqualFold(s.roleAssignments[i].AssigneeType, "User") {
				if _, ok := removedIDs[s.roleAssignments[i].AssigneeID]; ok {
					continue
				}
			}
			nextAssignments = append(nextAssignments, s.roleAssignments[i])
		}
		s.roleAssignments = nextAssignments
		// clean up team memberships
		nextMemberships := make([]TeamMembership, 0, len(s.teamMemberships))
		for i := range s.teamMemberships {
			if strings.EqualFold(s.teamMemberships[i].MemberType, "User") {
				if _, ok := removedIDs[s.teamMemberships[i].MemberID]; ok {
					continue
				}
			}
			nextMemberships = append(nextMemberships, s.teamMemberships[i])
		}
		s.teamMemberships = nextMemberships
		if err := s.persistLocked(); err != nil {
			return BatchUserDeleteResult{}, err
		}
	}

	return BatchUserDeleteResult{
		TenantID: filterTenantID,
		Deleted:  deleted,
		NotFound: notFound,
		UserIDs:  deletedIDs,
	}, nil
}

func (s *Service) BatchInviteUsers(tenantID string, userIDs []string, deliveryMethod string) (BatchUserInviteResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return BatchUserInviteResult{}, ErrTenantIDRequired
	}
	if len(userIDs) == 0 {
		return BatchUserInviteResult{}, ErrUserIDsRequired
	}
	nextMethod := strings.ToLower(strings.TrimSpace(deliveryMethod))
	if nextMethod == "" {
		nextMethod = "email"
	}
	if nextMethod != "email" && nextMethod != "email_qr" {
		return BatchUserInviteResult{}, ErrDeliveryMethodInvalid
	}

	targetSet := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if v := strings.TrimSpace(id); v != "" {
			targetSet[v] = struct{}{}
		}
	}

	var queued, skipped, notFound int
	queuedIDs := make([]string, 0, len(targetSet))

	s.mu.RLock()
	users := cloneAccessUsers(s.users)
	s.mu.RUnlock()

	found := make(map[string]struct{}, len(targetSet))
	for i := range users {
		if users[i].TenantID != filterTenantID {
			continue
		}
		if _, ok := targetSet[users[i].ID]; !ok {
			continue
		}
		found[users[i].ID] = struct{}{}
		if strings.EqualFold(users[i].Status, "suspended") || strings.EqualFold(users[i].Status, "disabled") {
			skipped++
			continue
		}
		_, _, err := s.CreateUserInvitationDelivery(filterTenantID, users[i].ID, nextMethod)
		if err != nil {
			skipped++
			continue
		}
		queued++
		queuedIDs = append(queuedIDs, users[i].ID)
	}
	notFound = len(targetSet) - len(found)

	return BatchUserInviteResult{
		TenantID: filterTenantID,
		Queued:   queued,
		Skipped:  skipped,
		NotFound: notFound,
		UserIDs:  queuedIDs,
	}, nil
}

func (s *Service) ExportUsersCSV(tenantID string) (string, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return "", ErrTenantIDRequired
	}

	s.mu.RLock()
	users := cloneAccessUsers(s.users)
	s.mu.RUnlock()

	var buf strings.Builder
	buf.WriteString("id,name,email,role,status,building_id,sync_source,created_at\n")
	for i := range users {
		if users[i].TenantID != filterTenantID {
			continue
		}
		buf.WriteString(csvEscape(users[i].ID))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Name))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Email))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Role))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Status))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].BuildingID))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].SyncSource))
		buf.WriteByte(',')
		buf.WriteString(users[i].CreatedAt.UTC().Format(time.RFC3339))
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

func (s *Service) ImportUsersCSV(tenantID string, records []UserImportRecord) (UserImportResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return UserImportResult{}, ErrTenantIDRequired
	}
	if len(records) == 0 {
		return UserImportResult{}, ErrUsersImportRecordsRequired
	}

	var created, updated, skipped, errCount int

	for i := range records {
		email := normalizeEmail(records[i].Email)
		if email == "" {
			errCount++
			continue
		}
		name := strings.TrimSpace(records[i].Name)
		if name == "" {
			name = email
		}
		role := strings.ToLower(strings.TrimSpace(records[i].Role))
		if role == "" {
			role = "employee"
		}
		status := strings.ToLower(strings.TrimSpace(records[i].Status))
		if status == "" {
			status = "active"
		}
		buildingID := strings.TrimSpace(records[i].BuildingID)

		_, isNew, err := s.UpsertUserByEmail(
			filterTenantID,
			buildingID,
			name,
			email,
			role,
			status,
			nil,
		)
		if err != nil {
			errCount++
			continue
		}
		if isNew {
			created++
		} else {
			updated++
		}
	}

	return UserImportResult{
		TenantID: filterTenantID,
		Created:  created,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errCount,
	}, nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func (s *Service) ListUserInvitationDeliveries(tenantID, userID string) []UserInvitationDelivery {
	filterTenantID := strings.TrimSpace(tenantID)
	filterUserID := strings.TrimSpace(userID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]UserInvitationDelivery, 0, len(s.userInvitationDeliveries))
	for i := range s.userInvitationDeliveries {
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		if filterUserID != "" && s.userInvitationDeliveries[i].UserID != filterUserID {
			continue
		}
		items = append(items, cloneUserInvitationDelivery(s.userInvitationDeliveries[i]))
	}
	return items
}

func (s *Service) CreateUserInvitationDelivery(tenantID, userID, deliveryMethod string) (UserInvitationDelivery, AccessUser, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserNotFound
	}
	nextMethod, err := normalizeUserInvitationDeliveryMethod(deliveryMethod)
	if err != nil {
		return UserInvitationDelivery{}, AccessUser{}, err
	}
	filterTenantID := strings.TrimSpace(tenantID)

	id, err := accessID("user_invite_")
	if err != nil {
		return UserInvitationDelivery{}, AccessUser{}, err
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	userIndex := -1
	for i := range s.users {
		if s.users[i].ID != nextUserID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return UserInvitationDelivery{}, AccessUser{}, ErrUserNotFound
		}
		userIndex = i
		break
	}
	if userIndex < 0 {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserNotFound
	}

	user := cloneAccessUser(s.users[userIndex])
	record := UserInvitationDelivery{
		ID:             id,
		ResourceType:   "UserInvitationDelivery",
		TenantID:       user.TenantID,
		UserID:         user.ID,
		Email:          user.Email,
		PlaceID:        user.BuildingID,
		DeliveryMethod: nextMethod,
		Status:         "queued",
		QueuedAt:       now,
		UpdatedAt:      now,
	}
	s.userInvitationDeliveries = append([]UserInvitationDelivery{record}, s.userInvitationDeliveries...)
	if err := s.persistLocked(); err != nil {
		return UserInvitationDelivery{}, AccessUser{}, err
	}
	return cloneUserInvitationDelivery(record), user, nil
}

func (s *Service) RecordUserInvitationReceipt(
	tenantID, userID, deliveryID, status, provider, providerDeliveryID, providerError string,
	retryable bool,
) (UserInvitationDelivery, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextUserID := strings.TrimSpace(userID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextUserID == "" {
		return UserInvitationDelivery{}, ErrUserNotFound
	}
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}
	nextStatus, err := normalizeUserInvitationStatus(status)
	if err != nil {
		return UserInvitationDelivery{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	deliveryIndex := -1
	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID || s.userInvitationDeliveries[i].UserID != nextUserID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
		}
		deliveryIndex = i
		break
	}
	if deliveryIndex < 0 {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}

	now := time.Now().UTC()
	s.userInvitationDeliveries[deliveryIndex].Status = nextStatus
	s.userInvitationDeliveries[deliveryIndex].Provider = strings.TrimSpace(provider)
	s.userInvitationDeliveries[deliveryIndex].ProviderDeliveryID = strings.TrimSpace(providerDeliveryID)
	s.userInvitationDeliveries[deliveryIndex].ProviderError = strings.TrimSpace(providerError)
	s.userInvitationDeliveries[deliveryIndex].Retryable = retryable
	s.userInvitationDeliveries[deliveryIndex].UpdatedAt = now
	if nextStatus == "sent" {
		deliveredAt := now
		s.userInvitationDeliveries[deliveryIndex].DeliveredAt = &deliveredAt
	}
	if err := s.persistLocked(); err != nil {
		return UserInvitationDelivery{}, err
	}
	return cloneUserInvitationDelivery(s.userInvitationDeliveries[deliveryIndex]), nil
}

// --- Independent invitation resource methods ---

func (s *Service) GetInvitationDelivery(tenantID, deliveryID string) (UserInvitationDelivery, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		return cloneUserInvitationDelivery(s.userInvitationDeliveries[i]), nil
	}
	return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
}

func (s *Service) CancelInvitationDelivery(tenantID, deliveryID string) (UserInvitationDelivery, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		if s.userInvitationDeliveries[i].Status == "sent" {
			return UserInvitationDelivery{}, errors.New("cannot cancel a delivered invitation")
		}
		if s.userInvitationDeliveries[i].Status == "cancelled" {
			return cloneUserInvitationDelivery(s.userInvitationDeliveries[i]), nil
		}
		s.userInvitationDeliveries[i].Status = "cancelled"
		s.userInvitationDeliveries[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserInvitationDelivery{}, err
		}
		return cloneUserInvitationDelivery(s.userInvitationDeliveries[i]), nil
	}
	return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
}

func (s *Service) ResendInvitationDelivery(tenantID, deliveryID string) (UserInvitationDelivery, AccessUser, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserInvitationDeliveryNotFound
	}

	s.mu.RLock()
	var original *UserInvitationDelivery
	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		d := cloneUserInvitationDelivery(s.userInvitationDeliveries[i])
		original = &d
		break
	}
	s.mu.RUnlock()

	if original == nil {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserInvitationDeliveryNotFound
	}
	return s.CreateUserInvitationDelivery(original.TenantID, original.UserID, original.DeliveryMethod)
}

func (s *Service) UpsertUserByEmail(
	tenantID, buildingID, name, email, role, status string,
	groupIDs []string,
) (AccessUser, bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AccessUser{}, false, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return AccessUser{}, false, ErrUserNameRequired
	}

	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, false, ErrUserEmailRequired
	}

	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return AccessUser{}, false, err
	}

	nextRole := strings.ToLower(strings.TrimSpace(role))
	if nextRole == "" {
		nextRole = "employee"
	}

	nextBuildingID := strings.TrimSpace(buildingID)
	nextGroupIDs := uniqueIDs(groupIDs)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.users {
		if s.users[i].TenantID != nextTenantID {
			continue
		}
		if normalizeEmail(s.users[i].Email) != nextEmail {
			continue
		}

		s.users[i].BuildingID = nextBuildingID
		s.users[i].Name = nextName
		s.users[i].Email = nextEmail
		s.users[i].Role = nextRole
		s.users[i].Status = nextStatus
		s.users[i].GroupIDs = nextGroupIDs
		if err := s.persistLocked(); err != nil {
			return AccessUser{}, false, err
		}

		return s.users[i], false, nil
	}

	id, err := accessID("usr_")
	if err != nil {
		return AccessUser{}, false, err
	}

	record := AccessUser{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: nextBuildingID,
		Name:       nextName,
		Email:      nextEmail,
		Role:       nextRole,
		Status:     nextStatus,
		GroupIDs:   nextGroupIDs,
		CreatedAt:  time.Now().UTC(),
	}
	s.users = append([]AccessUser{record}, s.users...)
	if err := s.persistLocked(); err != nil {
		return AccessUser{}, false, err
	}

	return record, true, nil
}

func (s *Service) UpsertUsersByEmail(
	tenantID string,
	inputs []BatchUpsertUserByEmailInput,
) (created, updated, rejected int, err error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return 0, 0, 0, ErrTenantIDRequired
	}
	if len(inputs) == 0 {
		return 0, 0, 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	originalUsers := cloneAccessUsers(s.users)
	existingByEmail := make(map[string]int)
	existingBySyncRef := make(map[string]int)
	for i := range s.users {
		if s.users[i].TenantID != nextTenantID {
			continue
		}
		existingByEmail[normalizeEmail(s.users[i].Email)] = i
		if syncKey := accessSyncIdentityKey(s.users[i].SyncSource, s.users[i].SyncRef); syncKey != "" {
			existingBySyncRef[syncKey] = i
		}
	}

	createdRecords := make([]AccessUser, 0, len(inputs))
	createdByEmail := make(map[string]int)
	createdBySyncRef := make(map[string]int)

	for i := range inputs {
		nextName := strings.TrimSpace(inputs[i].Name)
		nextEmail := normalizeEmail(inputs[i].Email)
		if nextName == "" || nextEmail == "" {
			rejected++
			continue
		}

		nextStatus, normalizeErr := normalizeUserStatus(inputs[i].Status)
		if normalizeErr != nil {
			rejected++
			continue
		}

		nextRole := strings.ToLower(strings.TrimSpace(inputs[i].Role))
		if nextRole == "" {
			nextRole = "employee"
		}
		nextBuildingID := strings.TrimSpace(inputs[i].BuildingID)
		nextGroupIDs := uniqueIDs(inputs[i].GroupIDs)
		nextSyncSource := normalizeSyncSource(inputs[i].SyncSource)
		nextSyncRef := strings.TrimSpace(inputs[i].SyncRef)
		if (nextSyncSource == "") != (nextSyncRef == "") {
			rejected++
			continue
		}
		nextSyncKey := accessSyncIdentityKey(nextSyncSource, nextSyncRef)

		existingSyncIndex, hasExistingSyncMatch := -1, false
		if nextSyncKey != "" {
			existingSyncIndex, hasExistingSyncMatch = existingBySyncRef[nextSyncKey]
		}
		createdSyncIndex, hasCreatedSyncMatch := -1, false
		if nextSyncKey != "" {
			createdSyncIndex, hasCreatedSyncMatch = createdBySyncRef[nextSyncKey]
		}

		existingEmailIndex, hasExistingEmailMatch := existingByEmail[nextEmail]
		createdEmailIndex, hasCreatedEmailMatch := createdByEmail[nextEmail]

		hasSyncMatch := hasExistingSyncMatch || hasCreatedSyncMatch
		hasEmailMatch := hasExistingEmailMatch || hasCreatedEmailMatch

		targetIsCreated := false
		targetIndex := -1
		switch {
		case hasSyncMatch && hasEmailMatch:
			switch {
			case hasExistingSyncMatch && hasExistingEmailMatch && existingSyncIndex == existingEmailIndex:
				targetIndex = existingSyncIndex
			case hasCreatedSyncMatch && hasCreatedEmailMatch && createdSyncIndex == createdEmailIndex:
				targetIndex = createdSyncIndex
				targetIsCreated = true
			default:
				rejected++
				continue
			}
		case hasSyncMatch:
			if hasExistingSyncMatch {
				targetIndex = existingSyncIndex
			} else {
				targetIndex = createdSyncIndex
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
			currentSyncKey := accessSyncIdentityKey(createdRecords[targetIndex].SyncSource, createdRecords[targetIndex].SyncRef)
			if nextSyncKey != "" && currentSyncKey != "" && nextSyncKey != currentSyncKey {
				rejected++
				continue
			}
			previousEmail := normalizeEmail(createdRecords[targetIndex].Email)
			createdRecords[targetIndex].BuildingID = nextBuildingID
			createdRecords[targetIndex].Name = nextName
			createdRecords[targetIndex].Email = nextEmail
			createdRecords[targetIndex].Role = nextRole
			createdRecords[targetIndex].Status = nextStatus
			createdRecords[targetIndex].GroupIDs = nextGroupIDs
			if nextSyncKey != "" {
				createdRecords[targetIndex].SyncSource = nextSyncSource
				createdRecords[targetIndex].SyncRef = nextSyncRef
				createdBySyncRef[nextSyncKey] = targetIndex
			}
			if previousEmail != nextEmail {
				if previousIndex, exists := createdByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(createdByEmail, previousEmail)
				}
			}
			createdByEmail[nextEmail] = targetIndex
			continue
		}

		if targetIndex >= 0 {
			currentSyncKey := accessSyncIdentityKey(s.users[targetIndex].SyncSource, s.users[targetIndex].SyncRef)
			if nextSyncKey != "" && currentSyncKey != "" && nextSyncKey != currentSyncKey {
				rejected++
				continue
			}
			previousEmail := normalizeEmail(s.users[targetIndex].Email)
			s.users[targetIndex].BuildingID = nextBuildingID
			s.users[targetIndex].Name = nextName
			s.users[targetIndex].Email = nextEmail
			s.users[targetIndex].Role = nextRole
			s.users[targetIndex].Status = nextStatus
			s.users[targetIndex].GroupIDs = nextGroupIDs
			if nextSyncKey != "" {
				s.users[targetIndex].SyncSource = nextSyncSource
				s.users[targetIndex].SyncRef = nextSyncRef
				existingBySyncRef[nextSyncKey] = targetIndex
			}
			if previousEmail != nextEmail {
				if previousIndex, exists := existingByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(existingByEmail, previousEmail)
				}
			}
			existingByEmail[nextEmail] = targetIndex
			updated++
			continue
		}

		id, idErr := accessID("usr_")
		if idErr != nil {
			s.users = originalUsers
			return 0, 0, 0, idErr
		}

		record := AccessUser{
			ID:         id,
			TenantID:   nextTenantID,
			BuildingID: nextBuildingID,
			Name:       nextName,
			Email:      nextEmail,
			Role:       nextRole,
			Status:     nextStatus,
			GroupIDs:   nextGroupIDs,
			SyncSource: nextSyncSource,
			SyncRef:    nextSyncRef,
			CreatedAt:  time.Now().UTC(),
		}
		createdIndex := len(createdRecords)
		createdByEmail[nextEmail] = createdIndex
		if nextSyncKey != "" {
			createdBySyncRef[nextSyncKey] = createdIndex
		}
		createdRecords = append(createdRecords, record)
		created++
	}

	if len(createdRecords) > 0 {
		s.users = append(createdRecords, s.users...)
	}

	if created > 0 || updated > 0 {
		if persistErr := s.persistLocked(); persistErr != nil {
			s.users = originalUsers
			return 0, 0, 0, persistErr
		}
	}

	return created, updated, rejected, nil
}

func (s *Service) ListUserGroups(tenantID string) []UserGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]UserGroup, 0, len(s.userGroups))
	for i := range s.userGroups {
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneUserGroup(s.userGroups[i]))
	}
	return items
}

func (s *Service) GetUserGroup(tenantID, groupID string) (UserGroup, error) {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return UserGroup{}, ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return UserGroup{}, ErrUserGroupNotFound
		}
		return cloneUserGroup(s.userGroups[i]), nil
	}

	return UserGroup{}, ErrUserGroupNotFound
}

func (s *Service) ListTeams(tenantID string) []Team {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Team, 0, len(s.teams))
	for i := range s.teams {
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.teams[i])
	}
	return items
}

func (s *Service) GetTeam(tenantID, teamID string) (Team, error) {
	nextID := strings.TrimSpace(teamID)
	if nextID == "" {
		return Team{}, ErrTeamNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.teams {
		if s.teams[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			return Team{}, ErrTeamNotFound
		}
		return s.teams[i], nil
	}

	return Team{}, ErrTeamNotFound
}

func (s *Service) ListTeamMemberships(tenantID string) []TeamMembership {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]TeamMembership, 0, len(s.teamMemberships))
	for i := range s.teamMemberships {
		if filterTenantID != "" && s.teamMemberships[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.teamMemberships[i])
	}
	return items
}

func (s *Service) CreateTeam(tenantID, name, scope, placeID, description, source string) (Team, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Team{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Team{}, ErrTeamNameRequired
	}
	nextScope, err := normalizeTeamScope(scope, placeID)
	if err != nil {
		return Team{}, err
	}
	nextPlaceID := strings.TrimSpace(placeID)
	if nextScope == "organization" {
		nextPlaceID = ""
	}
	id, err := accessID("team_")
	if err != nil {
		return Team{}, err
	}
	now := time.Now().UTC()
	record := Team{
		ID:           id,
		ResourceType: "Team",
		TenantID:     nextTenantID,
		Name:         nextName,
		Scope:        nextScope,
		PlaceID:      nextPlaceID,
		Description:  strings.TrimSpace(description),
		Source:       firstNonEmpty(strings.TrimSpace(source), "Manual"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	s.teams = append([]Team{record}, s.teams...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Team{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateTeam(tenantID, teamID, name, scope, placeID, description, source string) (Team, error) {
	nextID := strings.TrimSpace(teamID)
	if nextID == "" {
		return Team{}, ErrTeamNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Team{}, ErrTeamNameRequired
	}
	nextScope, err := normalizeTeamScope(scope, placeID)
	if err != nil {
		return Team{}, err
	}
	nextPlaceID := strings.TrimSpace(placeID)
	if nextScope == "organization" {
		nextPlaceID = ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.teams {
		if s.teams[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			return Team{}, ErrTeamNotFound
		}
		s.teams[i].Name = nextName
		s.teams[i].Scope = nextScope
		s.teams[i].PlaceID = nextPlaceID
		s.teams[i].Description = strings.TrimSpace(description)
		s.teams[i].Source = firstNonEmpty(strings.TrimSpace(source), s.teams[i].Source, "Manual")
		s.teams[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Team{}, err
		}
		return s.teams[i], nil
	}

	return Team{}, ErrTeamNotFound
}

func (s *Service) DeleteTeam(tenantID, teamID string) error {
	nextID := strings.TrimSpace(teamID)
	if nextID == "" {
		return ErrTeamNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.teams {
		if s.teams[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			return ErrTeamNotFound
		}
		foundIndex = i
		break
	}
	if foundIndex < 0 {
		return ErrTeamNotFound
	}

	originalTeams := cloneTeams(s.teams)
	originalMemberships := cloneTeamMemberships(s.teamMemberships)
	originalRoleAssignments := cloneRoleAssignments(s.roleAssignments)

	s.teams = append(s.teams[:foundIndex], s.teams[foundIndex+1:]...)
	nextMemberships := make([]TeamMembership, 0, len(s.teamMemberships))
	for i := range s.teamMemberships {
		if s.teamMemberships[i].TeamID == nextID {
			continue
		}
		nextMemberships = append(nextMemberships, s.teamMemberships[i])
	}
	s.teamMemberships = nextMemberships
	nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if s.roleAssignments[i].AssigneeType == "Team" && s.roleAssignments[i].AssigneeID == nextID {
			continue
		}
		nextAssignments = append(nextAssignments, s.roleAssignments[i])
	}
	s.roleAssignments = nextAssignments

	if err := s.persistLocked(); err != nil {
		s.teams = originalTeams
		s.teamMemberships = originalMemberships
		s.roleAssignments = originalRoleAssignments
		return err
	}

	return nil
}

func (s *Service) CreateTeamMembership(tenantID, teamID, memberType, memberID, memberEmail, memberName, source string) (TeamMembership, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return TeamMembership{}, ErrTenantIDRequired
	}
	nextTeamID := strings.TrimSpace(teamID)
	if nextTeamID == "" {
		return TeamMembership{}, ErrTeamIDRequired
	}
	nextMemberType, err := normalizeTeamMemberType(memberType)
	if err != nil {
		return TeamMembership{}, err
	}
	nextMemberID := strings.TrimSpace(memberID)
	if nextMemberID == "" {
		return TeamMembership{}, ErrTeamMemberIDRequired
	}
	id, err := accessID("tm_")
	if err != nil {
		return TeamMembership{}, err
	}
	now := time.Now().UTC()
	record := TeamMembership{
		ID:           id,
		ResourceType: "TeamMembership",
		TenantID:     nextTenantID,
		TeamID:       nextTeamID,
		MemberType:   nextMemberType,
		MemberID:     nextMemberID,
		MemberEmail:  strings.TrimSpace(memberEmail),
		MemberName:   strings.TrimSpace(memberName),
		Source:       firstNonEmpty(strings.TrimSpace(source), "Manual"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	teamFound := false
	for i := range s.teams {
		if s.teams[i].ID == nextTeamID && s.teams[i].TenantID == nextTenantID {
			teamFound = true
			break
		}
	}
	if !teamFound {
		return TeamMembership{}, ErrTeamNotFound
	}
	for i := range s.teamMemberships {
		if s.teamMemberships[i].TenantID == nextTenantID &&
			s.teamMemberships[i].TeamID == nextTeamID &&
			s.teamMemberships[i].MemberType == nextMemberType &&
			s.teamMemberships[i].MemberID == nextMemberID {
			s.teamMemberships[i].MemberEmail = record.MemberEmail
			s.teamMemberships[i].MemberName = record.MemberName
			s.teamMemberships[i].Source = record.Source
			s.teamMemberships[i].UpdatedAt = now
			if err := s.persistLocked(); err != nil {
				return TeamMembership{}, err
			}
			return s.teamMemberships[i], nil
		}
	}

	s.teamMemberships = append([]TeamMembership{record}, s.teamMemberships...)
	if err := s.persistLocked(); err != nil {
		return TeamMembership{}, err
	}
	return record, nil
}

func (s *Service) DeleteTeamMembership(tenantID, membershipID string) error {
	nextID := strings.TrimSpace(membershipID)
	if nextID == "" {
		return ErrTeamMembershipNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.teamMemberships {
		if s.teamMemberships[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teamMemberships[i].TenantID != filterTenantID {
			return ErrTeamMembershipNotFound
		}
		original := cloneTeamMemberships(s.teamMemberships)
		s.teamMemberships = append(s.teamMemberships[:i], s.teamMemberships[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.teamMemberships = original
			return err
		}
		return nil
	}

	return ErrTeamMembershipNotFound
}

func (s *Service) CreateUserGroup(tenantID, buildingID, name, description string, members []string) (UserGroup, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return UserGroup{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return UserGroup{}, ErrUserGroupNameRequired
	}

	id, err := accessID("ug_")
	if err != nil {
		return UserGroup{}, err
	}

	now := time.Now().UTC()
	record := UserGroup{
		ID:                        id,
		ResourceType:              "Group",
		TenantID:                  nextTenantID,
		BuildingID:                strings.TrimSpace(buildingID),
		PlaceID:                   strings.TrimSpace(buildingID),
		Name:                      nextName,
		Description:               strings.TrimSpace(description),
		LoginEnabled:              true,
		GeofenceRestrictionRadius: 150,
		Members:                   uniqueIDs(members),
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	s.mu.Lock()
	s.userGroups = append([]UserGroup{record}, s.userGroups...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return UserGroup{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateUserGroup(tenantID, groupID, buildingID, name, description string, members []string) (UserGroup, error) {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return UserGroup{}, ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return UserGroup{}, ErrUserGroupNameRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return UserGroup{}, ErrUserGroupNotFound
		}

		s.userGroups[i].Name = nextName
		s.userGroups[i].BuildingID = strings.TrimSpace(buildingID)
		s.userGroups[i].PlaceID = strings.TrimSpace(buildingID)
		s.userGroups[i].Description = strings.TrimSpace(description)
		s.userGroups[i].Members = uniqueIDs(members)
		s.userGroups[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserGroup{}, err
		}
		return cloneUserGroup(s.userGroups[i]), nil
	}

	return UserGroup{}, ErrUserGroupNotFound
}

func (s *Service) DeleteUserGroup(tenantID, groupID string) error {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return ErrUserGroupNotFound
		}
		foundIndex = i
		break
	}
	if foundIndex == -1 {
		return ErrUserGroupNotFound
	}

	originalGroups := cloneUserGroups(s.userGroups)
	originalUsers := cloneAccessUsers(s.users)
	originalRoleAssignments := cloneRoleAssignments(s.roleAssignments)
	originalGroupLinks := cloneGroupLinks(s.groupLinks)

	s.userGroups = append(s.userGroups[:foundIndex], s.userGroups[foundIndex+1:]...)
	for i := range s.users {
		s.users[i].GroupIDs = removeID(s.users[i].GroupIDs, nextID)
	}
	nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if s.roleAssignments[i].AppliesToType == "Group" && s.roleAssignments[i].AppliesToID == nextID {
			continue
		}
		nextAssignments = append(nextAssignments, s.roleAssignments[i])
	}
	s.roleAssignments = nextAssignments
	nextGroupLinks := make([]GroupLink, 0, len(s.groupLinks))
	for i := range s.groupLinks {
		if s.groupLinks[i].GroupID == nextID {
			continue
		}
		nextGroupLinks = append(nextGroupLinks, s.groupLinks[i])
	}
	s.groupLinks = nextGroupLinks

	if err := s.persistLocked(); err != nil {
		s.userGroups = originalGroups
		s.users = originalUsers
		s.roleAssignments = originalRoleAssignments
		s.groupLinks = originalGroupLinks
		return err
	}

	return nil
}

func (s *Service) UpdateUserGroupRestrictions(tenantID, groupID string, input UserGroupRestrictionsInput) (UserGroup, error) {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return UserGroup{}, ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return UserGroup{}, ErrUserGroupNotFound
		}
		applyUserGroupDefaults(&s.userGroups[i])
		if input.LoginEnabled != nil {
			s.userGroups[i].LoginEnabled = *input.LoginEnabled
		}
		if input.GeofenceRestrictionEnabled != nil {
			s.userGroups[i].GeofenceRestrictionEnabled = *input.GeofenceRestrictionEnabled
		}
		if input.GeofenceRestrictionRadius != nil {
			s.userGroups[i].GeofenceRestrictionRadius = *input.GeofenceRestrictionRadius
		}
		if input.GeofenceRestrictionLatitude != nil {
			s.userGroups[i].GeofenceRestrictionLatitude = *input.GeofenceRestrictionLatitude
		}
		if input.GeofenceRestrictionLongitude != nil {
			s.userGroups[i].GeofenceRestrictionLongitude = *input.GeofenceRestrictionLongitude
		}
		if input.PrimaryDeviceRestrictionEnabled != nil {
			s.userGroups[i].PrimaryDeviceRestrictionEnabled = *input.PrimaryDeviceRestrictionEnabled
		}
		if input.ManagedDeviceRestrictionEnabled != nil {
			s.userGroups[i].ManagedDeviceRestrictionEnabled = *input.ManagedDeviceRestrictionEnabled
		}
		if input.ReaderRestrictionEnabled != nil {
			s.userGroups[i].ReaderRestrictionEnabled = *input.ReaderRestrictionEnabled
		}
		if input.TimeRestrictionEnabled != nil {
			s.userGroups[i].TimeRestrictionEnabled = *input.TimeRestrictionEnabled
		}
		if input.TapToAccessRestrictionEnabled != nil {
			s.userGroups[i].TapToAccessRestrictionEnabled = *input.TapToAccessRestrictionEnabled
		}
		if input.TimeRestrictionTimeZone != nil {
			s.userGroups[i].TimeRestrictionTimeZone = strings.TrimSpace(*input.TimeRestrictionTimeZone)
		}
		s.userGroups[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserGroup{}, err
		}
		return cloneUserGroup(s.userGroups[i]), nil
	}

	return UserGroup{}, ErrUserGroupNotFound
}

func (s *Service) ListGroupLinks(tenantID string) []GroupLink {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]GroupLink, 0, len(s.groupLinks))
	for i := range s.groupLinks {
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneGroupLink(s.groupLinks[i]))
	}
	return items
}

func (s *Service) GetGroupLink(tenantID, groupLinkID string) (GroupLink, error) {
	nextID := strings.TrimSpace(groupLinkID)
	if nextID == "" {
		return GroupLink{}, ErrGroupLinkNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.groupLinks {
		if s.groupLinks[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			return GroupLink{}, ErrGroupLinkNotFound
		}
		return cloneGroupLink(s.groupLinks[i]), nil
	}
	return GroupLink{}, ErrGroupLinkNotFound
}

func (s *Service) CreateGroupLink(input GroupLinkInput) (GroupLink, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return GroupLink{}, ErrTenantIDRequired
	}
	nextGroupID := strings.TrimSpace(input.GroupID)
	if nextGroupID == "" {
		return GroupLink{}, ErrGroupIDRequired
	}
	nextName := strings.TrimSpace(input.Name)
	if nextName == "" {
		return GroupLink{}, ErrGroupLinkNameRequired
	}
	qrType, err := normalizeGroupLinkQRCodeType(input.QuickResponseCodeType)
	if err != nil {
		return GroupLink{}, err
	}
	id, err := accessID("gl_")
	if err != nil {
		return GroupLink{}, err
	}
	secret, err := accessID("gls_")
	if err != nil {
		return GroupLink{}, err
	}
	qrToken := ""
	if qrType != "" {
		qrToken, err = accessID("glq_")
		if err != nil {
			return GroupLink{}, err
		}
	}
	linkEnabled := true
	if input.LinkEnabled != nil {
		linkEnabled = *input.LinkEnabled
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	groupName := ""
	for i := range s.userGroups {
		if s.userGroups[i].ID != nextGroupID {
			continue
		}
		if s.userGroups[i].TenantID != nextTenantID {
			return GroupLink{}, ErrUserGroupNotFound
		}
		groupName = s.userGroups[i].Name
		break
	}
	if groupName == "" {
		return GroupLink{}, ErrUserGroupNotFound
	}

	record := GroupLink{
		ID:                     id,
		ResourceType:           "GroupLink",
		TenantID:               nextTenantID,
		GroupID:                nextGroupID,
		GroupName:              groupName,
		Name:                   nextName,
		Email:                  normalizeEmail(input.Email),
		Phone:                  strings.TrimSpace(input.Phone),
		LinkEnabled:            linkEnabled,
		QuickResponseCodeType:  qrType,
		ValidFrom:              strings.TrimSpace(input.ValidFrom),
		ValidUntil:             strings.TrimSpace(input.ValidUntil),
		CreatedByType:          normalizeGroupLinkCreatorType(input.CreatedByType),
		CreatedByID:            strings.TrimSpace(input.CreatedByID),
		CreatedByEmail:         normalizeEmail(input.CreatedByEmail),
		CreatedByName:          strings.TrimSpace(input.CreatedByName),
		IssuedByID:             strings.TrimSpace(input.CreatedByID),
		Secret:                 secret,
		QuickResponseCodeToken: qrToken,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	s.groupLinks = append([]GroupLink{record}, s.groupLinks...)
	if err := s.persistLocked(); err != nil {
		return GroupLink{}, err
	}
	return cloneGroupLink(record), nil
}

func (s *Service) UpdateGroupLink(tenantID, groupLinkID string, input GroupLinkUpdateInput) (GroupLink, error) {
	nextID := strings.TrimSpace(groupLinkID)
	if nextID == "" {
		return GroupLink{}, ErrGroupLinkNotFound
	}
	filterTenantID := firstNonEmpty(tenantID, input.TenantID)

	var nextGroupID string
	if input.GroupID != nil {
		nextGroupID = strings.TrimSpace(*input.GroupID)
		if nextGroupID == "" {
			return GroupLink{}, ErrGroupIDRequired
		}
	}
	var nextName string
	if input.Name != nil {
		nextName = strings.TrimSpace(*input.Name)
		if nextName == "" {
			return GroupLink{}, ErrGroupLinkNameRequired
		}
	}
	var nextQRCodeType string
	if input.QuickResponseCodeType != nil {
		qrType, err := normalizeGroupLinkQRCodeType(*input.QuickResponseCodeType)
		if err != nil {
			return GroupLink{}, err
		}
		nextQRCodeType = qrType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.groupLinks {
		if s.groupLinks[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			return GroupLink{}, ErrGroupLinkNotFound
		}

		original := cloneGroupLinks(s.groupLinks)
		record := s.groupLinks[i]
		if input.GroupID != nil {
			groupName := ""
			for j := range s.userGroups {
				if s.userGroups[j].ID != nextGroupID {
					continue
				}
				if s.userGroups[j].TenantID != record.TenantID {
					return GroupLink{}, ErrUserGroupNotFound
				}
				groupName = s.userGroups[j].Name
				break
			}
			if groupName == "" {
				return GroupLink{}, ErrUserGroupNotFound
			}
			record.GroupID = nextGroupID
			record.GroupName = groupName
		}
		if input.Name != nil {
			record.Name = nextName
		}
		if input.Email != nil {
			record.Email = normalizeEmail(*input.Email)
		}
		if input.Phone != nil {
			record.Phone = strings.TrimSpace(*input.Phone)
		}
		if input.LinkEnabled != nil {
			record.LinkEnabled = *input.LinkEnabled
		}
		if input.QuickResponseCodeType != nil {
			record.QuickResponseCodeType = nextQRCodeType
			if nextQRCodeType == "" {
				record.QuickResponseCodeToken = ""
				record.QuickResponseCodeImage = ""
			} else if record.QuickResponseCodeToken == "" {
				qrToken, err := accessID("glq_")
				if err != nil {
					return GroupLink{}, err
				}
				record.QuickResponseCodeToken = qrToken
			}
		}
		if input.ValidFrom != nil {
			record.ValidFrom = strings.TrimSpace(*input.ValidFrom)
		}
		if input.ValidUntil != nil {
			record.ValidUntil = strings.TrimSpace(*input.ValidUntil)
		}
		record.ResourceType = "GroupLink"
		record.UpdatedAt = time.Now().UTC()
		s.groupLinks[i] = record
		if err := s.persistLocked(); err != nil {
			s.groupLinks = original
			return GroupLink{}, err
		}
		return cloneGroupLink(record), nil
	}
	return GroupLink{}, ErrGroupLinkNotFound
}

func (s *Service) DeleteGroupLink(tenantID, groupLinkID string) error {
	nextID := strings.TrimSpace(groupLinkID)
	if nextID == "" {
		return ErrGroupLinkNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.groupLinks {
		if s.groupLinks[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			return ErrGroupLinkNotFound
		}
		original := cloneGroupLinks(s.groupLinks)
		s.groupLinks = append(s.groupLinks[:i], s.groupLinks[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.groupLinks = original
			return err
		}
		return nil
	}
	return ErrGroupLinkNotFound
}

func (s *Service) VerifyGroupLinkToken(tenantID, token string, verifiedAt time.Time) (GroupLink, error) {
	nextToken := strings.TrimSpace(token)
	if nextToken == "" {
		return GroupLink{}, ErrGroupLinkTokenRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
	} else {
		verifiedAt = verifiedAt.UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.groupLinks {
		record := s.groupLinks[i]
		if filterTenantID != "" && record.TenantID != filterTenantID {
			continue
		}
		if record.Secret != nextToken && record.QuickResponseCodeToken != nextToken {
			continue
		}
		if !record.LinkEnabled {
			return GroupLink{}, ErrGroupLinkDisabled
		}
		if validFrom, ok, err := parseGroupLinkInstant(record.ValidFrom); err != nil {
			return GroupLink{}, err
		} else if ok && verifiedAt.Before(validFrom) {
			return GroupLink{}, ErrGroupLinkNotYetValid
		}
		if validUntil, ok, err := parseGroupLinkInstant(record.ValidUntil); err != nil {
			return GroupLink{}, err
		} else if ok && verifiedAt.After(validUntil) {
			return GroupLink{}, ErrGroupLinkExpired
		}

		original := cloneGroupLinks(s.groupLinks)
		record.LastUsedAt = verifiedAt.Format(time.RFC3339)
		record.ResourceType = "GroupLink"
		record.UpdatedAt = verifiedAt
		s.groupLinks[i] = record
		if err := s.persistLocked(); err != nil {
			s.groupLinks = original
			return GroupLink{}, err
		}
		return cloneGroupLink(record), nil
	}
	return GroupLink{}, ErrGroupLinkTokenInvalid
}


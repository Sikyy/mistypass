package access

import "testing"

func TestUpsertUsersByEmailSyncIdentityKeepsSingleUserOnEmailChange(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_access_sync_identity"

	created, updated, rejected, err := svc.UpsertUsersByEmail(
		tenantID,
		[]BatchUpsertUserByEmailInput{
			{
				Name:       "Sync Identity User",
				Email:      "sync.identity.one@example.com",
				Status:     "active",
				SyncSource: "enterprise_employee_sync",
				SyncRef:    "external_id:hr-1001",
			},
		},
	)
	if err != nil {
		t.Fatalf("first upsert should succeed: %v", err)
	}
	if created != 1 || updated != 0 || rejected != 0 {
		t.Fatalf("unexpected first upsert counters: created=%d updated=%d rejected=%d", created, updated, rejected)
	}

	created, updated, rejected, err = svc.UpsertUsersByEmail(
		tenantID,
		[]BatchUpsertUserByEmailInput{
			{
				Name:       "Sync Identity User Updated",
				Email:      "sync.identity.two@example.com",
				Status:     "active",
				SyncSource: "enterprise_employee_sync",
				SyncRef:    "external_id:hr-1001",
			},
		},
	)
	if err != nil {
		t.Fatalf("second upsert should succeed: %v", err)
	}
	if created != 0 || updated != 1 || rejected != 0 {
		t.Fatalf("unexpected second upsert counters: created=%d updated=%d rejected=%d", created, updated, rejected)
	}

	users := svc.ListUsers(tenantID)
	if len(users) != 1 {
		t.Fatalf("expected one access user after email change, got %d", len(users))
	}
	if users[0].Email != "sync.identity.two@example.com" {
		t.Fatalf("expected updated email, got %s", users[0].Email)
	}
	if users[0].SyncSource != "enterprise_employee_sync" || users[0].SyncRef != "external_id:hr-1001" {
		t.Fatalf("sync identity should be persisted, got source=%q ref=%q", users[0].SyncSource, users[0].SyncRef)
	}
}

func TestUpsertUsersByEmailRejectsSyncIdentityEmailConflict(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_access_sync_conflict"

	created, updated, rejected, err := svc.UpsertUsersByEmail(
		tenantID,
		[]BatchUpsertUserByEmailInput{
			{
				Name:       "User One",
				Email:      "sync.conflict.one@example.com",
				Status:     "active",
				SyncSource: "enterprise_employee_sync",
				SyncRef:    "external_id:hr-2001",
			},
			{
				Name:       "User Two",
				Email:      "sync.conflict.two@example.com",
				Status:     "active",
				SyncSource: "enterprise_employee_sync",
				SyncRef:    "external_id:hr-2002",
			},
		},
	)
	if err != nil {
		t.Fatalf("seed upsert should succeed: %v", err)
	}
	if created != 2 || updated != 0 || rejected != 0 {
		t.Fatalf("unexpected seed counters: created=%d updated=%d rejected=%d", created, updated, rejected)
	}

	created, updated, rejected, err = svc.UpsertUsersByEmail(
		tenantID,
		[]BatchUpsertUserByEmailInput{
			{
				Name:       "Conflict Attempt",
				Email:      "sync.conflict.two@example.com",
				Status:     "active",
				SyncSource: "enterprise_employee_sync",
				SyncRef:    "external_id:hr-2001",
			},
		},
	)
	if err != nil {
		t.Fatalf("conflict upsert should not return hard error: %v", err)
	}
	if created != 0 || updated != 0 || rejected != 1 {
		t.Fatalf("unexpected conflict counters: created=%d updated=%d rejected=%d", created, updated, rejected)
	}

	users := svc.ListUsers(tenantID)
	if len(users) != 2 {
		t.Fatalf("expected two users to remain after conflict rejection, got %d", len(users))
	}

	one, found := accessUserBySyncRef(users, "external_id:hr-2001")
	if !found {
		t.Fatalf("expected user with sync_ref external_id:hr-2001")
	}
	if one.Email != "sync.conflict.one@example.com" {
		t.Fatalf("user one email should remain unchanged, got %s", one.Email)
	}

	two, found := accessUserBySyncRef(users, "external_id:hr-2002")
	if !found {
		t.Fatalf("expected user with sync_ref external_id:hr-2002")
	}
	if two.Email != "sync.conflict.two@example.com" {
		t.Fatalf("user two email should remain unchanged, got %s", two.Email)
	}
}

func TestUpsertUsersByEmailRejectsPartialSyncIdentity(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_access_sync_partial"

	created, updated, rejected, err := svc.UpsertUsersByEmail(
		tenantID,
		[]BatchUpsertUserByEmailInput{
			{
				Name:       "Partial Identity",
				Email:      "sync.partial@example.com",
				Status:     "active",
				SyncSource: "enterprise_employee_sync",
				SyncRef:    "",
			},
		},
	)
	if err != nil {
		t.Fatalf("partial identity should be soft-rejected without hard error: %v", err)
	}
	if created != 0 || updated != 0 || rejected != 1 {
		t.Fatalf("unexpected partial identity counters: created=%d updated=%d rejected=%d", created, updated, rejected)
	}
	if len(svc.ListUsers(tenantID)) != 0 {
		t.Fatalf("no users should be created for partial sync identity input")
	}
}

func TestUserCRUDUpdatesAndDeletesAccessReferences(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_access_user_crud"

	user, err := svc.CreateUser(
		tenantID,
		"building_demo_001",
		"CRUD User",
		"crud.user@example.com",
		"employee",
		"active",
		nil,
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	group, err := svc.CreateUserGroup(tenantID, "building_demo_001", "CRUD Group", "", []string{user.ID})
	if err != nil {
		t.Fatalf("create user group: %v", err)
	}
	team, err := svc.CreateTeam(tenantID, "CRUD Team", "place", "building_demo_001", "", "Manual")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if _, err := svc.CreateTeamMembership(tenantID, team.ID, "User", user.ID, user.Email, user.Name, "Manual"); err != nil {
		t.Fatalf("create team membership: %v", err)
	}
	assignment, err := svc.CreateRoleAssignment(RoleAssignmentInput{
		TenantID:      tenantID,
		RoleID:        "role_place_admin",
		AppliesToType: "Place",
		AppliesToID:   "building_demo_001",
		AssigneeType:  "User",
		AssigneeID:    user.ID,
		AssigneeEmail: user.Email,
	})
	if err != nil {
		t.Fatalf("create role assignment: %v", err)
	}

	updated, err := svc.UpdateUser(
		tenantID,
		user.ID,
		"building_demo_002",
		"CRUD User Updated",
		"crud.user.updated@example.com",
		"manager",
		"suspended",
		[]string{group.ID},
	)
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Name != "CRUD User Updated" || updated.Email != "crud.user.updated@example.com" || updated.Status != "suspended" || updated.BuildingID != "building_demo_002" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}

	removed, err := svc.DeleteUser(tenantID, user.ID)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if removed.ID != user.ID {
		t.Fatalf("unexpected removed user: %#v", removed)
	}
	if _, err := svc.GetUser(tenantID, user.ID); err != ErrUserNotFound {
		t.Fatalf("expected deleted user not found, got %v", err)
	}
	deletedGroup, err := svc.GetUserGroup(tenantID, group.ID)
	if err != nil {
		t.Fatalf("get group after user delete: %v", err)
	}
	for _, memberID := range deletedGroup.Members {
		if memberID == user.ID {
			t.Fatalf("deleted user should be removed from group members: %#v", deletedGroup.Members)
		}
	}
	for _, membership := range svc.ListTeamMemberships(tenantID) {
		if membership.MemberID == user.ID {
			t.Fatalf("deleted user should be removed from team memberships: %#v", membership)
		}
	}
	for _, item := range svc.ListRoleAssignments(tenantID) {
		if item.ID == assignment.ID {
			t.Fatalf("deleted user role assignment should be removed: %#v", item)
		}
	}
}

func TestUserInvitationDeliveryQueueAndReceipt(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_access_user_invitation"

	user, err := svc.CreateUser(
		tenantID,
		"building_demo_001",
		"Invitation User",
		"invitation.user@example.com",
		"employee",
		"inactive",
		nil,
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	delivery, deliveredUser, err := svc.CreateUserInvitationDelivery(tenantID, user.ID, "email")
	if err != nil {
		t.Fatalf("create invitation delivery: %v", err)
	}
	if deliveredUser.ID != user.ID || delivery.UserID != user.ID || delivery.Email != user.Email || delivery.Status != "queued" || delivery.ResourceType != "UserInvitationDelivery" {
		t.Fatalf("unexpected queued delivery: delivery=%#v user=%#v", delivery, deliveredUser)
	}

	deliveries := svc.ListUserInvitationDeliveries(tenantID, user.ID)
	if len(deliveries) != 1 || deliveries[0].ID != delivery.ID {
		t.Fatalf("expected queued delivery to be listable, got %#v", deliveries)
	}

	receipt, err := svc.RecordUserInvitationReceipt(
		tenantID,
		user.ID,
		delivery.ID,
		"sent",
		"mailgun",
		"mg_invite_001",
		"",
		false,
	)
	if err != nil {
		t.Fatalf("record invitation receipt: %v", err)
	}
	if receipt.Status != "sent" || receipt.Provider != "mailgun" || receipt.ProviderDeliveryID != "mg_invite_001" || receipt.DeliveredAt == nil {
		t.Fatalf("unexpected invitation receipt: %#v", receipt)
	}

	if _, err := svc.RecordUserInvitationReceipt(tenantID, user.ID, delivery.ID, "unknown", "", "", "", false); err != ErrInvalidUserInvitationStatus {
		t.Fatalf("expected invalid invitation status error, got %v", err)
	}
}

func TestReviewAccessRightsMarksRoleAssignmentsAndShares(t *testing.T) {
	svc := NewService()
	tenantID := "tenant_access_rights_review"

	assignment, err := svc.CreateRoleAssignment(RoleAssignmentInput{
		TenantID:      tenantID,
		RoleID:        "role_place_admin",
		AppliesToType: "Place",
		AppliesToID:   "building_demo_001",
		AssigneeType:  "User",
		AssigneeID:    "usr_review_001",
		AssigneeEmail: "review.user@example.com",
		ValidUntil:    "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("create role assignment: %v", err)
	}
	share, err := svc.CreateTemporaryAccessWithInput(TemporaryAccessInput{
		TenantID:       tenantID,
		ScopeType:      "building",
		BuildingID:     "building_demo_001",
		GroupID:        "ug_common_office_jkt",
		RoleID:         "role_group_access",
		DeliveryMethod: "email_qr",
		GranteeName:    "Review Guest",
		GranteePhone:   "not_provided",
		GranteeEmail:   "review.guest@example.com",
		PassType:       "share",
		ValidUntil:     "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}

	result, err := svc.ReviewAccessRights(tenantID, []string{assignment.ID}, []string{share.ID}, "reviewer@example.com")
	if err != nil {
		t.Fatalf("review access rights: %v", err)
	}
	if result.ReviewedCount != 2 || result.SkippedCount != 0 || result.ReviewedAt == "" || result.ReviewedBy != "reviewer@example.com" {
		t.Fatalf("unexpected review result: %#v", result)
	}
	if len(result.ReviewedRoleAssignmentIDs) != 1 || result.ReviewedRoleAssignmentIDs[0] != assignment.ID {
		t.Fatalf("unexpected reviewed role assignment ids: %#v", result.ReviewedRoleAssignmentIDs)
	}
	if len(result.ReviewedShareIDs) != 1 || result.ReviewedShareIDs[0] != share.ID {
		t.Fatalf("unexpected reviewed share ids: %#v", result.ReviewedShareIDs)
	}

	reviewedAssignment, err := svc.GetRoleAssignment(tenantID, assignment.ID)
	if err != nil {
		t.Fatalf("get reviewed assignment: %v", err)
	}
	if reviewedAssignment.ReviewedAt == "" || reviewedAssignment.ReviewedBy != "reviewer@example.com" {
		t.Fatalf("expected assignment review metadata, got %#v", reviewedAssignment)
	}
	reviewedShare, err := svc.GetTemporaryAccess(tenantID, share.ID)
	if err != nil {
		t.Fatalf("get reviewed share: %v", err)
	}
	if reviewedShare.ReviewedAt == "" || reviewedShare.ReviewedBy != "reviewer@example.com" {
		t.Fatalf("expected share review metadata, got %#v", reviewedShare)
	}

	second, err := svc.ReviewAccessRights(tenantID, []string{assignment.ID}, []string{share.ID}, "reviewer@example.com")
	if err != nil {
		t.Fatalf("review access rights again: %v", err)
	}
	if second.ReviewedCount != 0 || second.SkippedCount != 2 {
		t.Fatalf("expected already reviewed items to be skipped, got %#v", second)
	}
}

func accessUserBySyncRef(items []AccessUser, syncRef string) (AccessUser, bool) {
	for i := range items {
		if items[i].SyncRef == syncRef {
			return items[i], true
		}
	}
	return AccessUser{}, false
}

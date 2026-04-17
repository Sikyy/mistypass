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

func accessUserBySyncRef(items []AccessUser, syncRef string) (AccessUser, bool) {
	for i := range items {
		if items[i].SyncRef == syncRef {
			return items[i], true
		}
	}
	return AccessUser{}, false
}

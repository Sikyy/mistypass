package integration

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Lark ComputeSyncActions tests
// ---------------------------------------------------------------------------

func TestComputeSyncActions_NewUser(t *testing.T) {
	larkUsers := []LarkUser{
		{
			UserID: "u001",
			Name:   "Alice",
			Email:  "alice@company.com",
			Status: struct {
				IsActivated bool `json:"is_activated"`
				IsFrozen    bool `json:"is_frozen"`
				IsResigned  bool `json:"is_resigned"`
			}{IsActivated: true},
		},
	}
	existing := map[string]string{} // empty — user does not exist in MistyPass

	actions := ComputeSyncActions(larkUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "create" {
		t.Errorf("expected action 'create', got %q", actions[0].Action)
	}
	if actions[0].LarkUser.Email != "alice@company.com" {
		t.Errorf("expected email alice@company.com, got %q", actions[0].LarkUser.Email)
	}
	if actions[0].Reason != "new employee in Lark" {
		t.Errorf("expected reason 'new employee in Lark', got %q", actions[0].Reason)
	}
}

func TestComputeSyncActions_ExistingUser(t *testing.T) {
	larkUsers := []LarkUser{
		{
			UserID: "u002",
			Name:   "Bob",
			Email:  "bob@company.com",
			Status: struct {
				IsActivated bool `json:"is_activated"`
				IsFrozen    bool `json:"is_frozen"`
				IsResigned  bool `json:"is_resigned"`
			}{IsActivated: true},
		},
	}
	existing := map[string]string{"bob@company.com": "uuid-bob"}

	actions := ComputeSyncActions(larkUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "update" {
		t.Errorf("expected action 'update', got %q", actions[0].Action)
	}
	if actions[0].LarkUser.Email != "bob@company.com" {
		t.Errorf("expected email bob@company.com, got %q", actions[0].LarkUser.Email)
	}
}

func TestComputeSyncActions_ResignedUser(t *testing.T) {
	larkUsers := []LarkUser{
		{
			UserID: "u003",
			Name:   "Carol",
			Email:  "carol@company.com",
			Status: struct {
				IsActivated bool `json:"is_activated"`
				IsFrozen    bool `json:"is_frozen"`
				IsResigned  bool `json:"is_resigned"`
			}{IsResigned: true},
		},
	}
	existing := map[string]string{"carol@company.com": "uuid-carol"}

	actions := ComputeSyncActions(larkUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "deactivate" {
		t.Errorf("expected action 'deactivate', got %q", actions[0].Action)
	}
	if actions[0].Reason != "resigned in Lark" {
		t.Errorf("expected reason 'resigned in Lark', got %q", actions[0].Reason)
	}
}

func TestComputeSyncActions_FrozenUser(t *testing.T) {
	larkUsers := []LarkUser{
		{
			UserID: "u004",
			Name:   "Dave",
			Email:  "dave@company.com",
			Status: struct {
				IsActivated bool `json:"is_activated"`
				IsFrozen    bool `json:"is_frozen"`
				IsResigned  bool `json:"is_resigned"`
			}{IsFrozen: true},
		},
	}
	existing := map[string]string{"dave@company.com": "uuid-dave"}

	actions := ComputeSyncActions(larkUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "deactivate" {
		t.Errorf("expected action 'deactivate', got %q", actions[0].Action)
	}
	if actions[0].Reason != "frozen in Lark" {
		t.Errorf("expected reason 'frozen in Lark', got %q", actions[0].Reason)
	}
}

func TestComputeSyncActions_EmptyLark(t *testing.T) {
	// No Lark users, but MistyPass has users → those should be deactivated.
	existing := map[string]string{"orphan@company.com": "uuid-orphan"}

	actions := ComputeSyncActions(nil, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "deactivate" {
		t.Errorf("expected action 'deactivate', got %q", actions[0].Action)
	}
	if actions[0].Reason != "not found in Lark directory" {
		t.Errorf("expected reason 'not found in Lark directory', got %q", actions[0].Reason)
	}
}

func TestComputeSyncActions_EmptyBothSides(t *testing.T) {
	actions := ComputeSyncActions(nil, map[string]string{})
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}
}

func TestComputeSyncActions_SkipsEmptyEmail(t *testing.T) {
	larkUsers := []LarkUser{
		{
			UserID: "u005",
			Name:   "NoEmail",
			Email:  "", // should be skipped
			Status: struct {
				IsActivated bool `json:"is_activated"`
				IsFrozen    bool `json:"is_frozen"`
				IsResigned  bool `json:"is_resigned"`
			}{IsActivated: true},
		},
	}
	actions := ComputeSyncActions(larkUsers, map[string]string{})
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions for user without email, got %d", len(actions))
	}
}

func TestComputeSyncActions_ResignedUserNotInMistyPass(t *testing.T) {
	// Resigned user that is NOT in MistyPass should produce no action.
	larkUsers := []LarkUser{
		{
			UserID: "u006",
			Name:   "Gone",
			Email:  "gone@company.com",
			Status: struct {
				IsActivated bool `json:"is_activated"`
				IsFrozen    bool `json:"is_frozen"`
				IsResigned  bool `json:"is_resigned"`
			}{IsResigned: true},
		},
	}
	actions := ComputeSyncActions(larkUsers, map[string]string{})
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions for resigned user not in MistyPass, got %d", len(actions))
	}
}

// ---------------------------------------------------------------------------
// Google Workspace ComputeGWSyncActions tests
// ---------------------------------------------------------------------------

func TestComputeGWSyncActions_NewUser(t *testing.T) {
	gwsUsers := []GoogleWorkspaceUser{
		{
			ID:           "gws001",
			PrimaryEmail: "alice@company.com",
			FullName:     "Alice Smith",
			Suspended:    false,
		},
	}
	existing := map[string]string{} // not in MistyPass

	actions := ComputeGWSyncActions(gwsUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "create" {
		t.Errorf("expected action 'create', got %q", actions[0].Action)
	}
	if actions[0].LarkUser.Email != "alice@company.com" {
		t.Errorf("expected email alice@company.com, got %q", actions[0].LarkUser.Email)
	}
	if actions[0].Reason != "new employee in Google Workspace" {
		t.Errorf("expected reason 'new employee in Google Workspace', got %q", actions[0].Reason)
	}
}

func TestComputeGWSyncActions_SuspendedUser(t *testing.T) {
	gwsUsers := []GoogleWorkspaceUser{
		{
			ID:           "gws002",
			PrimaryEmail: "bob@company.com",
			FullName:     "Bob Jones",
			Suspended:    true,
		},
	}
	existing := map[string]string{"bob@company.com": "uuid-bob"}

	actions := ComputeGWSyncActions(gwsUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "deactivate" {
		t.Errorf("expected action 'deactivate', got %q", actions[0].Action)
	}
	if actions[0].Reason != "suspended in Google Workspace" {
		t.Errorf("expected reason 'suspended in Google Workspace', got %q", actions[0].Reason)
	}
}

func TestComputeGWSyncActions_NotInGWS(t *testing.T) {
	gwsUsers := []GoogleWorkspaceUser{} // empty — nobody in GWS
	existing := map[string]string{"orphan@company.com": "uuid-orphan"}

	actions := ComputeGWSyncActions(gwsUsers, existing)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != "deactivate" {
		t.Errorf("expected action 'deactivate', got %q", actions[0].Action)
	}
	if actions[0].Reason != "not found in Google Workspace directory" {
		t.Errorf("expected reason 'not found in Google Workspace directory', got %q", actions[0].Reason)
	}
}

func TestComputeGWSyncActions_EmptyGWS(t *testing.T) {
	actions := ComputeGWSyncActions(nil, map[string]string{})
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions, got %d", len(actions))
	}
}

func TestComputeGWSyncActions_SuspendedUserNotInMistyPass(t *testing.T) {
	// Suspended GWS user that is NOT in MistyPass should produce no action.
	gwsUsers := []GoogleWorkspaceUser{
		{
			ID:           "gws003",
			PrimaryEmail: "gone@company.com",
			FullName:     "Gone Person",
			Suspended:    true,
		},
	}
	actions := ComputeGWSyncActions(gwsUsers, map[string]string{})
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions for suspended user not in MistyPass, got %d", len(actions))
	}
}

func TestComputeGWSyncActions_CaseInsensitiveEmail(t *testing.T) {
	// GWS email is mixed case; existing map uses lowercase.
	gwsUsers := []GoogleWorkspaceUser{
		{
			ID:           "gws004",
			PrimaryEmail: "Alice@Company.COM",
			FullName:     "Alice",
			Suspended:    false,
		},
	}
	existing := map[string]string{"alice@company.com": "uuid-alice"}

	actions := ComputeGWSyncActions(gwsUsers, existing)

	// The user exists in MistyPass and is active in GWS, so no create/deactivate.
	// The function does not produce an "update" for GWS, so 0 actions expected.
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions (existing active user), got %d", len(actions))
	}
}

// ---------------------------------------------------------------------------
// Lark event type JSON parsing tests
// ---------------------------------------------------------------------------

func TestLarkEventParsing_UserCreated(t *testing.T) {
	raw := `{
		"schema": "2.0",
		"header": {
			"event_id": "evt-001",
			"event_type": "contact.user.created_v3",
			"app_id": "cli_test",
			"tenant_key": "tk_test",
			"token": "verify123"
		},
		"event": {
			"object": {
				"user_id": "u100",
				"open_id": "ou_abc",
				"name": "New Employee",
				"email": "new@company.com",
				"mobile": "+628123456",
				"department_id": "dept01",
				"status": {
					"is_activated": true,
					"is_frozen": false,
					"is_resigned": false
				}
			}
		}
	}`

	var evt LarkEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("failed to unmarshal LarkEvent: %v", err)
	}

	if evt.Header.EventType != "contact.user.created_v3" {
		t.Errorf("expected event_type contact.user.created_v3, got %q", evt.Header.EventType)
	}
	if evt.Header.EventID != "evt-001" {
		t.Errorf("expected event_id evt-001, got %q", evt.Header.EventID)
	}
	if evt.Header.AppID != "cli_test" {
		t.Errorf("expected app_id cli_test, got %q", evt.Header.AppID)
	}

	var created LarkUserCreatedEvent
	if err := json.Unmarshal(evt.Event, &created); err != nil {
		t.Fatalf("failed to unmarshal LarkUserCreatedEvent: %v", err)
	}

	if created.Object.UserID != "u100" {
		t.Errorf("expected user_id u100, got %q", created.Object.UserID)
	}
	if created.Object.Name != "New Employee" {
		t.Errorf("expected name 'New Employee', got %q", created.Object.Name)
	}
	if created.Object.Email != "new@company.com" {
		t.Errorf("expected email new@company.com, got %q", created.Object.Email)
	}
	if !created.Object.Status.IsActivated {
		t.Error("expected IsActivated to be true")
	}
}

func TestLarkEventParsing_UserDeleted(t *testing.T) {
	raw := `{
		"schema": "2.0",
		"header": {
			"event_id": "evt-002",
			"event_type": "contact.user.deleted_v3",
			"app_id": "cli_test",
			"tenant_key": "tk_test",
			"token": "verify123"
		},
		"event": {
			"object": {
				"user_id": "u200",
				"open_id": "ou_def",
				"name": "Departed Employee",
				"email": "departed@company.com"
			}
		}
	}`

	var evt LarkEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("failed to unmarshal LarkEvent: %v", err)
	}

	if evt.Header.EventType != "contact.user.deleted_v3" {
		t.Errorf("expected event_type contact.user.deleted_v3, got %q", evt.Header.EventType)
	}

	var deleted LarkUserDeletedEvent
	if err := json.Unmarshal(evt.Event, &deleted); err != nil {
		t.Fatalf("failed to unmarshal LarkUserDeletedEvent: %v", err)
	}

	if deleted.Object.UserID != "u200" {
		t.Errorf("expected user_id u200, got %q", deleted.Object.UserID)
	}
	if deleted.Object.Name != "Departed Employee" {
		t.Errorf("expected name 'Departed Employee', got %q", deleted.Object.Name)
	}
	if deleted.Object.Email != "departed@company.com" {
		t.Errorf("expected email departed@company.com, got %q", deleted.Object.Email)
	}
}

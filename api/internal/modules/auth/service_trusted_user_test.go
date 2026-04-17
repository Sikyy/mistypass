package auth

import "testing"

func TestLoginByTrustedUserUpsertAndRefresh(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	trusted := User{
		ID:       "usr_ent_jit_test_001",
		Email:    "jit.employee@sudirman.co",
		Role:     "resident",
		TenantID: "tenant_demo_jakarta",
	}

	login, err := svc.LoginByTrustedUser(trusted)
	if err != nil {
		t.Fatalf("expected trusted user login to succeed: %v", err)
	}
	if login.AccessToken == "" || login.RefreshToken == "" {
		t.Fatalf("expected non-empty token pair")
	}
	if login.User.Email != trusted.Email {
		t.Fatalf("unexpected login user email: got=%s want=%s", login.User.Email, trusted.Email)
	}
	if login.User.Role != trusted.Role {
		t.Fatalf("unexpected login user role: got=%s want=%s", login.User.Role, trusted.Role)
	}

	me, err := svc.Me(login.AccessToken)
	if err != nil {
		t.Fatalf("expected me() to work for trusted user: %v", err)
	}
	if me.Email != trusted.Email || me.TenantID != trusted.TenantID {
		t.Fatalf("unexpected me() user: %+v", me)
	}

	refresh, err := svc.Refresh(RefreshRequest{RefreshToken: login.RefreshToken})
	if err != nil {
		t.Fatalf("expected refresh to succeed for trusted user session: %v", err)
	}
	if refresh.AccessToken == "" || refresh.RefreshToken == "" {
		t.Fatalf("expected refreshed token pair")
	}
	if refresh.User.ID != trusted.ID {
		t.Fatalf("unexpected refreshed user id: got=%s want=%s", refresh.User.ID, trusted.ID)
	}
}

func TestLoginByTrustedUserRequiresCoreFields(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	_, err := svc.LoginByTrustedUser(User{
		ID:       "",
		Email:    "",
		Role:     "",
		TenantID: "tenant_demo_jakarta",
	})
	if err == nil {
		t.Fatalf("expected LoginByTrustedUser to reject empty core fields")
	}
	if err != ErrInvalidCredentials {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevokeRefreshTokenSuccessAndInvalidAfterRevoke(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	login, err := svc.LoginByTrustedUser(User{
		ID:       "usr_ent_jit_test_revoke_001",
		Email:    "revoke.employee@sudirman.co",
		Role:     "resident",
		TenantID: "tenant_demo_jakarta",
	})
	if err != nil {
		t.Fatalf("expected login to succeed: %v", err)
	}

	if err := svc.RevokeRefreshToken(login.RefreshToken); err != nil {
		t.Fatalf("expected revoke refresh token to succeed: %v", err)
	}

	if _, err := svc.Refresh(RefreshRequest{RefreshToken: login.RefreshToken}); err != ErrInvalidRefreshToken {
		t.Fatalf("expected refresh token to be invalid after revoke, got: %v", err)
	}

	if err := svc.RevokeRefreshToken(login.RefreshToken); err != ErrInvalidRefreshToken {
		t.Fatalf("expected revoked refresh token to become invalid, got: %v", err)
	}
}

func TestRevokeRefreshTokenRejectsInvalidToken(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	if err := svc.RevokeRefreshToken(""); err != ErrInvalidRefreshToken {
		t.Fatalf("expected empty token to be invalid, got: %v", err)
	}
	if err := svc.RevokeRefreshToken("not-a-jwt"); err != ErrInvalidRefreshToken {
		t.Fatalf("expected malformed token to be invalid, got: %v", err)
	}
}

func TestRevokeRefreshTokensByUserEmail(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	loginA, err := svc.LoginByTrustedUser(User{
		ID:       "usr_ent_jit_test_revoke_multi_001",
		Email:    "revoke.multi@sudirman.co",
		Role:     "resident",
		TenantID: "tenant_demo_jakarta",
	})
	if err != nil {
		t.Fatalf("expected first login success: %v", err)
	}
	loginB, err := svc.LoginByTrustedUser(User{
		ID:       "usr_ent_jit_test_revoke_multi_001",
		Email:    "revoke.multi@sudirman.co",
		Role:     "resident",
		TenantID: "tenant_demo_jakarta",
	})
	if err != nil {
		t.Fatalf("expected second login success: %v", err)
	}

	revoked := svc.RevokeRefreshTokensByUserEmail("revoke.multi@sudirman.co")
	if revoked < 2 {
		t.Fatalf("expected revoked refresh sessions >= 2, got %d", revoked)
	}

	if _, err := svc.Refresh(RefreshRequest{RefreshToken: loginA.RefreshToken}); err != ErrInvalidRefreshToken {
		t.Fatalf("expected loginA refresh token invalid after mass revoke, got: %v", err)
	}
	if _, err := svc.Refresh(RefreshRequest{RefreshToken: loginB.RefreshToken}); err != ErrInvalidRefreshToken {
		t.Fatalf("expected loginB refresh token invalid after mass revoke, got: %v", err)
	}

	if got := svc.RevokeRefreshTokensByUserEmail("unknown.user@sudirman.co"); got != 0 {
		t.Fatalf("expected unknown user revoke count 0, got %d", got)
	}
}

func TestDowngradeTrustedUserToLeastPrivilegeByEmail(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	_, err := svc.LoginByTrustedUser(User{
		ID:          "usr_ent_jit_downgrade_001",
		Email:       "downgrade.employee@sudirman.co",
		Role:        "building_admin",
		TenantID:    "tenant_demo_jakarta",
		BuildingIDs: []string{"building_demo_001"},
	})
	if err != nil {
		t.Fatalf("expected seed trusted user login success: %v", err)
	}

	before, after, applied := svc.DowngradeTrustedUserToLeastPrivilegeByEmail(
		"downgrade.employee@sudirman.co",
		"tenant_demo_jakarta",
	)
	if !applied {
		t.Fatalf("expected downgrade to be applied")
	}
	if before.Role != "building_admin" {
		t.Fatalf("unexpected before role: %s", before.Role)
	}
	if after.Role != "resident" {
		t.Fatalf("unexpected after role: %s", after.Role)
	}
	if len(after.BuildingIDs) != 0 {
		t.Fatalf("expected building scope cleared, got: %+v", after.BuildingIDs)
	}

	login, err := svc.LoginByTrustedIdentity("downgrade.employee@sudirman.co")
	if err != nil {
		t.Fatalf("expected trusted login after downgrade success: %v", err)
	}
	if login.User.Role != "resident" {
		t.Fatalf("expected downgraded role resident, got %s", login.User.Role)
	}
	if len(login.User.BuildingIDs) != 0 {
		t.Fatalf("expected downgraded building scope empty, got %+v", login.User.BuildingIDs)
	}
}

func TestDowngradeTrustedUserToLeastPrivilegeByEmailNotFound(t *testing.T) {
	svc := NewService("", "", 0, 0, true)

	_, _, applied := svc.DowngradeTrustedUserToLeastPrivilegeByEmail(
		"missing.user@sudirman.co",
		"tenant_demo_jakarta",
	)
	if applied {
		t.Fatalf("expected missing user downgrade not applied")
	}
}

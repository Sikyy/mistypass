package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestInvitationsListAndDetail(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "invitations-list-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// create a user first
	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Invite Test","email":"invite-test@example.com","role":"employee","status":"inactive"}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating user, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var createdUser struct{ ID string `json:"id"` }
	_ = json.Unmarshal(createRec.Body.Bytes(), &createdUser)

	// send invitation
	inviteBody := []byte(`{"tenant_id":"tenant_demo_jakarta"}`)
	inviteRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+createdUser.ID+"/invite", token, inviteBody)
	if inviteRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", inviteRec.Code, inviteRec.Body.String())
	}
	var delivery struct {
		ID     string `json:"id"`
		Email  string `json:"email"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(inviteRec.Body.Bytes(), &delivery)
	if delivery.ID == "" {
		t.Fatalf("expected delivery ID")
	}

	// list all invitations
	listRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/invitations?tenant_id=tenant_demo_jakarta", token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), delivery.ID) {
		t.Errorf("list should contain the invitation")
	}

	// get detail
	getRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/invitations/"+delivery.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
	var detail struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	_ = json.Unmarshal(getRec.Body.Bytes(), &detail)
	if detail.ID != delivery.ID {
		t.Errorf("expected detail ID=%s, got %s", delivery.ID, detail.ID)
	}

	// filter by status
	filterRec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/invitations?tenant_id=tenant_demo_jakarta&status=queued", token, nil)
	if filterRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", filterRec.Code)
	}
	if !strings.Contains(filterRec.Body.String(), delivery.ID) {
		t.Errorf("filtered list should contain the queued invitation")
	}
}

func TestInvitationsCancelAndResend(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "invitations-cancel-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	// create user and invite
	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Cancel Test","email":"cancel-test@example.com","role":"employee","status":"inactive"}`)
	createRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users", token, createBody)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}
	var user struct{ ID string `json:"id"` }
	_ = json.Unmarshal(createRec.Body.Bytes(), &user)

	inviteRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/users/"+user.ID+"/invite", token, []byte(`{"tenant_id":"tenant_demo_jakarta"}`))
	if inviteRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", inviteRec.Code)
	}
	var delivery struct{ ID string `json:"id"` }
	_ = json.Unmarshal(inviteRec.Body.Bytes(), &delivery)

	// cancel
	cancelRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/invitations/"+delivery.ID+"/cancel?tenant_id=tenant_demo_jakarta", token, nil)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	var cancelled struct{ Status string `json:"status"` }
	_ = json.Unmarshal(cancelRec.Body.Bytes(), &cancelled)
	if cancelled.Status != "cancelled" {
		t.Errorf("expected status=cancelled, got %s", cancelled.Status)
	}

	// resend (creates a new delivery)
	resendRec := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/invitations/"+delivery.ID+"/resend?tenant_id=tenant_demo_jakarta", token, nil)
	if resendRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", resendRec.Code, resendRec.Body.String())
	}
	var resent struct{ ID string `json:"id"` }
	_ = json.Unmarshal(resendRec.Body.Bytes(), &resent)
	if resent.ID == delivery.ID {
		t.Errorf("resend should create a new delivery, got same ID")
	}

	// check audit logs
	assertReferenceAuditLog(t, router, token, "invitation_cancelled", "delivery_id="+delivery.ID)
	assertReferenceAuditLog(t, router, token, "invitation_resent", "delivery_id="+resent.ID)
}

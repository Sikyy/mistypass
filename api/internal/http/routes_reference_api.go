package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
)

var errInvalidReferenceAlertPolicyPayload = errors.New("invalid alert policy payload")
var errInvalidReferenceIntegrationPayload = errors.New("invalid integration payload")

type referencePlacePatchPayload struct {
	TenantID string  `json:"tenant_id"`
	Name     *string `json:"name"`
	Address  *string `json:"address"`
	Region   *string `json:"region"`
}

type referencePlaceMutationRequest struct {
	Place    *referencePlacePatchPayload `json:"place"`
	TenantID string                      `json:"tenant_id"`
	Name     *string                     `json:"name"`
	Address  *string                     `json:"address"`
	Region   *string                     `json:"region"`
}

func (request referencePlaceMutationRequest) payload() referencePlacePatchPayload {
	if request.Place != nil {
		return *request.Place
	}
	return referencePlacePatchPayload{
		TenantID: request.TenantID,
		Name:     request.Name,
		Address:  request.Address,
		Region:   request.Region,
	}
}

type referenceLockPatchPayload struct {
	TenantID   string  `json:"tenant_id"`
	PlaceID    *string `json:"place_id"`
	BuildingID *string `json:"building_id"`
	FloorID    *string `json:"floor_id"`
	AreaID     *string `json:"area_id"`
	Name       *string `json:"name"`
	GatewayID  *string `json:"gateway_id"`
	Kind       *string `json:"kind"`
	Status     *string `json:"status"`
	Mode       *string `json:"mode"`
	State      *string `json:"state"`
}

type referenceLockMutationRequest struct {
	Lock       *referenceLockPatchPayload `json:"lock"`
	TenantID   string                     `json:"tenant_id"`
	PlaceID    *string                    `json:"place_id"`
	BuildingID *string                    `json:"building_id"`
	FloorID    *string                    `json:"floor_id"`
	AreaID     *string                    `json:"area_id"`
	Name       *string                    `json:"name"`
	GatewayID  *string                    `json:"gateway_id"`
	Kind       *string                    `json:"kind"`
	Status     *string                    `json:"status"`
	Mode       *string                    `json:"mode"`
	State      *string                    `json:"state"`
}

func (request referenceLockMutationRequest) payload() referenceLockPatchPayload {
	if request.Lock != nil {
		return *request.Lock
	}
	return referenceLockPatchPayload{
		TenantID:   request.TenantID,
		PlaceID:    request.PlaceID,
		BuildingID: request.BuildingID,
		FloorID:    request.FloorID,
		AreaID:     request.AreaID,
		Name:       request.Name,
		GatewayID:  request.GatewayID,
		Kind:       request.Kind,
		Status:     request.Status,
		Mode:       request.Mode,
		State:      request.State,
	}
}

type referenceSpaceActionResponse struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	TenantID     string `json:"tenant_id"`
	PlaceID      string `json:"place_id,omitempty"`
	LockID       string `json:"lock_id,omitempty"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	LockCount    int    `json:"lock_count,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type referenceRoleAssignmentPayload struct {
	TenantID      string `json:"tenant_id"`
	RoleID        string `json:"role_id"`
	AppliesToType string `json:"applies_to_type"`
	AppliesToID   string `json:"applies_to_id"`
	AssigneeType  string `json:"assignee_type"`
	AssigneeID    string `json:"assignee_id"`
	AssigneeEmail string `json:"assignee_email"`
	ValidFrom     string `json:"valid_from"`
	ValidUntil    string `json:"valid_until"`
}

type referenceTeamPayload struct {
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	PlaceID     string `json:"place_id"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type referenceTeamMembershipPayload struct {
	TenantID    string `json:"tenant_id"`
	TeamID      string `json:"team_id"`
	MemberType  string `json:"member_type"`
	MemberID    string `json:"member_id"`
	MemberEmail string `json:"member_email"`
	MemberName  string `json:"member_name"`
	Source      string `json:"source"`
}

type referenceShare struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	Email             string `json:"email"`
	GroupID           string `json:"group_id,omitempty"`
	RoleID            string `json:"role_id"`
	PlaceID           string `json:"place_id,omitempty"`
	AreaID            string `json:"area_id,omitempty"`
	LockID            string `json:"lock_id,omitempty"`
	ValidFrom         string `json:"valid_from,omitempty"`
	ValidUntil        string `json:"valid_until"`
	Status            string `json:"status"`
	DeliveryMethod    string `json:"delivery_method"`
	GranteeName       string `json:"grantee_name,omitempty"`
	GranteePhone      string `json:"grantee_phone,omitempty"`
	MobileModel       string `json:"mobile_model,omitempty"`
	PassType          string `json:"pass_type,omitempty"`
	AuthorizedByID    string `json:"authorized_by_id,omitempty"`
	AuthorizedByEmail string `json:"authorized_by_email,omitempty"`
	AuthorizedByRole  string `json:"authorized_by_role,omitempty"`
	AuthorizedAt      string `json:"authorized_at,omitempty"`
	ReviewedAt        string `json:"reviewed_at,omitempty"`
	ReviewedBy        string `json:"reviewed_by,omitempty"`
	CreatedAt         string `json:"created_at"`
}

type referenceSharePayload struct {
	TenantID       string  `json:"tenant_id"`
	Email          string  `json:"email"`
	GroupID        string  `json:"group_id"`
	RoleID         string  `json:"role_id"`
	PlaceID        string  `json:"place_id"`
	BuildingID     string  `json:"building_id"`
	AreaID         string  `json:"area_id"`
	LockID         string  `json:"lock_id"`
	DoorID         string  `json:"door_id"`
	ValidFrom      *string `json:"valid_from"`
	ValidUntil     string  `json:"valid_until"`
	DeliveryMethod string  `json:"delivery_method"`
	GranteeName    string  `json:"grantee_name"`
	GranteePhone   string  `json:"grantee_phone"`
	MobileModel    string  `json:"mobile_model"`
	PassType       string  `json:"pass_type"`
}

type referenceGroupLock struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	GroupID   string `json:"group_id"`
	LockID    string `json:"lock_id"`
	CreatedAt string `json:"created_at"`
}

type referenceGroupPayload struct {
	TenantID                        string   `json:"tenant_id"`
	PlaceID                         string   `json:"place_id"`
	BuildingID                      string   `json:"building_id"`
	Name                            string   `json:"name"`
	Description                     *string  `json:"description"`
	LoginEnabled                    *bool    `json:"login_enabled"`
	GeofenceRestrictionEnabled      *bool    `json:"geofence_restriction_enabled"`
	GeofenceRestrictionRadius       *float64 `json:"geofence_restriction_radius"`
	PrimaryDeviceRestrictionEnabled *bool    `json:"primary_device_restriction_enabled"`
	ManagedDeviceRestrictionEnabled *bool    `json:"managed_device_restriction_enabled"`
	ReaderRestrictionEnabled        *bool    `json:"reader_restriction_enabled"`
	TimeRestrictionEnabled          *bool    `json:"time_restriction_enabled"`
	TapToAccessRestrictionEnabled   *bool    `json:"tap_to_access_restriction_enabled"`
	TimeRestrictionTimeZone         *string  `json:"time_restriction_time_zone"`
	MemberIDs                       []string `json:"member_ids"`
	Members                         []string `json:"members"`
}

type referenceGroupMutationRequest struct {
	Group                           *referenceGroupPayload `json:"group"`
	TenantID                        string                 `json:"tenant_id"`
	PlaceID                         string                 `json:"place_id"`
	BuildingID                      string                 `json:"building_id"`
	Name                            string                 `json:"name"`
	Description                     *string                `json:"description"`
	LoginEnabled                    *bool                  `json:"login_enabled"`
	GeofenceRestrictionEnabled      *bool                  `json:"geofence_restriction_enabled"`
	GeofenceRestrictionRadius       *float64               `json:"geofence_restriction_radius"`
	PrimaryDeviceRestrictionEnabled *bool                  `json:"primary_device_restriction_enabled"`
	ManagedDeviceRestrictionEnabled *bool                  `json:"managed_device_restriction_enabled"`
	ReaderRestrictionEnabled        *bool                  `json:"reader_restriction_enabled"`
	TimeRestrictionEnabled          *bool                  `json:"time_restriction_enabled"`
	TapToAccessRestrictionEnabled   *bool                  `json:"tap_to_access_restriction_enabled"`
	TimeRestrictionTimeZone         *string                `json:"time_restriction_time_zone"`
	MemberIDs                       []string               `json:"member_ids"`
	Members                         []string               `json:"members"`
}

func (request referenceGroupMutationRequest) payload() referenceGroupPayload {
	if request.Group != nil {
		return *request.Group
	}
	return referenceGroupPayload{
		TenantID:                        request.TenantID,
		PlaceID:                         request.PlaceID,
		BuildingID:                      request.BuildingID,
		Name:                            request.Name,
		Description:                     request.Description,
		LoginEnabled:                    request.LoginEnabled,
		GeofenceRestrictionEnabled:      request.GeofenceRestrictionEnabled,
		GeofenceRestrictionRadius:       request.GeofenceRestrictionRadius,
		PrimaryDeviceRestrictionEnabled: request.PrimaryDeviceRestrictionEnabled,
		ManagedDeviceRestrictionEnabled: request.ManagedDeviceRestrictionEnabled,
		ReaderRestrictionEnabled:        request.ReaderRestrictionEnabled,
		TimeRestrictionEnabled:          request.TimeRestrictionEnabled,
		TapToAccessRestrictionEnabled:   request.TapToAccessRestrictionEnabled,
		TimeRestrictionTimeZone:         request.TimeRestrictionTimeZone,
		MemberIDs:                       request.MemberIDs,
		Members:                         request.Members,
	}
}

func referenceGroupPayloadHasRestrictions(payload referenceGroupPayload) bool {
	return payload.LoginEnabled != nil ||
		payload.GeofenceRestrictionEnabled != nil ||
		payload.GeofenceRestrictionRadius != nil ||
		payload.PrimaryDeviceRestrictionEnabled != nil ||
		payload.ManagedDeviceRestrictionEnabled != nil ||
		payload.ReaderRestrictionEnabled != nil ||
		payload.TimeRestrictionEnabled != nil ||
		payload.TapToAccessRestrictionEnabled != nil ||
		payload.TimeRestrictionTimeZone != nil
}

func referenceGroupRestrictionsInput(payload referenceGroupPayload) access.UserGroupRestrictionsInput {
	return access.UserGroupRestrictionsInput{
		LoginEnabled:                    payload.LoginEnabled,
		GeofenceRestrictionEnabled:      payload.GeofenceRestrictionEnabled,
		GeofenceRestrictionRadius:       payload.GeofenceRestrictionRadius,
		PrimaryDeviceRestrictionEnabled: payload.PrimaryDeviceRestrictionEnabled,
		ManagedDeviceRestrictionEnabled: payload.ManagedDeviceRestrictionEnabled,
		ReaderRestrictionEnabled:        payload.ReaderRestrictionEnabled,
		TimeRestrictionEnabled:          payload.TimeRestrictionEnabled,
		TapToAccessRestrictionEnabled:   payload.TapToAccessRestrictionEnabled,
		TimeRestrictionTimeZone:         payload.TimeRestrictionTimeZone,
	}
}

type referenceGroupLockPayload struct {
	TenantID string `json:"tenant_id"`
	GroupID  string `json:"group_id"`
	LockID   string `json:"lock_id"`
	DoorID   string `json:"door_id"`
	PlaceID  string `json:"place_id"`
}

type referenceGroupLinkPayload struct {
	TenantID              string `json:"tenant_id"`
	GroupID               string `json:"group_id"`
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Phone                 string `json:"phone"`
	LinkEnabled           *bool  `json:"link_enabled"`
	QuickResponseCodeType string `json:"quick_response_code_type"`
	ValidFrom             string `json:"valid_from"`
	ValidUntil            string `json:"valid_until"`
}

type referenceGroupLinkPatchPayload struct {
	TenantID              string  `json:"tenant_id"`
	GroupID               *string `json:"group_id"`
	Name                  *string `json:"name"`
	Email                 *string `json:"email"`
	Phone                 *string `json:"phone"`
	LinkEnabled           *bool   `json:"link_enabled"`
	QuickResponseCodeType *string `json:"quick_response_code_type"`
	ValidFrom             *string `json:"valid_from"`
	ValidUntil            *string `json:"valid_until"`
}

type referenceGroupLinkMutationRequest struct {
	GroupLink             *referenceGroupLinkPatchPayload `json:"group_link"`
	TenantID              string                          `json:"tenant_id"`
	GroupID               *string                         `json:"group_id"`
	Name                  *string                         `json:"name"`
	Email                 *string                         `json:"email"`
	Phone                 *string                         `json:"phone"`
	LinkEnabled           *bool                           `json:"link_enabled"`
	QuickResponseCodeType *string                         `json:"quick_response_code_type"`
	ValidFrom             *string                         `json:"valid_from"`
	ValidUntil            *string                         `json:"valid_until"`
}

func (request referenceGroupLinkMutationRequest) payload() referenceGroupLinkPatchPayload {
	if request.GroupLink != nil {
		return *request.GroupLink
	}
	return referenceGroupLinkPatchPayload{
		TenantID:              request.TenantID,
		GroupID:               request.GroupID,
		Name:                  request.Name,
		Email:                 request.Email,
		Phone:                 request.Phone,
		LinkEnabled:           request.LinkEnabled,
		QuickResponseCodeType: request.QuickResponseCodeType,
		ValidFrom:             request.ValidFrom,
		ValidUntil:            request.ValidUntil,
	}
}

type referenceGroupLinkVerificationPayload struct {
	TenantID               string `json:"tenant_id"`
	Token                  string `json:"token"`
	Secret                 string `json:"secret"`
	QuickResponseCodeToken string `json:"quick_response_code_token"`
}

type referenceGroupLinkVerificationRequest struct {
	GroupLink              *referenceGroupLinkVerificationPayload `json:"group_link"`
	TenantID               string                                 `json:"tenant_id"`
	Token                  string                                 `json:"token"`
	Secret                 string                                 `json:"secret"`
	QuickResponseCodeToken string                                 `json:"quick_response_code_token"`
}

func (request referenceGroupLinkVerificationRequest) payload() referenceGroupLinkVerificationPayload {
	if request.GroupLink != nil {
		return *request.GroupLink
	}
	return referenceGroupLinkVerificationPayload{
		TenantID:               request.TenantID,
		Token:                  request.Token,
		Secret:                 request.Secret,
		QuickResponseCodeToken: request.QuickResponseCodeToken,
	}
}

type referenceGroupLinkVerificationResponse struct {
	Valid      bool             `json:"valid"`
	Status     string           `json:"status"`
	VerifiedAt string           `json:"verified_at"`
	ClaimedAt  string           `json:"claimed_at"`
	GroupLink  access.GroupLink `json:"group_link"`
}

type referenceGroupZone struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	GroupID   string `json:"group_id"`
	ZoneID    string `json:"zone_id"`
	PlaceID   string `json:"place_id"`
	FloorID   string `json:"floor_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type referenceController struct {
	ID           string   `json:"id"`
	ResourceType string   `json:"resource_type"`
	TenantID     string   `json:"tenant_id"`
	PlaceID      string   `json:"place_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	DeviceID     string   `json:"device_id"`
	Token        string   `json:"token"`
	Status       string   `json:"status"`
	Configured   bool     `json:"configured"`
	LockIDs      []string `json:"lock_ids,omitempty"`
	LastSeenAt   string   `json:"last_seen_at"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type referenceReader struct {
	ID           string   `json:"id"`
	ResourceType string   `json:"resource_type"`
	TenantID     string   `json:"tenant_id"`
	PlaceID      string   `json:"place_id"`
	ControllerID string   `json:"controller_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	DeviceID     string   `json:"device_id"`
	Token        string   `json:"token"`
	Model        string   `json:"model"`
	Protocol     string   `json:"protocol"`
	Status       string   `json:"status"`
	Configured   bool     `json:"configured"`
	LockIDs      []string `json:"lock_ids,omitempty"`
	LastSeenAt   string   `json:"last_seen_at"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type referenceControllerAssignPayload struct {
	TenantID       string `json:"tenant_id"`
	PlaceID        string `json:"place_id"`
	BuildingID     string `json:"building_id"`
	DeviceCapacity int    `json:"device_capacity"`
}

type referenceControllerAssignRequest struct {
	Controller     *referenceControllerAssignPayload `json:"controller"`
	TenantID       string                            `json:"tenant_id"`
	PlaceID        string                            `json:"place_id"`
	BuildingID     string                            `json:"building_id"`
	DeviceCapacity int                               `json:"device_capacity"`
}

func (request referenceControllerAssignRequest) payload() referenceControllerAssignPayload {
	if request.Controller != nil {
		return *request.Controller
	}
	return referenceControllerAssignPayload{
		TenantID:       request.TenantID,
		PlaceID:        request.PlaceID,
		BuildingID:     request.BuildingID,
		DeviceCapacity: request.DeviceCapacity,
	}
}

type referenceReaderAssignPayload struct {
	TenantID     string               `json:"tenant_id"`
	ControllerID string               `json:"controller_id"`
	GatewayID    string               `json:"gateway_id"`
	Kind         string               `json:"kind"`
	Source       string               `json:"source"`
	Protocol     string               `json:"protocol"`
	Status       string               `json:"status"`
	RS485Config  *gateway.RS485Config `json:"rs485_config"`
}

type referenceReaderAssignRequest struct {
	Reader       *referenceReaderAssignPayload `json:"reader"`
	TenantID     string                        `json:"tenant_id"`
	ControllerID string                        `json:"controller_id"`
	GatewayID    string                        `json:"gateway_id"`
	Kind         string                        `json:"kind"`
	Source       string                        `json:"source"`
	Protocol     string                        `json:"protocol"`
	Status       string                        `json:"status"`
	RS485Config  *gateway.RS485Config          `json:"rs485_config"`
}

func (request referenceReaderAssignRequest) payload() referenceReaderAssignPayload {
	if request.Reader != nil {
		return *request.Reader
	}
	return referenceReaderAssignPayload{
		TenantID:     request.TenantID,
		ControllerID: request.ControllerID,
		GatewayID:    request.GatewayID,
		Kind:         request.Kind,
		Source:       request.Source,
		Protocol:     request.Protocol,
		Status:       request.Status,
		RS485Config:  request.RS485Config,
	}
}

type referenceControllerLockRequest struct {
	TenantID string `json:"tenant_id"`
	LockID   string `json:"lock_id"`
}

type referenceControllerCommandRequest struct {
	Controller *struct {
		TenantID string `json:"tenant_id"`
		Version  string `json:"version"`
	} `json:"controller"`
	TenantID string `json:"tenant_id"`
	Version  string `json:"version"`
}

func (request referenceControllerCommandRequest) tenantID() string {
	if request.Controller != nil {
		return request.Controller.TenantID
	}
	return request.TenantID
}

func (request referenceControllerCommandRequest) version() string {
	if request.Controller != nil {
		return request.Controller.Version
	}
	return request.Version
}

type referenceTerminalPlace struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	Name         string `json:"name"`
}

type referenceTerminal struct {
	ID                        string                 `json:"id"`
	ResourceType              string                 `json:"resource_type"`
	TenantID                  string                 `json:"tenant_id"`
	CreatedAt                 string                 `json:"created_at"`
	UpdatedAt                 string                 `json:"updated_at"`
	Name                      string                 `json:"name"`
	Description               string                 `json:"description"`
	PlaceID                   string                 `json:"place_id"`
	Place                     referenceTerminalPlace `json:"place"`
	MarketplaceInstallationID *string                `json:"marketplace_installation_id"`
	ControllerID              string                 `json:"controller_id,omitempty"`
	ReaderID                  string                 `json:"reader_id,omitempty"`
	Status                    string                 `json:"status,omitempty"`
	LastSeenAt                string                 `json:"last_seen_at,omitempty"`
}

type referenceIntegration struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	TenantID     string `json:"tenant_id"`
	Type         string `json:"type"`
	Provider     string `json:"provider"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Configured   bool   `json:"configured"`
	SyncMode     string `json:"sync_mode,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	LastSyncAt   string `json:"last_sync_at,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type referenceIntegrationPayload struct {
	TenantID         string   `json:"tenant_id"`
	Type             string   `json:"type"`
	Provider         string   `json:"provider"`
	Status           string   `json:"status"`
	SyncMode         string   `json:"sync_mode"`
	CredentialRef    string   `json:"credential_ref"`
	WebhookSecretRef string   `json:"webhook_secret_ref"`
	IssuerURL        string   `json:"issuer_url"`
	ClientID         string   `json:"client_id"`
	AuthURL          string   `json:"auth_url"`
	TokenURL         string   `json:"token_url"`
	JWKSURL          string   `json:"jwks_url"`
	UserInfoURL      string   `json:"user_info_url"`
	SAMLACSURL       string   `json:"saml_acs_url"`
	SAMLX509Cert     string   `json:"saml_x509_cert"`
	Scopes           []string `json:"scopes"`
	Actor            string   `json:"actor"`
}

type referenceIntegrationMutationRequest struct {
	Integration      *referenceIntegrationPayload `json:"integration"`
	TenantID         string                       `json:"tenant_id"`
	Type             string                       `json:"type"`
	Provider         string                       `json:"provider"`
	Status           string                       `json:"status"`
	SyncMode         string                       `json:"sync_mode"`
	CredentialRef    string                       `json:"credential_ref"`
	WebhookSecretRef string                       `json:"webhook_secret_ref"`
	IssuerURL        string                       `json:"issuer_url"`
	ClientID         string                       `json:"client_id"`
	AuthURL          string                       `json:"auth_url"`
	TokenURL         string                       `json:"token_url"`
	JWKSURL          string                       `json:"jwks_url"`
	UserInfoURL      string                       `json:"user_info_url"`
	SAMLACSURL       string                       `json:"saml_acs_url"`
	SAMLX509Cert     string                       `json:"saml_x509_cert"`
	Scopes           []string                     `json:"scopes"`
	Actor            string                       `json:"actor"`
}

func (request referenceIntegrationMutationRequest) payload() referenceIntegrationPayload {
	if request.Integration != nil {
		return *request.Integration
	}
	return referenceIntegrationPayload{
		TenantID:         request.TenantID,
		Type:             request.Type,
		Provider:         request.Provider,
		Status:           request.Status,
		SyncMode:         request.SyncMode,
		CredentialRef:    request.CredentialRef,
		WebhookSecretRef: request.WebhookSecretRef,
		IssuerURL:        request.IssuerURL,
		ClientID:         request.ClientID,
		AuthURL:          request.AuthURL,
		TokenURL:         request.TokenURL,
		JWKSURL:          request.JWKSURL,
		UserInfoURL:      request.UserInfoURL,
		SAMLACSURL:       request.SAMLACSURL,
		SAMLX509Cert:     request.SAMLX509Cert,
		Scopes:           request.Scopes,
		Actor:            request.Actor,
	}
}

type referenceAlertPolicyChannels struct {
	Email    bool `json:"email"`
	WhatsApp bool `json:"whatsapp"`
}

type referenceAlertPolicy struct {
	ID              string                       `json:"id"`
	ResourceType    string                       `json:"resource_type"`
	TenantID        string                       `json:"tenant_id"`
	Name            string                       `json:"name"`
	Description     string                       `json:"description"`
	Category        string                       `json:"category"`
	Trigger         string                       `json:"trigger"`
	Severity        string                       `json:"severity"`
	Condition       string                       `json:"condition_expression,omitempty"`
	Status          string                       `json:"status"`
	Enabled         bool                         `json:"enabled"`
	Threshold       int                          `json:"threshold"`
	WindowSeconds   int64                        `json:"window_seconds"`
	CooldownSeconds int64                        `json:"cooldown_seconds"`
	Channels        referenceAlertPolicyChannels `json:"channels"`
	ReceiverGroups  []string                     `json:"receiver_groups"`
	UpdatedAt       string                       `json:"updated_at"`
}

type referenceAlertPolicyPayload struct {
	TenantID        string `json:"tenant_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	Trigger         string `json:"trigger"`
	Severity        string `json:"severity"`
	Condition       string `json:"condition_expression"`
	Status          string `json:"status"`
	Enabled         *bool  `json:"enabled"`
	Threshold       *int   `json:"threshold"`
	WindowSeconds   *int64 `json:"window_seconds"`
	CooldownSeconds *int64 `json:"cooldown_seconds"`
	Channels        *struct {
		Email    *bool `json:"email"`
		WhatsApp *bool `json:"whatsapp"`
	} `json:"channels"`
	ReceiverGroups []string `json:"receiver_groups"`
	Actor          string   `json:"actor"`
}

type referenceAlertPolicyMutationRequest struct {
	AlertPolicy     *referenceAlertPolicyPayload `json:"alert_policy"`
	TenantID        string                       `json:"tenant_id"`
	Name            string                       `json:"name"`
	Description     string                       `json:"description"`
	Category        string                       `json:"category"`
	Trigger         string                       `json:"trigger"`
	Severity        string                       `json:"severity"`
	Condition       string                       `json:"condition_expression"`
	Status          string                       `json:"status"`
	Enabled         *bool                        `json:"enabled"`
	Threshold       *int                         `json:"threshold"`
	WindowSeconds   *int64                       `json:"window_seconds"`
	CooldownSeconds *int64                       `json:"cooldown_seconds"`
	Channels        *struct {
		Email    *bool `json:"email"`
		WhatsApp *bool `json:"whatsapp"`
	} `json:"channels"`
	ReceiverGroups []string `json:"receiver_groups"`
	Actor          string   `json:"actor"`
}

func (request referenceAlertPolicyMutationRequest) payload() referenceAlertPolicyPayload {
	if request.AlertPolicy != nil {
		return *request.AlertPolicy
	}
	return referenceAlertPolicyPayload{
		TenantID:        request.TenantID,
		Name:            request.Name,
		Description:     request.Description,
		Category:        request.Category,
		Trigger:         request.Trigger,
		Severity:        request.Severity,
		Condition:       request.Condition,
		Status:          request.Status,
		Enabled:         request.Enabled,
		Threshold:       request.Threshold,
		WindowSeconds:   request.WindowSeconds,
		CooldownSeconds: request.CooldownSeconds,
		Channels:        request.Channels,
		ReceiverGroups:  request.ReceiverGroups,
		Actor:           request.Actor,
	}
}

type referenceCard struct {
	ID              string `json:"id"`
	ResourceType    string `json:"resource_type"`
	TenantID        string `json:"tenant_id"`
	Status          string `json:"status"`
	Token           string `json:"token"`
	UID             string `json:"uid,omitempty"`
	CardNumber      string `json:"card_number,omitempty"`
	Provider        string `json:"provider"`
	CredentialKind  string `json:"credential_kind"`
	TemplateID      string `json:"template_id"`
	UserID          string `json:"user_id,omitempty"`
	AssigneeType    string `json:"assignee_type,omitempty"`
	AssigneeID      string `json:"assignee_id,omitempty"`
	ActivationToken string `json:"activation_token,omitempty"`
	SaveLink        string `json:"save_link,omitempty"`
	LastUsedAt      string `json:"last_used_at,omitempty"`
	IssuedAt        string `json:"issued_at"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type referenceCardPayload struct {
	TenantID     string `json:"tenant_id"`
	TemplateID   string `json:"template_id"`
	Token        string `json:"token"`
	UID          string `json:"uid"`
	CardNumber   string `json:"card_number"`
	Type         string `json:"type"`
	AssigneeType string `json:"assignee_type"`
	AssigneeID   string `json:"assignee_id"`
	UserID       string `json:"user_id"`
	GuestID      string `json:"guest_id"`
	Email        string `json:"email"`
	ExpiresAt    string `json:"expires_at"`
}

type referenceCardAssignmentPayload struct {
	TenantID     string `json:"tenant_id"`
	CardID       string `json:"card_id"`
	AssigneeType string `json:"assignee_type"`
	AssigneeID   string `json:"assignee_id"`
	UserID       string `json:"user_id"`
	GuestID      string `json:"guest_id"`
	Email        string `json:"email"`
}

type referenceCardAssignment struct {
	ID           string        `json:"id"`
	ResourceType string        `json:"resource_type"`
	TenantID     string        `json:"tenant_id"`
	Status       string        `json:"status"`
	CardID       string        `json:"card_id"`
	Card         referenceCard `json:"card"`
	AssigneeType string        `json:"assignee_type"`
	AssigneeID   string        `json:"assignee_id"`
	UserID       string        `json:"user_id,omitempty"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

type referenceEventSetPayload struct {
	Interval        string `json:"interval"`
	EventPlaceID    string `json:"event_place_id"`
	PlaceID         string `json:"place_id"`
	EventType       string `json:"event_type"`
	EventUUID       string `json:"event_uuid"`
	EventSuccess    *bool  `json:"event_success"`
	EventObjectID   string `json:"event_object_id"`
	EventObjectType string `json:"event_object_type"`
}

type referenceEventSet struct {
	ID              string           `json:"id"`
	CreatedAt       string           `json:"created_at"`
	Status          string           `json:"status"`
	Interval        string           `json:"interval,omitempty"`
	EventPlaceID    string           `json:"event_place_id,omitempty"`
	PlaceID         string           `json:"place_id,omitempty"`
	EventType       string           `json:"event_type,omitempty"`
	EventUUID       string           `json:"event_uuid,omitempty"`
	EventSuccess    *bool            `json:"event_success,omitempty"`
	EventObjectID   string           `json:"event_object_id,omitempty"`
	EventObjectType string           `json:"event_object_type,omitempty"`
	Events          []referenceEvent `json:"events"`
	Cursor          string           `json:"cursor,omitempty"`
}

type referenceEvent struct {
	UUID       string `json:"uuid"`
	TenantID   string `json:"tenant_id,omitempty"`
	Type       string `json:"type"`
	ActorType  string `json:"actor_type,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	ActorName  string `json:"actor_name,omitempty"`
	ActorEmail string `json:"actor_email,omitempty"`
	ObjectType string `json:"object_type,omitempty"`
	ObjectID   string `json:"object_id,omitempty"`
	ObjectName string `json:"object_name,omitempty"`
	PlaceID    string `json:"place_id,omitempty"`
	AreaID     string `json:"area_id,omitempty"`
	LockID     string `json:"lock_id,omitempty"`
	GatewayID  string `json:"gateway_id,omitempty"`
	Success    bool   `json:"success"`
	Result     string `json:"result"`
	Detail     string `json:"detail,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func (s *server) listReferencePlaces(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	items := s.spaceSvc.ListBuildings(tenantID)
	if referenceIncludeArchivedPlaces(r) {
		items = s.spaceSvc.ListBuildingsIncludingArchived(tenantID)
	}
	if buildingScope != nil {
		items = filterBuildingsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterBuildingsByID(items, placeID)
	}
	if status != "" {
		items = filterBuildingsByStatus(items, status)
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferencePlace(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Place struct {
			TenantID string `json:"tenant_id"`
			Name     string `json:"name"`
			Address  string `json:"address"`
			Region   string `json:"region"`
		} `json:"place"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Place.TenantID)
	if !ok {
		return
	}
	created, err := s.spaceSvc.CreateBuilding(tenantID, request.Place.Name, request.Place.Address, request.Place.Region)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired), errors.Is(err, space.ErrBuildingNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getReferencePlace(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	var (
		record space.Building
		err    error
	)
	if referenceIncludeArchivedPlaces(r) {
		record, err = s.spaceSvc.GetBuildingIncludingArchived(tenantID, chi.URLParam(r, "placeID"))
	} else {
		record, err = s.spaceSvc.GetBuilding(tenantID, chi.URLParam(r, "placeID"))
	}
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.ID) {
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) updateReferencePlace(w http.ResponseWriter, r *http.Request) {
	var request referencePlaceMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(payload.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	placeID := chi.URLParam(r, "placeID")
	current, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.ID) {
		return
	}

	name := current.Name
	if payload.Name != nil {
		name = *payload.Name
	}
	address := current.Address
	if payload.Address != nil {
		address = *payload.Address
	}
	region := current.Region
	if payload.Region != nil {
		region = *payload.Region
	}
	updated, err := s.spaceSvc.UpdateBuilding(tenantID, placeID, name, address, region)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteReferencePlace(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	placeID := chi.URLParam(r, "placeID")
	current, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.ID) {
		return
	}
	if err := s.spaceSvc.DeleteBuilding(tenantID, placeID); err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_place_deleted", fmt.Sprintf("place_id=%s,name=%s", current.ID, current.Name), "space")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) lockDownReferencePlace(w http.ResponseWriter, r *http.Request) {
	s.writeReferencePlaceAction(w, r, "lock_down")
}

func (s *server) cancelReferencePlaceLockdown(w http.ResponseWriter, r *http.Request) {
	s.writeReferencePlaceAction(w, r, "cancel_lockdown")
}

func (s *server) writeReferencePlaceAction(w http.ResponseWriter, r *http.Request, action string) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	placeID := chi.URLParam(r, "placeID")
	record, err := s.spaceSvc.GetBuilding(tenantID, placeID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.ID) {
		return
	}
	lockCount := 0
	for _, door := range s.spaceSvc.ListDoors(tenantID) {
		if door.BuildingID == record.ID {
			lockCount++
		}
	}
	writeJSON(w, http.StatusOK, referenceSpaceActionResponse{
		ID:           fmt.Sprintf("%s:%s:%d", record.ID, action, time.Now().UTC().UnixNano()),
		ResourceType: "PlaceAction",
		TenantID:     record.TenantID,
		PlaceID:      record.ID,
		Action:       action,
		Status:       "accepted",
		LockCount:    lockCount,
		CreatedAt:    time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (s *server) listReferenceLocks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListDoors(tenantID)
	if buildingScope != nil {
		items = filterDoorsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterDoorsByPlaceID(items, placeID)
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferenceLock(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Lock struct {
			TenantID   string `json:"tenant_id"`
			PlaceID    string `json:"place_id"`
			BuildingID string `json:"building_id"`
			FloorID    string `json:"floor_id"`
			AreaID     string `json:"area_id"`
			Name       string `json:"name"`
			GatewayID  string `json:"gateway_id"`
			Kind       string `json:"kind"`
			Status     string `json:"status"`
			Mode       string `json:"mode"`
			State      string `json:"state"`
		} `json:"lock"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Lock.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(request.Lock.PlaceID, request.Lock.BuildingID)
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	kind := firstNonEmptyString(request.Lock.Kind, request.Lock.Mode)
	status := firstNonEmptyString(request.Lock.Status, request.Lock.State)

	created, err := s.spaceSvc.CreateDoor(
		tenantID,
		buildingID,
		request.Lock.FloorID,
		request.Lock.AreaID,
		request.Lock.Name,
		request.Lock.GatewayID,
		kind,
		status,
	)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getReferenceLock(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	record, err := s.spaceSvc.GetDoor(tenantID, chi.URLParam(r, "lockID"))
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) updateReferenceLock(w http.ResponseWriter, r *http.Request) {
	var request referenceLockMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(payload.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	lockID := chi.URLParam(r, "lockID")
	current, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}

	buildingID := current.BuildingID
	if payload.BuildingID != nil {
		buildingID = *payload.BuildingID
	}
	if payload.PlaceID != nil {
		buildingID = *payload.PlaceID
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	floorID := current.FloorID
	if payload.FloorID != nil {
		floorID = *payload.FloorID
	}
	areaID := current.AreaID
	if payload.AreaID != nil {
		areaID = *payload.AreaID
	}
	name := current.Name
	if payload.Name != nil {
		name = *payload.Name
	}
	gatewayID := current.GatewayID
	if payload.GatewayID != nil {
		gatewayID = *payload.GatewayID
	}
	kind := current.Kind
	if payload.Kind != nil {
		kind = *payload.Kind
	}
	if payload.Mode != nil {
		kind = *payload.Mode
	}
	status := current.Status
	if payload.Status != nil {
		status = *payload.Status
	}
	if payload.State != nil {
		status = *payload.State
	}

	updated, err := s.spaceSvc.UpdateDoor(tenantID, lockID, buildingID, floorID, areaID, name, gatewayID, kind, status)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteReferenceLock(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	lockID := chi.URLParam(r, "lockID")
	current, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}
	if err := s.spaceSvc.DeleteDoor(tenantID, lockID); err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_lock_deleted", fmt.Sprintf("lock_id=%s,place_id=%s,name=%s", current.ID, current.BuildingID, current.Name), "space")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) unlockReferenceLock(w http.ResponseWriter, r *http.Request) {
	s.writeReferenceLockAction(w, r, "unlock")
}

func (s *server) lockDownReferenceLock(w http.ResponseWriter, r *http.Request) {
	s.writeReferenceLockAction(w, r, "lock_down")
}

func (s *server) cancelReferenceLockLockdown(w http.ResponseWriter, r *http.Request) {
	s.writeReferenceLockAction(w, r, "cancel_lockdown")
}

func (s *server) writeReferenceLockAction(w http.ResponseWriter, r *http.Request, action string) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	lockID := chi.URLParam(r, "lockID")
	record, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, referenceSpaceActionResponse{
		ID:           fmt.Sprintf("%s:%s:%d", record.ID, action, time.Now().UTC().UnixNano()),
		ResourceType: "LockAction",
		TenantID:     record.TenantID,
		PlaceID:      record.BuildingID,
		LockID:       record.ID,
		Action:       action,
		Status:       "accepted",
		CreatedAt:    time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (s *server) listReferenceGroups(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListUserGroups(tenantID)
	if buildingScope != nil {
		items = filterUserGroupsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterUserGroupsByPlaceID(items, placeID)
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) listReferenceDoorGroups(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListDoorGroups(tenantID)
	if buildingScope != nil {
		items = filterDoorGroupsByScope(items, allowedDoorIDsByBuildingScope(s.spaceSvc.ListDoors(tenantID), buildingScope))
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferenceGroup(w http.ResponseWriter, r *http.Request) {
	var request referenceGroupMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(payload.PlaceID, payload.BuildingID)
	if buildingScope != nil && buildingID == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for Place Admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	members := payload.MemberIDs
	if len(members) == 0 {
		members = payload.Members
	}
	description := ""
	if payload.Description != nil {
		description = *payload.Description
	}
	created, err := s.accessSvc.CreateUserGroup(tenantID, buildingID, payload.Name, description, members)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired), errors.Is(err, access.ErrUserGroupNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if referenceGroupPayloadHasRestrictions(payload) {
		created, err = s.accessSvc.UpdateUserGroupRestrictions(tenantID, created.ID, referenceGroupRestrictionsInput(payload))
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getReferenceGroup(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	group, err := s.accessSvc.GetUserGroup(tenantID, chi.URLParam(r, "groupID"))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, group.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (s *server) updateReferenceGroup(w http.ResponseWriter, r *http.Request) {
	var request referenceGroupMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(payload.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupID")
	current, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}

	buildingID := firstNonEmptyString(payload.PlaceID, payload.BuildingID, current.BuildingID)
	if buildingScope != nil && buildingID == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for Place Admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	name := firstNonEmptyString(payload.Name, current.Name)
	description := current.Description
	if payload.Description != nil {
		description = *payload.Description
	}
	members := payload.MemberIDs
	if len(members) == 0 {
		members = payload.Members
	}
	if len(members) == 0 {
		members = current.Members
	}
	updated, err := s.accessSvc.UpdateUserGroup(tenantID, groupID, buildingID, name, description, members)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if referenceGroupPayloadHasRestrictions(payload) {
		updated, err = s.accessSvc.UpdateUserGroupRestrictions(tenantID, groupID, referenceGroupRestrictionsInput(payload))
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteReferenceGroup(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupID")
	current, err := s.accessSvc.GetUserGroup(tenantID, groupID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}
	if err := s.accessSvc.DeleteUserGroup(tenantID, groupID); err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_group_deleted", fmt.Sprintf("group_id=%s,place_id=%s,name=%s", current.ID, current.BuildingID, current.Name), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceGroupLocks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	doors := s.spaceSvc.ListDoors(tenantID)
	if buildingScope != nil {
		allowedDoorIDs := allowedDoorIDsByBuildingScope(doors, buildingScope)
		doorGroups = filterDoorGroupsByScope(doorGroups, allowedDoorIDs)
		doors = filterDoorsByScope(doors, buildingScope)
	}
	doorsByID := make(map[string]space.Door, len(doors))
	for i := range doors {
		doorsByID[doors[i].ID] = doors[i]
	}
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	lockID := strings.TrimSpace(r.URL.Query().Get("lock_id"))
	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))

	items := make([]referenceGroupLock, 0)
	for i := range doorGroups {
		if groupID != "" && doorGroups[i].ID != groupID {
			continue
		}
		for _, doorID := range doorGroups[i].DoorIDs {
			if lockID != "" && doorID != lockID {
				continue
			}
			door, exists := doorsByID[doorID]
			if !exists {
				continue
			}
			if placeID != "" && door.BuildingID != placeID {
				continue
			}
			items = append(items, referenceGroupLock{
				ID:        doorGroups[i].ID + ":" + doorID,
				TenantID:  doorGroups[i].TenantID,
				GroupID:   doorGroups[i].ID,
				LockID:    doorID,
				CreatedAt: doorGroups[i].CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferenceGroupLock(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GroupLock referenceGroupLockPayload `json:"group_lock"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.GroupLock.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	lockID := firstNonEmptyString(request.GroupLock.LockID, request.GroupLock.DoorID)
	if !s.requireReferenceGroupLockScope(w, tenantID, buildingScope, lockID) {
		return
	}
	updated, err := s.spaceSvc.AddDoorToGroup(tenantID, request.GroupLock.GroupID, lockID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, referenceGroupLockFromDoorGroup(updated, lockID))
}

func (s *server) deleteReferenceGroupLock(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupID, lockID := referenceGroupLockParts(chi.URLParam(r, "groupLockID"))
	if groupID == "" {
		groupID = strings.TrimSpace(r.URL.Query().Get("group_id"))
	}
	if lockID == "" {
		lockID = firstNonEmptyString(r.URL.Query().Get("lock_id"), r.URL.Query().Get("door_id"))
	}
	if !s.requireReferenceGroupLockScope(w, tenantID, buildingScope, lockID) {
		return
	}
	if _, err := s.spaceSvc.RemoveDoorFromGroup(tenantID, groupID, lockID); err != nil {
		handleSpaceReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_group_lock_deleted", fmt.Sprintf("group_id=%s,lock_id=%s", groupID, lockID), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceGroupZones(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	doors := s.spaceSvc.ListDoors(tenantID)
	areas := s.spaceSvc.ListAreas(tenantID)
	if buildingScope != nil {
		allowedDoorIDs := allowedDoorIDsByBuildingScope(doors, buildingScope)
		doorGroups = filterDoorGroupsByScope(doorGroups, allowedDoorIDs)
		doors = filterDoorsByScope(doors, buildingScope)
		areas = filterAreasByScope(areas, buildingScope)
	}
	doorsByID := make(map[string]space.Door, len(doors))
	for i := range doors {
		doorsByID[doors[i].ID] = doors[i]
	}
	areasByID := make(map[string]space.Area, len(areas))
	for i := range areas {
		areasByID[areas[i].ID] = areas[i]
	}
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	zoneID := firstNonEmptyString(r.URL.Query().Get("zone_id"), r.URL.Query().Get("area_id"))
	placeID := firstNonEmptyString(r.URL.Query().Get("place_id"), r.URL.Query().Get("building_id"))

	items := make([]referenceGroupZone, 0)
	for i := range doorGroups {
		if groupID != "" && doorGroups[i].ID != groupID {
			continue
		}
		seenZoneIDs := make(map[string]struct{})
		for _, doorID := range doorGroups[i].DoorIDs {
			door, doorExists := doorsByID[doorID]
			if !doorExists {
				continue
			}
			area, areaExists := areasByID[door.AreaID]
			if !areaExists {
				continue
			}
			if zoneID != "" && area.ID != zoneID {
				continue
			}
			if placeID != "" && area.BuildingID != placeID {
				continue
			}
			if _, exists := seenZoneIDs[area.ID]; exists {
				continue
			}
			seenZoneIDs[area.ID] = struct{}{}
			items = append(items, referenceGroupZone{
				ID:        doorGroups[i].ID + ":" + area.ID,
				TenantID:  doorGroups[i].TenantID,
				GroupID:   doorGroups[i].ID,
				ZoneID:    area.ID,
				PlaceID:   area.BuildingID,
				FloorID:   area.FloorID,
				Name:      area.Name,
				CreatedAt: doorGroups[i].CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) listReferenceGroupLinks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	idFilter := commaSet(r.URL.Query().Get("ids"))

	items := make([]access.GroupLink, 0)
	for _, link := range s.accessSvc.ListGroupLinks(tenantID) {
		if len(idFilter) > 0 {
			if _, exists := idFilter[link.ID]; !exists {
				continue
			}
		}
		if groupID != "" && link.GroupID != groupID {
			continue
		}
		group, err := s.accessSvc.GetUserGroup(link.TenantID, link.GroupID)
		if err != nil {
			continue
		}
		if !s.allowedBuildingScope(buildingScope, group.BuildingID) {
			continue
		}
		link.GroupName = group.Name
		if query != "" && !groupLinkMatchesQuery(link, query) {
			continue
		}
		items = append(items, link)
	}
	sortGroupLinks(items, r)
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferenceGroupLink(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GroupLink referenceGroupLinkPayload `json:"group_link"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.GroupLink.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	group, err := s.accessSvc.GetUserGroup(tenantID, request.GroupLink.GroupID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, group.BuildingID) {
		return
	}
	createdByType := "User"
	createdByID := ""
	createdByEmail := ""
	createdByName := ""
	if user, ok := authenticatedUser(r); ok {
		createdByID = strings.TrimSpace(user.ID)
		createdByEmail = strings.TrimSpace(user.Email)
		createdByName = strings.TrimSpace(user.Email)
	}
	created, err := s.accessSvc.CreateGroupLink(access.GroupLinkInput{
		TenantID:              tenantID,
		GroupID:               request.GroupLink.GroupID,
		Name:                  request.GroupLink.Name,
		Email:                 request.GroupLink.Email,
		Phone:                 request.GroupLink.Phone,
		LinkEnabled:           request.GroupLink.LinkEnabled,
		QuickResponseCodeType: request.GroupLink.QuickResponseCodeType,
		ValidFrom:             request.GroupLink.ValidFrom,
		ValidUntil:            request.GroupLink.ValidUntil,
		CreatedByType:         createdByType,
		CreatedByID:           createdByID,
		CreatedByEmail:        createdByEmail,
		CreatedByName:         createdByName,
	})
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getReferenceGroupLink(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupLinkID := chi.URLParam(r, "groupLinkID")
	link, err := s.accessSvc.GetGroupLink(tenantID, groupLinkID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	group, err := s.accessSvc.GetUserGroup(link.TenantID, link.GroupID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, group.BuildingID) {
		return
	}
	link.GroupName = group.Name
	writeJSON(w, http.StatusOK, link)
}

func (s *server) updateReferenceGroupLink(w http.ResponseWriter, r *http.Request) {
	var request referenceGroupLinkMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(payload.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupLinkID := chi.URLParam(r, "groupLinkID")
	link, err := s.accessSvc.GetGroupLink(tenantID, groupLinkID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	group, err := s.accessSvc.GetUserGroup(link.TenantID, link.GroupID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, group.BuildingID) {
		return
	}
	if payload.GroupID != nil && strings.TrimSpace(*payload.GroupID) != "" {
		targetGroup, err := s.accessSvc.GetUserGroup(link.TenantID, strings.TrimSpace(*payload.GroupID))
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
		if !s.requireBuildingScope(w, buildingScope, targetGroup.BuildingID) {
			return
		}
	}
	updated, err := s.accessSvc.UpdateGroupLink(tenantID, groupLinkID, access.GroupLinkUpdateInput{
		TenantID:              tenantID,
		GroupID:               payload.GroupID,
		Name:                  payload.Name,
		Email:                 payload.Email,
		Phone:                 payload.Phone,
		LinkEnabled:           payload.LinkEnabled,
		QuickResponseCodeType: payload.QuickResponseCodeType,
		ValidFrom:             payload.ValidFrom,
		ValidUntil:            payload.ValidUntil,
	})
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteReferenceGroupLink(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	groupLinkID := chi.URLParam(r, "groupLinkID")
	link, err := s.accessSvc.GetGroupLink(tenantID, groupLinkID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	group, err := s.accessSvc.GetUserGroup(link.TenantID, link.GroupID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, group.BuildingID) {
		return
	}
	if err := s.accessSvc.DeleteGroupLink(link.TenantID, groupLinkID); err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, link.TenantID, "reference_group_link_deleted", fmt.Sprintf("group_link_id=%s,group_id=%s,name=%s", link.ID, link.GroupID, link.Name), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) verifyReferenceGroupLinkToken(w http.ResponseWriter, r *http.Request) {
	payload := referenceGroupLinkVerificationPayload{
		TenantID:               r.URL.Query().Get("tenant_id"),
		Token:                  r.URL.Query().Get("token"),
		Secret:                 r.URL.Query().Get("secret"),
		QuickResponseCodeToken: r.URL.Query().Get("quick_response_code_token"),
	}
	if r.Method != http.MethodGet {
		var request referenceGroupLinkVerificationRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		bodyPayload := request.payload()
		payload.TenantID = firstNonEmptyString(bodyPayload.TenantID, payload.TenantID)
		payload.Token = firstNonEmptyString(bodyPayload.Token, payload.Token)
		payload.Secret = firstNonEmptyString(bodyPayload.Secret, payload.Secret)
		payload.QuickResponseCodeToken = firstNonEmptyString(bodyPayload.QuickResponseCodeToken, payload.QuickResponseCodeToken)
	}

	verifiedAt := time.Now().UTC()
	link, err := s.accessSvc.VerifyGroupLinkToken(
		payload.TenantID,
		firstNonEmptyString(payload.Token, payload.Secret, payload.QuickResponseCodeToken),
		verifiedAt,
	)
	if err != nil {
		handleGroupLinkVerificationError(w, err)
		return
	}
	if group, err := s.accessSvc.GetUserGroup(link.TenantID, link.GroupID); err == nil {
		link.GroupName = group.Name
	}
	s.appendAuditLog(
		r,
		link.TenantID,
		"reference_group_link_claimed",
		fmt.Sprintf("group_link_id=%s,group_id=%s,email=%s,last_used_at=%s", link.ID, link.GroupID, link.Email, link.LastUsedAt),
		"access_link",
	)
	link.Secret = ""
	link.QuickResponseCodeToken = ""
	claimedAt := firstNonEmptyString(link.LastUsedAt, verifiedAt.Format(time.RFC3339))
	writeJSON(w, http.StatusOK, referenceGroupLinkVerificationResponse{
		Valid:      true,
		Status:     "valid",
		VerifiedAt: verifiedAt.Format(time.RFC3339),
		ClaimedAt:  claimedAt,
		GroupLink:  link,
	})
}

func (s *server) listReferenceControllers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	controllers := make([]referenceController, 0, len(items))
	for i := range items {
		controller := referenceControllerFromGateway(items[i])
		if !controllerMatchesQuery(controller, r) {
			continue
		}
		controllers = append(controllers, controller)
	}
	sortReferenceControllers(controllers, r)
	writeCollection(w, r, http.StatusOK, controllers)
}

func (s *server) listReferenceReaders(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	readers := make([]referenceReader, 0)
	for i := range items {
		for j := range items[i].Devices {
			if !isReaderDevice(items[i].Devices[j]) {
				continue
			}
			reader := referenceReaderFromGatewayDevice(items[i], items[i].Devices[j])
			if !readerMatchesQuery(reader, r) {
				continue
			}
			readers = append(readers, reader)
		}
	}
	sortReferenceReaders(readers, r)
	writeCollection(w, r, http.StatusOK, readers)
}

func (s *server) listReferenceTerminals(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	places := map[string]space.Building{}
	for _, place := range s.spaceSvc.ListBuildings(tenantID) {
		places[place.ID] = place
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	terminals := make([]referenceTerminal, 0)
	for i := range items {
		placeName := items[i].BuildingID
		if place, exists := places[items[i].BuildingID]; exists {
			placeName = place.Name
		}
		for j := range items[i].Devices {
			if !isReaderDevice(items[i].Devices[j]) {
				continue
			}
			terminal := referenceTerminalFromGatewayDevice(items[i], items[i].Devices[j], placeName)
			if !terminalMatchesQuery(terminal, r) {
				continue
			}
			terminals = append(terminals, terminal)
		}
	}
	sortReferenceTerminals(terminals, r)
	writeCollection(w, r, http.StatusOK, terminals)
}

func (s *server) getReferenceTerminal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, chi.URLParam(r, "terminalID"))
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "terminal not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}
	placeName := controller.BuildingID
	if place, err := s.spaceSvc.GetBuilding(tenantID, controller.BuildingID); err == nil {
		placeName = place.Name
	}
	writeJSON(w, http.StatusOK, referenceTerminalFromGatewayDevice(controller, device, placeName))
}

func (s *server) assignReferenceController(w http.ResponseWriter, r *http.Request) {
	var request referenceControllerAssignRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(payload.PlaceID, payload.BuildingID)
	if buildingScope != nil && strings.TrimSpace(buildingID) == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for Place Admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}

	created, err := s.gatewaySvc.Register(chi.URLParam(r, "controllerToken"), tenantID, buildingID, payload.DeviceCapacity)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_assigned", fmt.Sprintf("controller_id=%s,place_id=%s,device_id=%s", created.ID, created.BuildingID, created.SerialNumber), "gateway")
	writeJSON(w, http.StatusCreated, referenceControllerFromGateway(created))
}

func (s *server) assignReferenceReader(w http.ResponseWriter, r *http.Request) {
	var request referenceReaderAssignRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	controllerID := firstNonEmptyString(payload.ControllerID, payload.GatewayID)
	controller, exists := s.findGatewayByTenant(tenantID, controllerID)
	if !exists {
		writeError(w, http.StatusNotFound, "controller not found")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}

	updated, err := s.gatewaySvc.RegisterDevice(
		tenantID,
		controllerID,
		chi.URLParam(r, "readerToken"),
		firstNonEmptyString(payload.Kind, "reader"),
		payload.Source,
		payload.Status,
		payload.Protocol,
		payload.RS485Config,
	)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	device, exists := gatewayDeviceBySerial(updated, chi.URLParam(r, "readerToken"))
	if !exists {
		writeError(w, http.StatusInternalServerError, "reader assignment missing device")
		return
	}
	s.appendAuditLog(r, tenantID, "reference_reader_assigned", fmt.Sprintf("reader_id=%s,controller_id=%s,device_id=%s,kind=%s,protocol=%s", device.ID, updated.ID, device.SerialNumber, device.Kind, device.Protocol), "gateway")
	writeJSON(w, http.StatusCreated, referenceReaderFromGatewayDevice(updated, device))
}

func (s *server) deassignReferenceController(w http.ResponseWriter, r *http.Request) {
	controllerID := chi.URLParam(r, "controllerID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if !s.requireReferenceControllerScope(w, r, tenantID, controllerID) {
		return
	}

	removed, err := s.gatewaySvc.Deassign(tenantID, controllerID)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_deassigned", fmt.Sprintf("controller_id=%s,place_id=%s,device_id=%s", removed.ID, removed.BuildingID, removed.SerialNumber), "gateway")
	writeJSON(w, http.StatusOK, referenceControllerFromGateway(removed))
}

func (s *server) deassignReferenceReader(w http.ResponseWriter, r *http.Request) {
	readerID := chi.URLParam(r, "readerID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, readerID)
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "reader not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}

	parent, removed, err := s.gatewaySvc.DeassignDevice(tenantID, readerID)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_reader_deassigned", fmt.Sprintf("reader_id=%s,controller_id=%s,device_id=%s", removed.ID, parent.ID, removed.SerialNumber), "gateway")
	writeJSON(w, http.StatusOK, referenceReaderFromGatewayDevice(parent, removed))
}

func (s *server) bindReferenceControllerLock(w http.ResponseWriter, r *http.Request) {
	controllerID := chi.URLParam(r, "controllerID")
	var request referenceControllerLockRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	if !s.requireReferenceControllerScope(w, r, tenantID, controllerID) {
		return
	}
	if !s.requireReferenceLockScope(w, r, tenantID, request.LockID) {
		return
	}

	updated, err := s.gatewaySvc.BindDoor(tenantID, controllerID, request.LockID)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_lock_bound", fmt.Sprintf("controller_id=%s,lock_id=%s", updated.ID, request.LockID), "gateway")
	writeJSON(w, http.StatusOK, referenceControllerFromGateway(updated))
}

func (s *server) unbindReferenceControllerLock(w http.ResponseWriter, r *http.Request) {
	controllerID := chi.URLParam(r, "controllerID")
	lockID := chi.URLParam(r, "lockID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if !s.requireReferenceControllerScope(w, r, tenantID, controllerID) {
		return
	}
	if !s.requireReferenceLockScope(w, r, tenantID, lockID) {
		return
	}

	if _, err := s.gatewaySvc.UnbindDoor(tenantID, controllerID, lockID); err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_lock_unbound", fmt.Sprintf("controller_id=%s,lock_id=%s", controllerID, lockID), "gateway")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) publishReferenceControllerConfig(w http.ResponseWriter, r *http.Request) {
	var request referenceControllerCommandRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	controllerID := chi.URLParam(r, "controllerID")
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.tenantID(), r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	if !s.requireReferenceControllerScope(w, r, tenantID, controllerID) {
		return
	}
	ack, err := s.gatewaySvc.PublishConfig(tenantID, controllerID, request.version())
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_config_published", fmt.Sprintf("controller_id=%s,version=%s,task_id=%s", controllerID, strings.TrimSpace(request.version()), ack.TaskID), "gateway")
	writeJSON(w, http.StatusAccepted, ack)
}

func (s *server) rebootReferenceController(w http.ResponseWriter, r *http.Request) {
	controllerID := chi.URLParam(r, "controllerID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if !s.requireReferenceControllerScope(w, r, tenantID, controllerID) {
		return
	}
	ack, err := s.gatewaySvc.Reboot(tenantID, controllerID)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_controller_reboot_queued", fmt.Sprintf("controller_id=%s,task_id=%s", controllerID, ack.TaskID), "gateway")
	writeJSON(w, http.StatusAccepted, ack)
}

func (s *server) rebootReferenceReader(w http.ResponseWriter, r *http.Request) {
	s.rebootReferenceDeviceParentController(w, r, chi.URLParam(r, "readerID"), "reader")
}

func (s *server) rebootReferenceTerminal(w http.ResponseWriter, r *http.Request) {
	s.rebootReferenceDeviceParentController(w, r, chi.URLParam(r, "terminalID"), "terminal")
}

func (s *server) triggerReferenceTerminal(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, chi.URLParam(r, "terminalID"))
	if !exists || !isReaderDevice(device) {
		writeError(w, http.StatusNotFound, "terminal not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}
	s.appendAuditLog(r, tenantID, "reference_terminal_triggered", fmt.Sprintf("terminal_id=terminal_%s,reader_id=%s,controller_id=%s,device_id=%s", device.ID, device.ID, controller.ID, device.SerialNumber), "gateway")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) rebootReferenceDeviceParentController(w http.ResponseWriter, r *http.Request, deviceID, label string) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	controller, device, exists := s.referenceGatewayDeviceByID(tenantID, deviceID)
	if !exists {
		writeError(w, http.StatusNotFound, label+" not found")
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !s.requireBuildingScope(w, buildingScope, controller.BuildingID) {
		return
	}
	ack, err := s.gatewaySvc.Reboot(tenantID, controller.ID)
	if err != nil {
		handleReferenceGatewayMutationError(w, err)
		return
	}
	targetID := device.ID
	if label == "terminal" {
		targetID = "terminal_" + device.ID
	}
	s.appendAuditLog(r, tenantID, "reference_"+label+"_reboot_queued", fmt.Sprintf("%s_id=%s,controller_id=%s,task_id=%s", label, targetID, controller.ID, ack.TaskID), "gateway")
	writeJSON(w, http.StatusAccepted, ack)
}

func (s *server) listReferenceIntegrations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	integrations := make([]referenceIntegration, 0)
	if tenantID != "" {
		if config, err := s.enterpriseSvc.GetIDPConfig(tenantID); err == nil {
			integration := referenceIntegrationFromIDPConfig(config)
			if integrationMatchesQuery(integration, r) {
				integrations = append(integrations, integration)
			}
		} else if !errors.Is(err, enterprise.ErrIDPConfigNotFound) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	connectors := s.enterpriseSvc.ListHRISConnectors(tenantID)
	for i := range connectors {
		integration := referenceIntegrationFromHRISConnector(connectors[i])
		if !integrationMatchesQuery(integration, r) {
			continue
		}
		integrations = append(integrations, integration)
	}
	sortReferenceIntegrations(integrations, r)
	writeCollection(w, r, http.StatusOK, integrations)
}

func (s *server) getReferenceIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	integration, found, err := s.referenceIntegrationByID(tenantID, chi.URLParam(r, "integrationID"))
	if err != nil {
		handleReferenceIntegrationMutationError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "integration not found")
		return
	}
	writeJSON(w, http.StatusOK, integration)
}

func (s *server) createReferenceIntegration(w http.ResponseWriter, r *http.Request) {
	var request referenceIntegrationMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	if strings.TrimSpace(tenantID) == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	payload.Actor = s.referenceIntegrationActor(r, payload.Actor)

	integrationType := normalizeReferenceIntegrationType(payload.Type)
	switch integrationType {
	case "identity_provider":
		config, err := s.enterpriseSvc.UpsertIDPConfig(
			tenantID,
			payload.Provider,
			payload.IssuerURL,
			payload.ClientID,
			payload.AuthURL,
			payload.TokenURL,
			payload.JWKSURL,
			payload.UserInfoURL,
			payload.SAMLACSURL,
			payload.SAMLX509Cert,
			payload.Status,
			payload.SyncMode,
			payload.Actor,
			payload.Scopes,
		)
		if err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
		integration := referenceIntegrationFromIDPConfig(config)
		s.appendAuditLog(r, tenantID, "reference_integration_created", fmt.Sprintf("integration_id=%s,type=%s,provider=%s", integration.ID, integration.Type, integration.Provider), "identity_provider")
		writeJSON(w, http.StatusCreated, integration)
	case "hris":
		created, err := s.enterpriseSvc.CreateHRISConnector(
			tenantID,
			payload.Provider,
			payload.Status,
			payload.SyncMode,
			payload.CredentialRef,
			payload.WebhookSecretRef,
			payload.Actor,
		)
		if err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
		integration := referenceIntegrationFromHRISConnector(created)
		s.appendAuditLog(r, tenantID, "reference_integration_created", fmt.Sprintf("integration_id=%s,type=%s,provider=%s", integration.ID, integration.Type, integration.Provider), "enterprise_sync")
		writeJSON(w, http.StatusCreated, integration)
	default:
		writeError(w, http.StatusBadRequest, errInvalidReferenceIntegrationPayload.Error())
	}
}

func (s *server) updateReferenceIntegration(w http.ResponseWriter, r *http.Request) {
	var request referenceIntegrationMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	integration, found, err := s.referenceIntegrationByID(tenantID, chi.URLParam(r, "integrationID"))
	if err != nil {
		handleReferenceIntegrationMutationError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "integration not found")
		return
	}
	payload.Actor = s.referenceIntegrationActor(r, payload.Actor)

	switch integration.Type {
	case "identity_provider":
		current, err := s.enterpriseSvc.GetIDPConfig(integration.TenantID)
		if err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
		scopes := current.Scopes
		if len(payload.Scopes) > 0 {
			scopes = payload.Scopes
		}
		updated, err := s.enterpriseSvc.UpsertIDPConfig(
			integration.TenantID,
			firstNonEmptyString(payload.Provider, current.Provider),
			firstNonEmptyString(payload.IssuerURL, current.IssuerURL),
			firstNonEmptyString(payload.ClientID, current.ClientID),
			firstNonEmptyString(payload.AuthURL, current.AuthURL),
			firstNonEmptyString(payload.TokenURL, current.TokenURL),
			firstNonEmptyString(payload.JWKSURL, current.JWKSURL),
			firstNonEmptyString(payload.UserInfoURL, current.UserInfoURL),
			firstNonEmptyString(payload.SAMLACSURL, current.SAMLACSURL),
			firstNonEmptyString(payload.SAMLX509Cert, current.SAMLX509Cert),
			firstNonEmptyString(payload.Status, current.Status),
			firstNonEmptyString(payload.SyncMode, current.SyncMode),
			payload.Actor,
			scopes,
		)
		if err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
		nextIntegration := referenceIntegrationFromIDPConfig(updated)
		s.appendAuditLog(r, integration.TenantID, "reference_integration_updated", fmt.Sprintf("integration_id=%s,type=%s,provider=%s", nextIntegration.ID, nextIntegration.Type, nextIntegration.Provider), "identity_provider")
		writeJSON(w, http.StatusOK, nextIntegration)
	case "hris":
		updated, err := s.enterpriseSvc.UpdateHRISConnector(
			integration.TenantID,
			integration.SourceID,
			payload.Status,
			payload.SyncMode,
			payload.CredentialRef,
			payload.WebhookSecretRef,
			payload.Actor,
		)
		if err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
		nextIntegration := referenceIntegrationFromHRISConnector(updated)
		s.appendAuditLog(r, integration.TenantID, "reference_integration_updated", fmt.Sprintf("integration_id=%s,type=%s,provider=%s", nextIntegration.ID, nextIntegration.Type, nextIntegration.Provider), "enterprise_sync")
		writeJSON(w, http.StatusOK, nextIntegration)
	default:
		writeError(w, http.StatusBadRequest, errInvalidReferenceIntegrationPayload.Error())
	}
}

func (s *server) deleteReferenceIntegration(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	integration, found, err := s.referenceIntegrationByID(tenantID, chi.URLParam(r, "integrationID"))
	if err != nil {
		handleReferenceIntegrationMutationError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "integration not found")
		return
	}
	actor := s.referenceIntegrationActor(r, "")

	switch integration.Type {
	case "identity_provider":
		current, err := s.enterpriseSvc.GetIDPConfig(integration.TenantID)
		if err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
		if _, err := s.enterpriseSvc.UpsertIDPConfig(
			integration.TenantID,
			current.Provider,
			current.IssuerURL,
			current.ClientID,
			current.AuthURL,
			current.TokenURL,
			current.JWKSURL,
			current.UserInfoURL,
			current.SAMLACSURL,
			current.SAMLX509Cert,
			"inactive",
			current.SyncMode,
			actor,
			current.Scopes,
		); err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
	case "hris":
		if _, err := s.enterpriseSvc.UpdateHRISConnector(
			integration.TenantID,
			integration.SourceID,
			"inactive",
			"",
			"",
			"",
			actor,
		); err != nil {
			handleReferenceIntegrationMutationError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errInvalidReferenceIntegrationPayload.Error())
		return
	}
	s.appendAuditLog(r, integration.TenantID, "reference_integration_deleted", fmt.Sprintf("integration_id=%s,type=%s,provider=%s", integration.ID, integration.Type, integration.Provider), "enterprise_sync")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceAlertPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	policies := make([]referenceAlertPolicy, 0)
	for _, nextTenantID := range s.referenceAlertPolicyTenantIDs(tenantID) {
		enterpriseSubscription, found := s.enterpriseSvc.GetSyncWorkerAlertSubscription(nextTenantID)
		if !found {
			enterpriseSubscription = s.defaultEnterpriseSyncWorkerAlertSubscription(nextTenantID)
		}
		enterprisePolicy := referenceAlertPolicyFromEnterpriseSubscription(enterpriseSubscription)
		if referenceAlertPolicyMatchesQuery(enterprisePolicy, r) {
			policies = append(policies, enterprisePolicy)
		}

		walletSubscription, found := s.walletSvc.GetJobAlertSubscription(nextTenantID)
		if !found {
			walletSubscription = s.defaultWalletJobAlertSubscription(nextTenantID)
		}
		walletPolicy := referenceAlertPolicyFromWalletSubscription(walletSubscription)
		if referenceAlertPolicyMatchesQuery(walletPolicy, r) {
			policies = append(policies, walletPolicy)
		}
		for _, customPolicy := range s.referenceCustomAlertPolicies(nextTenantID) {
			if referenceAlertPolicyMatchesQuery(customPolicy, r) {
				policies = append(policies, customPolicy)
			}
		}
	}
	sortReferenceAlertPolicies(policies, r)
	writeCollection(w, r, http.StatusOK, policies)
}

func (s *server) getReferenceAlertPolicy(w http.ResponseWriter, r *http.Request) {
	policyKind, tenantID, ok := s.resolveReferenceAlertPolicyRequest(w, r)
	if !ok {
		return
	}
	policy, ok := s.referenceAlertPolicyByKind(policyKind, tenantID)
	if !ok {
		writeError(w, http.StatusNotFound, "alert policy not found")
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (s *server) createReferenceAlertPolicy(w http.ResponseWriter, r *http.Request) {
	var request referenceAlertPolicyMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	policyKind := referenceAlertPolicyKindFromPayload(payload)
	if policyKind == "" {
		writeError(w, http.StatusBadRequest, errInvalidReferenceAlertPolicyPayload.Error())
		return
	}
	if payload.Enabled == nil && strings.TrimSpace(payload.Status) == "" {
		enabled := true
		payload.Enabled = &enabled
	}

	switch policyKind {
	case "enterprise_sync_worker":
		created, err := s.updateReferenceEnterpriseSyncWorkerAlertPolicy(tenantID, payload)
		if err != nil {
			handleReferenceAlertPolicyMutationError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case "wallet_jobs":
		created, err := s.updateReferenceWalletJobAlertPolicy(tenantID, payload)
		if err != nil {
			handleReferenceAlertPolicyMutationError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case "custom":
		created, err := s.createReferenceCustomAlertPolicy(tenantID, payload)
		if err != nil {
			handleReferenceAlertPolicyMutationError(w, err)
			return
		}
		s.appendAuditLog(r, tenantID, "reference_custom_alert_policy_created", fmt.Sprintf("policy_id=%s,trigger=%s,severity=%s", created.ID, created.Trigger, created.Severity), "alert_policy")
		writeJSON(w, http.StatusCreated, created)
	default:
		writeError(w, http.StatusBadRequest, errInvalidReferenceAlertPolicyPayload.Error())
	}
}

func (s *server) updateReferenceAlertPolicy(w http.ResponseWriter, r *http.Request) {
	policyKind, tenantID, ok := s.resolveReferenceAlertPolicyRequest(w, r)
	if !ok {
		return
	}
	var request referenceAlertPolicyMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	if payloadTenantID := strings.TrimSpace(payload.TenantID); payloadTenantID != "" && payloadTenantID != tenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return
	}

	switch policyKind {
	case "enterprise_sync_worker":
		updated, err := s.updateReferenceEnterpriseSyncWorkerAlertPolicy(tenantID, payload)
		if err != nil {
			handleReferenceAlertPolicyMutationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case "wallet_jobs":
		updated, err := s.updateReferenceWalletJobAlertPolicy(tenantID, payload)
		if err != nil {
			handleReferenceAlertPolicyMutationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		updated, err := s.updateReferenceCustomAlertPolicy(policyKind, tenantID, payload)
		if err != nil {
			handleReferenceAlertPolicyMutationError(w, err)
			return
		}
		s.appendAuditLog(r, tenantID, "reference_custom_alert_policy_updated", fmt.Sprintf("policy_id=%s,trigger=%s,status=%s", updated.ID, updated.Trigger, updated.Status), "alert_policy")
		writeJSON(w, http.StatusOK, updated)
	}
}

func (s *server) deleteReferenceAlertPolicy(w http.ResponseWriter, r *http.Request) {
	policyKind, tenantID, ok := s.resolveReferenceAlertPolicyRequest(w, r)
	if !ok {
		return
	}
	enabled := false
	payload := referenceAlertPolicyPayload{Enabled: &enabled}
	var err error
	auditCategory := policyKind
	switch policyKind {
	case "enterprise_sync_worker":
		_, err = s.updateReferenceEnterpriseSyncWorkerAlertPolicy(tenantID, payload)
	case "wallet_jobs":
		_, err = s.updateReferenceWalletJobAlertPolicy(tenantID, payload)
	default:
		_, err = s.updateReferenceCustomAlertPolicy(policyKind, tenantID, payload)
		auditCategory = "custom"
	}
	if err != nil {
		handleReferenceAlertPolicyMutationError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_alert_policy_disabled", fmt.Sprintf("policy_id=%s,category=%s", chi.URLParam(r, "policyID"), auditCategory), "alert_policy")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceTeams(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListTeams(tenantID)
	if buildingScope != nil {
		items = filterTeamsByScope(items, buildingScope)
	}
	items = filterTeamsByQuery(items, r)
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getReferenceTeam(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	record, err := s.accessSvc.GetTeam(tenantID, chi.URLParam(r, "teamID"))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.PlaceID) {
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) createReferenceTeam(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Team referenceTeamPayload `json:"team"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Team.TenantID)
	if !ok {
		return
	}
	record, err := s.accessSvc.CreateTeam(
		tenantID,
		request.Team.Name,
		request.Team.Scope,
		request.Team.PlaceID,
		request.Team.Description,
		request.Team.Source,
	)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_team_created", referenceTeamAuditTarget(record), "access")
	writeJSON(w, http.StatusCreated, record)
}

func (s *server) updateReferenceTeam(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Team referenceTeamPayload `json:"team"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Team.TenantID)
	if !ok {
		return
	}
	record, err := s.accessSvc.UpdateTeam(
		tenantID,
		chi.URLParam(r, "teamID"),
		request.Team.Name,
		request.Team.Scope,
		request.Team.PlaceID,
		request.Team.Description,
		request.Team.Source,
	)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_team_updated", referenceTeamAuditTarget(record), "access")
	writeJSON(w, http.StatusOK, record)
}

func (s *server) deleteReferenceTeam(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if err := s.accessSvc.DeleteTeam(tenantID, chi.URLParam(r, "teamID")); err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_team_deleted", fmt.Sprintf("team_id=%s", chi.URLParam(r, "teamID")), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceTeamMemberships(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	teams := s.accessSvc.ListTeams(tenantID)
	var teamIDs map[string]struct{}
	if buildingScope != nil {
		teams = filterTeamsByScope(teams, buildingScope)
		teamIDs = make(map[string]struct{}, len(teams))
		for i := range teams {
			teamIDs[teams[i].ID] = struct{}{}
		}
	}
	items := s.accessSvc.ListTeamMemberships(tenantID)
	items = filterTeamMembershipsByQuery(items, teamIDs, r)
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferenceTeamMembership(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TeamMembership referenceTeamMembershipPayload `json:"team_membership"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TeamMembership.TenantID)
	if !ok {
		return
	}
	record, err := s.accessSvc.CreateTeamMembership(
		tenantID,
		request.TeamMembership.TeamID,
		request.TeamMembership.MemberType,
		request.TeamMembership.MemberID,
		request.TeamMembership.MemberEmail,
		request.TeamMembership.MemberName,
		request.TeamMembership.Source,
	)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_team_membership_created", referenceTeamMembershipAuditTarget(record), "access")
	writeJSON(w, http.StatusCreated, record)
}

func (s *server) deleteReferenceTeamMembership(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	if err := s.accessSvc.DeleteTeamMembership(tenantID, chi.URLParam(r, "membershipID")); err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_team_membership_deleted", fmt.Sprintf("team_membership_id=%s", chi.URLParam(r, "membershipID")), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceRoles(w http.ResponseWriter, r *http.Request) {
	appliesTo := strings.TrimSpace(r.URL.Query().Get("applies_to"))
	items := s.accessSvc.ListRoles()
	if appliesTo != "" {
		filtered := make([]access.Role, 0, len(items))
		for i := range items {
			if strings.EqualFold(items[i].AppliesTo, appliesTo) {
				filtered = append(filtered, items[i])
			}
		}
		items = filtered
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getReferenceRole(w http.ResponseWriter, r *http.Request) {
	roleID := strings.TrimSpace(chi.URLParam(r, "roleID"))
	for _, role := range s.accessSvc.ListRoles() {
		if role.ID == roleID {
			writeJSON(w, http.StatusOK, role)
			return
		}
	}
	writeError(w, http.StatusNotFound, access.ErrRoleNotFound.Error())
}

func (s *server) listReferenceRoleAssignments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListRoleAssignments(tenantID)
	if buildingScope != nil {
		items = filterRoleAssignmentsByBuildingScope(items, buildingScope)
	}
	items = filterRoleAssignmentsByQuery(items, r)
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) createReferenceRoleAssignment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RoleAssignment referenceRoleAssignmentPayload `json:"role_assignment"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.RoleAssignment.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !validateRoleAssignmentScope(w, buildingScope, request.RoleAssignment.AppliesToType, request.RoleAssignment.AppliesToID) {
		return
	}
	created, err := s.accessSvc.CreateRoleAssignment(referenceRoleAssignmentInput(tenantID, request.RoleAssignment))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_role_assignment_created", referenceRoleAssignmentAuditTarget(created), "access")
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) getReferenceRoleAssignment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	assignment, err := s.accessSvc.GetRoleAssignment(tenantID, chi.URLParam(r, "assignmentID"))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !roleAssignmentAllowedByBuildingScope(assignment, buildingScope) {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (s *server) updateReferenceRoleAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID := chi.URLParam(r, "assignmentID")
	var request struct {
		RoleAssignment referenceRoleAssignmentPayload `json:"role_assignment"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.RoleAssignment.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !validateRoleAssignmentScope(w, buildingScope, request.RoleAssignment.AppliesToType, request.RoleAssignment.AppliesToID) {
		return
	}
	updated, err := s.accessSvc.UpdateRoleAssignment(tenantID, assignmentID, referenceRoleAssignmentInput(tenantID, request.RoleAssignment))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_role_assignment_updated", referenceRoleAssignmentAuditTarget(updated), "access")
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteReferenceRoleAssignment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	assignmentID := chi.URLParam(r, "assignmentID")
	assignment, err := s.accessSvc.GetRoleAssignment(tenantID, assignmentID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !roleAssignmentAllowedByBuildingScope(assignment, buildingScope) {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return
	}
	if err := s.accessSvc.DeleteRoleAssignment(tenantID, assignmentID); err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_role_assignment_deleted", referenceRoleAssignmentAuditTarget(assignment), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceMembers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListUsers(tenantID)
	if buildingScope != nil {
		items = filterUsersByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterAccessUsersByPlaceID(items, placeID)
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) listReferenceShares(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListTemporaryAccess(tenantID)
	if buildingScope != nil {
		items = filterTemporaryAccessByScope(items, buildingScope)
	}
	shares := make([]referenceShare, 0, len(items))
	for i := range items {
		share := referenceShareFromTemporaryAccess(items[i])
		if !shareMatchesQuery(share, r) {
			continue
		}
		shares = append(shares, share)
	}
	writeCollection(w, r, http.StatusOK, shares)
}

func (s *server) createReferenceShare(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Share referenceSharePayload `json:"share"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Share.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(request.Share.PlaceID, request.Share.BuildingID)
	if buildingScope != nil && buildingID == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for Place Admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	areaID := strings.TrimSpace(request.Share.AreaID)
	lockID := firstNonEmptyString(request.Share.LockID, request.Share.DoorID)
	scopeType := "all"
	if buildingID != "" {
		scopeType = "building"
	}
	if areaID != "" {
		scopeType = "area"
	}
	if lockID != "" {
		scopeType = "door"
	}
	authorizedByID := "system"
	authorizedByEmail := ""
	authorizedByRole := "system"
	if user, ok := authenticatedUser(r); ok {
		authorizedByID = strings.TrimSpace(user.ID)
		authorizedByEmail = strings.TrimSpace(user.Email)
		authorizedByRole = strings.TrimSpace(user.Role)
	}
	granteeName := firstNonEmptyString(request.Share.GranteeName, request.Share.Email)
	granteePhone := firstNonEmptyString(request.Share.GranteePhone, "not_provided")
	passType := firstNonEmptyString(request.Share.PassType, "share")
	validFrom := ""
	if request.Share.ValidFrom != nil {
		validFrom = strings.TrimSpace(*request.Share.ValidFrom)
	}

	created, err := s.accessSvc.CreateTemporaryAccessWithInput(access.TemporaryAccessInput{
		TenantID:          tenantID,
		ScopeType:         scopeType,
		BuildingID:        buildingID,
		AreaID:            areaID,
		DoorID:            lockID,
		GroupID:           request.Share.GroupID,
		RoleID:            request.Share.RoleID,
		DeliveryMethod:    firstNonEmptyString(request.Share.DeliveryMethod, "email_qr"),
		GranteeName:       granteeName,
		GranteePhone:      granteePhone,
		GranteeEmail:      request.Share.Email,
		MobileModel:       request.Share.MobileModel,
		PassType:          passType,
		ValidFrom:         validFrom,
		ValidUntil:        request.Share.ValidUntil,
		AuthorizedByID:    authorizedByID,
		AuthorizedByEmail: authorizedByEmail,
		AuthorizedByRole:  authorizedByRole,
	})
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_share_created", referenceTemporaryAccessShareAuditTarget(created), "access")
	writeJSON(w, http.StatusCreated, referenceShareFromTemporaryAccess(created))
}

func (s *server) getReferenceShare(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	record, err := s.accessSvc.GetTemporaryAccess(tenantID, chi.URLParam(r, "shareID"))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}
	writeJSON(w, http.StatusOK, referenceShareFromTemporaryAccess(record))
}

func (s *server) updateReferenceShare(w http.ResponseWriter, r *http.Request) {
	shareID := chi.URLParam(r, "shareID")
	var request struct {
		Share referenceSharePayload `json:"share"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(request.Share.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	current, err := s.accessSvc.GetTemporaryAccess(tenantID, shareID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, current.BuildingID) {
		return
	}
	buildingID := firstNonEmptyString(request.Share.PlaceID, request.Share.BuildingID, current.BuildingID)
	lockID := firstNonEmptyString(request.Share.LockID, request.Share.DoorID, current.DoorID)
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	scopeType := current.ScopeType
	if strings.TrimSpace(request.Share.PlaceID) != "" || strings.TrimSpace(request.Share.BuildingID) != "" {
		scopeType = "building"
	}
	if lockID != "" {
		scopeType = "door"
	}
	validFrom := current.ValidFrom
	if request.Share.ValidFrom != nil {
		validFrom = strings.TrimSpace(*request.Share.ValidFrom)
	}
	updated, err := s.accessSvc.UpdateTemporaryAccess(tenantID, shareID, access.TemporaryAccessInput{
		TenantID:           tenantID,
		ScopeType:          scopeType,
		BuildingID:         buildingID,
		AreaID:             current.AreaID,
		DoorID:             lockID,
		GroupID:            firstNonEmptyString(request.Share.GroupID, current.GroupID),
		RoleID:             firstNonEmptyString(request.Share.RoleID, current.RoleID),
		DeliveryMethod:     firstNonEmptyString(request.Share.DeliveryMethod, current.DeliveryMethod),
		GranteeName:        firstNonEmptyString(request.Share.GranteeName, current.GranteeName, request.Share.Email),
		GranteeGender:      current.GranteeGender,
		GranteePhone:       firstNonEmptyString(request.Share.GranteePhone, current.GranteePhone, "not_provided"),
		GranteeEmail:       firstNonEmptyString(request.Share.Email, current.GranteeEmail),
		MobileModel:        firstNonEmptyString(request.Share.MobileModel, current.MobileModel),
		PassType:           firstNonEmptyString(request.Share.PassType, current.PassType, "share"),
		ValidFrom:          validFrom,
		ValidUntil:         firstNonEmptyString(request.Share.ValidUntil, current.ValidUntil),
		AuthorizedByID:     current.AuthorizedByID,
		AuthorizedByEmail:  current.AuthorizedByEmail,
		AuthorizedByRole:   current.AuthorizedByRole,
		KeepAuthorizedTime: true,
		ReviewedAt:         current.ReviewedAt,
		ReviewedBy:         current.ReviewedBy,
	})
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_share_updated", referenceTemporaryAccessShareAuditTarget(updated), "access")
	writeJSON(w, http.StatusOK, referenceShareFromTemporaryAccess(updated))
}

func (s *server) deleteReferenceShare(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	shareID := chi.URLParam(r, "shareID")
	record, err := s.accessSvc.GetTemporaryAccess(tenantID, shareID)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	if !s.requireBuildingScope(w, buildingScope, record.BuildingID) {
		return
	}
	if err := s.accessSvc.DeleteTemporaryAccess(tenantID, shareID); err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_share_deleted", referenceTemporaryAccessShareAuditTarget(record), "access")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listReferenceCards(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	passes := s.walletSvc.ListPasses(tenantID)
	items := make([]referenceCard, 0, len(passes))
	for i := range passes {
		card := referenceCardFromPass(passes[i])
		if !cardMatchesQuery(card, r) {
			continue
		}
		items = append(items, card)
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getReferenceCard(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	record, err := s.walletSvc.GetPass(tenantID, strings.TrimSpace(chi.URLParam(r, "cardID")))
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, referenceCardFromPass(record))
}

func (s *server) createReferenceCard(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Card referenceCardPayload `json:"card"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Card.TenantID)
	if !ok {
		return
	}
	if referenceCardPayloadIsAppleWallet(request.Card) {
		writeError(w, http.StatusUnprocessableEntity, "apple wallet passes must be enrolled through the user app")
		return
	}
	templateID := firstNonEmptyString(request.Card.TemplateID, defaultReferenceCardTemplateID(request.Card))
	created, err := s.walletSvc.CreateUnassignedCard(
		tenantID,
		templateID,
		request.Card.Token,
		request.Card.UID,
		request.Card.CardNumber,
		request.Card.Type,
		request.Card.ExpiresAt,
		referenceRequestActor(r),
	)
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_created", referenceWalletPassAuditTarget(created), "wallet")
	if referenceCardPayloadHasAssignee(request.Card) {
		targetType, targetID, err := s.referenceCardAssigneeTarget(tenantID, request.Card)
		if err != nil {
			handleWalletReferenceError(w, err)
			return
		}
		created, err = s.walletSvc.AssignPass(tenantID, created.ID, targetType, targetID, referenceRequestActor(r))
		if err != nil {
			handleWalletReferenceError(w, err)
			return
		}
		s.appendAuditLog(r, tenantID, "reference_card_assigned", referenceWalletPassAuditTarget(created), "wallet")
	}
	writeJSON(w, http.StatusCreated, referenceCardFromPass(created))
}

func (s *server) listReferenceCardAssignments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	passes := s.walletSvc.ListPasses(tenantID)
	items := make([]referenceCardAssignment, 0, len(passes))
	for i := range passes {
		assignment := referenceCardAssignmentFromPass(passes[i])
		if !cardAssignmentMatchesQuery(assignment, r) {
			continue
		}
		items = append(items, assignment)
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getReferenceCardAssignment(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	passID := referenceCardAssignmentPassID(chi.URLParam(r, "assignmentID"))
	record, err := s.walletSvc.GetPass(tenantID, passID)
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, referenceCardAssignmentFromPass(record))
}

func (s *server) createReferenceCardAssignment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CardAssignment referenceCardAssignmentPayload `json:"card_assignment"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.CardAssignment.TenantID)
	if !ok {
		return
	}
	targetType, targetID, err := s.referenceCardAssignmentTarget(tenantID, request.CardAssignment)
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	cardID := strings.TrimSpace(request.CardAssignment.CardID)
	if cardID == "" {
		writeError(w, http.StatusBadRequest, "card_id is required")
		return
	}
	assigned, err := s.walletSvc.AssignPass(tenantID, cardID, targetType, targetID, referenceRequestActor(r))
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_assigned", referenceWalletPassAuditTarget(assigned), "wallet")
	writeJSON(w, http.StatusOK, referenceCardAssignmentFromPass(assigned))
}

func (s *server) assignReferenceCard(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Card referenceCardPayload `json:"card"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Card.TenantID)
	if !ok {
		return
	}
	targetType, targetID, err := s.referenceCardAssigneeTarget(tenantID, request.Card)
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	assigned, err := s.walletSvc.AssignPass(tenantID, strings.TrimSpace(chi.URLParam(r, "cardID")), targetType, targetID, referenceRequestActor(r))
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_assigned", referenceWalletPassAuditTarget(assigned), "wallet")
	writeJSON(w, http.StatusOK, referenceCardFromPass(assigned))
}

func (s *server) deassignReferenceCard(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	updated, err := s.walletSvc.DeassignPass(tenantID, strings.TrimSpace(chi.URLParam(r, "cardID")), referenceRequestActor(r))
	if err != nil {
		handleWalletReferenceError(w, err)
		return
	}
	s.appendAuditLog(r, tenantID, "reference_card_deassigned", referenceWalletPassAuditTarget(updated), "wallet")
	writeJSON(w, http.StatusOK, referenceCardFromPass(updated))
}

func (s *server) activateReferenceCard(w http.ResponseWriter, r *http.Request) {
	s.changeReferenceCardStatus(w, r, "active")
}

func (s *server) deactivateReferenceCard(w http.ResponseWriter, r *http.Request) {
	s.changeReferenceCardStatus(w, r, "suspended")
}

func (s *server) revokeReferenceCard(w http.ResponseWriter, r *http.Request) {
	s.changeReferenceCardStatus(w, r, "revoked")
}

func (s *server) createReferenceEventSet(w http.ResponseWriter, r *http.Request) {
	var request struct {
		EventSet referenceEventSetPayload `json:"event_set"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	placeID := firstNonEmptyString(request.EventSet.PlaceID, request.EventSet.EventPlaceID)
	if !s.requireBuildingScope(w, buildingScope, placeIDForEventSetScope(placeID, buildingScope)) {
		return
	}
	writeJSON(w, http.StatusOK, s.referenceEventSetSnapshot(tenantID, buildingScope, request.EventSet))
}

func (s *server) getReferenceEventSet(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	payload := referenceEventSetPayload{
		PlaceID:   strings.TrimSpace(r.URL.Query().Get("place_id")),
		EventType: strings.TrimSpace(r.URL.Query().Get("event_type")),
		EventUUID: strings.TrimSpace(r.URL.Query().Get("event_uuid")),
		Interval:  strings.TrimSpace(r.URL.Query().Get("interval")),
	}
	if !s.requireBuildingScope(w, buildingScope, placeIDForEventSetScope(payload.PlaceID, buildingScope)) {
		return
	}
	set := s.referenceEventSetSnapshot(tenantID, buildingScope, payload)
	set.ID = strings.TrimSpace(chi.URLParam(r, "eventSetID"))
	if set.ID == "" {
		set.ID = referenceEventSetID(tenantID, payload)
	}
	writeJSON(w, http.StatusOK, set)
}

func (s *server) getReferenceEventMetadata(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"object_type_to_action": map[string][]string{
			"Lock":       {"access_granted", "access_denied"},
			"Controller": {"gateway_event"},
			"Reader":     {"gateway_event"},
		},
	})
}

func (s *server) listReferenceEventTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{
		"access_granted",
		"access_denied",
		"gateway_event",
	})
}

func (s *server) changeReferenceCardStatus(w http.ResponseWriter, r *http.Request, status string) {
	cardID := strings.TrimSpace(chi.URLParam(r, "cardID"))
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	var (
		record wallet.PassInstance
		err    error
	)
	switch status {
	case "active":
		record, err = s.walletSvc.ActivatePass(tenantID, cardID, referenceRequestActor(r))
	case "suspended":
		record, err = s.walletSvc.SuspendPass(tenantID, cardID, referenceRequestActor(r))
	case "revoked":
		record, err = s.walletSvc.RevokePass(tenantID, cardID, referenceRequestActor(r))
	default:
		writeError(w, http.StatusBadRequest, "invalid card status operation")
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrInvalidPassTransition):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s.appendAuditLog(r, tenantID, referenceCardStatusAuditAction(status), referenceWalletPassAuditTarget(record), "wallet")
	writeJSON(w, http.StatusOK, referenceCardFromPass(record))
}

func referenceRoleAssignmentInput(tenantID string, payload referenceRoleAssignmentPayload) access.RoleAssignmentInput {
	return access.RoleAssignmentInput{
		TenantID:      tenantID,
		RoleID:        payload.RoleID,
		AppliesToType: payload.AppliesToType,
		AppliesToID:   payload.AppliesToID,
		AssigneeType:  payload.AssigneeType,
		AssigneeID:    payload.AssigneeID,
		AssigneeEmail: payload.AssigneeEmail,
		ValidFrom:     payload.ValidFrom,
		ValidUntil:    payload.ValidUntil,
	}
}

func referenceRequestActor(r *http.Request) string {
	if user, ok := authenticatedUser(r); ok {
		return firstNonEmptyString(user.Email, user.ID, "admin")
	}
	return "admin"
}

func defaultReferenceCardTemplateID(payload referenceCardPayload) string {
	assigneeType := strings.ToLower(strings.TrimSpace(payload.AssigneeType))
	if assigneeType == "guest" || strings.TrimSpace(payload.GuestID) != "" {
		return "wpt_visitor_demo"
	}
	return "wpt_employee_demo"
}

func referenceCardPayloadIsAppleWallet(payload referenceCardPayload) bool {
	switch strings.ToLower(strings.TrimSpace(payload.Type)) {
	case "apple", "apple_wallet", "apple_pass", "apple_passes":
		return true
	default:
		return false
	}
}

func referenceCardPayloadHasAssignee(payload referenceCardPayload) bool {
	return strings.TrimSpace(payload.AssigneeID) != "" ||
		strings.TrimSpace(payload.UserID) != "" ||
		strings.TrimSpace(payload.GuestID) != "" ||
		strings.TrimSpace(payload.Email) != ""
}

func (s *server) referenceCardAssigneeTarget(tenantID string, payload referenceCardPayload) (string, string, error) {
	assignmentPayload := referenceCardAssignmentPayload{
		TenantID:     tenantID,
		AssigneeType: payload.AssigneeType,
		AssigneeID:   payload.AssigneeID,
		UserID:       payload.UserID,
		GuestID:      payload.GuestID,
		Email:        payload.Email,
	}
	return s.referenceCardAssignmentTarget(tenantID, assignmentPayload)
}

func (s *server) referenceCardAssignmentTarget(tenantID string, payload referenceCardAssignmentPayload) (string, string, error) {
	assigneeType := strings.TrimSpace(payload.AssigneeType)
	assigneeID := firstNonEmptyString(payload.AssigneeID, payload.UserID, payload.GuestID)
	if assigneeType == "" {
		switch {
		case strings.TrimSpace(payload.UserID) != "":
			assigneeType = "User"
		case strings.TrimSpace(payload.GuestID) != "":
			assigneeType = "Guest"
		case strings.TrimSpace(payload.Email) != "":
			assigneeType = "User"
		}
	}
	if strings.EqualFold(assigneeType, "User") && assigneeID == "" && strings.TrimSpace(payload.Email) != "" {
		assigneeID = s.referenceUserIDByEmail(tenantID, payload.Email)
		if assigneeID == "" {
			assigneeID = strings.TrimSpace(payload.Email)
		}
	}
	if strings.EqualFold(assigneeType, "Guest") && assigneeID == "" {
		assigneeID = strings.TrimSpace(payload.Email)
	}
	switch strings.ToLower(strings.TrimSpace(assigneeType)) {
	case "user":
		return "user", assigneeID, validateReferenceCardTargetID(assigneeID)
	case "guest":
		return "visitor", assigneeID, validateReferenceCardTargetID(assigneeID)
	default:
		return "", "", wallet.ErrInvalidTargetType
	}
}

func (s *server) referenceUserIDByEmail(tenantID, email string) string {
	targetEmail := strings.TrimSpace(email)
	if targetEmail == "" {
		return ""
	}
	users := s.accessSvc.ListUsers(tenantID)
	for i := range users {
		if strings.EqualFold(strings.TrimSpace(users[i].Email), targetEmail) {
			return users[i].ID
		}
	}
	return ""
}

func validateReferenceCardTargetID(targetID string) error {
	if strings.TrimSpace(targetID) == "" {
		return wallet.ErrTargetIDRequired
	}
	return nil
}

func validateRoleAssignmentScope(w http.ResponseWriter, scope map[string]struct{}, appliesToType, appliesToID string) bool {
	if scope == nil {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(appliesToType), "Place") {
		writeError(w, http.StatusForbidden, "place scope forbidden")
		return false
	}
	placeID := strings.TrimSpace(appliesToID)
	if placeID == "" {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	if _, exists := scope[placeID]; !exists {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	return true
}

func (s *server) allowedBuildingScope(scope map[string]struct{}, buildingID string) bool {
	if scope == nil {
		return true
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	if nextBuildingID == "" {
		return false
	}
	_, exists := scope[nextBuildingID]
	return exists
}

func filterRoleAssignmentsByBuildingScope(items []access.RoleAssignment, scope map[string]struct{}) []access.RoleAssignment {
	filtered := make([]access.RoleAssignment, 0, len(items))
	for i := range items {
		if !roleAssignmentAllowedByBuildingScope(items[i], scope) {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func roleAssignmentAllowedByBuildingScope(item access.RoleAssignment, scope map[string]struct{}) bool {
	if scope == nil {
		return true
	}
	if item.AppliesToType != "Place" {
		return false
	}
	_, exists := scope[item.AppliesToID]
	return exists
}

func filterTeamsByScope(items []access.Team, scope map[string]struct{}) []access.Team {
	filtered := make([]access.Team, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].PlaceID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterTeamsByQuery(items []access.Team, r *http.Request) []access.Team {
	idFilter := commaSet(r.URL.Query().Get("ids"))
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))

	filtered := make([]access.Team, 0, len(items))
	for i := range items {
		if len(idFilter) > 0 {
			if _, exists := idFilter[items[i].ID]; !exists {
				continue
			}
		}
		if query != "" && !strings.Contains(strings.ToLower(items[i].Name), query) {
			continue
		}
		if scope != "" && !strings.EqualFold(items[i].Scope, scope) {
			continue
		}
		if placeID != "" && items[i].PlaceID != placeID {
			continue
		}
		filtered = append(filtered, items[i])
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return filtered[i].Name > filtered[j].Name
		}
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

func filterTeamMembershipsByQuery(items []access.TeamMembership, allowedTeamIDs map[string]struct{}, r *http.Request) []access.TeamMembership {
	idFilter := commaSet(r.URL.Query().Get("ids"))
	teamID := strings.TrimSpace(r.URL.Query().Get("team_id"))
	memberType := strings.TrimSpace(r.URL.Query().Get("member_type"))
	memberID := strings.TrimSpace(r.URL.Query().Get("member_id"))

	filtered := make([]access.TeamMembership, 0, len(items))
	for i := range items {
		if allowedTeamIDs != nil {
			if _, exists := allowedTeamIDs[items[i].TeamID]; !exists {
				continue
			}
		}
		if len(idFilter) > 0 {
			if _, exists := idFilter[items[i].ID]; !exists {
				continue
			}
		}
		if teamID != "" && items[i].TeamID != teamID {
			continue
		}
		if memberType != "" && !strings.EqualFold(items[i].MemberType, memberType) {
			continue
		}
		if memberID != "" && items[i].MemberID != memberID {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterRoleAssignmentsByQuery(items []access.RoleAssignment, r *http.Request) []access.RoleAssignment {
	roleID := strings.TrimSpace(r.URL.Query().Get("role_id"))
	appliesToType := strings.TrimSpace(r.URL.Query().Get("applies_to_type"))
	appliesToID := strings.TrimSpace(r.URL.Query().Get("applies_to_id"))
	assigneeType := strings.TrimSpace(r.URL.Query().Get("assignee_type"))
	assigneeID := strings.TrimSpace(r.URL.Query().Get("assignee_id"))

	filtered := make([]access.RoleAssignment, 0, len(items))
	for i := range items {
		if roleID != "" && items[i].RoleID != roleID {
			continue
		}
		if appliesToType != "" && !strings.EqualFold(items[i].AppliesToType, appliesToType) {
			continue
		}
		if appliesToID != "" && items[i].AppliesToID != appliesToID {
			continue
		}
		if assigneeType != "" && !strings.EqualFold(items[i].AssigneeType, assigneeType) {
			continue
		}
		if assigneeID != "" && items[i].AssigneeID != assigneeID {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func referenceShareFromTemporaryAccess(item access.TemporaryAccess) referenceShare {
	status := "active"
	return referenceShare{
		ID:                item.ID,
		TenantID:          item.TenantID,
		Email:             item.GranteeEmail,
		GroupID:           item.GroupID,
		RoleID:            firstNonEmptyString(item.RoleID, "role_group_access"),
		PlaceID:           item.BuildingID,
		AreaID:            item.AreaID,
		LockID:            item.DoorID,
		ValidFrom:         item.ValidFrom,
		ValidUntil:        item.ValidUntil,
		Status:            status,
		DeliveryMethod:    item.DeliveryMethod,
		GranteeName:       item.GranteeName,
		GranteePhone:      item.GranteePhone,
		MobileModel:       item.MobileModel,
		PassType:          item.PassType,
		AuthorizedByID:    item.AuthorizedByID,
		AuthorizedByEmail: item.AuthorizedByEmail,
		AuthorizedByRole:  item.AuthorizedByRole,
		AuthorizedAt:      item.AuthorizedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ReviewedAt:        item.ReviewedAt,
		ReviewedBy:        item.ReviewedBy,
		CreatedAt:         item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceCardFromPass(item wallet.PassInstance) referenceCard {
	assigneeType := ""
	userID := ""
	if strings.EqualFold(item.TargetType, "user") {
		assigneeType = "User"
		userID = item.TargetID
	} else if strings.EqualFold(item.TargetType, "visitor") {
		assigneeType = "Guest"
	}

	credentialKind := referenceCardCredentialKind(item)
	token := strings.TrimSpace(item.Token)
	if token == "" {
		token = item.ObjectID
	}
	uid := strings.TrimSpace(item.UID)
	cardNumber := strings.TrimSpace(item.CardNumber)
	if credentialKind == "physical_card" && uid == "" && cardNumber == "" {
		uid = item.ObjectID
	}

	return referenceCard{
		ID:              item.ID,
		ResourceType:    "Card",
		TenantID:        item.TenantID,
		Status:          referenceCardStatus(item.Status),
		Token:           token,
		UID:             uid,
		CardNumber:      cardNumber,
		Provider:        referenceCardProvider(item, credentialKind),
		CredentialKind:  credentialKind,
		TemplateID:      item.TemplateID,
		UserID:          userID,
		AssigneeType:    assigneeType,
		AssigneeID:      item.TargetID,
		ActivationToken: item.ID,
		SaveLink:        item.SaveLink,
		IssuedAt:        item.IssuedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt:       item.ExpiresAt,
		CreatedAt:       item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceCardCredentialKind(item wallet.PassInstance) string {
	kind := strings.ToLower(strings.TrimSpace(item.CredentialKind))
	if kind != "" {
		return kind
	}
	provider := strings.ToLower(strings.TrimSpace(item.Provider))
	switch provider {
	case "apple":
		return "apple_wallet"
	case "physical_card", "desfire", "mifare", "mifare_desfire", "third_party_hf", "hid", "fob":
		return "physical_card"
	case "google", "":
		if strings.TrimSpace(item.ObjectID) != "" && item.ID != "" && !strings.Contains(item.ObjectID, item.ID) {
			return "physical_card"
		}
		return "google_wallet"
	default:
		return "credential"
	}
}

func referenceCardProvider(item wallet.PassInstance, credentialKind string) string {
	provider := strings.ToLower(strings.TrimSpace(item.Provider))
	if provider != "" {
		return provider
	}
	switch credentialKind {
	case "apple_wallet":
		return "apple"
	case "physical_card":
		return "physical_card"
	default:
		return "google"
	}
}

func referenceCardAssignmentFromPass(item wallet.PassInstance) referenceCardAssignment {
	card := referenceCardFromPass(item)
	return referenceCardAssignment{
		ID:           "ca_" + item.ID,
		ResourceType: "CardAssignment",
		TenantID:     item.TenantID,
		Status:       referenceCardAssignmentStatus(item.Status),
		CardID:       item.ID,
		Card:         card,
		AssigneeType: card.AssigneeType,
		AssigneeID:   card.AssigneeID,
		UserID:       card.UserID,
		CreatedAt:    item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceCardAssignmentPassID(assignmentID string) string {
	trimmed := strings.TrimSpace(assignmentID)
	if strings.HasPrefix(trimmed, "ca_") {
		return strings.TrimPrefix(trimmed, "ca_")
	}
	return trimmed
}

func referenceGroupLockFromDoorGroup(group space.DoorGroup, lockID string) referenceGroupLock {
	return referenceGroupLock{
		ID:        group.ID + ":" + lockID,
		TenantID:  group.TenantID,
		GroupID:   group.ID,
		LockID:    lockID,
		CreatedAt: group.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceGroupLockParts(value string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(value), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func (s *server) requireReferenceGroupLockScope(w http.ResponseWriter, tenantID string, buildingScope map[string]struct{}, lockID string) bool {
	nextLockID := strings.TrimSpace(lockID)
	if nextLockID == "" {
		writeError(w, http.StatusBadRequest, "lock_id is required")
		return false
	}
	if buildingScope == nil {
		return true
	}
	allowedDoorIDs := allowedDoorIDsByBuildingScope(s.spaceSvc.ListDoors(tenantID), buildingScope)
	if _, exists := allowedDoorIDs[nextLockID]; !exists {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	return true
}

func (s *server) referenceEventSetSnapshot(tenantID string, buildingScope map[string]struct{}, payload referenceEventSetPayload) referenceEventSet {
	accessEvents := s.eventSvc.ListAccessEvents(tenantID)
	deviceEvents := s.eventSvc.ListDeviceEvents(tenantID)
	if buildingScope != nil {
		accessEvents = filterAccessEventsByScope(accessEvents, buildingScope)
		deviceEvents = filterDeviceEventsByScope(deviceEvents, buildingScope)
	}

	events := make([]referenceEvent, 0, len(accessEvents)+len(deviceEvents))
	for i := range accessEvents {
		item := referenceEventFromAccessEvent(accessEvents[i])
		if !referenceEventMatchesPayload(item, payload) {
			continue
		}
		events = append(events, item)
	}
	for i := range deviceEvents {
		item := referenceEventFromDeviceEvent(deviceEvents[i])
		if !referenceEventMatchesPayload(item, payload) {
			continue
		}
		events = append(events, item)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})

	now := ""
	if len(events) > 0 {
		now = events[0].CreatedAt
	} else {
		now = "1970-01-01T00:00:00Z"
	}
	placeID := firstNonEmptyString(payload.PlaceID, payload.EventPlaceID)
	return referenceEventSet{
		ID:              referenceEventSetID(tenantID, payload),
		CreatedAt:       now,
		Status:          "finished",
		Interval:        strings.TrimSpace(payload.Interval),
		EventPlaceID:    strings.TrimSpace(payload.EventPlaceID),
		PlaceID:         strings.TrimSpace(placeID),
		EventType:       strings.TrimSpace(payload.EventType),
		EventUUID:       strings.TrimSpace(payload.EventUUID),
		EventSuccess:    payload.EventSuccess,
		EventObjectID:   strings.TrimSpace(payload.EventObjectID),
		EventObjectType: strings.TrimSpace(payload.EventObjectType),
		Events:          events,
		Cursor:          "",
	}
}

func referenceEventFromAccessEvent(item event.AccessEvent) referenceEvent {
	return referenceEvent{
		UUID:       item.ID,
		TenantID:   item.TenantID,
		Type:       item.Type,
		ActorType:  "User",
		ActorID:    item.Actor,
		ActorName:  item.Actor,
		ActorEmail: actorEmail(item.Actor),
		ObjectType: "Lock",
		ObjectID:   item.DoorID,
		ObjectName: item.DoorID,
		PlaceID:    item.BuildingID,
		AreaID:     item.AreaID,
		LockID:     item.DoorID,
		GatewayID:  item.GatewayID,
		Success:    item.Result == "success",
		Result:     item.Result,
		CreatedAt:  item.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceEventFromDeviceEvent(item event.DeviceEvent) referenceEvent {
	return referenceEvent{
		UUID:       item.ID,
		TenantID:   item.TenantID,
		Type:       item.Type,
		ActorType:  "Controller",
		ActorID:    item.GatewayID,
		ActorName:  item.GatewayID,
		ObjectType: "Controller",
		ObjectID:   item.GatewayID,
		ObjectName: item.GatewayID,
		PlaceID:    item.BuildingID,
		GatewayID:  item.GatewayID,
		Success:    item.Result == "success",
		Result:     item.Result,
		Detail:     item.Detail,
		CreatedAt:  item.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceEventMatchesPayload(item referenceEvent, payload referenceEventSetPayload) bool {
	placeID := firstNonEmptyString(payload.PlaceID, payload.EventPlaceID)
	if placeID != "" && item.PlaceID != placeID {
		return false
	}
	if payload.EventType != "" && !commaSetContains(payload.EventType, item.Type) {
		return false
	}
	if payload.EventUUID != "" && !commaSetContains(payload.EventUUID, item.UUID) {
		return false
	}
	if payload.EventSuccess != nil && item.Success != *payload.EventSuccess {
		return false
	}
	if payload.EventObjectID != "" && item.ObjectID != payload.EventObjectID {
		return false
	}
	if payload.EventObjectType != "" && !strings.EqualFold(item.ObjectType, payload.EventObjectType) {
		return false
	}
	return true
}

func referenceEventSetID(tenantID string, payload referenceEventSetPayload) string {
	placeID := firstNonEmptyString(payload.PlaceID, payload.EventPlaceID, "all")
	eventType := strings.TrimSpace(payload.EventType)
	if eventType == "" {
		eventType = "all"
	}
	return "event_set_" + strings.NewReplacer(" ", "_", ",", "_", "/", "_").Replace(tenantID+"_"+placeID+"_"+eventType)
}

func placeIDForEventSetScope(placeID string, scope map[string]struct{}) string {
	nextPlaceID := strings.TrimSpace(placeID)
	if nextPlaceID != "" || scope == nil {
		return nextPlaceID
	}
	if len(scope) == 1 {
		for id := range scope {
			return id
		}
	}
	return ""
}

func actorEmail(actor string) string {
	nextActor := strings.TrimSpace(actor)
	if strings.Contains(nextActor, "@") {
		return nextActor
	}
	if nextActor == "" || strings.Contains(nextActor, " ") {
		return ""
	}
	return nextActor + "@mistypass.local"
}

func (s *server) requireReferenceControllerScope(w http.ResponseWriter, r *http.Request, tenantID, controllerID string) bool {
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return false
	}
	controller, exists := s.findGatewayByTenant(tenantID, controllerID)
	if !exists {
		writeError(w, http.StatusNotFound, "controller not found")
		return false
	}
	return s.requireBuildingScope(w, buildingScope, controller.BuildingID)
}

func (s *server) requireReferenceLockScope(w http.ResponseWriter, r *http.Request, tenantID, lockID string) bool {
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return false
	}
	record, err := s.spaceSvc.GetDoor(tenantID, lockID)
	if err != nil {
		handleSpaceReferenceError(w, err)
		return false
	}
	return s.requireBuildingScope(w, buildingScope, record.BuildingID)
}

func (s *server) referenceGatewayDeviceByID(tenantID, deviceID string) (gateway.Gateway, gateway.GatewayDevice, bool) {
	nextDeviceID := strings.TrimSpace(deviceID)
	nextDeviceID = strings.TrimPrefix(nextDeviceID, "terminal_")
	if nextDeviceID == "" {
		return gateway.Gateway{}, gateway.GatewayDevice{}, false
	}
	items := s.gatewaySvc.List(strings.TrimSpace(tenantID))
	for i := range items {
		for j := range items[i].Devices {
			if items[i].Devices[j].ID == nextDeviceID {
				return items[i], items[i].Devices[j], true
			}
		}
	}
	return gateway.Gateway{}, gateway.GatewayDevice{}, false
}

func gatewayDeviceBySerial(parent gateway.Gateway, serialNumber string) (gateway.GatewayDevice, bool) {
	nextSerialNumber := strings.TrimSpace(serialNumber)
	if nextSerialNumber == "" {
		return gateway.GatewayDevice{}, false
	}
	for i := range parent.Devices {
		if strings.EqualFold(parent.Devices[i].SerialNumber, nextSerialNumber) {
			return parent.Devices[i], true
		}
	}
	return gateway.GatewayDevice{}, false
}

func referenceControllerFromGateway(item gateway.Gateway) referenceController {
	timestamp := item.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return referenceController{
		ID:           item.ID,
		ResourceType: "Controller",
		TenantID:     item.TenantID,
		PlaceID:      item.BuildingID,
		Name:         item.SerialNumber,
		Description:  fmt.Sprintf("%d-device controller", item.DeviceCapacity),
		DeviceID:     item.SerialNumber,
		Token:        item.SerialNumber,
		Status:       item.Status,
		Configured:   len(item.BoundDoorIDs) > 0 || len(item.Devices) > 0,
		LockIDs:      append([]string(nil), item.BoundDoorIDs...),
		LastSeenAt:   timestamp,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}
}

func referenceReaderFromGatewayDevice(parent gateway.Gateway, item gateway.GatewayDevice) referenceReader {
	timestamp := item.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return referenceReader{
		ID:           item.ID,
		ResourceType: "Reader",
		TenantID:     parent.TenantID,
		PlaceID:      parent.BuildingID,
		ControllerID: parent.ID,
		Name:         item.SerialNumber,
		Description:  titleizeReference(item.Kind),
		DeviceID:     item.SerialNumber,
		Token:        item.SerialNumber,
		Model:        readerModel(item),
		Protocol:     item.Protocol,
		Status:       item.Status,
		Configured:   item.GatewayID != "",
		LockIDs:      append([]string(nil), parent.BoundDoorIDs...),
		LastSeenAt:   timestamp,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}
}

func referenceTerminalFromGatewayDevice(parent gateway.Gateway, item gateway.GatewayDevice, placeName string) referenceTerminal {
	timestamp := item.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	terminalName := item.SerialNumber
	if strings.TrimSpace(terminalName) == "" {
		terminalName = item.ID
	}
	return referenceTerminal{
		ID:           "terminal_" + item.ID,
		ResourceType: "Terminal",
		TenantID:     parent.TenantID,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
		Name:         terminalName + " Terminal",
		Description:  titleizeReference(item.Kind) + " access terminal",
		PlaceID:      parent.BuildingID,
		Place: referenceTerminalPlace{
			ID:           parent.BuildingID,
			ResourceType: "Place",
			Name:         placeName,
		},
		MarketplaceInstallationID: nil,
		ControllerID:              parent.ID,
		ReaderID:                  item.ID,
		Status:                    item.Status,
		LastSeenAt:                timestamp,
	}
}

func referenceIntegrationFromIDPConfig(config enterprise.IDPConfig) referenceIntegration {
	createdAt := config.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	updatedAt := config.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		provider = "idp"
	}
	return referenceIntegration{
		ID:           "integration_" + config.ID,
		ResourceType: "Integration",
		TenantID:     config.TenantID,
		Type:         "identity_provider",
		Provider:     provider,
		Name:         strings.ToUpper(provider) + " Identity Provider",
		Description:  "Admin sign-in and just-in-time user sync",
		Status:       config.Status,
		Configured:   strings.TrimSpace(config.IssuerURL) != "" && strings.TrimSpace(config.ClientID) != "",
		SyncMode:     config.SyncMode,
		SourceID:     config.ID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func referenceIntegrationFromHRISConnector(connector enterprise.HRISConnector) referenceIntegration {
	createdAt := connector.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	updatedAt := connector.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	lastSyncAt := ""
	if connector.LastSyncAt != nil {
		lastSyncAt = connector.LastSyncAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	vendor := strings.TrimSpace(connector.Vendor)
	if vendor == "" {
		vendor = "hris"
	}
	return referenceIntegration{
		ID:           "integration_" + connector.ID,
		ResourceType: "Integration",
		TenantID:     connector.TenantID,
		Type:         "hris",
		Provider:     vendor,
		Name:         strings.ToUpper(vendor) + " HRIS",
		Description:  "Directory and employee access sync",
		Status:       connector.Status,
		Configured:   strings.TrimSpace(connector.CredentialRef) != "" || strings.TrimSpace(connector.WebhookSecretRef) != "",
		SyncMode:     connector.SyncStrategy,
		SourceID:     connector.ID,
		LastSyncAt:   lastSyncAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func (s *server) referenceIntegrationByID(tenantID string, integrationID string) (referenceIntegration, bool, error) {
	nextIntegrationID := strings.TrimSpace(integrationID)
	if nextIntegrationID == "" {
		return referenceIntegration{}, false, nil
	}
	filterTenantID := strings.TrimSpace(tenantID)
	sourceID := referenceIntegrationSourceID(nextIntegrationID)

	if filterTenantID != "" {
		if config, err := s.enterpriseSvc.GetIDPConfig(filterTenantID); err == nil {
			integration := referenceIntegrationFromIDPConfig(config)
			if integration.ID == nextIntegrationID || integration.SourceID == sourceID {
				return integration, true, nil
			}
		} else if !errors.Is(err, enterprise.ErrIDPConfigNotFound) {
			return referenceIntegration{}, false, err
		}

		connector, err := s.enterpriseSvc.GetHRISConnector(filterTenantID, sourceID)
		if err == nil {
			integration := referenceIntegrationFromHRISConnector(connector)
			if integration.ID == nextIntegrationID || integration.SourceID == sourceID {
				return integration, true, nil
			}
		}
		if errors.Is(err, enterprise.ErrHRISConnectorNotFound) {
			return referenceIntegration{}, false, nil
		}
		return referenceIntegration{}, false, err
	}

	connector, err := s.enterpriseSvc.GetHRISConnectorByID(sourceID)
	if err == nil {
		integration := referenceIntegrationFromHRISConnector(connector)
		if integration.ID == nextIntegrationID || integration.SourceID == sourceID {
			return integration, true, nil
		}
	}
	if errors.Is(err, enterprise.ErrHRISConnectorNotFound) {
		return referenceIntegration{}, false, nil
	}
	return referenceIntegration{}, false, err
}

func referenceIntegrationSourceID(integrationID string) string {
	nextIntegrationID := strings.TrimSpace(integrationID)
	if strings.HasPrefix(nextIntegrationID, "integration_") {
		return strings.TrimSpace(strings.TrimPrefix(nextIntegrationID, "integration_"))
	}
	return nextIntegrationID
}

func normalizeReferenceIntegrationType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "identity_provider", "idp", "sso", "saml", "oidc":
		return "identity_provider"
	case "hris", "directory":
		return "hris"
	default:
		return ""
	}
}

func (s *server) referenceIntegrationActor(r *http.Request, actor string) string {
	nextActor := strings.TrimSpace(actor)
	if nextActor != "" {
		return nextActor
	}
	if user, exists := authenticatedUser(r); exists {
		return strings.TrimSpace(user.Email)
	}
	return "system"
}

func (s *server) referenceAlertPolicyTenantIDs(tenantID string) []string {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID != "" {
		return []string{nextTenantID}
	}
	tenants := s.tenantSvc.List()
	ids := make([]string, 0, len(tenants))
	for i := range tenants {
		if id := strings.TrimSpace(tenants[i].ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *server) resolveReferenceAlertPolicyRequest(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	policyID := strings.TrimSpace(chi.URLParam(r, "policyID"))
	policyKind, tenantID := referenceAlertPolicyIDParts(policyID)
	if policyKind == "" || tenantID == "" {
		if customPolicy, exists := s.referenceCustomAlertPolicyByID(policyID); exists {
			policyKind = customPolicy.ID
			tenantID = customPolicy.TenantID
		}
	}
	if policyKind == "" || tenantID == "" {
		writeError(w, http.StatusNotFound, "alert policy not found")
		return "", "", false
	}
	resolvedTenantID, ok := s.resolveTenantID(w, r, tenantID)
	if !ok {
		return "", "", false
	}
	if strings.TrimSpace(resolvedTenantID) == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return "", "", false
	}
	return policyKind, resolvedTenantID, true
}

func referenceAlertPolicyIDParts(policyID string) (string, string) {
	nextPolicyID := strings.TrimSpace(policyID)
	switch {
	case strings.HasPrefix(nextPolicyID, "ap_enterprise_sync_worker_"):
		tenantID := strings.TrimSpace(strings.TrimPrefix(nextPolicyID, "ap_enterprise_sync_worker_"))
		if tenantID == "" {
			return "", ""
		}
		return "enterprise_sync_worker", tenantID
	case strings.HasPrefix(nextPolicyID, "ap_wallet_jobs_"):
		tenantID := strings.TrimSpace(strings.TrimPrefix(nextPolicyID, "ap_wallet_jobs_"))
		if tenantID == "" {
			return "", ""
		}
		return "wallet_jobs", tenantID
	default:
		return "", ""
	}
}

func referenceAlertPolicyKindFromPayload(payload referenceAlertPolicyPayload) string {
	category := strings.ToLower(strings.TrimSpace(payload.Category))
	switch category {
	case "enterprise_sync_worker", "wallet_jobs", "custom":
		return category
	case "enterprise_sync", "sync_worker":
		return "enterprise_sync_worker"
	case "wallet", "wallet_job":
		return "wallet_jobs"
	}

	switch strings.ToLower(strings.TrimSpace(payload.Trigger)) {
	case "worker_failure_threshold", "enterprise_sync_worker_failure_threshold":
		return "enterprise_sync_worker"
	case "wallet_job_dlq_threshold", "wallet_jobs_dlq_threshold":
		return "wallet_jobs"
	default:
		return ""
	}
}

func (s *server) referenceAlertPolicyByKind(policyKind string, tenantID string) (referenceAlertPolicy, bool) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return referenceAlertPolicy{}, false
	}
	switch policyKind {
	case "enterprise_sync_worker":
		subscription, found := s.enterpriseSvc.GetSyncWorkerAlertSubscription(nextTenantID)
		if !found {
			subscription = s.defaultEnterpriseSyncWorkerAlertSubscription(nextTenantID)
		}
		return referenceAlertPolicyFromEnterpriseSubscription(subscription), true
	case "wallet_jobs":
		subscription, found := s.walletSvc.GetJobAlertSubscription(nextTenantID)
		if !found {
			subscription = s.defaultWalletJobAlertSubscription(nextTenantID)
		}
		return referenceAlertPolicyFromWalletSubscription(subscription), true
	default:
		policy, exists := s.referenceCustomAlertPolicyByID(policyKind)
		if !exists || policy.TenantID != nextTenantID {
			return referenceAlertPolicy{}, false
		}
		return policy, true
	}
}

func (s *server) updateReferenceEnterpriseSyncWorkerAlertPolicy(
	tenantID string,
	payload referenceAlertPolicyPayload,
) (referenceAlertPolicy, error) {
	current, found := s.enterpriseSvc.GetSyncWorkerAlertSubscription(tenantID)
	if !found {
		current = s.defaultEnterpriseSyncWorkerAlertSubscription(tenantID)
	}
	enabled, err := resolveReferenceAlertPolicyEnabled(current.Enabled, payload)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	threshold := current.WorkerAlertThreshold
	if payload.Threshold != nil {
		threshold = *payload.Threshold
	}
	windowSeconds := current.WindowSeconds
	if payload.WindowSeconds != nil {
		windowSeconds = *payload.WindowSeconds
	}
	cooldownSeconds := current.CooldownSeconds
	if payload.CooldownSeconds != nil {
		cooldownSeconds = *payload.CooldownSeconds
	}
	emailEnabled := current.Channels.Email
	whatsAppEnabled := current.Channels.WhatsApp
	if payload.Channels != nil {
		if payload.Channels.Email != nil {
			emailEnabled = *payload.Channels.Email
		}
		if payload.Channels.WhatsApp != nil {
			whatsAppEnabled = *payload.Channels.WhatsApp
		}
	}
	receiverGroups := current.ReceiverGroups
	if payload.ReceiverGroups != nil {
		receiverGroups = payload.ReceiverGroups
	}
	record, err := s.enterpriseSvc.UpsertSyncWorkerAlertSubscription(
		enterprise.SyncWorkerAlertSubscriptionUpsertOptions{
			TenantID:             tenantID,
			Enabled:              enabled,
			WorkerAlertThreshold: threshold,
			Window:               time.Duration(windowSeconds) * time.Second,
			Cooldown:             time.Duration(cooldownSeconds) * time.Second,
			EmailEnabled:         emailEnabled,
			WhatsAppEnabled:      whatsAppEnabled,
			ReceiverGroups:       receiverGroups,
		},
	)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	return referenceAlertPolicyFromEnterpriseSubscription(record), nil
}

func (s *server) updateReferenceWalletJobAlertPolicy(
	tenantID string,
	payload referenceAlertPolicyPayload,
) (referenceAlertPolicy, error) {
	current, found := s.walletSvc.GetJobAlertSubscription(tenantID)
	if !found {
		current = s.defaultWalletJobAlertSubscription(tenantID)
	}
	enabled, err := resolveReferenceAlertPolicyEnabled(current.Enabled, payload)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	threshold := current.DLQAlertThreshold
	if payload.Threshold != nil {
		threshold = *payload.Threshold
	}
	windowSeconds := current.WindowSeconds
	if payload.WindowSeconds != nil {
		windowSeconds = *payload.WindowSeconds
	}
	cooldownSeconds := current.CooldownSeconds
	if payload.CooldownSeconds != nil {
		cooldownSeconds = *payload.CooldownSeconds
	}
	emailEnabled := current.Channels.Email
	whatsAppEnabled := current.Channels.WhatsApp
	if payload.Channels != nil {
		if payload.Channels.Email != nil {
			emailEnabled = *payload.Channels.Email
		}
		if payload.Channels.WhatsApp != nil {
			whatsAppEnabled = *payload.Channels.WhatsApp
		}
	}
	receiverGroups := current.ReceiverGroups
	if payload.ReceiverGroups != nil {
		receiverGroups = payload.ReceiverGroups
	}
	record, err := s.walletSvc.UpsertJobAlertSubscription(
		wallet.JobAlertSubscriptionUpsertOptions{
			TenantID:          tenantID,
			Enabled:           enabled,
			DLQAlertThreshold: threshold,
			Window:            time.Duration(windowSeconds) * time.Second,
			Cooldown:          time.Duration(cooldownSeconds) * time.Second,
			EmailEnabled:      emailEnabled,
			WhatsAppEnabled:   whatsAppEnabled,
			ReceiverGroups:    receiverGroups,
			Actor:             payload.Actor,
		},
	)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	return referenceAlertPolicyFromWalletSubscription(record), nil
}

func (s *server) createReferenceCustomAlertPolicy(
	tenantID string,
	payload referenceAlertPolicyPayload,
) (referenceAlertPolicy, error) {
	trigger := strings.TrimSpace(payload.Trigger)
	if trigger == "" {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	enabled, err := resolveReferenceAlertPolicyEnabled(false, payload)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	policy, err := referenceCustomAlertPolicyFromPayload("", tenantID, enabled, payload)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	policy.UpdatedAt = now

	s.customAlertPolicyMu.Lock()
	defer s.customAlertPolicyMu.Unlock()
	s.customAlertPolicySeq++
	policy.ID = fmt.Sprintf("ap_custom_%06d", s.customAlertPolicySeq)
	s.customAlertPolicies[policy.ID] = policy
	return policy, nil
}

func (s *server) updateReferenceCustomAlertPolicy(
	policyID string,
	tenantID string,
	payload referenceAlertPolicyPayload,
) (referenceAlertPolicy, error) {
	current, exists := s.referenceCustomAlertPolicyByID(policyID)
	if !exists || current.TenantID != tenantID {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	enabled, err := resolveReferenceAlertPolicyEnabled(current.Enabled, payload)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	merged := referenceAlertPolicyPayload{
		TenantID:    tenantID,
		Name:        firstNonEmptyString(payload.Name, current.Name),
		Description: firstNonEmptyString(payload.Description, current.Description),
		Category:    "custom",
		Trigger:     firstNonEmptyString(payload.Trigger, current.Trigger),
		Severity:    firstNonEmptyString(payload.Severity, current.Severity),
		Condition:   firstNonEmptyString(payload.Condition, current.Condition),
		Enabled:     &enabled,
	}
	threshold := current.Threshold
	if payload.Threshold != nil {
		threshold = *payload.Threshold
	}
	windowSeconds := current.WindowSeconds
	if payload.WindowSeconds != nil {
		windowSeconds = *payload.WindowSeconds
	}
	cooldownSeconds := current.CooldownSeconds
	if payload.CooldownSeconds != nil {
		cooldownSeconds = *payload.CooldownSeconds
	}
	merged.Threshold = &threshold
	merged.WindowSeconds = &windowSeconds
	merged.CooldownSeconds = &cooldownSeconds
	if payload.Channels != nil {
		merged.Channels = payload.Channels
	} else {
		email := current.Channels.Email
		whatsApp := current.Channels.WhatsApp
		merged.Channels = &struct {
			Email    *bool `json:"email"`
			WhatsApp *bool `json:"whatsapp"`
		}{Email: &email, WhatsApp: &whatsApp}
	}
	if payload.ReceiverGroups != nil {
		merged.ReceiverGroups = payload.ReceiverGroups
	} else {
		merged.ReceiverGroups = append([]string(nil), current.ReceiverGroups...)
	}
	policy, err := referenceCustomAlertPolicyFromPayload(current.ID, tenantID, enabled, merged)
	if err != nil {
		return referenceAlertPolicy{}, err
	}
	policy.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	s.customAlertPolicyMu.Lock()
	defer s.customAlertPolicyMu.Unlock()
	s.customAlertPolicies[policy.ID] = policy
	return policy, nil
}

func referenceCustomAlertPolicyFromPayload(
	policyID string,
	tenantID string,
	enabled bool,
	payload referenceAlertPolicyPayload,
) (referenceAlertPolicy, error) {
	trigger := strings.TrimSpace(payload.Trigger)
	if trigger == "" {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	threshold := 1
	if payload.Threshold != nil {
		threshold = *payload.Threshold
	}
	if threshold < 1 {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	windowSeconds := int64(900)
	if payload.WindowSeconds != nil {
		windowSeconds = *payload.WindowSeconds
	}
	if windowSeconds < 1 {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	cooldownSeconds := int64(900)
	if payload.CooldownSeconds != nil {
		cooldownSeconds = *payload.CooldownSeconds
	}
	if cooldownSeconds < 0 {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	emailEnabled := true
	whatsAppEnabled := false
	if payload.Channels != nil {
		if payload.Channels.Email != nil {
			emailEnabled = *payload.Channels.Email
		}
		if payload.Channels.WhatsApp != nil {
			whatsAppEnabled = *payload.Channels.WhatsApp
		}
	}
	if enabled && !emailEnabled && !whatsAppEnabled {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	receiverGroups := uniqueStrings(payload.ReceiverGroups)
	if len(receiverGroups) == 0 {
		receiverGroups = []string{"security"}
	}
	if err := validateReferenceAlertPolicyCondition(payload.Condition); err != nil {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	severity := normalizeReferenceAlertPolicySeverity(payload.Severity)
	if severity == "" {
		return referenceAlertPolicy{}, errInvalidReferenceAlertPolicyPayload
	}
	name := firstNonEmptyString(payload.Name, titleizeReference(trigger))
	description := firstNonEmptyString(payload.Description, "Custom alert policy for "+trigger+".")
	return referenceAlertPolicy{
		ID:              strings.TrimSpace(policyID),
		ResourceType:    "AlertPolicy",
		TenantID:        strings.TrimSpace(tenantID),
		Name:            name,
		Description:     description,
		Category:        "custom",
		Trigger:         trigger,
		Severity:        severity,
		Condition:       strings.TrimSpace(payload.Condition),
		Status:          referenceAlertPolicyStatus(enabled),
		Enabled:         enabled,
		Threshold:       threshold,
		WindowSeconds:   windowSeconds,
		CooldownSeconds: cooldownSeconds,
		Channels: referenceAlertPolicyChannels{
			Email:    emailEnabled,
			WhatsApp: whatsAppEnabled,
		},
		ReceiverGroups: receiverGroups,
	}, nil
}

func (s *server) referenceCustomAlertPolicies(tenantID string) []referenceAlertPolicy {
	s.customAlertPolicyMu.RLock()
	defer s.customAlertPolicyMu.RUnlock()
	items := make([]referenceAlertPolicy, 0, len(s.customAlertPolicies))
	for _, policy := range s.customAlertPolicies {
		if strings.TrimSpace(policy.TenantID) != strings.TrimSpace(tenantID) {
			continue
		}
		items = append(items, policy)
	}
	return items
}

func (s *server) referenceCustomAlertPolicyByID(policyID string) (referenceAlertPolicy, bool) {
	s.customAlertPolicyMu.RLock()
	defer s.customAlertPolicyMu.RUnlock()
	policy, exists := s.customAlertPolicies[strings.TrimSpace(policyID)]
	return policy, exists
}

func resolveReferenceAlertPolicyEnabled(current bool, payload referenceAlertPolicyPayload) (bool, error) {
	enabled := current
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}
	switch strings.ToLower(strings.TrimSpace(payload.Status)) {
	case "":
	case "active", "enabled":
		if payload.Enabled != nil && !*payload.Enabled {
			return false, errInvalidReferenceAlertPolicyPayload
		}
		enabled = true
	case "inactive", "disabled":
		if payload.Enabled != nil && *payload.Enabled {
			return false, errInvalidReferenceAlertPolicyPayload
		}
		enabled = false
	default:
		return false, errInvalidReferenceAlertPolicyPayload
	}
	return enabled, nil
}

func referenceAlertPolicyFromEnterpriseSubscription(item enterprise.SyncWorkerAlertSubscription) referenceAlertPolicy {
	updatedAt := item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return referenceAlertPolicy{
		ID:              "ap_enterprise_sync_worker_" + item.TenantID,
		ResourceType:    "AlertPolicy",
		TenantID:        item.TenantID,
		Name:            "Enterprise Sync Worker Alerts",
		Description:     "Notify operations when enterprise directory workers cross the failure threshold.",
		Category:        "enterprise_sync_worker",
		Trigger:         "worker_failure_threshold",
		Severity:        "high",
		Status:          referenceAlertPolicyStatus(item.Enabled),
		Enabled:         item.Enabled,
		Threshold:       item.WorkerAlertThreshold,
		WindowSeconds:   item.WindowSeconds,
		CooldownSeconds: item.CooldownSeconds,
		Channels: referenceAlertPolicyChannels{
			Email:    item.Channels.Email,
			WhatsApp: item.Channels.WhatsApp,
		},
		ReceiverGroups: append([]string(nil), item.ReceiverGroups...),
		UpdatedAt:      updatedAt,
	}
}

func referenceAlertPolicyFromWalletSubscription(item wallet.JobAlertSubscription) referenceAlertPolicy {
	updatedAt := item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return referenceAlertPolicy{
		ID:              "ap_wallet_jobs_" + item.TenantID,
		ResourceType:    "AlertPolicy",
		TenantID:        item.TenantID,
		Name:            "Wallet Job Queue Alerts",
		Description:     "Notify operations when wallet job failures or DLQ backlog exceed the configured threshold.",
		Category:        "wallet_jobs",
		Trigger:         "wallet_job_dlq_threshold",
		Severity:        "high",
		Status:          referenceAlertPolicyStatus(item.Enabled),
		Enabled:         item.Enabled,
		Threshold:       item.DLQAlertThreshold,
		WindowSeconds:   item.WindowSeconds,
		CooldownSeconds: item.CooldownSeconds,
		Channels: referenceAlertPolicyChannels{
			Email:    item.Channels.Email,
			WhatsApp: item.Channels.WhatsApp,
		},
		ReceiverGroups: append([]string(nil), item.ReceiverGroups...),
		UpdatedAt:      updatedAt,
	}
}

func referenceAlertPolicyStatus(enabled bool) string {
	if enabled {
		return "active"
	}
	return "inactive"
}

func normalizeReferenceAlertPolicySeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "warning":
		return "warning"
	case "info", "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func isReaderDevice(item gateway.GatewayDevice) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(item.Kind)), "reader")
}

func readerModel(item gateway.GatewayDevice) string {
	if strings.Contains(strings.ToLower(item.Kind), "legacy") {
		return "reader-1.0"
	}
	return "reader-pro-3.0"
}

func titleizeReference(value string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "_", " "), "-", " "))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}

func referenceTeamAuditTarget(item access.Team) string {
	return fmt.Sprintf(
		"team_id=%s,name=%s,scope=%s,place_id=%s,source=%s",
		item.ID,
		item.Name,
		item.Scope,
		item.PlaceID,
		item.Source,
	)
}

func referenceTeamMembershipAuditTarget(item access.TeamMembership) string {
	return fmt.Sprintf(
		"team_membership_id=%s,team_id=%s,member_type=%s,member_id=%s,email=%s,source=%s",
		item.ID,
		item.TeamID,
		item.MemberType,
		item.MemberID,
		item.MemberEmail,
		item.Source,
	)
}

func referenceRoleAssignmentAuditTarget(item access.RoleAssignment) string {
	return fmt.Sprintf(
		"role_assignment_id=%s,role_id=%s,applies_to_type=%s,applies_to_id=%s,assignee_type=%s,assignee_id=%s,email=%s",
		item.ID,
		item.RoleID,
		item.AppliesToType,
		item.AppliesToID,
		item.AssigneeType,
		item.AssigneeID,
		item.AssigneeEmail,
	)
}

func referenceTemporaryAccessShareAuditTarget(item access.TemporaryAccess) string {
	return fmt.Sprintf(
		"share_id=%s,scope_type=%s,place_id=%s,area_id=%s,lock_id=%s,group_id=%s,role_id=%s,email=%s,pass_type=%s",
		item.ID,
		item.ScopeType,
		item.BuildingID,
		item.AreaID,
		item.DoorID,
		item.GroupID,
		item.RoleID,
		item.GranteeEmail,
		item.PassType,
	)
}

func referenceWalletPassAuditTarget(item wallet.PassInstance) string {
	return fmt.Sprintf(
		"card_id=%s,object_id=%s,target_type=%s,target_id=%s,status=%s",
		item.ID,
		item.ObjectID,
		item.TargetType,
		item.TargetID,
		referenceCardStatus(item.Status),
	)
}

func referenceCardStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "activated"
	case "issued":
		return "unassigned"
	case "revoked":
		return "revoked"
	default:
		return "deactivated"
	}
}

func referenceCardStatusAuditAction(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "reference_card_activated"
	case "suspended":
		return "reference_card_deactivated"
	case "revoked":
		return "reference_card_revoked"
	default:
		return "reference_card_status_changed"
	}
}

func referenceCardAssignmentStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "active") {
		return "activated"
	}
	return "deactivated"
}

func shareMatchesQuery(share referenceShare, r *http.Request) bool {
	if roleID := strings.TrimSpace(r.URL.Query().Get("role_id")); roleID != "" && share.RoleID != roleID {
		return false
	}
	if groupID := strings.TrimSpace(r.URL.Query().Get("group_id")); groupID != "" && share.GroupID != groupID {
		return false
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && share.PlaceID != placeID {
		return false
	}
	if areaID := strings.TrimSpace(r.URL.Query().Get("area_id")); areaID != "" && share.AreaID != areaID {
		return false
	}
	if lockID := strings.TrimSpace(r.URL.Query().Get("lock_id")); lockID != "" && share.LockID != lockID {
		return false
	}
	if email := strings.TrimSpace(r.URL.Query().Get("email")); email != "" && !strings.EqualFold(share.Email, email) {
		return false
	}
	return true
}

func groupLinkMatchesQuery(link access.GroupLink, query string) bool {
	nextQuery := strings.ToLower(strings.TrimSpace(query))
	if nextQuery == "" {
		return true
	}
	values := []string{link.Name, link.Email, link.Phone, link.GroupName}
	for i := range values {
		if strings.Contains(strings.ToLower(strings.TrimSpace(values[i])), nextQuery) {
			return true
		}
	}
	return false
}

func cardMatchesQuery(card referenceCard, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[card.ID]; !exists {
			return false
		}
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(card.Status, status) {
		return false
	}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" && !strings.EqualFold(card.Provider, provider) {
		return false
	}
	if credentialKind := strings.TrimSpace(r.URL.Query().Get("credential_kind")); credentialKind != "" && !strings.EqualFold(card.CredentialKind, credentialKind) {
		return false
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" && card.Token != token {
		return false
	}
	if uid := strings.TrimSpace(r.URL.Query().Get("uid")); uid != "" && card.UID != uid {
		return false
	}
	if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" && userID != "me" && card.UserID != userID {
		return false
	}
	if cardNumber := strings.TrimSpace(r.URL.Query().Get("card_number")); cardNumber != "" && card.CardNumber != cardNumber {
		return false
	}
	if cardID := strings.TrimSpace(r.URL.Query().Get("card_id")); cardID != "" && card.ID != cardID {
		return false
	}
	return true
}

func cardAssignmentMatchesQuery(assignment referenceCardAssignment, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[assignment.ID]; !exists {
			return false
		}
	}
	if cardID := strings.TrimSpace(r.URL.Query().Get("card_id")); cardID != "" && assignment.CardID != cardID {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(assignment.Status, status) {
		return false
	}
	if assigneeType := strings.TrimSpace(r.URL.Query().Get("assignee_type")); assigneeType != "" && !strings.EqualFold(assignment.AssigneeType, assigneeType) {
		return false
	}
	if assigneeID := strings.TrimSpace(r.URL.Query().Get("assignee_id")); assigneeID != "" && assignment.AssigneeID != assigneeID {
		return false
	}
	if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" && userID != "me" && assignment.UserID != userID {
		return false
	}
	return true
}

func controllerMatchesQuery(controller referenceController, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[controller.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(controller.Name + " " + controller.DeviceID + " " + controller.Token)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && controller.PlaceID != placeID {
		return false
	}
	if lockID := strings.TrimSpace(r.URL.Query().Get("lock_id")); lockID != "" && !containsString(controller.LockIDs, lockID) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(controller.Status, status) {
		return false
	}
	return true
}

func readerMatchesQuery(reader referenceReader, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[reader.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(reader.Name + " " + reader.DeviceID + " " + reader.Token)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if model := strings.TrimSpace(r.URL.Query().Get("model")); model != "" && !strings.EqualFold(reader.Model, model) {
		return false
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && reader.PlaceID != placeID {
		return false
	}
	if lockID := strings.TrimSpace(r.URL.Query().Get("lock_id")); lockID != "" && !containsString(reader.LockIDs, lockID) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(reader.Status, status) {
		return false
	}
	return true
}

func terminalMatchesQuery(terminal referenceTerminal, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[terminal.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(terminal.Name + " " + terminal.Description + " " + terminal.Place.Name)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && terminal.PlaceID != placeID {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(terminal.Status, status) {
		return false
	}
	return true
}

func integrationMatchesQuery(integration referenceIntegration, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[integration.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(integration.Name + " " + integration.Description + " " + integration.Provider)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if integrationType := strings.TrimSpace(r.URL.Query().Get("type")); integrationType != "" && !strings.EqualFold(integration.Type, integrationType) {
		return false
	}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" && !strings.EqualFold(integration.Provider, provider) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(integration.Status, status) {
		return false
	}
	return true
}

func referenceAlertPolicyMatchesQuery(policy referenceAlertPolicy, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[policy.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(firstNonEmptyString(r.URL.Query().Get("query"), r.URL.Query().Get("q")))); query != "" {
		haystack := strings.ToLower(policy.Name + " " + policy.Description + " " + policy.Category + " " + policy.Trigger)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if category := strings.TrimSpace(r.URL.Query().Get("category")); category != "" && !strings.EqualFold(policy.Category, category) {
		return false
	}
	if trigger := strings.TrimSpace(r.URL.Query().Get("trigger")); trigger != "" && !strings.EqualFold(policy.Trigger, trigger) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(policy.Status, status) {
		return false
	}
	return true
}

func sortReferenceControllers(items []referenceController, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
}

func sortReferenceReaders(items []referenceReader, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
}

func sortReferenceTerminals(items []referenceTerminal, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
}

func sortReferenceIntegrations(items []referenceIntegration, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		if items[i].Type == items[j].Type {
			return items[i].Name < items[j].Name
		}
		return items[i].Type < items[j].Type
	})
}

func sortReferenceAlertPolicies(items []referenceAlertPolicy, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		if items[i].TenantID == items[j].TenantID {
			return items[i].Name < items[j].Name
		}
		return items[i].TenantID < items[j].TenantID
	})
}

func sortGroupLinks(items []access.GroupLink, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		switch strings.TrimSpace(r.URL.Query().Get("sort")) {
		case "-name":
			return items[i].Name > items[j].Name
		case "valid_until":
			return items[i].ValidUntil < items[j].ValidUntil
		case "-valid_until":
			return items[i].ValidUntil > items[j].ValidUntil
		case "group_name":
			return items[i].GroupName < items[j].GroupName
		case "-group_name":
			return items[i].GroupName > items[j].GroupName
		default:
			return items[i].Name < items[j].Name
		}
	})
}

func containsString(values []string, target string) bool {
	for i := range values {
		if values[i] == target {
			return true
		}
	}
	return false
}

func commaSetContains(values string, target string) bool {
	_, exists := commaSet(values)[strings.TrimSpace(target)]
	return exists
}

func commaSet(value string) map[string]struct{} {
	parts := strings.Split(value, ",")
	items := make(map[string]struct{}, len(parts))
	for i := range parts {
		next := strings.TrimSpace(parts[i])
		if next == "" {
			continue
		}
		items[next] = struct{}{}
	}
	return items
}

func filterBuildingsByID(items []space.Building, id string) []space.Building {
	filtered := make([]space.Building, 0, 1)
	for i := range items {
		if items[i].ID == id {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func filterBuildingsByStatus(items []space.Building, status string) []space.Building {
	filtered := make([]space.Building, 0, len(items))
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].Status), status) {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func referenceIncludeArchivedPlaces(r *http.Request) bool {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "archived" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_archived"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func filterDoorsByPlaceID(items []space.Door, placeID string) []space.Door {
	filtered := make([]space.Door, 0, len(items))
	for i := range items {
		if items[i].BuildingID == placeID {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func filterUserGroupsByPlaceID(items []access.UserGroup, placeID string) []access.UserGroup {
	filtered := make([]access.UserGroup, 0, len(items))
	for i := range items {
		if items[i].BuildingID == placeID {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func filterAccessUsersByPlaceID(items []access.AccessUser, placeID string) []access.AccessUser {
	filtered := make([]access.AccessUser, 0, len(items))
	for i := range items {
		if items[i].BuildingID == placeID {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func writeCollection(w http.ResponseWriter, r *http.Request, status int, items any) {
	paginatedItems, pagination := paginateCollection(items, r)
	last := pagination.Offset + pagination.Count - 1
	if pagination.Count == 0 {
		last = pagination.Offset
	}
	w.Header().Set("X-Collection-Range", fmt.Sprintf("items %d-%d/%d", pagination.Offset, last, pagination.Total))
	writeJSON(w, status, map[string]any{
		"items": paginatedItems,
		"pagination": map[string]any{
			"offset":   pagination.Offset,
			"limit":    pagination.Limit,
			"total":    pagination.Total,
			"has_more": pagination.HasMore,
		},
	})
}

type collectionPagination struct {
	Offset  int
	Limit   int
	Total   int
	Count   int
	HasMore bool
}

func paginateCollection(items any, r *http.Request) (any, collectionPagination) {
	total := collectionCount(items)
	offset := parseCollectionPaginationInt(r, "offset", 0)
	if offset > total {
		offset = total
	}
	limit := parseCollectionPaginationInt(r, "limit", total-offset)
	if limit > total-offset {
		limit = total - offset
	}
	if limit < 0 {
		limit = 0
	}
	count := limit
	end := offset + count
	if end > total {
		end = total
		count = end - offset
	}
	return sliceCollection(items, offset, end), collectionPagination{
		Offset:  offset,
		Limit:   limit,
		Total:   total,
		Count:   count,
		HasMore: end < total,
	}
}

func parseCollectionPaginationInt(r *http.Request, key string, fallback int) int {
	if r == nil {
		return fallback
	}
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func sliceCollection(items any, start, end int) any {
	value := reflect.ValueOf(items)
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return items
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > value.Len() {
		end = value.Len()
	}
	if start > value.Len() {
		start = value.Len()
	}
	return value.Slice(start, end).Interface()
}

func collectionCount(items any) int {
	value := reflect.ValueOf(items)
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return 0
	}
	return value.Len()
}

func handleAccessReferenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, access.ErrTenantIDRequired),
		errors.Is(err, access.ErrRoleIDRequired),
		errors.Is(err, access.ErrInvalidRoleScope),
		errors.Is(err, access.ErrInvalidAssigneeType),
		errors.Is(err, access.ErrAppliesToIDRequired),
		errors.Is(err, access.ErrAssigneeIDRequired),
		errors.Is(err, access.ErrInvalidScopeType),
		errors.Is(err, access.ErrUserGroupNameRequired),
		errors.Is(err, access.ErrGroupIDRequired),
		errors.Is(err, access.ErrGroupLinkNameRequired),
		errors.Is(err, access.ErrInvalidGroupLinkQRCodeType),
		errors.Is(err, access.ErrTeamNameRequired),
		errors.Is(err, access.ErrTeamIDRequired),
		errors.Is(err, access.ErrInvalidTeamMemberType),
		errors.Is(err, access.ErrTeamMemberIDRequired),
		errors.Is(err, access.ErrDeliveryMethodInvalid),
		errors.Is(err, access.ErrGranteeNameRequired),
		errors.Is(err, access.ErrGranteePhoneRequired),
		errors.Is(err, access.ErrGranteeEmailRequired),
		errors.Is(err, access.ErrValidUntilRequired),
		errors.Is(err, access.ErrAccessRightSelectionRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, access.ErrRoleNotFound),
		errors.Is(err, access.ErrRoleAssignmentNotFound),
		errors.Is(err, access.ErrUserGroupNotFound),
		errors.Is(err, access.ErrTemporaryAccessNotFound),
		errors.Is(err, access.ErrGroupLinkNotFound),
		errors.Is(err, access.ErrTeamNotFound),
		errors.Is(err, access.ErrTeamMembershipNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleGroupLinkVerificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, access.ErrGroupLinkTokenRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, access.ErrGroupLinkTokenInvalid),
		errors.Is(err, access.ErrGroupLinkNotFound),
		errors.Is(err, access.ErrUserGroupNotFound):
		writeError(w, http.StatusNotFound, access.ErrGroupLinkTokenInvalid.Error())
	case errors.Is(err, access.ErrGroupLinkDisabled),
		errors.Is(err, access.ErrGroupLinkNotYetValid):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, access.ErrGroupLinkExpired):
		writeError(w, http.StatusGone, err.Error())
	case errors.Is(err, access.ErrInvalidGroupLinkValidityWindow):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleWalletReferenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, wallet.ErrTemplateIDRequired),
		errors.Is(err, wallet.ErrInvalidTargetType),
		errors.Is(err, wallet.ErrTargetIDRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, wallet.ErrTemplateNotFound), errors.Is(err, wallet.ErrPassNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, wallet.ErrTemplateInactive), errors.Is(err, wallet.ErrInvalidPassTransition):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleReferenceAlertPolicyMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInvalidReferenceAlertPolicyPayload),
		errors.Is(err, enterprise.ErrTenantIDRequired),
		errors.Is(err, enterprise.ErrInvalidSyncWorkerAlertSubscriptionOptions),
		errors.Is(err, wallet.ErrInvalidJobAlertSubscriptionOptions):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleReferenceIntegrationMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInvalidReferenceIntegrationPayload),
		errors.Is(err, enterprise.ErrTenantIDRequired),
		errors.Is(err, enterprise.ErrInvalidHRISConnectorVendor),
		errors.Is(err, enterprise.ErrInvalidHRISConnectorStatus),
		errors.Is(err, enterprise.ErrInvalidHRISConnectorSyncStrategy),
		errors.Is(err, enterprise.ErrInvalidIDPProvider),
		errors.Is(err, enterprise.ErrIssuerURLRequired),
		errors.Is(err, enterprise.ErrClientIDRequired),
		errors.Is(err, enterprise.ErrSAMLACSURLRequired),
		errors.Is(err, enterprise.ErrInvalidSAMLACSURL),
		errors.Is(err, enterprise.ErrSAMLX509CertRequired),
		errors.Is(err, enterprise.ErrInvalidIDPStatus):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, enterprise.ErrHRISConnectorNotFound),
		errors.Is(err, enterprise.ErrIDPConfigNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, enterprise.ErrHRISConnectorAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleReferenceGatewayMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gateway.ErrTenantIDRequired),
		errors.Is(err, gateway.ErrSerialNumberRequired),
		errors.Is(err, gateway.ErrSerialNumberInvalid),
		errors.Is(err, gateway.ErrInvalidDeviceCapacity),
		errors.Is(err, gateway.ErrGatewayIDRequired),
		errors.Is(err, gateway.ErrDoorIDRequired),
		errors.Is(err, gateway.ErrConfigVersionRequired),
		errors.Is(err, gateway.ErrGatewayDeviceIDRequired),
		errors.Is(err, gateway.ErrGatewayDeviceSerialRequired),
		errors.Is(err, gateway.ErrGatewayDeviceCapacityExceeded),
		errors.Is(err, gateway.ErrGatewayDeviceKindInvalid),
		errors.Is(err, gateway.ErrGatewayDeviceSourceInvalid),
		errors.Is(err, gateway.ErrGatewayDeviceProtocolInvalid),
		errors.Is(err, gateway.ErrGatewayDeviceRS485ConfigInvalid),
		errors.Is(err, gateway.ErrGatewayDeviceRS485ConfigProtocolMismatch),
		errors.Is(err, gateway.ErrGatewayDeviceStatusInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, gateway.ErrGatewayNotFound),
		errors.Is(err, gateway.ErrGatewayDeviceNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrGatewaySerialAlreadyRegistered),
		errors.Is(err, gateway.ErrSerialNumberNotProvisioned),
		errors.Is(err, gateway.ErrSerialNumberTenantMismatch),
		errors.Is(err, gateway.ErrSerialNumberProductTypeMismatch),
		errors.Is(err, gateway.ErrSerialNumberNotAvailable),
		errors.Is(err, gateway.ErrSerialNumberAlreadyConsumed),
		errors.Is(err, gateway.ErrGatewayDeviceSerialConflict),
		errors.Is(err, gateway.ErrGatewayDeviceSerialNotProvisioned),
		errors.Is(err, gateway.ErrGatewayDeviceSerialNotAvailable),
		errors.Is(err, gateway.ErrGatewayDeviceSerialProductTypeMismatch):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleSpaceReferenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, space.ErrTenantIDRequired),
		errors.Is(err, space.ErrBuildingIDRequired),
		errors.Is(err, space.ErrBuildingNameRequired),
		errors.Is(err, space.ErrFloorNameRequired),
		errors.Is(err, space.ErrFloorIDRequired),
		errors.Is(err, space.ErrAreaIDRequired),
		errors.Is(err, space.ErrAreaNameRequired),
		errors.Is(err, space.ErrDoorNameRequired),
		errors.Is(err, space.ErrInvalidDoorKind),
		errors.Is(err, space.ErrInvalidDoorStatus),
		errors.Is(err, space.ErrDoorGroupNameRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, space.ErrTenantOwnershipMismatch):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, space.ErrBuildingNotFound),
		errors.Is(err, space.ErrFloorNotFound),
		errors.Is(err, space.ErrAreaNotFound),
		errors.Is(err, space.ErrDoorNotFound),
		errors.Is(err, space.ErrDoorGroupNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleSpaceMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, space.ErrTenantIDRequired),
		errors.Is(err, space.ErrBuildingIDRequired),
		errors.Is(err, space.ErrBuildingNameRequired),
		errors.Is(err, space.ErrFloorNameRequired),
		errors.Is(err, space.ErrFloorIDRequired),
		errors.Is(err, space.ErrAreaIDRequired),
		errors.Is(err, space.ErrAreaNameRequired),
		errors.Is(err, space.ErrDoorNameRequired),
		errors.Is(err, space.ErrFloorBuildingMismatch),
		errors.Is(err, space.ErrAreaFloorMismatch),
		errors.Is(err, space.ErrInvalidDoorKind),
		errors.Is(err, space.ErrInvalidDoorStatus):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, space.ErrTenantOwnershipMismatch):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, space.ErrBuildingNotFound),
		errors.Is(err, space.ErrFloorNotFound),
		errors.Is(err, space.ErrAreaNotFound),
		errors.Is(err, space.ErrDoorNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

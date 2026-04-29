package bus

// GatewayCommand is published to NATS for a gateway to execute.
// Subject: mistypass.gateway.{gateway_id}.command
type GatewayCommand struct {
	RequestID string `json:"request_id"`
	GatewayID string `json:"gateway_id"`
	Command   string `json:"command"` // "unlock", "lock_down", "cancel_lockdown"
	LockID    string `json:"lock_id,omitempty"`
	PlaceID   string `json:"place_id,omitempty"`
	TenantID  string `json:"tenant_id"`
	IssuedBy  string `json:"issued_by,omitempty"`
	IssuedAt  string `json:"issued_at"`
}

// GatewayEvent is published by a gateway after executing a command.
// Subject: mistypass.gateway.{gateway_id}.event
type GatewayEvent struct {
	RequestID  string `json:"request_id"`
	GatewayID  string `json:"gateway_id"`
	EventType  string `json:"event_type"` // "command_ack", "access_granted", "access_denied", "heartbeat"
	Command    string `json:"command,omitempty"`
	LockID     string `json:"lock_id,omitempty"`
	PlaceID    string `json:"place_id,omitempty"`
	TenantID   string `json:"tenant_id"`
	Status     string `json:"status"` // "success", "failed", "timeout"
	Error      string `json:"error,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

// CredentialVerifyRequest is published by a reader/gateway requesting credential verification.
// Subject: mistypass.gateway.{gateway_id}.verify
type CredentialVerifyRequest struct {
	RequestID      string `json:"request_id"`
	GatewayID      string `json:"gateway_id"`
	ReaderID       string `json:"reader_id"`
	LockID         string `json:"lock_id"`
	TenantID       string `json:"tenant_id"`
	CredentialType string `json:"credential_type"` // "nfc_uid", "ble_token", "qr_code", "pin"
	CredentialData string `json:"credential_data"` // the raw credential payload
	OccurredAt     string `json:"occurred_at"`
}

// CredentialVerifyResponse is returned to the gateway after verification.
// Subject: mistypass.gateway.{gateway_id}.verify_result
type CredentialVerifyResponse struct {
	RequestID  string `json:"request_id"`
	GatewayID  string `json:"gateway_id"`
	LockID     string `json:"lock_id"`
	Decision   string `json:"decision"` // "allow", "deny"
	Reason     string `json:"reason,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	UserEmail  string `json:"user_email,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

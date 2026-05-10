package gateway

import (
	"strings"
	"time"
)

const (
	certificateRevocationSourceRuntime     = "runtime"
	certificateRevocationSourceEnvironment = "environment"
)

type GatewayCertificateRevocation struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenant_id,omitempty"`
	GatewayID    string     `json:"gateway_id,omitempty"`
	SerialNumber string     `json:"serial_number"`
	Reason       string     `json:"reason,omitempty"`
	RevokedBy    string     `json:"revoked_by,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	Source       string     `json:"source"`
}

func (s *Service) ListCertificateRevocations(tenantID string) []GatewayCertificateRevocation {
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]GatewayCertificateRevocation, 0, len(s.certificateRevocations))
	for i := range s.certificateRevocations {
		if filterTenantID != "" && s.certificateRevocations[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneGatewayCertificateRevocation(s.certificateRevocations[i]))
	}
	return items
}

func (s *Service) RevokeCertificateSerial(tenantID, gatewayID, serialNumber, reason, revokedBy string) (GatewayCertificateRevocation, error) {
	serial, err := normalizeCertificateSerialInput(serialNumber)
	if err != nil {
		return GatewayCertificateRevocation{}, err
	}

	nextTenantID := strings.TrimSpace(tenantID)
	nextGatewayID := strings.TrimSpace(gatewayID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if nextGatewayID != "" {
		foundGateway := false
		for i := range s.gateways {
			if s.gateways[i].ID != nextGatewayID {
				continue
			}
			if nextTenantID != "" && s.gateways[i].TenantID != nextTenantID {
				return GatewayCertificateRevocation{}, ErrGatewayNotFound
			}
			if nextTenantID == "" {
				nextTenantID = s.gateways[i].TenantID
			}
			foundGateway = true
			break
		}
		if !foundGateway {
			return GatewayCertificateRevocation{}, ErrGatewayNotFound
		}
	}

	now := time.Now().UTC()
	record := GatewayCertificateRevocation{
		ID:           certificateRevocationID(serial),
		TenantID:     nextTenantID,
		GatewayID:    nextGatewayID,
		SerialNumber: serial,
		Reason:       strings.TrimSpace(reason),
		RevokedBy:    strings.TrimSpace(revokedBy),
		RevokedAt:    &now,
		Source:       certificateRevocationSourceRuntime,
	}

	for i := range s.certificateRevocations {
		if s.certificateRevocations[i].SerialNumber != serial {
			continue
		}
		if nextTenantID != "" && s.certificateRevocations[i].TenantID != "" && s.certificateRevocations[i].TenantID != nextTenantID {
			return GatewayCertificateRevocation{}, ErrGatewayCertificateSerialConflict
		}
		if s.certificateRevocations[i].TenantID == "" {
			s.certificateRevocations[i].TenantID = nextTenantID
		}
		if nextGatewayID != "" {
			s.certificateRevocations[i].GatewayID = nextGatewayID
		}
		s.certificateRevocations[i].Reason = record.Reason
		s.certificateRevocations[i].RevokedBy = record.RevokedBy
		s.certificateRevocations[i].RevokedAt = record.RevokedAt
		s.certificateRevocations[i].Source = certificateRevocationSourceRuntime
		if err := s.persistLocked(); err != nil {
			return GatewayCertificateRevocation{}, err
		}
		return cloneGatewayCertificateRevocation(s.certificateRevocations[i]), nil
	}

	s.certificateRevocations = append([]GatewayCertificateRevocation{record}, s.certificateRevocations...)
	if err := s.persistLocked(); err != nil {
		return GatewayCertificateRevocation{}, err
	}
	return cloneGatewayCertificateRevocation(record), nil
}

func (s *Service) RestoreCertificateSerial(tenantID, serialNumber string) (GatewayCertificateRevocation, error) {
	serial, err := normalizeCertificateSerialInput(serialNumber)
	if err != nil {
		return GatewayCertificateRevocation{}, err
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.certificateRevocations {
		if s.certificateRevocations[i].SerialNumber != serial {
			continue
		}
		if filterTenantID != "" && s.certificateRevocations[i].TenantID != "" && s.certificateRevocations[i].TenantID != filterTenantID {
			return GatewayCertificateRevocation{}, ErrGatewayCertificateSerialNotFound
		}
		removed := cloneGatewayCertificateRevocation(s.certificateRevocations[i])
		s.certificateRevocations = append(s.certificateRevocations[:i], s.certificateRevocations[i+1:]...)
		if err := s.persistLocked(); err != nil {
			return GatewayCertificateRevocation{}, err
		}
		return removed, nil
	}

	return GatewayCertificateRevocation{}, ErrGatewayCertificateSerialNotFound
}

func (s *Service) IsCertificateSerialRevoked(serialNumber string) bool {
	serial := NormalizeCertificateSerial(serialNumber)
	if serial == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.certificateRevocations {
		if s.certificateRevocations[i].SerialNumber == serial {
			return true
		}
	}
	return false
}

func NormalizeCertificateSerial(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, ch := range raw {
		if isCertificateSerialHex(ch) {
			b.WriteRune(ch)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	normalized := strings.TrimLeft(b.String(), "0")
	if normalized == "" {
		return "0"
	}
	return normalized
}

func normalizeCertificateSerialInput(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", ErrGatewayCertificateSerialRequired
	}
	var b strings.Builder
	for _, ch := range value {
		switch {
		case isCertificateSerialHex(ch):
			b.WriteRune(ch)
		case ch == ':' || ch == '-' || ch == ' ' || ch == '\t':
			continue
		default:
			return "", ErrGatewayCertificateSerialInvalid
		}
	}
	if b.Len() == 0 {
		return "", ErrGatewayCertificateSerialRequired
	}
	normalized := strings.TrimLeft(b.String(), "0")
	if normalized == "" {
		normalized = "0"
	}
	if len(normalized) > 128 {
		return "", ErrGatewayCertificateSerialInvalid
	}
	return normalized, nil
}

func isCertificateSerialHex(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
}

func certificateRevocationID(serial string) string {
	return "gw_cert_rev_" + serial
}

func cloneGatewayCertificateRevocations(items []GatewayCertificateRevocation) []GatewayCertificateRevocation {
	if len(items) == 0 {
		return nil
	}
	output := make([]GatewayCertificateRevocation, 0, len(items))
	for i := range items {
		output = append(output, cloneGatewayCertificateRevocation(items[i]))
	}
	return output
}

func cloneGatewayCertificateRevocation(item GatewayCertificateRevocation) GatewayCertificateRevocation {
	record := item
	if item.RevokedAt != nil {
		value := *item.RevokedAt
		record.RevokedAt = &value
	}
	if record.Source == "" {
		record.Source = certificateRevocationSourceRuntime
	}
	return record
}

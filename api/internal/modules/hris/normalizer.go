package hris

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
)

var ErrNormalizerNotFound = errors.New("hris normalizer not found")
var ErrUnsupportedWebhookEvent = errors.New("unsupported hris webhook event")
var ErrDeferredWebhookEvent = errors.New("deferred hris webhook event")
var ErrInvalidWebhookPayload = errors.New("invalid hris webhook payload")
var ErrNormalizedEmployeesRequired = errors.New("normalized employees are required")

const SyncActor = "enterprise.sync.worker"

type NormalizedSyncRequest struct {
	TenantID      string                         `json:"tenant_id"`
	Source        string                         `json:"source"`
	Actor         string                         `json:"actor"`
	RequestID     string                         `json:"request_id"`
	ConnectorID   string                         `json:"connector_id,omitempty"`
	RawPayloadRef string                         `json:"raw_payload_ref,omitempty"`
	EventType     string                         `json:"event_type,omitempty"`
	Employees     []enterprise.EmployeeSyncInput `json:"employees"`
}

type WebhookNormalizer interface {
	Vendor() string
	NormalizeWebhook(receipt enterprise.HRISWebhookReceipt) (NormalizedSyncRequest, error)
}

type Registry struct {
	mu          sync.RWMutex
	normalizers map[string]WebhookNormalizer
}

func NewRegistry(normalizers ...WebhookNormalizer) *Registry {
	registry := &Registry{
		normalizers: make(map[string]WebhookNormalizer, len(normalizers)),
	}
	for i := range normalizers {
		if normalizers[i] == nil {
			continue
		}
		registry.Register(normalizers[i])
	}
	return registry
}

func (r *Registry) Register(normalizer WebhookNormalizer) {
	if r == nil || normalizer == nil {
		return
	}
	vendor := NormalizeVendor(normalizer.Vendor())
	if vendor == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.normalizers == nil {
		r.normalizers = make(map[string]WebhookNormalizer)
	}
	r.normalizers[vendor] = normalizer
}

func (r *Registry) Get(vendor string) (WebhookNormalizer, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	normalizer, ok := r.normalizers[NormalizeVendor(vendor)]
	return normalizer, ok
}

func (r *Registry) NormalizeWebhook(receipt enterprise.HRISWebhookReceipt) (NormalizedSyncRequest, error) {
	normalizer, ok := r.Get(receipt.Vendor)
	if !ok {
		return NormalizedSyncRequest{}, ErrNormalizerNotFound
	}
	return normalizer.NormalizeWebhook(receipt)
}

func NormalizeSyncRequest(request NormalizedSyncRequest) (NormalizedSyncRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.Source = strings.TrimSpace(request.Source)
	request.Actor = strings.TrimSpace(request.Actor)
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.ConnectorID = strings.TrimSpace(request.ConnectorID)
	request.RawPayloadRef = strings.TrimSpace(request.RawPayloadRef)
	request.EventType = strings.TrimSpace(request.EventType)
	if request.Actor == "" {
		request.Actor = SyncActor
	}
	if len(request.Employees) == 0 {
		return NormalizedSyncRequest{}, ErrNormalizedEmployeesRequired
	}
	return request, nil
}

func NormalizeVendor(vendor string) string {
	return strings.ToLower(strings.TrimSpace(vendor))
}

func SyncSourceForVendor(vendor string) string {
	nextVendor := NormalizeVendor(vendor)
	if nextVendor == "" {
		return "hris"
	}
	return "hris_" + nextVendor
}

func RawPayloadRef(receipt enterprise.HRISWebhookReceipt) string {
	if strings.TrimSpace(receipt.ID) == "" {
		return ""
	}
	return "hris_webhook_receipt:" + strings.TrimSpace(receipt.ID)
}

func StableRequestID(receipt enterprise.HRISWebhookReceipt, employeeKey, effectiveAt string) string {
	if nextRequestID := strings.TrimSpace(receipt.RequestID); nextRequestID != "" {
		return nextRequestID
	}

	parts := []string{
		NormalizeVendor(receipt.Vendor),
		normalizeRequestIDPart(receipt.TenantID),
		normalizeRequestIDPart(receipt.EventType),
		normalizeRequestIDPart(employeeKey),
		normalizeRequestIDPart(effectiveAt),
	}

	nonEmpty := make([]string, 0, len(parts))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		nonEmpty = append(nonEmpty, parts[i])
	}
	if len(nonEmpty) >= 4 {
		return strings.Join(nonEmpty, ":")
	}

	sum := sha256.Sum256([]byte(strings.Join([]string{
		NormalizeVendor(receipt.Vendor),
		strings.TrimSpace(receipt.TenantID),
		strings.TrimSpace(receipt.EventType),
		strings.TrimSpace(employeeKey),
		strings.TrimSpace(effectiveAt),
		strings.TrimSpace(receipt.RawPayload),
	}, "|")))
	prefix := NormalizeVendor(receipt.Vendor)
	if prefix == "" {
		prefix = "hris"
	}
	return prefix + ":" + hex.EncodeToString(sum[:12])
}

func normalizeRequestIDPart(input string) string {
	next := strings.ToLower(strings.TrimSpace(input))
	if next == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		"\n", "_",
		"\r", "_",
		"\t", "_",
	)
	return replacer.Replace(next)
}

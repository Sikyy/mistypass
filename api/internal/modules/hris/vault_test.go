package hris

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type memoryStateStore struct {
	items              map[string][]byte
	compareAndSwapHook func(key string, expectedExists bool, expectedPayload []byte, nextPayload []byte)
}

func (s *memoryStateStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *memoryStateStore) Save(key string, value any) error {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.items[key] = payload
	return nil
}

func (s *memoryStateStore) CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error) {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}

	var expectedPayload []byte
	var err error
	if expectedExists {
		expectedPayload, err = json.Marshal(expected)
		if err != nil {
			return false, err
		}
	}

	nextPayload, err := json.Marshal(next)
	if err != nil {
		return false, err
	}

	if s.compareAndSwapHook != nil {
		hook := s.compareAndSwapHook
		s.compareAndSwapHook = nil
		hook(key, expectedExists, expectedPayload, nextPayload)
	}

	currentPayload, found := s.items[key]
	if found != expectedExists {
		return false, nil
	}
	if expectedExists {
		sameExpected, err := testJSONPayloadEqual(currentPayload, expectedPayload)
		if err != nil {
			return false, err
		}
		if !sameExpected {
			return false, nil
		}
	}
	if found {
		sameNext, err := testJSONPayloadEqual(currentPayload, nextPayload)
		if err != nil {
			return false, err
		}
		if sameNext {
			return true, nil
		}
	}
	s.items[key] = nextPayload
	return true, nil
}

func testJSONPayloadEqual(left, right []byte) (bool, error) {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false, err
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false, err
	}
	return reflect.DeepEqual(leftValue, rightValue), nil
}

func TestVaultServiceUpsertListAndResolve(t *testing.T) {
	svc := NewVaultService("vault-master-key-001")

	metadata, err := svc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/webhook_secret",
		"webhook_secret",
		"super-secret-value",
		"tenant.admin@sudirman.co",
	)
	if err != nil {
		t.Fatalf("expected upsert secret to succeed: %v", err)
	}
	if metadata.Ref != "vault://tenant_demo_jakarta/hris/talenta/webhook_secret" {
		t.Fatalf("unexpected secret ref: %s", metadata.Ref)
	}
	if metadata.Kind != "webhook_secret" {
		t.Fatalf("unexpected secret kind: %s", metadata.Kind)
	}

	items := svc.ListSecrets("tenant_demo_jakarta")
	if len(items) != 1 {
		t.Fatalf("expected one secret metadata item, got %d", len(items))
	}

	resolved, err := svc.ResolveSecretRef(metadata.Ref)
	if err != nil {
		t.Fatalf("expected resolve secret ref to succeed: %v", err)
	}
	if resolved.Value != "super-secret-value" {
		t.Fatalf("unexpected resolved secret value: %s", resolved.Value)
	}
}

func TestVaultServicePersistsEncryptedState(t *testing.T) {
	store := &memoryStateStore{}
	svc, err := NewVaultServiceWithStateStore("vault-master-key-001", store)
	if err != nil {
		t.Fatalf("expected vault service with state store to initialize: %v", err)
	}

	metadata, err := svc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"client_secret",
		"talenta-client-secret-001",
		"ops@sudirman.co",
	)
	if err != nil {
		t.Fatalf("expected upsert secret to succeed: %v", err)
	}

	rawState := string(store.items[stateKey])
	if strings.Contains(rawState, "talenta-client-secret-001") {
		t.Fatalf("expected persisted state to be encrypted, got raw state=%s", rawState)
	}

	restored, err := NewVaultServiceWithStateStore("vault-master-key-001", store)
	if err != nil {
		t.Fatalf("expected restored vault service to initialize: %v", err)
	}
	resolved, err := restored.ResolveSecretRef(metadata.Ref)
	if err != nil {
		t.Fatalf("expected resolve secret ref after restore to succeed: %v", err)
	}
	if resolved.Value != "talenta-client-secret-001" {
		t.Fatalf("unexpected restored secret value: %s", resolved.Value)
	}
}

func TestVaultServiceRejectsInvalidSecretRef(t *testing.T) {
	svc := NewVaultService("vault-master-key-001")

	if _, err := svc.ResolveSecretRef("env://HRIS_SECRET"); err != ErrInvalidSecretRef {
		t.Fatalf("expected ErrInvalidSecretRef, got %v", err)
	}
}

func TestVaultServiceGetSecretMetadataByRefDerivesTenantFromRef(t *testing.T) {
	svc := NewVaultService("vault-master-key-001")

	metadata, err := svc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/credential",
		"connector_credential",
		`{"client_id":"talenta-client-001"}`,
		"tenant.admin@sudirman.co",
	)
	if err != nil {
		t.Fatalf("expected upsert secret to succeed: %v", err)
	}

	item, err := svc.GetSecretMetadataByRef("", metadata.Ref)
	if err != nil {
		t.Fatalf("expected metadata lookup by ref without tenant to succeed: %v", err)
	}
	if item.Ref != metadata.Ref || item.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected metadata lookup result: %+v", item)
	}
}

func TestVaultServiceGetSecretMetadataByRefRejectsCrossTenantLookup(t *testing.T) {
	svc := NewVaultService("vault-master-key-001")

	metadata, err := svc.UpsertSecret(
		"tenant_demo_jakarta",
		"hris/talenta/webhook_secret",
		"webhook_secret",
		"super-secret-value",
		"tenant.admin@sudirman.co",
	)
	if err != nil {
		t.Fatalf("expected upsert secret to succeed: %v", err)
	}

	if _, err := svc.GetSecretMetadataByRef("tenant_other", metadata.Ref); err != ErrSecretNotFound {
		t.Fatalf("expected cross-tenant metadata lookup to return ErrSecretNotFound, got %v", err)
	}
}

func TestConnectorSecretName(t *testing.T) {
	name := ConnectorSecretName("Talenta", "webhook_secret")
	if name != "hris/talenta/webhook_secret" {
		t.Fatalf("unexpected connector secret name: %s", name)
	}
}

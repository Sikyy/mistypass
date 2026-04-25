package hris

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrSecretNameRequired = errors.New("secret name is required")
var ErrSecretValueRequired = errors.New("secret value is required")
var ErrSecretNotFound = errors.New("hris vault secret not found")
var ErrInvalidSecretName = errors.New("invalid hris vault secret name")
var ErrInvalidSecretKind = errors.New("invalid hris vault secret kind")
var ErrInvalidSecretRef = errors.New("invalid hris vault secret ref")

const (
	stateKey            = "module_hris"
	defaultVaultActor   = "system"
	defaultVaultKeySeed = "mistypass-dev-hris-vault"
	defaultSecretKind   = "generic"
	connectorPathPrefix = "hris"
	connectorNameSep    = "/"
)

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

type SecretMetadata struct {
	Ref       string    `json:"ref"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ResolvedSecret struct {
	Metadata SecretMetadata `json:"metadata"`
	Value    string         `json:"value"`
}

type secretRecord struct {
	Metadata        SecretMetadata `json:"metadata"`
	EncryptedValue  string         `json:"encrypted_value"`
	EncryptionNonce string         `json:"encryption_nonce"`
}

type stateSnapshot struct {
	Secrets []secretRecord `json:"secrets,omitempty"`
}

type VaultService struct {
	mu         sync.RWMutex
	secrets    []secretRecord
	stateStore StateStore
	key        []byte
}

func NewVaultService(masterKey string) *VaultService {
	return &VaultService{
		secrets: []secretRecord{},
		key:     deriveVaultKey(masterKey),
	}
}

func NewVaultServiceWithStateStore(masterKey string, store StateStore) (*VaultService, error) {
	svc := NewVaultService(masterKey)
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *VaultService) ListSecrets(tenantID string) []SecretMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]SecretMetadata, 0, len(s.secrets))
	for i := range s.secrets {
		item := s.secrets[i].Metadata
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneSecretMetadata(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].Ref > items[j].Ref
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (s *VaultService) UpsertSecret(tenantID, name, kind, value, updatedBy string) (SecretMetadata, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SecretMetadata{}, ErrTenantIDRequired
	}
	nextName, err := normalizeSecretName(name)
	if err != nil {
		return SecretMetadata{}, err
	}
	nextKind, err := normalizeSecretKind(kind)
	if err != nil {
		return SecretMetadata{}, err
	}
	nextValue := strings.TrimSpace(value)
	if nextValue == "" {
		return SecretMetadata{}, ErrSecretValueRequired
	}
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	if nextUpdatedBy == "" {
		nextUpdatedBy = defaultVaultActor
	}

	nonce, ciphertext, err := s.encrypt(nextValue)
	if err != nil {
		return SecretMetadata{}, err
	}
	now := time.Now().UTC()
	ref := secretRef(nextTenantID, nextName)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.secrets {
		if s.secrets[i].Metadata.Ref != ref {
			continue
		}
		s.secrets[i].Metadata.Kind = nextKind
		s.secrets[i].Metadata.UpdatedBy = nextUpdatedBy
		s.secrets[i].Metadata.UpdatedAt = now
		s.secrets[i].EncryptedValue = ciphertext
		s.secrets[i].EncryptionNonce = nonce
		if err := s.persistLocked(); err != nil {
			return SecretMetadata{}, err
		}
		return cloneSecretMetadata(s.secrets[i].Metadata), nil
	}

	record := secretRecord{
		Metadata: SecretMetadata{
			Ref:       ref,
			TenantID:  nextTenantID,
			Name:      nextName,
			Kind:      nextKind,
			UpdatedBy: nextUpdatedBy,
			CreatedAt: now,
			UpdatedAt: now,
		},
		EncryptedValue:  ciphertext,
		EncryptionNonce: nonce,
	}
	s.secrets = append([]secretRecord{record}, s.secrets...)
	if err := s.persistLocked(); err != nil {
		return SecretMetadata{}, err
	}
	return cloneSecretMetadata(record.Metadata), nil
}

func (s *VaultService) GetSecretMetadataByRef(tenantID, ref string) (SecretMetadata, error) {
	parsedRef, parsedTenantID, _, err := parseSecretRef(ref)
	if err != nil {
		return SecretMetadata{}, err
	}
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		nextTenantID = parsedTenantID
	}
	if parsedTenantID != nextTenantID {
		return SecretMetadata{}, ErrSecretNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.secrets {
		if s.secrets[i].Metadata.Ref != parsedRef {
			continue
		}
		return cloneSecretMetadata(s.secrets[i].Metadata), nil
	}
	return SecretMetadata{}, ErrSecretNotFound
}

func (s *VaultService) ResolveSecretRef(ref string) (ResolvedSecret, error) {
	parsedRef, _, _, err := parseSecretRef(ref)
	if err != nil {
		return ResolvedSecret{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.secrets {
		if s.secrets[i].Metadata.Ref != parsedRef {
			continue
		}
		value, err := s.decrypt(s.secrets[i].EncryptionNonce, s.secrets[i].EncryptedValue)
		if err != nil {
			return ResolvedSecret{}, err
		}
		return ResolvedSecret{
			Metadata: cloneSecretMetadata(s.secrets[i].Metadata),
			Value:    value,
		}, nil
	}
	return ResolvedSecret{}, ErrSecretNotFound
}

func ConnectorSecretName(vendor, purpose string) string {
	nextVendor := NormalizeVendor(vendor)
	nextPurpose := strings.ToLower(strings.TrimSpace(purpose))
	if nextVendor == "" || nextPurpose == "" {
		return ""
	}
	return connectorPathPrefix + connectorNameSep + nextVendor + connectorNameSep + nextPurpose
}

func secretRef(tenantID, name string) string {
	return "vault://" + strings.TrimSpace(tenantID) + "/" + strings.TrimSpace(name)
}

func parseSecretRef(ref string) (normalizedRef, tenantID, name string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(ref))
	if err != nil || parsed == nil {
		return "", "", "", ErrInvalidSecretRef
	}
	if strings.ToLower(strings.TrimSpace(parsed.Scheme)) != "vault" {
		return "", "", "", ErrInvalidSecretRef
	}
	nextTenantID := strings.TrimSpace(parsed.Host)
	if nextTenantID == "" {
		return "", "", "", ErrInvalidSecretRef
	}
	nextName, normalizeErr := normalizeSecretName(strings.TrimPrefix(parsed.Path, "/"))
	if normalizeErr != nil {
		return "", "", "", ErrInvalidSecretRef
	}
	return secretRef(nextTenantID, nextName), nextTenantID, nextName, nil
}

func (s *VaultService) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		return s.stateStore.Save(stateKey, stateSnapshot{
			Secrets: cloneSecretRecords(s.secrets),
		})
	}

	s.mu.Lock()
	s.secrets = cloneSecretRecords(snapshot.Secrets)
	s.mu.Unlock()
	return nil
}

func (s *VaultService) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Secrets: cloneSecretRecords(s.secrets),
	})
}

func (s *VaultService) encrypt(plaintext string) (nonce string, ciphertext string, err error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	rawNonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, rawNonce); err != nil {
		return "", "", err
	}
	sealed := gcm.Seal(nil, rawNonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(rawNonce), base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *VaultService) decrypt(nonce, ciphertext string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	rawNonce, err := base64.StdEncoding.DecodeString(strings.TrimSpace(nonce))
	if err != nil {
		return "", err
	}
	rawCiphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, rawNonce, rawCiphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func deriveVaultKey(input string) []byte {
	seed := strings.TrimSpace(input)
	if seed == "" {
		seed = defaultVaultKeySeed
	}
	sum := sha256.Sum256([]byte(seed))
	key := make([]byte, 32)
	copy(key, sum[:])
	return key
}

func normalizeSecretKind(kind string) (string, error) {
	next := strings.ToLower(strings.TrimSpace(kind))
	if next == "" {
		return defaultSecretKind, nil
	}
	if !isAllowedSecretPath(next, false) {
		return "", ErrInvalidSecretKind
	}
	return next, nil
}

func normalizeSecretName(name string) (string, error) {
	next := strings.Trim(strings.TrimSpace(name), "/")
	if next == "" {
		return "", ErrSecretNameRequired
	}
	if !isAllowedSecretPath(next, true) {
		return "", ErrInvalidSecretName
	}
	return next, nil
}

func isAllowedSecretPath(value string, allowSlash bool) bool {
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '-', '_', '.':
			continue
		case '/':
			if allowSlash {
				continue
			}
		}
		return false
	}
	return true
}

func cloneSecretMetadata(input SecretMetadata) SecretMetadata {
	return input
}

func cloneSecretRecord(input secretRecord) secretRecord {
	return secretRecord{
		Metadata:        cloneSecretMetadata(input.Metadata),
		EncryptedValue:  input.EncryptedValue,
		EncryptionNonce: input.EncryptionNonce,
	}
}

func cloneSecretRecords(items []secretRecord) []secretRecord {
	output := make([]secretRecord, 0, len(items))
	for i := range items {
		output = append(output, cloneSecretRecord(items[i]))
	}
	return output
}

package httpx

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
	"github.com/mistypass/cloud/api/internal/redistore"
)

type httpMemoryStateStore struct {
	items                   map[string][]byte
	compareAndSwapHook      func(key string, expectedExists bool, expectedPayload []byte, nextPayload []byte)
	afterCompareAndSwapHook func(key string, nextPayload []byte)
	compareAndSwapErr       error
	compareAndSwapCall      int
	failCASCall             int
}

type stubWorkerQueueStore struct {
	mu               sync.Mutex
	beforeDequeue    func(queueName string, batchSize int)
	enqueueErr       error
	dequeueErr       error
	ackErr           error
	requeueErr       error
	enqueueCalls     int
	dequeueCalls     int
	ackCalls         int
	requeueCalls     int
	lastQueueName    string
	lastBatchSize    int
	enqueuedIDs      []string
	itemsByQueue     map[string][]string
	indexByQueue     map[string]map[string]struct{}
	claimsByQueue    map[string]map[string]redistore.WorkerQueueClaim
	deadlinesByQueue map[string]map[string]time.Time
}

func stubWorkerQueueContainsItem(items []string, target string) bool {
	nextTarget := strings.TrimSpace(target)
	if nextTarget == "" {
		return false
	}
	for i := range items {
		if strings.TrimSpace(items[i]) == nextTarget {
			return true
		}
	}
	return false
}

func stubWorkerQueueRemoveAllItems(items []string, target string) []string {
	nextTarget := strings.TrimSpace(target)
	if nextTarget == "" || len(items) == 0 {
		return append([]string(nil), items...)
	}
	nextItems := make([]string, 0, len(items))
	for i := range items {
		nextItem := strings.TrimSpace(items[i])
		if nextItem == "" || nextItem == nextTarget {
			continue
		}
		nextItems = append(nextItems, nextItem)
	}
	return nextItems
}

func stubWorkerQueueCompactItems(
	items []string,
	claims map[string]redistore.WorkerQueueClaim,
) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	nextItems := make([]string, 0, len(items))
	for i := range items {
		nextItem := strings.TrimSpace(items[i])
		if nextItem == "" {
			continue
		}
		if claims != nil {
			if _, claimed := claims[nextItem]; claimed {
				continue
			}
		}
		if _, exists := seen[nextItem]; exists {
			continue
		}
		seen[nextItem] = struct{}{}
		nextItems = append(nextItems, nextItem)
	}
	return nextItems
}

func (s *stubWorkerQueueStore) EnqueueWorkerQueue(queueName, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enqueueCalls++
	s.lastQueueName = queueName
	if s.enqueueErr != nil {
		return s.enqueueErr
	}
	if s.itemsByQueue == nil {
		s.itemsByQueue = make(map[string][]string)
	}
	if s.indexByQueue == nil {
		s.indexByQueue = make(map[string]map[string]struct{})
	}
	if s.indexByQueue[queueName] == nil {
		s.indexByQueue[queueName] = make(map[string]struct{})
	}
	if s.claimsByQueue != nil && s.claimsByQueue[queueName] != nil {
		if _, exists := s.claimsByQueue[queueName][itemID]; exists {
			s.indexByQueue[queueName][itemID] = struct{}{}
			s.itemsByQueue[queueName] = stubWorkerQueueRemoveAllItems(s.itemsByQueue[queueName], itemID)
			return nil
		}
	}
	s.itemsByQueue[queueName] = stubWorkerQueueCompactItems(s.itemsByQueue[queueName], s.claimsByQueue[queueName])
	if stubWorkerQueueContainsItem(s.itemsByQueue[queueName], itemID) {
		s.indexByQueue[queueName][itemID] = struct{}{}
		return nil
	}
	s.indexByQueue[queueName][itemID] = struct{}{}
	s.itemsByQueue[queueName] = append(s.itemsByQueue[queueName], itemID)
	s.enqueuedIDs = append(s.enqueuedIDs, itemID)
	return nil
}

func (s *stubWorkerQueueStore) ClaimWorkerQueueBatch(
	queueName string,
	batchSize int,
	visibilityTimeout time.Duration,
) ([]redistore.WorkerQueueClaim, error) {
	if s.beforeDequeue != nil {
		s.beforeDequeue(queueName, batchSize)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.dequeueCalls++
	s.lastQueueName = queueName
	s.lastBatchSize = batchSize
	if s.dequeueErr != nil {
		return nil, s.dequeueErr
	}
	if s.itemsByQueue == nil {
		s.itemsByQueue = make(map[string][]string)
	}
	if s.claimsByQueue == nil {
		s.claimsByQueue = make(map[string]map[string]redistore.WorkerQueueClaim)
	}
	if s.claimsByQueue[queueName] == nil {
		s.claimsByQueue[queueName] = make(map[string]redistore.WorkerQueueClaim)
	}
	if s.deadlinesByQueue == nil {
		s.deadlinesByQueue = make(map[string]map[string]time.Time)
	}
	if s.deadlinesByQueue[queueName] == nil {
		s.deadlinesByQueue[queueName] = make(map[string]time.Time)
	}
	now := time.Now().UTC()
	for itemID := range s.claimsByQueue[queueName] {
		deadline := s.deadlinesByQueue[queueName][itemID]
		if !deadline.IsZero() && deadline.After(now) {
			continue
		}
		delete(s.claimsByQueue[queueName], itemID)
		delete(s.deadlinesByQueue[queueName], itemID)
		if s.indexByQueue == nil {
			s.indexByQueue = make(map[string]map[string]struct{})
		}
		if s.indexByQueue[queueName] == nil {
			s.indexByQueue[queueName] = make(map[string]struct{})
		}
		s.indexByQueue[queueName][itemID] = struct{}{}
		if !stubWorkerQueueContainsItem(s.itemsByQueue[queueName], itemID) {
			s.itemsByQueue[queueName] = append(s.itemsByQueue[queueName], itemID)
		}
	}
	s.itemsByQueue[queueName] = stubWorkerQueueCompactItems(s.itemsByQueue[queueName], s.claimsByQueue[queueName])
	items := append([]string(nil), s.itemsByQueue[queueName]...)
	if len(items) == 0 {
		return nil, nil
	}
	if batchSize > 0 && len(items) > batchSize {
		items = items[:batchSize]
	}
	s.itemsByQueue[queueName] = append([]string(nil), s.itemsByQueue[queueName][len(items):]...)
	claims := make([]redistore.WorkerQueueClaim, 0, len(items))
	for i := range items {
		deadlineAt := now.Add(visibilityTimeout).UTC()
		deadline := deadlineAt.UnixMilli()
		claim := redistore.WorkerQueueClaim{
			ItemID:     items[i],
			ClaimToken: strconv.FormatInt(deadline, 10),
		}
		s.claimsByQueue[queueName][items[i]] = claim
		s.deadlinesByQueue[queueName][items[i]] = deadlineAt
		claims = append(claims, claim)
	}
	return claims, nil
}

func (s *stubWorkerQueueStore) AckWorkerQueue(queueName, itemID, claimToken string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ackCalls++
	s.lastQueueName = queueName
	if s.ackErr != nil {
		return false, s.ackErr
	}
	claim, ok := s.claimsByQueue[queueName][itemID]
	if !ok || claim.ClaimToken != claimToken {
		return false, nil
	}
	delete(s.claimsByQueue[queueName], itemID)
	delete(s.deadlinesByQueue[queueName], itemID)
	s.itemsByQueue[queueName] = stubWorkerQueueRemoveAllItems(s.itemsByQueue[queueName], itemID)
	if s.indexByQueue != nil {
		delete(s.indexByQueue[queueName], itemID)
	}
	return true, nil
}

func (s *stubWorkerQueueStore) RequeueWorkerQueue(queueName, itemID, claimToken string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requeueCalls++
	s.lastQueueName = queueName
	if s.requeueErr != nil {
		return false, s.requeueErr
	}
	claim, ok := s.claimsByQueue[queueName][itemID]
	if !ok || claim.ClaimToken != claimToken {
		return false, nil
	}
	delete(s.claimsByQueue[queueName], itemID)
	delete(s.deadlinesByQueue[queueName], itemID)
	if s.indexByQueue == nil {
		s.indexByQueue = make(map[string]map[string]struct{})
	}
	if s.indexByQueue[queueName] == nil {
		s.indexByQueue[queueName] = make(map[string]struct{})
	}
	s.indexByQueue[queueName][itemID] = struct{}{}
	s.itemsByQueue[queueName] = stubWorkerQueueRemoveAllItems(s.itemsByQueue[queueName], itemID)
	s.itemsByQueue[queueName] = append(s.itemsByQueue[queueName], itemID)
	return true, nil
}

func (s *stubWorkerQueueStore) DescribeWorkerQueue(
	queueName string,
	itemIDs []string,
) (redistore.WorkerQueueTelemetry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	telemetry := redistore.WorkerQueueTelemetry{
		PendingCount: len(stubWorkerQueueCompactItems(s.itemsByQueue[queueName], s.claimsByQueue[queueName])),
		ClaimedCount: len(s.claimsByQueue[queueName]),
		Items:        make(map[string]redistore.WorkerQueueItemState, len(itemIDs)),
	}
	pendingItems := stubWorkerQueueCompactItems(s.itemsByQueue[queueName], s.claimsByQueue[queueName])
	for i := range itemIDs {
		itemID := strings.TrimSpace(itemIDs[i])
		if itemID == "" {
			continue
		}
		if deadlineAt, ok := s.deadlinesByQueue[queueName][itemID]; ok {
			nextDeadlineAt := deadlineAt
			telemetry.Items[itemID] = redistore.WorkerQueueItemState{
				State:                redistore.WorkerQueueStateClaimed,
				VisibilityDeadlineAt: &nextDeadlineAt,
			}
			continue
		}
		if stubWorkerQueueContainsItem(pendingItems, itemID) {
			telemetry.Items[itemID] = redistore.WorkerQueueItemState{
				State: redistore.WorkerQueueStateQueued,
			}
			continue
		}
		telemetry.Items[itemID] = redistore.WorkerQueueItemState{
			State: redistore.WorkerQueueStateMissing,
		}
	}
	return telemetry, nil
}

func (s *httpMemoryStateStore) Load(key string, dst any) (bool, error) {
	payload, ok := s.items[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return false, err
	}
	return true, nil
}

func (s *httpMemoryStateStore) Save(key string, value any) error {
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

func (s *httpMemoryStateStore) CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error) {
	if s.items == nil {
		s.items = make(map[string][]byte)
	}
	s.compareAndSwapCall++

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
	if s.failCASCall > 0 && s.compareAndSwapCall == s.failCASCall && s.compareAndSwapErr != nil {
		return false, s.compareAndSwapErr
	}

	currentPayload, found := s.items[key]
	if found != expectedExists {
		return false, nil
	}
	if expectedExists {
		sameExpected, err := testHTTPJSONPayloadEqual(currentPayload, expectedPayload)
		if err != nil {
			return false, err
		}
		if !sameExpected {
			return false, nil
		}
	}
	if found {
		sameNext, err := testHTTPJSONPayloadEqual(currentPayload, nextPayload)
		if err != nil {
			return false, err
		}
		if sameNext {
			return true, nil
		}
	}
	s.items[key] = nextPayload
	if s.afterCompareAndSwapHook != nil {
		hook := s.afterCompareAndSwapHook
		s.afterCompareAndSwapHook = nil
		hook(key, nextPayload)
	}
	return true, nil
}

func testHTTPJSONPayloadEqual(left, right []byte) (bool, error) {
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

func waitForEnterpriseHRISWebhookReceiptStatus(
	t *testing.T,
	s *server,
	tenantID string,
	receiptID string,
	expectedStatus string,
) enterprise.HRISWebhookReceipt {
	t.Helper()

	var last enterprise.HRISWebhookReceipt
	for attempt := 0; attempt < 50; attempt++ {
		item, err := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, receiptID)
		if err == nil {
			last = item
			if strings.TrimSpace(item.Status) == expectedStatus {
				return item
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected receipt %s status %s, got %+v", receiptID, expectedStatus, last)
	return enterprise.HRISWebhookReceipt{}
}

func TestReceiveEnterpriseHRISWebhook(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
		hrisVaultSvc:  hris.NewVaultService("vault-master-key-001"),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"talenta.employee.detail.created","employee":{"id":"EMP-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID   string `json:"receipt_id"`
		ConnectorID string `json:"connector_id"`
		Vendor      string `json:"vendor"`
		EventType   string `json:"event_type"`
		RequestID   string `json:"request_id"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}
	if response.ReceiptID == "" {
		t.Fatalf("expected non-empty receipt_id")
	}
	if response.ConnectorID != connector.ID {
		t.Fatalf("connector_id mismatch: %s", response.ConnectorID)
	}
	if response.Vendor != "talenta" {
		t.Fatalf("vendor mismatch: %s", response.Vendor)
	}
	if response.RequestID != "mekari-evt-001" {
		t.Fatalf("request_id mismatch: %s", response.RequestID)
	}

	receipts := s.enterpriseSvc.ListHRISWebhookReceipts("tenant_demo_jakarta", connector.ID, 10)
	if len(receipts) != 1 {
		t.Fatalf("expected one stored receipt, got %d", len(receipts))
	}
	if receipts[0].SourceIP != "203.0.113.10" {
		t.Fatalf("source_ip mismatch: %s", receipts[0].SourceIP)
	}
	if receipts[0].RawPayload != body {
		t.Fatalf("raw_payload mismatch: %s", receipts[0].RawPayload)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_received", "enterprise_webhook", 10)
	if len(logs) == 0 {
		t.Fatalf("expected webhook received audit log")
	}
}

func TestReceiveEnterpriseHRISWebhookQueuesWakeSignalWhenWorkerEnabled(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:                enterprise.NewService(),
		auditSvc:                     audit.NewService(),
		hrisVaultSvc:                 hris.NewVaultService("vault-master-key-001"),
		hrisWebhookReceiptWorkerWake: make(chan struct{}, 1),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"talenta.employee.detail.created","employee":{"id":"EMP-QUEUE-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-queue-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	select {
	case <-s.hrisWebhookReceiptWorkerWake:
	default:
		t.Fatalf("expected webhook receipt queue path to notify worker wake signal")
	}
}

func TestListEnterpriseHRISWebhookReceiptsIncludesQueueRuntimeFields(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc: enterprise.NewService(),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	received, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.created",
			RequestID:  "receipt-ready",
			RawPayload: `{"employee_id":"READY-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create ready receipt should succeed: %v", err)
	}

	failed, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-cooldown",
			RawPayload: `{"employee_id":"FAIL-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create failed receipt should succeed: %v", err)
	}
	failedClaimedAt := time.Now().UTC().Add(-2 * time.Minute)
	failedClaimed, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		failed.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		failedClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim failed receipt should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim skip reason for failed receipt, got %q", reason)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", failed.ID, errors.New("forced receipt failure")); err != nil {
		t.Fatalf("mark failed receipt should succeed: %v", err)
	}

	processing, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-processing",
			RawPayload: `{"employee_id":"PROCESSING-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create processing receipt should succeed: %v", err)
	}
	processingClaimedAt := time.Now().UTC().Add(-3 * time.Minute)
	processingClaimed, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		processing.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		processingClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim processing receipt should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim skip reason for processing receipt, got %q", reason)
	}

	staleProcessing, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-stale-processing",
			RawPayload: `{"employee_id":"STALE-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create stale processing receipt should succeed: %v", err)
	}
	staleProcessingClaimedAt := time.Now().UTC().Add(-20 * time.Minute)
	staleProcessingClaimed, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		staleProcessing.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		staleProcessingClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim stale processing receipt should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim skip reason for stale processing receipt, got %q", reason)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-receipts?tenant_id=tenant_demo_jakarta&connector_id="+connector.ID,
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookReceipts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []struct {
			ID                   string     `json:"id"`
			RequestID            string     `json:"request_id"`
			Status               string     `json:"status"`
			QueueState           string     `json:"queue_state"`
			AttemptCount         int        `json:"attempt_count"`
			RemainingAttempts    int        `json:"remaining_attempts"`
			CooldownRemainingSec int64      `json:"cooldown_remaining_seconds"`
			StaleInFlight        bool       `json:"stale_in_flight"`
			NextRetryAt          *time.Time `json:"next_retry_at,omitempty"`
			ProcessingDeadlineAt *time.Time `json:"processing_deadline_at,omitempty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid receipt list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 4 {
		t.Fatalf("expected four receipt items, got %d", len(payload.Items))
	}

	itemsByRequestID := map[string]struct {
		ID                   string
		Status               string
		QueueState           string
		AttemptCount         int
		RemainingAttempts    int
		CooldownRemainingSec int64
		StaleInFlight        bool
		NextRetryAt          *time.Time
		ProcessingDeadlineAt *time.Time
	}{}
	for i := range payload.Items {
		itemsByRequestID[payload.Items[i].RequestID] = struct {
			ID                   string
			Status               string
			QueueState           string
			AttemptCount         int
			RemainingAttempts    int
			CooldownRemainingSec int64
			StaleInFlight        bool
			NextRetryAt          *time.Time
			ProcessingDeadlineAt *time.Time
		}{
			ID:                   payload.Items[i].ID,
			Status:               payload.Items[i].Status,
			QueueState:           payload.Items[i].QueueState,
			AttemptCount:         payload.Items[i].AttemptCount,
			RemainingAttempts:    payload.Items[i].RemainingAttempts,
			CooldownRemainingSec: payload.Items[i].CooldownRemainingSec,
			StaleInFlight:        payload.Items[i].StaleInFlight,
			NextRetryAt:          payload.Items[i].NextRetryAt,
			ProcessingDeadlineAt: payload.Items[i].ProcessingDeadlineAt,
		}
	}

	readyItem, ok := itemsByRequestID[received.RequestID]
	if !ok {
		t.Fatalf("expected ready receipt to be listed")
	}
	if readyItem.QueueState != "ready" ||
		readyItem.RemainingAttempts != 3 ||
		readyItem.CooldownRemainingSec != 0 ||
		readyItem.StaleInFlight ||
		readyItem.NextRetryAt != nil ||
		readyItem.ProcessingDeadlineAt != nil {
		t.Fatalf("unexpected ready receipt runtime fields: %+v", readyItem)
	}

	cooldownItem, ok := itemsByRequestID[failed.RequestID]
	if !ok {
		t.Fatalf("expected cooldown receipt to be listed")
	}
	expectedNextRetryAt := failedClaimed.LastAttemptAt.Add(5 * time.Minute)
	if cooldownItem.Status != "failed" ||
		cooldownItem.QueueState != enterprise.HRISWebhookReceiptClaimReasonCooldown ||
		cooldownItem.AttemptCount != 1 ||
		cooldownItem.RemainingAttempts != 2 ||
		cooldownItem.CooldownRemainingSec != 180 ||
		cooldownItem.StaleInFlight ||
		cooldownItem.NextRetryAt == nil ||
		!cooldownItem.NextRetryAt.Equal(expectedNextRetryAt) ||
		cooldownItem.ProcessingDeadlineAt != nil {
		t.Fatalf("unexpected cooldown receipt runtime fields: %+v expected_next_retry_at=%s", cooldownItem, expectedNextRetryAt.Format(time.RFC3339))
	}

	processingItem, ok := itemsByRequestID[processing.RequestID]
	if !ok {
		t.Fatalf("expected processing receipt to be listed")
	}
	expectedProcessingDeadline := processingClaimed.LastAttemptAt.Add(10 * time.Minute)
	if processingItem.Status != "processing" ||
		processingItem.QueueState != enterprise.HRISWebhookReceiptClaimReasonInFlight ||
		processingItem.AttemptCount != 1 ||
		processingItem.RemainingAttempts != 2 ||
		processingItem.CooldownRemainingSec != 0 ||
		processingItem.StaleInFlight ||
		processingItem.NextRetryAt != nil ||
		processingItem.ProcessingDeadlineAt == nil ||
		!processingItem.ProcessingDeadlineAt.Equal(expectedProcessingDeadline) {
		t.Fatalf("unexpected processing receipt runtime fields: %+v expected_deadline=%s", processingItem, expectedProcessingDeadline.Format(time.RFC3339))
	}

	staleProcessingItem, ok := itemsByRequestID[staleProcessing.RequestID]
	if !ok {
		t.Fatalf("expected stale processing receipt to be listed")
	}
	expectedStaleDeadline := staleProcessingClaimed.LastAttemptAt.Add(10 * time.Minute)
	if staleProcessingItem.Status != "processing" ||
		staleProcessingItem.QueueState != "ready" ||
		staleProcessingItem.AttemptCount != 1 ||
		staleProcessingItem.RemainingAttempts != 2 ||
		staleProcessingItem.CooldownRemainingSec != 0 ||
		!staleProcessingItem.StaleInFlight ||
		staleProcessingItem.NextRetryAt != nil ||
		staleProcessingItem.ProcessingDeadlineAt == nil ||
		!staleProcessingItem.ProcessingDeadlineAt.Equal(expectedStaleDeadline) {
		t.Fatalf("unexpected stale processing receipt runtime fields: %+v expected_deadline=%s", staleProcessingItem, expectedStaleDeadline.Format(time.RFC3339))
	}
}

func TestListEnterpriseHRISWebhookReceiptsRefreshesSharedState(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}

	connector, err := firstEnterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	receipt, err := firstEnterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.updated",
			RequestID:  "talenta-receipt-refresh-001",
			RawPayload: `{"event_type":"talenta.employee.detail.updated"}`,
		},
	)
	if err != nil {
		t.Fatalf("create receipt should succeed: %v", err)
	}
	if len(secondEnterpriseSvc.ListAllHRISWebhookReceipts("tenant_demo_jakarta", connector.ID)) != 0 {
		t.Fatalf("expected second service receipt view to be stale before list refresh")
	}

	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc: secondEnterpriseSvc,
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-receipts?tenant_id=tenant_demo_jakarta&connector_id="+connector.ID,
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookReceipts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from receipt list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookReceiptListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid receipt list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != receipt.ID {
		t.Fatalf("expected refreshed receipt list to include latest receipt, got %+v", payload.Items)
	}
}

func TestListEnterpriseHRISWebhookReceiptsSupportsQueueFilterPaginationAndCounts(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc: enterprise.NewService(),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	readyReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.created",
			RequestID:  "receipt-ready",
			RawPayload: `{"employee_id":"READY-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create ready receipt should succeed: %v", err)
	}

	cooldownReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-cooldown",
			RawPayload: `{"employee_id":"FAIL-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create cooldown receipt should succeed: %v", err)
	}
	cooldownClaimedAt := time.Now().UTC().Add(-2 * time.Minute)
	if _, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		cooldownReceipt.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		cooldownClaimedAt,
	); err != nil {
		t.Fatalf("claim cooldown receipt should succeed: %v", err)
	} else if reason != "" {
		t.Fatalf("expected empty claim reason for cooldown receipt, got %q", reason)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", cooldownReceipt.ID, errors.New("forced receipt failure")); err != nil {
		t.Fatalf("mark cooldown receipt failed should succeed: %v", err)
	}

	inFlightReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-processing",
			RawPayload: `{"employee_id":"PROCESSING-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create in-flight receipt should succeed: %v", err)
	}
	inFlightClaimedAt := time.Now().UTC().Add(-3 * time.Minute)
	if _, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		inFlightReceipt.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		inFlightClaimedAt,
	); err != nil {
		t.Fatalf("claim in-flight receipt should succeed: %v", err)
	} else if reason != "" {
		t.Fatalf("expected empty claim reason for in-flight receipt, got %q", reason)
	}

	staleReadyReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-stale-ready",
			RawPayload: `{"employee_id":"STALE-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create stale ready receipt should succeed: %v", err)
	}
	staleClaimedAt := time.Now().UTC().Add(-20 * time.Minute)
	if _, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		staleReadyReceipt.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		staleClaimedAt,
	); err != nil {
		t.Fatalf("claim stale ready receipt should succeed: %v", err)
	} else if reason != "" {
		t.Fatalf("expected empty claim reason for stale ready receipt, got %q", reason)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-receipts?tenant_id=tenant_demo_jakarta&connector_id="+connector.ID+"&queue_state=ready&limit=1",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookReceipts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Items []struct {
			RequestID string `json:"request_id"`
		} `json:"items"`
		Total       int  `json:"total"`
		Offset      int  `json:"offset"`
		Limit       int  `json:"limit"`
		NextOffset  int  `json:"next_offset,omitempty"`
		HasMore     bool `json:"has_more"`
		QueueCounts struct {
			All          int `json:"all"`
			Ready        int `json:"ready"`
			Cooldown     int `json:"cooldown"`
			InFlight     int `json:"in_flight"`
			AttemptLimit int `json:"attempt_limit"`
			Terminal     int `json:"terminal"`
		} `json:"queue_counts"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid paged receipt payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 || payload.Items[0].RequestID != staleReadyReceipt.RequestID {
		t.Fatalf("expected first ready page to contain stale ready receipt, got %+v", payload.Items)
	}
	if payload.Total != 2 || payload.Offset != 0 || payload.Limit != 1 || !payload.HasMore || payload.NextOffset != 1 {
		t.Fatalf("unexpected pagination payload: %+v", payload)
	}
	if payload.QueueCounts.All != 4 ||
		payload.QueueCounts.Ready != 2 ||
		payload.QueueCounts.Cooldown != 1 ||
		payload.QueueCounts.InFlight != 1 ||
		payload.QueueCounts.AttemptLimit != 0 ||
		payload.QueueCounts.Terminal != 0 {
		t.Fatalf("unexpected queue counts: %+v ready=%s", payload.QueueCounts, readyReceipt.RequestID)
	}
}

func TestProcessBatchEnterpriseHRISWebhookReceiptsFlow(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	readyReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-batch-ready",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create ready receipt should succeed: %v", err)
	}

	cooldownReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-batch-cooldown",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create cooldown receipt should succeed: %v", err)
	}
	cooldownClaimedAt := time.Now().UTC().Add(-2 * time.Minute)
	cooldownClaimed, reason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		"tenant_demo_jakarta",
		cooldownReceipt.ID,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		cooldownClaimedAt,
	)
	if err != nil {
		t.Fatalf("claim cooldown receipt should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected empty claim reason before cooldown failure, got %q", reason)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", cooldownReceipt.ID, errors.New("forced cooldown failure")); err != nil {
		t.Fatalf("mark cooldown receipt failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":   "tenant_demo_jakarta",
		"receipt_ids": []string{readyReceipt.ID, cooldownReceipt.ID},
	})
	if err != nil {
		t.Fatalf("marshal process-batch request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/process-batch",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processBatchEnterpriseHRISWebhookReceipts(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from receipt process batch, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		TenantID      string `json:"tenant_id"`
		TotalReceipts int    `json:"total_receipts"`
		Processed     int    `json:"processed"`
		Skipped       int    `json:"skipped"`
		Failed        int    `json:"failed"`
		DLQ           int    `json:"dlq"`
		Items         []struct {
			ReceiptID string                         `json:"receipt_id"`
			Status    string                         `json:"status"`
			Reason    string                         `json:"reason"`
			Item      *enterprise.HRISWebhookReceipt `json:"item"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid receipt process batch payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.TenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected tenant_id in receipt process batch payload: %+v", payload)
	}
	if payload.TotalReceipts != 2 || payload.Processed != 1 || payload.Skipped != 1 || payload.Failed != 0 || payload.DLQ != 0 {
		t.Fatalf("unexpected receipt process batch summary: %+v", payload)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two receipt process batch items, got %d", len(payload.Items))
	}

	itemsByReceiptID := make(map[string]struct {
		Status string
		Reason string
		Item   *enterprise.HRISWebhookReceipt
	})
	for i := range payload.Items {
		itemsByReceiptID[payload.Items[i].ReceiptID] = struct {
			Status string
			Reason string
			Item   *enterprise.HRISWebhookReceipt
		}{
			Status: payload.Items[i].Status,
			Reason: payload.Items[i].Reason,
			Item:   payload.Items[i].Item,
		}
	}

	processedItem, ok := itemsByReceiptID[readyReceipt.ID]
	if !ok || processedItem.Status != "processed" || processedItem.Item == nil || processedItem.Item.Status != "processed" {
		t.Fatalf("expected processed receipt result for %s, got %+v", readyReceipt.ID, processedItem)
	}
	skippedItem, ok := itemsByReceiptID[cooldownReceipt.ID]
	if !ok || skippedItem.Status != "skipped" || skippedItem.Reason != enterprise.HRISWebhookReceiptClaimReasonCooldown {
		t.Fatalf("expected cooldown skipped receipt result for %s, got %+v", cooldownReceipt.ID, skippedItem)
	}

	updatedReadyReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", readyReceipt.ID)
	if err != nil {
		t.Fatalf("lookup processed ready receipt should succeed: %v", err)
	}
	if updatedReadyReceipt.Status != "processed" {
		t.Fatalf("expected processed ready receipt status, got %+v", updatedReadyReceipt)
	}
	expectedNextRetryAt := cooldownClaimed.LastAttemptAt.Add(5 * time.Minute)
	updatedCooldownReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", cooldownReceipt.ID)
	if err != nil {
		t.Fatalf("lookup cooldown receipt should succeed: %v", err)
	}
	if updatedCooldownReceipt.Status != "failed" ||
		updatedCooldownReceipt.LastAttemptAt == nil ||
		!updatedCooldownReceipt.LastAttemptAt.Equal(*cooldownClaimed.LastAttemptAt) ||
		expectedNextRetryAt.Before(time.Now().UTC()) {
		t.Fatalf("expected cooldown receipt to remain failed and in cooldown, got %+v", updatedCooldownReceipt)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	foundEmployee := false
	for i := range employees {
		if employees[i].Email == "fail.ntimes@replay-sync.local" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected processed receipt employee to be synced")
	}
}

func TestProcessBatchEnterpriseHRISWebhookReceiptsQueuedFlow(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	firstReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-batch-queued-1",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create first queued receipt should succeed: %v", err)
	}
	secondReceipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "gadjian.employee.updated",
			RequestID:  "receipt-batch-queued-2",
			RawPayload: `{"employee_id":"GADJIAN-EMP-FAIL-N-001"}`,
		},
	)
	if err != nil {
		t.Fatalf("create second queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"receipt_ids":    []string{firstReceipt.ID, secondReceipt.ID},
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued process-batch request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/process-batch",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processBatchEnterpriseHRISWebhookReceipts(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued receipt process batch, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		ExecutionMode string `json:"execution_mode"`
		DispatchMode  string `json:"dispatch_mode"`
		Queued        int    `json:"queued"`
		Processed     int    `json:"processed"`
		Items         []struct {
			ReceiptID string                         `json:"receipt_id"`
			Status    string                         `json:"status"`
			Item      *enterprise.HRISWebhookReceipt `json:"item"`
		} `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid queued receipt batch payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.ExecutionMode != "queued" ||
		payload.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback ||
		payload.Queued != 2 ||
		payload.Processed != 0 {
		t.Fatalf("unexpected queued receipt batch summary: %+v", payload)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two queued receipt batch items, got %d", len(payload.Items))
	}
	for i := range payload.Items {
		if payload.Items[i].Status != "queued" || payload.Items[i].Item == nil || payload.Items[i].Item.Status != "processing" {
			t.Fatalf("expected queued receipt item, got %+v", payload.Items[i])
		}
	}

	waitForEnterpriseHRISWebhookReceiptStatus(t, s, "tenant_demo_jakarta", firstReceipt.ID, "processed")
	waitForEnterpriseHRISWebhookReceiptStatus(t, s, "tenant_demo_jakarta", secondReceipt.ID, "processed")

	queuedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_processing_queued", "enterprise_sync", 10)
	if len(queuedLogs) != 2 {
		t.Fatalf("expected two queued receipt audit logs, got %d", len(queuedLogs))
	}
}

func TestProcessEnterpriseHRISWebhookReceiptEntryFlow(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-single-ready",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-SINGLE-001",
						"employee_number":"TAL-SINGLE-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Single",
						"last_name":"Process",
						"email":"single.process@replay-sync.local",
						"mobile_phone":"+628110000444"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create ready receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id": "tenant_demo_jakarta",
	})
	if err != nil {
		t.Fatalf("marshal single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from single receipt process, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Item enterprise.HRISWebhookReceipt `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid single receipt process payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Item.ID != receipt.ID || payload.Item.Status != "processed" {
		t.Fatalf("unexpected single receipt process payload: %+v", payload)
	}

	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup processed receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %+v", updatedReceipt)
	}
}

func TestProcessEnterpriseHRISWebhookReceiptEntryQueuedFlow(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-single-queued",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-SINGLE-QUEUED-001",
						"employee_number":"TAL-SINGLE-QUEUED-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Single",
						"last_name":"Queued",
						"email":"single.queued@replay-sync.local",
						"mobile_phone":"+628110000445"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued single receipt process, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		ExecutionMode string                        `json:"execution_mode"`
		DispatchMode  string                        `json:"dispatch_mode"`
		ExecutionID   string                        `json:"execution_id"`
		Item          enterprise.HRISWebhookReceipt `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid queued single receipt payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.ExecutionMode != "queued" ||
		payload.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback ||
		payload.Item.ID != receipt.ID ||
		payload.Item.Status != "processing" {
		t.Fatalf("unexpected queued single receipt payload: %+v", payload)
	}
	if payload.ExecutionID == "" {
		t.Fatalf("expected queued single receipt payload to include execution_id")
	}

	updatedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(t, s, "tenant_demo_jakarta", receipt.ID, "processed")
	if updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected queued receipt to be processed asynchronously, got %+v", updatedReceipt)
	}
	execution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", payload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup queued receipt execution should succeed: %v", err)
	}
	if execution.Kind != enterprise.HRISWebhookExecutionKindReceiptProcess ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "processed" ||
		execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback {
		t.Fatalf("unexpected queued receipt execution record: %+v", execution)
	}
	if execution.StartedAt == nil || execution.FinishedAt == nil {
		t.Fatalf("expected queued receipt execution timestamps to be set: %+v", execution)
	}

	queuedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_processing_queued", "enterprise_sync", 10)
	if len(queuedLogs) != 1 {
		t.Fatalf("expected one queued receipt audit log, got %d", len(queuedLogs))
	}
}

func TestProcessEnterpriseHRISWebhookReceiptEntryQueuedFlowRejectsRequireWorkerWithoutWorker(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.created",
			RequestID:  "receipt-single-queued-require-worker",
			RawPayload: `{"event_type":"talenta.employee.detail.created"}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from queued single receipt process without worker, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), errEnterpriseHRISWebhookQueuedReceiptWorkerRequired.Error()) {
		t.Fatalf("expected require_worker conflict message, got body=%s", recorder.Body.String())
	}

	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "received" || updatedReceipt.AttemptCount != 0 {
		t.Fatalf("expected receipt to remain unclaimed after require_worker conflict, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected no execution record after require_worker conflict")
	}
}

func TestProcessEnterpriseHRISWebhookReceiptEntryQueuedFlowRestoresClaimWhenExecutionCreateFails(t *testing.T) {
	store := &httpMemoryStateStore{}
	enterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create enterprise service with state store should succeed: %v", err)
	}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "queue-restore.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-queued-create-execution-fail",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-QUEUED-FAIL-001"
					},
					"personal":{
						"first_name":"Queue",
						"last_name":"Restore",
						"email":"queue.restore@queue-restore.local"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	store.failCASCall = store.compareAndSwapCall + 2
	store.compareAndSwapErr = errors.New("forced execution record create failure")

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from queued single receipt process when execution create fails, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup restored receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "received" || updatedReceipt.AttemptCount != 0 || updatedReceipt.LastAttemptAt != nil || updatedReceipt.ProcessedAt != nil {
		t.Fatalf("expected receipt claim to be restored after queued dispatch failure, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected no execution record to persist after queued dispatch failure")
	}
	queuedLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_processing_queued", "enterprise_sync", 10)
	if len(queuedLogs) != 0 {
		t.Fatalf("expected no queued receipt audit log after queued dispatch failure, got %d", len(queuedLogs))
	}
}

func TestProcessEnterpriseHRISWebhookReceiptEntryQueuedFlowPersistsForWorkerTickWhenEnabled(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:                enterprise.NewService(),
		accessSvc:                    access.NewService(),
		auditSvc:                     audit.NewService(),
		hrisNormalizerRegistry:       hris.NewRegistry(talenta.NewNormalizer()),
		hrisWebhookReceiptWorkerWake: make(chan struct{}, 1),
		workerQueueStore:             queueStore,
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-single-queued-worker-dispatch",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-SINGLE-QUEUED-WORKER-001",
						"employee_number":"TAL-SINGLE-QUEUED-WORKER-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Single",
						"last_name":"QueuedWorker",
						"email":"single.queued.worker@replay-sync.local",
						"mobile_phone":"+628110000446"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued single receipt process, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	select {
	case <-s.hrisWebhookReceiptWorkerWake:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected queued receipt to notify worker wake")
	}

	var payload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid queued single receipt payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.ExecutionID == "" {
		t.Fatalf("expected queued single receipt payload to include execution_id")
	}
	execution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", payload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup queued receipt execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusQueued ||
		execution.TargetStatus != "processing" {
		t.Fatalf("unexpected queued worker execution record: %+v", execution)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected queued receipt dispatch to enqueue once, got %d", queueStore.enqueueCalls)
	}
	if queueStore.lastQueueName != enterpriseHRISWebhookReceiptExecutionQueue {
		t.Fatalf("expected queued receipt execution queue %s, got %s", enterpriseHRISWebhookReceiptExecutionQueue, queueStore.lastQueueName)
	}
	if len(queueStore.enqueuedIDs) != 1 || queueStore.enqueuedIDs[0] != payload.ExecutionID {
		t.Fatalf("expected queued receipt execution id %s to be enqueued, got %v", payload.ExecutionID, queueStore.enqueuedIDs)
	}

	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup queued receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processing" || updatedReceipt.ProcessedAt != nil {
		t.Fatalf("expected queued receipt to remain claimed but not yet processed, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected queued receipt worker-tick path to avoid inline employee sync")
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 3, 5*time.Minute, 15*time.Minute, 10*time.Minute, 1)

	processedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(t, s, "tenant_demo_jakarta", receipt.ID, "processed")
	if processedReceipt.ProcessedAt == nil {
		t.Fatalf("expected worker tick to process queued receipt, got %+v", processedReceipt)
	}
	execution, err = s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", payload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup queued receipt execution after worker tick should succeed: %v", err)
	}
	if execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded || execution.TargetStatus != "processed" {
		t.Fatalf("expected queued receipt execution to succeed after worker tick, got %+v", execution)
	}
	if queueStore.dequeueCalls == 0 {
		t.Fatalf("expected worker tick to dequeue queued receipt execution from external queue")
	}
}

func TestProcessEnterpriseHRISWebhookReceiptQueuedExecutionSurvivesServiceRestart(t *testing.T) {
	store := &httpMemoryStateStore{}
	enterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create enterprise service with state store should succeed: %v", err)
	}
	firstServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "restart-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-queued-restart-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-QUEUED-RESTART-001",
						"employee_number":"TAL-QUEUED-RESTART-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Queued",
						"last_name":"Restart",
						"email":"queued.restart@restart-sync.local",
						"mobile_phone":"+628110000447"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	firstServer.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued single receipt process, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var queuedPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &queuedPayload); err != nil {
		t.Fatalf("expected valid queued receipt payload: %v body=%s", err, recorder.Body.String())
	}
	if queuedPayload.ExecutionID == "" {
		t.Fatalf("expected queued receipt payload to include execution_id")
	}

	restartedEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("restart enterprise service with state store should succeed: %v", err)
	}
	restartedServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          restartedEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	restartedServer.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 3, 5*time.Minute, 15*time.Minute, 10*time.Minute, 1)

	updatedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(
		t,
		restartedServer,
		"tenant_demo_jakarta",
		receipt.ID,
		"processed",
	)
	if updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected restarted worker tick to process queued receipt, got %+v", updatedReceipt)
	}
	execution, err := restartedServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedPayload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup restarted queued receipt execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "processed" {
		t.Fatalf("unexpected restarted queued receipt execution: %+v", execution)
	}
}

func TestProcessEnterpriseHRISWebhookReceiptQueuedExecutionRefreshesSharedStateForRunningWorker(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}

	firstServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          firstEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	secondServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          secondEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "shared-refresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-queued-shared-worker-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-QUEUED-SHARED-001",
						"employee_number":"TAL-QUEUED-SHARED-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Queued",
						"last_name":"Shared",
						"email":"queued.shared@shared-refresh.local",
						"mobile_phone":"+628110000551"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	if _, err := secondServer.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID); !errors.Is(err, enterprise.ErrHRISWebhookReceiptNotFound) {
		t.Fatalf("expected second server receipt view to be stale before refresh, got err=%v", err)
	}
	if len(secondServer.enterpriseSvc.ListAllHRISWebhookExecutions("")) != 0 {
		t.Fatalf("expected second server execution view to be empty before refresh")
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	firstServer.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued single receipt process, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var queuedPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &queuedPayload); err != nil {
		t.Fatalf("expected valid queued receipt payload: %v body=%s", err, recorder.Body.String())
	}
	if queuedPayload.ExecutionID == "" {
		t.Fatalf("expected queued receipt payload to include execution_id")
	}

	if len(secondServer.enterpriseSvc.ListAllHRISWebhookExecutions("")) != 0 {
		t.Fatalf("expected second server execution view to remain stale before worker refresh")
	}

	secondServer.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 3, 5*time.Minute, 15*time.Minute, 10*time.Minute, 1)

	updatedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(
		t,
		secondServer,
		"tenant_demo_jakarta",
		receipt.ID,
		"processed",
	)
	if updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected running worker on second server to process queued receipt after refresh, got %+v", updatedReceipt)
	}
	execution, err := secondServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedPayload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup refreshed queued receipt execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "processed" {
		t.Fatalf("unexpected refreshed queued receipt execution: %+v", execution)
	}
}

func TestProcessEnterpriseHRISWebhookReceiptQueuedExecutionRefreshesSharedStateAfterExternalQueueDequeueMiss(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}
	queueStore := &stubWorkerQueueStore{}

	firstServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          firstEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}
	secondServer := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          secondEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "queue-refresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-queued-queue-refresh-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-QUEUED-QUEUE-REFRESH-001",
						"employee_number":"TAL-QUEUED-QUEUE-REFRESH-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Queued",
						"last_name":"QueueRefresh",
						"email":"queued.queue.refresh@queue-refresh.local",
						"mobile_phone":"+628110000552"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}
	if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", receipt.ID); err != nil {
		t.Fatalf("mark queued receipt processing should succeed: %v", err)
	}

	if _, err := secondServer.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID); !errors.Is(err, enterprise.ErrHRISWebhookReceiptNotFound) {
		t.Fatalf("expected second server receipt view to be stale before refresh, got err=%v", err)
	}

	var queuedExecutionID string
	var enqueueOnce sync.Once
	queueStore.beforeDequeue = func(queueName string, batchSize int) {
		if queueName != enterpriseHRISWebhookReceiptExecutionQueue {
			return
		}
		enqueueOnce.Do(func() {
			record, createErr := firstServer.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
				TenantID:      "tenant_demo_jakarta",
				Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
				TargetID:      receipt.ID,
				ReceiptID:     receipt.ID,
				ConnectorID:   connector.ID,
				Vendor:        "talenta",
				RequestID:     receipt.RequestID,
				EventType:     receipt.EventType,
				AuditSource:   "enterprise_sync",
				ExecutionMode: "queued",
				DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
				TargetStatus:  "processing",
			})
			if createErr != nil {
				t.Fatalf("create queued receipt execution during dequeue should succeed: %v", createErr)
			}
			record, createErr = firstServer.enterpriseSvc.RequeueHRISWebhookExecution(
				"tenant_demo_jakarta",
				record.ID,
				"processing",
				time.Now().UTC().Add(-time.Second),
				nil,
			)
			if createErr != nil {
				t.Fatalf("backdate queued receipt execution during dequeue should succeed: %v", createErr)
			}
			queuedExecutionID = record.ID
			if enqueueErr := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookReceiptExecutionQueue, record.ID); enqueueErr != nil {
				t.Fatalf("enqueue queued receipt execution during dequeue should succeed: %v", enqueueErr)
			}
		})
	}

	secondServer.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 3, 5*time.Minute, 15*time.Minute, 10*time.Minute, 1)

	if queuedExecutionID == "" {
		t.Fatalf("expected dequeue hook to create a queued receipt execution")
	}
	if queueStore.dequeueCalls == 0 {
		t.Fatalf("expected worker tick to dequeue queued receipt execution from external queue")
	}
	updatedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(
		t,
		secondServer,
		"tenant_demo_jakarta",
		receipt.ID,
		"processed",
	)
	if updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected queue-miss refresh to process queued receipt, got %+v", updatedReceipt)
	}
	execution, err := secondServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedExecutionID)
	if err != nil {
		t.Fatalf("lookup queue-miss refreshed queued receipt execution should succeed: %v", err)
	}
	if execution.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeWorkerTick ||
		execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		execution.TargetStatus != "processed" {
		t.Fatalf("unexpected queue-miss refreshed queued receipt execution: %+v", execution)
	}
}

func TestProcessEnterpriseHRISWebhookReceiptQueuedExecutionRefreshesTargetStateAfterPostClaimCompletion(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}
	queueStore := &stubWorkerQueueStore{}

	firstServer := &server{
		enterpriseSvc:          firstEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}
	secondServer := &server{
		enterpriseSvc:          secondEnterpriseSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "post-claim-refresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-post-claim-refresh-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-POST-CLAIM-REFRESH-001",
						"employee_number":"TAL-POST-CLAIM-REFRESH-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Post",
						"last_name":"ClaimRefresh",
						"email":"post.claim.refresh@post-claim-refresh.local",
						"mobile_phone":"+628110000991"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create receipt should succeed: %v", err)
	}
	if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", receipt.ID); err != nil {
		t.Fatalf("mark receipt processing should succeed: %v", err)
	}

	execution, err := firstServer.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      receipt.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookReceiptExecutionQueue, execution.ID); err != nil {
		t.Fatalf("seed queued receipt execution queue should succeed: %v", err)
	}
	initialEmployeeCount := len(secondServer.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialUserCount := len(secondServer.accessSvc.ListUsers("tenant_demo_jakarta"))

	store.afterCompareAndSwapHook = func(key string, nextPayload []byte) {
		if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptProcessed("tenant_demo_jakarta", receipt.ID); err != nil {
			t.Fatalf("mark receipt processed in post-claim hook should succeed: %v", err)
		}
	}

	secondServer.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		1,
	)

	updatedReceipt, err := secondServer.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup refreshed post-claim receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" || updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected post-claim completed receipt to stay processed, got %+v", updatedReceipt)
	}
	updatedExecution, err := secondServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", execution.ID)
	if err != nil {
		t.Fatalf("lookup post-claim receipt execution should succeed: %v", err)
	}
	employees := secondServer.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount {
		t.Fatalf(
			"expected post-claim completed receipt to avoid growing employee count, got employees=%+v receipt=%+v execution=%+v",
			employees,
			updatedReceipt,
			updatedExecution,
		)
	}
	for i := range employees {
		if employees[i].ExternalID == "EMP-POST-CLAIM-REFRESH-001" ||
			strings.EqualFold(employees[i].Email, "post.claim.refresh@post-claim-refresh.local") {
			t.Fatalf(
				"expected post-claim completed receipt to avoid syncing target employee, got employee=%+v receipt=%+v execution=%+v",
				employees[i],
				updatedReceipt,
				updatedExecution,
			)
		}
	}
	users := secondServer.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(users) != initialUserCount {
		t.Fatalf(
			"expected post-claim completed receipt to avoid growing access sync results, got users=%+v receipt=%+v execution=%+v",
			users,
			updatedReceipt,
			updatedExecution,
		)
	}
	for i := range users {
		if strings.EqualFold(users[i].Email, "post.claim.refresh@post-claim-refresh.local") {
			t.Fatalf(
				"expected post-claim completed receipt to avoid syncing target access user, got user=%+v receipt=%+v execution=%+v",
				users[i],
				updatedReceipt,
				updatedExecution,
			)
		}
	}
	if updatedExecution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		updatedExecution.TargetStatus != "processed" {
		t.Fatalf("expected post-claim receipt execution to converge to processed success, got %+v", updatedExecution)
	}
}

func TestProcessEnterpriseHRISWebhookDLQQueuedExecutionRefreshesTargetStateAfterPostClaimReplayCompletion(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}
	firstDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first dlq service with state store should succeed: %v", err)
	}
	secondDLQSvc, err := hris.NewDLQServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second dlq service with state store should succeed: %v", err)
	}
	queueStore := &stubWorkerQueueStore{}

	firstServer := &server{
		enterpriseSvc:          firstEnterpriseSvc,
		hrisDLQSvc:             firstDLQSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}
	secondServer := &server{
		enterpriseSvc:          secondEnterpriseSvc,
		hrisDLQSvc:             secondDLQSvc,
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerQueueStore:       queueStore,
	}

	_, err = firstServer.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "post-claim-dlq-refresh.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := firstServer.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := firstServer.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "dlq-post-claim-refresh-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-DLQ-POST-CLAIM-REFRESH-001",
						"employee_number":"TAL-DLQ-POST-CLAIM-REFRESH-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"DLQ",
						"last_name":"PostClaimRefresh",
						"email":"dlq.post.claim.refresh@post-claim-dlq-refresh.local",
						"mobile_phone":"+628110000992"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create receipt should succeed: %v", err)
	}
	entry, err := firstServer.hrisDLQSvc.AppendFailure(hris.DeadLetterFailureInput{
		TenantID:      "tenant_demo_jakarta",
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		ReceiptID:     receipt.ID,
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  "sync",
		Error:         "forced post-claim dlq replay refresh failure",
		RawPayloadRef: hris.RawPayloadRef(receipt),
	})
	if err != nil {
		t.Fatalf("append dlq failure should succeed: %v", err)
	}
	if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptDLQ(
		"tenant_demo_jakarta",
		receipt.ID,
		errors.New("forced post-claim dlq replay refresh failure"),
	); err != nil {
		t.Fatalf("mark receipt dlq should succeed: %v", err)
	}
	claimedEntry, reason, err := firstServer.hrisDLQSvc.ClaimEntryForReplay(
		entry.ID,
		0,
		0,
		10*time.Minute,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("claim dlq entry for replay should succeed: %v", err)
	}
	if reason != "" {
		t.Fatalf("expected claimed dlq entry to be replayable, got reason=%s entry=%+v", reason, claimedEntry)
	}

	execution, err := firstServer.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      entry.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		FailureStage:  entry.FailureStage,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  claimedEntry.Status,
	})
	if err != nil {
		t.Fatalf("create queued dlq execution should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookDLQExecutionQueue, execution.ID); err != nil {
		t.Fatalf("seed queued dlq execution queue should succeed: %v", err)
	}
	initialEmployeeCount := len(secondServer.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialUserCount := len(secondServer.accessSvc.ListUsers("tenant_demo_jakarta"))

	store.afterCompareAndSwapHook = func(key string, nextPayload []byte) {
		if _, err := firstServer.enterpriseSvc.MarkHRISWebhookReceiptProcessed("tenant_demo_jakarta", receipt.ID); err != nil {
			t.Fatalf("mark receipt processed in dlq post-claim hook should succeed: %v", err)
		}
		if _, err := firstServer.hrisDLQSvc.MarkResolved(entry.ID); err != nil {
			t.Fatalf("mark dlq entry resolved in post-claim hook should succeed: %v", err)
		}
	}

	secondServer.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
		1,
	)

	updatedReceipt, err := secondServer.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup refreshed post-claim dlq receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "processed" || updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected post-claim dlq receipt to stay processed, got %+v", updatedReceipt)
	}
	updatedEntry, err := secondServer.hrisDLQSvc.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("lookup refreshed post-claim dlq entry should succeed: %v", err)
	}
	if updatedEntry.Status != "resolved" || updatedEntry.ResolvedAt == nil {
		t.Fatalf("expected post-claim dlq entry to stay resolved, got %+v", updatedEntry)
	}
	updatedExecution, err := secondServer.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", execution.ID)
	if err != nil {
		t.Fatalf("lookup post-claim dlq execution should succeed: %v", err)
	}
	employees := secondServer.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount {
		t.Fatalf(
			"expected post-claim resolved dlq replay to avoid growing employee count, got employees=%+v receipt=%+v entry=%+v execution=%+v",
			employees,
			updatedReceipt,
			updatedEntry,
			updatedExecution,
		)
	}
	for i := range employees {
		if employees[i].ExternalID == "EMP-DLQ-POST-CLAIM-REFRESH-001" ||
			strings.EqualFold(employees[i].Email, "dlq.post.claim.refresh@post-claim-dlq-refresh.local") {
			t.Fatalf(
				"expected post-claim resolved dlq replay to avoid syncing target employee, got employee=%+v receipt=%+v entry=%+v execution=%+v",
				employees[i],
				updatedReceipt,
				updatedEntry,
				updatedExecution,
			)
		}
	}
	users := secondServer.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(users) != initialUserCount {
		t.Fatalf(
			"expected post-claim resolved dlq replay to avoid growing access sync results, got users=%+v receipt=%+v entry=%+v execution=%+v",
			users,
			updatedReceipt,
			updatedEntry,
			updatedExecution,
		)
	}
	for i := range users {
		if strings.EqualFold(users[i].Email, "dlq.post.claim.refresh@post-claim-dlq-refresh.local") {
			t.Fatalf(
				"expected post-claim resolved dlq replay to avoid syncing target access user, got user=%+v receipt=%+v entry=%+v execution=%+v",
				users[i],
				updatedReceipt,
				updatedEntry,
				updatedExecution,
			)
		}
	}
	if updatedExecution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		updatedExecution.TargetStatus != "resolved" {
		t.Fatalf("expected post-claim dlq execution to converge to resolved success, got %+v", updatedExecution)
	}
}

func TestProcessEnterpriseHRISWebhookReceiptQueuedExecutionRecoversStaleRunningExecution(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled:           true,
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: time.Millisecond,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "stale-execution.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "receipt-stale-execution-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-STALE-EXECUTION-001",
						"employee_number":"TAL-STALE-EXECUTION-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Stale",
						"last_name":"Execution",
						"email":"stale.execution@stale-execution.local",
						"mobile_phone":"+628110000781"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal queued single process request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/"+receipt.ID+"/process",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "receiptID", receipt.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processEnterpriseHRISWebhookReceiptEntry(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued single receipt process, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var queuedPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &queuedPayload); err != nil {
		t.Fatalf("expected valid queued receipt payload: %v body=%s", err, recorder.Body.String())
	}
	if queuedPayload.ExecutionID == "" {
		t.Fatalf("expected queued receipt payload to include execution_id")
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", queuedPayload.ExecutionID); err != nil {
		t.Fatalf("mark queued execution running should succeed: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		time.Millisecond,
		1,
	)

	updatedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(t, s, "tenant_demo_jakarta", receipt.ID, "processed")
	if updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected stale running execution recovery to process receipt, got %+v", updatedReceipt)
	}
	execution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", queuedPayload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup stale recovered execution should succeed: %v", err)
	}
	if execution.Status != enterprise.HRISWebhookExecutionStatusSucceeded || execution.TargetStatus != "processed" {
		t.Fatalf("expected stale running execution to recover to succeeded, got %+v", execution)
	}
}

func TestProcessEnterpriseHRISWebhookReceiptQueuedExecutionRequeuesFreshProcessingTarget(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.updated",
			RequestID:  "receipt-execution-requeue-001",
			RawPayload: `{"event_type":"talenta.employee.detail.updated","employee":{"employment":{"employee_id":"EMP-REQUEUE-001"}}}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}
	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      receipt.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create queued execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", execution.ID); err != nil {
		t.Fatalf("mark queued execution running should succeed: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	processingReceipt, err := s.enterpriseSvc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("mark receipt processing should succeed: %v", err)
	}
	if processingReceipt.LastAttemptAt == nil {
		t.Fatalf("expected processing receipt to set last_attempt_at")
	}

	processed := s.runQueuedEnterpriseHRISWebhookReceiptExecutions(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		50*time.Millisecond,
	)
	if processed != 1 {
		t.Fatalf("expected one queued receipt execution to be requeued, got %d", processed)
	}

	updatedExecution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", execution.ID)
	if err != nil {
		t.Fatalf("lookup requeued execution should succeed: %v", err)
	}
	expectedRetryAt := processingReceipt.LastAttemptAt.Add(50 * time.Millisecond)
	if updatedExecution.Status != enterprise.HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected execution to be requeued, got %+v", updatedExecution)
	}
	if updatedExecution.TargetStatus != "processing" {
		t.Fatalf("expected requeued execution target_status=processing, got %+v", updatedExecution)
	}
	if !updatedExecution.QueuedAt.Equal(expectedRetryAt) {
		t.Fatalf("expected requeued execution queued_at=%s, got %s", expectedRetryAt, updatedExecution.QueuedAt)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected requeued receipt execution to be re-enqueued once, got %d", queueStore.enqueueCalls)
	}
	items := queueStore.itemsByQueue[enterpriseHRISWebhookReceiptExecutionQueue]
	if len(items) != 1 || items[0] != execution.ID {
		t.Fatalf("expected requeued receipt execution to return to external queue, got %v", items)
	}
	claimed, reason, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
		"tenant_demo_jakarta",
		execution.ID,
		50*time.Millisecond,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("claim requeued execution should not error: %v", err)
	}
	if reason != enterprise.HRISWebhookExecutionClaimReasonCooldown {
		t.Fatalf("expected cooldown reason for requeued execution, got %s item=%+v", reason, claimed)
	}
}

func TestListEnterpriseHRISWebhookExecutions(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	first, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_list_001",
		ReceiptID:     "whr_exec_list_001",
		ConnectorID:   "connector-talenta",
		Vendor:        "talenta",
		RequestID:     "req-exec-list-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback,
		TargetStatus:  "processing",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create first execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", first.ID); err != nil {
		t.Fatalf("mark first execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionSucceeded("tenant_demo_jakarta", first.ID, "processed"); err != nil {
		t.Fatalf("mark first execution succeeded should succeed: %v", err)
	}

	second, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      "dlq_exec_list_001",
		ReceiptID:     "whr_exec_list_002",
		ConnectorID:   "connector-talenta",
		Vendor:        "talenta",
		RequestID:     "req-exec-list-002",
		EventType:     "talenta.employee.detail.created",
		FailureStage:  "merge",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTaskChannel,
		TargetStatus:  "replaying",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create second execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", second.ID); err != nil {
		t.Fatalf("mark second execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed("tenant_demo_jakarta", second.ID, "dlq", errors.New("forced replay failure")); err != nil {
		t.Fatalf("mark second execution failed should succeed: %v", err)
	}

	replayWorkerRequired := true
	replayed, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:                "tenant_demo_jakarta",
		Kind:                    enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:                "whr_exec_list_003",
		ReceiptID:               "whr_exec_list_003",
		ConnectorID:             "connector-talenta",
		Vendor:                  "talenta",
		RequestID:               "req-exec-list-003",
		EventType:               "talenta.employee.detail.updated",
		ExecutionMode:           "queued",
		DispatchMode:            enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:            "processing",
		RequestedBy:             "qa@example.com",
		ReplaySourceExecutionID: second.ID,
		ReplayRequireWorker:     &replayWorkerRequired,
	})
	if err != nil {
		t.Fatalf("create replayed execution should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta&q=merge&status=failed&execution_mode=queued&dispatch_mode=worker_task_channel&target_status=dlq",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from execution history list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items        []enterprise.HRISWebhookExecution `json:"items"`
		Total        int                               `json:"total"`
		StatusCounts hrisWebhookExecutionStatusCounts  `json:"status_counts"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid execution history payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Total != 1 || len(payload.Items) != 1 {
		t.Fatalf("expected one filtered execution item, got total=%d items=%d payload=%+v", payload.Total, len(payload.Items), payload)
	}
	if payload.Items[0].ID != second.ID || payload.Items[0].Status != enterprise.HRISWebhookExecutionStatusFailed {
		t.Fatalf("unexpected execution history item: %+v", payload.Items[0])
	}
	if payload.Items[0].AttemptCount != 1 {
		t.Fatalf("expected execution history to expose attempt_count=1, got %+v", payload.Items[0])
	}
	if payload.StatusCounts.All != 1 || payload.StatusCounts.Failed != 1 {
		t.Fatalf("unexpected execution history status counts: %+v", payload.StatusCounts)
	}

	replayRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta&replay_scope=worker_required",
		nil,
	)
	replayRequest = withAuthUser(replayRequest, auth.User{Role: "super_admin"})
	replayRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(replayRecorder, replayRequest)

	if replayRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from worker_required execution replay scope, got %d body=%s", replayRecorder.Code, replayRecorder.Body.String())
	}
	var replayPayload struct {
		Items        []enterprise.HRISWebhookExecution `json:"items"`
		Total        int                               `json:"total"`
		StatusCounts hrisWebhookExecutionStatusCounts  `json:"status_counts"`
	}
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayPayload); err != nil {
		t.Fatalf("expected valid replay scope execution payload: %v body=%s", err, replayRecorder.Body.String())
	}
	if replayPayload.Total != 1 || len(replayPayload.Items) != 1 {
		t.Fatalf("expected one worker_required replay execution, got total=%d items=%d payload=%+v", replayPayload.Total, len(replayPayload.Items), replayPayload)
	}
	if replayPayload.Items[0].ID != replayed.ID ||
		replayPayload.Items[0].ReplaySourceExecutionID != second.ID ||
		replayPayload.Items[0].ReplayRequireWorker == nil ||
		!*replayPayload.Items[0].ReplayRequireWorker {
		t.Fatalf("unexpected worker_required replay execution item: %+v", replayPayload.Items[0])
	}
	if replayPayload.StatusCounts.All != 1 || replayPayload.StatusCounts.Queued != 1 {
		t.Fatalf("unexpected replay-scope status counts: %+v", replayPayload.StatusCounts)
	}
}

func TestListEnterpriseHRISWebhookExecutionsRefreshesSharedState(t *testing.T) {
	store := &httpMemoryStateStore{}
	firstEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create first enterprise service with state store should succeed: %v", err)
	}
	secondEnterpriseSvc, err := enterprise.NewServiceWithStateStore(store)
	if err != nil {
		t.Fatalf("create second enterprise service with state store should succeed: %v", err)
	}

	record, err := firstEnterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_refresh_exec_001",
		ReceiptID:     "whr_refresh_exec_001",
		ConnectorID:   "connector-talenta-refresh",
		Vendor:        "talenta",
		RequestID:     "talenta-execution-refresh-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
	})
	if err != nil {
		t.Fatalf("create execution should succeed: %v", err)
	}
	if len(secondEnterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected second service execution view to be stale before list refresh")
	}

	s := &server{
		enterpriseSvc: secondEnterpriseSvc,
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from execution list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookExecutionListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid execution list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != record.ID {
		t.Fatalf("expected refreshed execution list to include latest execution, got %+v", payload.Items)
	}
	if payload.Items[0].QueueState != "ready" || payload.Items[0].StaleInFlight {
		t.Fatalf("expected queued worker execution to expose ready runtime state, got %+v", payload.Items[0])
	}
}

func TestGetEnterpriseHRISWebhookExecution(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	record, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_detail_001",
		ReceiptID:     "whr_exec_detail_001",
		ConnectorID:   "connector-talenta-detail",
		Vendor:        "talenta",
		RequestID:     "req-exec-detail-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", record.ID); err != nil {
		t.Fatalf("mark execution running should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookReceiptExecutionQueue, record.ID); err != nil {
		t.Fatalf("seed receipt execution queue should succeed: %v", err)
	}
	if claims, err := queueStore.ClaimWorkerQueueBatch(
		enterpriseHRISWebhookReceiptExecutionQueue,
		1,
		10*time.Minute,
	); err != nil {
		t.Fatalf("claim receipt execution queue should succeed: %v", err)
	} else if len(claims) != 1 || claims[0].ItemID != record.ID {
		t.Fatalf("expected one claimed receipt execution queue item, got %+v", claims)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions/"+record.ID+"?tenant_id=tenant_demo_jakarta",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	request = withURLParam(request, "executionID", record.ID)
	recorder := httptest.NewRecorder()

	s.getEnterpriseHRISWebhookExecution(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from execution detail, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Item hrisWebhookExecutionListItem `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid execution detail payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Item.ID != record.ID || payload.Item.Status != enterprise.HRISWebhookExecutionStatusRunning {
		t.Fatalf("unexpected execution detail item: %+v", payload.Item)
	}
	if payload.Item.AttemptCount != 1 || payload.Item.RequeueCount != 0 {
		t.Fatalf("expected execution detail to expose attempt/requeue audit counts, got %+v", payload.Item)
	}
	if payload.Item.QueueState != enterprise.HRISWebhookExecutionClaimReasonInFlight ||
		payload.Item.ProcessingDeadlineAt == nil ||
		payload.Item.StaleInFlight {
		t.Fatalf("expected execution detail to expose in-flight worker runtime, got %+v", payload.Item)
	}
	if payload.Item.ExternalQueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		payload.Item.ExternalQueueState != redistore.WorkerQueueStateClaimed ||
		payload.Item.ExternalQueueVisibilityDeadlineAt == nil {
		t.Fatalf("expected execution detail to expose claimed external queue telemetry, got %+v", payload.Item)
	}
}

func TestGetEnterpriseHRISWebhookExecutionReportsMissingExternalQueueTelemetryWithoutRedis(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc: enterprise.NewService(),
	}

	record, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_detail_no_redis_001",
		ReceiptID:     "whr_exec_detail_no_redis_001",
		ConnectorID:   "connector-talenta-no-redis-detail",
		Vendor:        "talenta",
		RequestID:     "req-exec-detail-no-redis-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create execution should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions/"+record.ID+"?tenant_id=tenant_demo_jakarta",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	request = withURLParam(request, "executionID", record.ID)
	recorder := httptest.NewRecorder()

	s.getEnterpriseHRISWebhookExecution(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from no-redis execution detail, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Item hrisWebhookExecutionListItem `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid no-redis execution detail payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.Item.ID != record.ID {
		t.Fatalf("unexpected execution detail item: %+v", payload.Item)
	}
	if payload.Item.ExternalQueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		payload.Item.ExternalQueueState != redistore.WorkerQueueStateMissing ||
		payload.Item.ExternalQueueVisibilityDeadlineAt != nil {
		t.Fatalf("expected no-redis execution detail to expose missing external queue telemetry, got %+v", payload.Item)
	}
}

func TestListEnterpriseHRISWebhookExecutionsIncludesExternalQueueTelemetry(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout:     10 * time.Minute,
		},
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	queuedExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_ext_queue_queued_001",
		ReceiptID:     "whr_exec_ext_queue_queued_001",
		ConnectorID:   "connector-talenta-ext-queue",
		Vendor:        "talenta",
		RequestID:     "req-ext-queue-queued-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued execution should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookReceiptExecutionQueue, queuedExecution.ID); err != nil {
		t.Fatalf("seed queued receipt execution queue should succeed: %v", err)
	}

	claimedExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      "hdlq_exec_ext_queue_claimed_001",
		ReceiptID:     "whr_exec_ext_queue_claimed_001",
		ConnectorID:   "connector-talenta-ext-queue",
		Vendor:        "talenta",
		RequestID:     "req-ext-queue-claimed-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "merge",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "replaying",
	})
	if err != nil {
		t.Fatalf("create claimed execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", claimedExecution.ID); err != nil {
		t.Fatalf("mark claimed execution running should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookDLQExecutionQueue, claimedExecution.ID); err != nil {
		t.Fatalf("seed claimed dlq execution queue should succeed: %v", err)
	}
	if claims, err := queueStore.ClaimWorkerQueueBatch(
		enterpriseHRISWebhookDLQExecutionQueue,
		1,
		10*time.Minute,
	); err != nil {
		t.Fatalf("claim dlq execution queue should succeed: %v", err)
	} else if len(claims) != 1 || claims[0].ItemID != claimedExecution.ID {
		t.Fatalf("expected one claimed dlq execution queue item, got %+v", claims)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-ext-queue",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from external queue execution list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookExecutionListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid external queue execution payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two execution items with external queue telemetry, got %+v", payload.Items)
	}
	if len(payload.ExternalQueues) != 2 {
		t.Fatalf("expected two external queue summaries, got %+v", payload.ExternalQueues)
	}

	itemsByRequestID := make(map[string]hrisWebhookExecutionListItem, len(payload.Items))
	for i := range payload.Items {
		itemsByRequestID[payload.Items[i].RequestID] = payload.Items[i]
	}

	queuedItem, ok := itemsByRequestID[queuedExecution.RequestID]
	if !ok {
		t.Fatalf("expected queued execution item to be listed")
	}
	if queuedItem.ExternalQueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		queuedItem.ExternalQueueState != redistore.WorkerQueueStateQueued ||
		queuedItem.ExternalQueueVisibilityDeadlineAt != nil {
		t.Fatalf("unexpected queued external queue telemetry: %+v", queuedItem)
	}

	claimedItem, ok := itemsByRequestID[claimedExecution.RequestID]
	if !ok {
		t.Fatalf("expected claimed execution item to be listed")
	}
	if claimedItem.ExternalQueueName != enterpriseHRISWebhookDLQExecutionQueue ||
		claimedItem.ExternalQueueState != redistore.WorkerQueueStateClaimed ||
		claimedItem.ExternalQueueVisibilityDeadlineAt == nil {
		t.Fatalf("unexpected claimed external queue telemetry: %+v", claimedItem)
	}

	summariesByKind := make(map[string]hrisWebhookExecutionExternalQueueSummary, len(payload.ExternalQueues))
	for i := range payload.ExternalQueues {
		summariesByKind[payload.ExternalQueues[i].Kind] = payload.ExternalQueues[i]
	}
	receiptSummary, ok := summariesByKind[enterprise.HRISWebhookExecutionKindReceiptProcess]
	if !ok {
		t.Fatalf("expected receipt external queue summary, got %+v", payload.ExternalQueues)
	}
	if receiptSummary.QueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		receiptSummary.PendingCount != 1 ||
		receiptSummary.ClaimedCount != 0 {
		t.Fatalf("unexpected receipt external queue summary: %+v", receiptSummary)
	}
	dlqSummary, ok := summariesByKind[enterprise.HRISWebhookExecutionKindDLQReplay]
	if !ok {
		t.Fatalf("expected dlq external queue summary, got %+v", payload.ExternalQueues)
	}
	if dlqSummary.QueueName != enterpriseHRISWebhookDLQExecutionQueue ||
		dlqSummary.PendingCount != 0 ||
		dlqSummary.ClaimedCount != 1 {
		t.Fatalf("unexpected dlq external queue summary: %+v", dlqSummary)
	}
}

func TestListEnterpriseHRISWebhookExecutionsReportsMissingExternalQueueTelemetryWithoutRedis(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
			EnterpriseHRISWebhookDLQWorkerProcessingTimeout:     10 * time.Minute,
		},
		enterpriseSvc: enterprise.NewService(),
	}

	queuedExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_ext_queue_no_redis_queued_001",
		ReceiptID:     "whr_exec_ext_queue_no_redis_queued_001",
		ConnectorID:   "connector-talenta-no-redis-ext-queue",
		Vendor:        "talenta",
		RequestID:     "req-ext-queue-no-redis-queued-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued execution should succeed: %v", err)
	}

	runningExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      "hdlq_exec_ext_queue_no_redis_running_001",
		ReceiptID:     "whr_exec_ext_queue_no_redis_running_001",
		ConnectorID:   "connector-talenta-no-redis-ext-queue",
		Vendor:        "talenta",
		RequestID:     "req-ext-queue-no-redis-running-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "merge",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "replaying",
	})
	if err != nil {
		t.Fatalf("create running execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", runningExecution.ID); err != nil {
		t.Fatalf("mark running execution should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-no-redis-ext-queue",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from no-redis external queue execution list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookExecutionListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid no-redis external queue execution payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two execution items with no-redis telemetry, got %+v", payload.Items)
	}
	if len(payload.ExternalQueues) != 2 {
		t.Fatalf("expected two no-redis external queue summaries, got %+v", payload.ExternalQueues)
	}

	itemsByRequestID := make(map[string]hrisWebhookExecutionListItem, len(payload.Items))
	for i := range payload.Items {
		itemsByRequestID[payload.Items[i].RequestID] = payload.Items[i]
	}

	queuedItem, ok := itemsByRequestID[queuedExecution.RequestID]
	if !ok {
		t.Fatalf("expected queued execution item to be listed")
	}
	if queuedItem.ExternalQueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		queuedItem.ExternalQueueState != redistore.WorkerQueueStateMissing ||
		queuedItem.ExternalQueueVisibilityDeadlineAt != nil {
		t.Fatalf("unexpected no-redis queued external queue telemetry: %+v", queuedItem)
	}

	runningItem, ok := itemsByRequestID[runningExecution.RequestID]
	if !ok {
		t.Fatalf("expected running execution item to be listed")
	}
	if runningItem.ExternalQueueName != enterpriseHRISWebhookDLQExecutionQueue ||
		runningItem.ExternalQueueState != redistore.WorkerQueueStateMissing ||
		runningItem.ExternalQueueVisibilityDeadlineAt != nil {
		t.Fatalf("unexpected no-redis running external queue telemetry: %+v", runningItem)
	}

	summariesByKind := make(map[string]hrisWebhookExecutionExternalQueueSummary, len(payload.ExternalQueues))
	for i := range payload.ExternalQueues {
		summariesByKind[payload.ExternalQueues[i].Kind] = payload.ExternalQueues[i]
	}
	receiptSummary, ok := summariesByKind[enterprise.HRISWebhookExecutionKindReceiptProcess]
	if !ok {
		t.Fatalf("expected no-redis receipt external queue summary, got %+v", payload.ExternalQueues)
	}
	if receiptSummary.QueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		receiptSummary.PendingCount != 0 ||
		receiptSummary.ClaimedCount != 0 {
		t.Fatalf("unexpected no-redis receipt external queue summary: %+v", receiptSummary)
	}
	dlqSummary, ok := summariesByKind[enterprise.HRISWebhookExecutionKindDLQReplay]
	if !ok {
		t.Fatalf("expected no-redis dlq external queue summary, got %+v", payload.ExternalQueues)
	}
	if dlqSummary.QueueName != enterpriseHRISWebhookDLQExecutionQueue ||
		dlqSummary.PendingCount != 0 ||
		dlqSummary.ClaimedCount != 0 {
		t.Fatalf("unexpected no-redis dlq external queue summary: %+v", dlqSummary)
	}
}

func TestListEnterpriseHRISWebhookExecutionsReportsMissingExternalQueueStateForIndexOnlyDrift(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_ext_queue_drift_001",
		ReceiptID:     "whr_exec_ext_queue_drift_001",
		ConnectorID:   "connector-talenta-ext-queue-drift",
		Vendor:        "talenta",
		RequestID:     "req-ext-queue-drift-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued execution should succeed: %v", err)
	}
	queueStore.indexByQueue = map[string]map[string]struct{}{
		enterpriseHRISWebhookReceiptExecutionQueue: {
			execution.ID: {},
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-ext-queue-drift",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from execution list with index-only drift, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookExecutionListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid execution list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one execution item, got %+v", payload.Items)
	}
	item := payload.Items[0]
	if item.ID != execution.ID {
		t.Fatalf("expected execution %s, got %+v", execution.ID, item)
	}
	if item.ExternalQueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		item.ExternalQueueState != redistore.WorkerQueueStateMissing ||
		item.ExternalQueueVisibilityDeadlineAt != nil {
		t.Fatalf("expected execution list to expose missing external queue drift, got %+v", item)
	}
	if len(payload.ExternalQueues) != 1 {
		t.Fatalf("expected one external queue summary, got %+v", payload.ExternalQueues)
	}
	if payload.ExternalQueues[0].QueueName != enterpriseHRISWebhookReceiptExecutionQueue ||
		payload.ExternalQueues[0].PendingCount != 0 ||
		payload.ExternalQueues[0].ClaimedCount != 0 {
		t.Fatalf("unexpected external queue summary for index-only drift: %+v", payload.ExternalQueues[0])
	}
}

func TestListEnterpriseHRISWebhookExecutionsDeduplicatesExternalQueuePendingCounts(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_pending_duplicate_001",
		ReceiptID:     "whr_exec_pending_duplicate_001",
		ConnectorID:   "connector-talenta-pending-duplicate",
		Vendor:        "talenta",
		RequestID:     "req-pending-duplicate-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued execution should succeed: %v", err)
	}
	queueStore.itemsByQueue = map[string][]string{
		enterpriseHRISWebhookReceiptExecutionQueue: {
			execution.ID,
			execution.ID,
		},
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-pending-duplicate",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from duplicate pending execution list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookExecutionListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid duplicate pending execution payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected one execution item, got %+v", payload.Items)
	}
	if payload.Items[0].ExternalQueueState != redistore.WorkerQueueStateQueued {
		t.Fatalf("expected duplicate pending item to remain queued, got %+v", payload.Items[0])
	}
	if len(payload.ExternalQueues) != 1 {
		t.Fatalf("expected one external queue summary, got %+v", payload.ExternalQueues)
	}
	if payload.ExternalQueues[0].PendingCount != 1 || payload.ExternalQueues[0].ClaimedCount != 0 {
		t.Fatalf("expected duplicate pending entries to collapse into one pending count, got %+v", payload.ExternalQueues[0])
	}
}

func TestListEnterpriseHRISWebhookExecutionsIncludesWorkerRuntime(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc: enterprise.NewService(),
	}

	cooldownExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_runtime_cooldown_001",
		ReceiptID:     "whr_exec_runtime_cooldown_001",
		ConnectorID:   "connector-talenta-runtime",
		Vendor:        "talenta",
		RequestID:     "req-exec-runtime-cooldown-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create cooldown execution should succeed: %v", err)
	}
	retryAt := time.Now().UTC().Add(15 * time.Minute).Round(time.Second)
	if _, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		"tenant_demo_jakarta",
		cooldownExecution.ID,
		"processing",
		retryAt,
		errors.New("cooldown execution"),
	); err != nil {
		t.Fatalf("requeue cooldown execution should succeed: %v", err)
	}

	staleExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_runtime_stale_001",
		ReceiptID:     "whr_exec_runtime_stale_001",
		ConnectorID:   "connector-talenta-runtime",
		Vendor:        "talenta",
		RequestID:     "req-exec-runtime-stale-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create stale execution should succeed: %v", err)
	}
	staleStartedAt := time.Now().UTC().Add(-20 * time.Minute).Round(time.Second)
	if _, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		"tenant_demo_jakarta",
		staleExecution.ID,
		"processing",
		staleStartedAt,
		errors.New("stale execution"),
	); err != nil {
		t.Fatalf("requeue stale execution should succeed: %v", err)
	}
	if _, _, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
		"tenant_demo_jakarta",
		staleExecution.ID,
		10*time.Minute,
		staleStartedAt,
	); err != nil {
		t.Fatalf("claim stale execution should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-runtime",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from execution runtime list, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload hrisWebhookExecutionListResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid execution runtime list payload: %v body=%s", err, recorder.Body.String())
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two runtime execution items, got %+v", payload.Items)
	}
	if payload.QueueCounts.All != 2 ||
		payload.QueueCounts.Ready != 1 ||
		payload.QueueCounts.Cooldown != 1 ||
		payload.QueueCounts.InFlight != 0 ||
		payload.QueueCounts.AttemptLimit != 0 ||
		payload.QueueCounts.Terminal != 0 {
		t.Fatalf("unexpected execution runtime queue counts: %+v", payload.QueueCounts)
	}

	itemsByRequestID := make(map[string]hrisWebhookExecutionListItem, len(payload.Items))
	for i := range payload.Items {
		itemsByRequestID[payload.Items[i].RequestID] = payload.Items[i]
	}

	cooldownItem, ok := itemsByRequestID[cooldownExecution.RequestID]
	if !ok {
		t.Fatalf("expected cooldown execution to be listed")
	}
	if cooldownItem.QueueState != enterprise.HRISWebhookExecutionClaimReasonCooldown ||
		cooldownItem.NextRetryAt == nil ||
		!cooldownItem.NextRetryAt.Equal(retryAt) ||
		cooldownItem.CooldownRemainingSec <= 0 ||
		cooldownItem.StaleInFlight {
		t.Fatalf("unexpected cooldown execution runtime item: %+v expected_retry_at=%s", cooldownItem, retryAt.Format(time.RFC3339))
	}

	staleItem, ok := itemsByRequestID[staleExecution.RequestID]
	if !ok {
		t.Fatalf("expected stale execution to be listed")
	}
	expectedStaleDeadline := staleStartedAt.Add(10 * time.Minute)
	if staleItem.Status != enterprise.HRISWebhookExecutionStatusRunning ||
		staleItem.QueueState != "ready" ||
		!staleItem.StaleInFlight ||
		staleItem.ProcessingDeadlineAt == nil ||
		!staleItem.ProcessingDeadlineAt.Equal(expectedStaleDeadline) {
		t.Fatalf("unexpected stale execution runtime item: %+v expected_deadline=%s", staleItem, expectedStaleDeadline.Format(time.RFC3339))
	}

	readyRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&connector_id=connector-talenta-runtime&queue_state=ready",
		nil,
	)
	readyRequest = withAuthUser(readyRequest, auth.User{Role: "super_admin"})
	readyRecorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(readyRecorder, readyRequest)

	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 from execution runtime ready filter, got %d body=%s", readyRecorder.Code, readyRecorder.Body.String())
	}
	var readyPayload hrisWebhookExecutionListResult
	if err := json.Unmarshal(readyRecorder.Body.Bytes(), &readyPayload); err != nil {
		t.Fatalf("expected valid execution runtime ready payload: %v body=%s", err, readyRecorder.Body.String())
	}
	if len(readyPayload.Items) != 1 {
		t.Fatalf("expected one ready execution item, got %+v", readyPayload.Items)
	}
	if readyPayload.Items[0].ID != staleExecution.ID ||
		readyPayload.Items[0].QueueState != "ready" ||
		!readyPayload.Items[0].StaleInFlight {
		t.Fatalf("unexpected ready execution runtime item: %+v", readyPayload.Items[0])
	}
	if readyPayload.QueueCounts.All != 2 ||
		readyPayload.QueueCounts.Ready != 1 ||
		readyPayload.QueueCounts.Cooldown != 1 ||
		readyPayload.QueueCounts.InFlight != 0 ||
		readyPayload.QueueCounts.AttemptLimit != 0 ||
		readyPayload.QueueCounts.Terminal != 0 {
		t.Fatalf("unexpected ready-filter queue counts: %+v", readyPayload.QueueCounts)
	}
}

func TestListEnterpriseHRISWebhookExecutionsRejectsInvalidQueueState(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&queue_state=invalid",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from invalid execution queue_state, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid invalid-queue-state payload: %v body=%s", err, recorder.Body.String())
	}
	if payload["error"] != "queue_state must be one of ready, cooldown, in_flight, attempt_limit, terminal" {
		t.Fatalf("unexpected invalid queue_state error payload: %+v", payload)
	}
}

func TestListEnterpriseHRISWebhookExecutionsRejectsInvalidReplayScope(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/enterprise/hris-webhook-executions?tenant_id=tenant_demo_jakarta&replay_scope=invalid",
		nil,
	)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.listEnterpriseHRISWebhookExecutions(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from invalid execution replay_scope, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid invalid-replay-scope payload: %v body=%s", err, recorder.Body.String())
	}
	if payload["error"] != "replay_scope must be one of replayed, worker_required" {
		t.Fatalf("unexpected invalid replay_scope error payload: %+v", payload)
	}
}

func TestReplayEnterpriseHRISWebhookExecutionQueuedReceiptFlow(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "execution-replay.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "talenta-execution-replay-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-EXEC-REPLAY-001",
						"employee_number":"TAL-EXEC-REPLAY-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Execution",
						"last_name":"Replay",
						"email":"execution.replay@execution-replay.local",
						"mobile_phone":"+628110000991"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create replay receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", receipt.ID, errors.New("seed receipt failure")); err != nil {
		t.Fatalf("mark replay receipt failed should succeed: %v", err)
	}

	sourceExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      receipt.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback,
		TargetStatus:  "failed",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create source execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", sourceExecution.ID); err != nil {
		t.Fatalf("mark source execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		sourceExecution.ID,
		"failed",
		errors.New("seed execution failure"),
	); err != nil {
		t.Fatalf("mark source execution failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
	})
	if err != nil {
		t.Fatalf("marshal execution replay request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "executionID", sourceExecution.ID)
	request = withAuthUser(request, auth.User{
		Role:  "super_admin",
		Email: "qa@example.com",
	})
	recorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from queued execution replay, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		SourceExecutionID string                          `json:"source_execution_id"`
		ExecutionMode     string                          `json:"execution_mode"`
		DispatchMode      string                          `json:"dispatch_mode"`
		ExecutionID       string                          `json:"execution_id"`
		Execution         enterprise.HRISWebhookExecution `json:"execution"`
		ReceiptItem       enterprise.HRISWebhookReceipt   `json:"receipt_item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid queued execution replay payload: %v body=%s", err, recorder.Body.String())
	}
	if payload.SourceExecutionID != sourceExecution.ID ||
		payload.ExecutionMode != "queued" ||
		payload.DispatchMode != enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback {
		t.Fatalf("unexpected queued execution replay payload: %+v", payload)
	}
	if payload.ExecutionID == "" || payload.Execution.ID != payload.ExecutionID {
		t.Fatalf("expected queued execution replay payload to include new execution: %+v", payload)
	}
	if payload.Execution.Kind != enterprise.HRISWebhookExecutionKindReceiptProcess ||
		payload.Execution.AuditSource != enterpriseHRISWebhookExecutionReplayAuditSource {
		t.Fatalf("unexpected queued replay execution metadata: %+v", payload.Execution)
	}
	if payload.Execution.ReplaySourceExecutionID != sourceExecution.ID ||
		payload.Execution.ReplayRequireWorker == nil ||
		*payload.Execution.ReplayRequireWorker {
		t.Fatalf("unexpected queued replay execution audit metadata: %+v", payload.Execution)
	}
	if payload.ReceiptItem.ID != receipt.ID || payload.ReceiptItem.Status != "processing" {
		t.Fatalf("expected replay to claim receipt for processing, got %+v", payload.ReceiptItem)
	}

	updatedReceipt := waitForEnterpriseHRISWebhookReceiptStatus(t, s, "tenant_demo_jakarta", receipt.ID, "processed")
	if updatedReceipt.ProcessedAt == nil {
		t.Fatalf("expected replayed receipt to be processed asynchronously, got %+v", updatedReceipt)
	}
	newExecution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", payload.ExecutionID)
	if err != nil {
		t.Fatalf("lookup replay execution should succeed: %v", err)
	}
	if newExecution.Status != enterprise.HRISWebhookExecutionStatusSucceeded ||
		newExecution.TargetStatus != "processed" ||
		newExecution.AuditSource != enterpriseHRISWebhookExecutionReplayAuditSource {
		t.Fatalf("unexpected replay execution record: %+v", newExecution)
	}
	if newExecution.ReplaySourceExecutionID != sourceExecution.ID ||
		newExecution.ReplayRequireWorker == nil ||
		*newExecution.ReplayRequireWorker {
		t.Fatalf("unexpected replay execution audit record: %+v", newExecution)
	}
	sourceAfterReplay, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", sourceExecution.ID)
	if err != nil {
		t.Fatalf("lookup source execution after replay should succeed: %v", err)
	}
	if sourceAfterReplay.Status != enterprise.HRISWebhookExecutionStatusFailed {
		t.Fatalf("expected source execution to remain failed history, got %+v", sourceAfterReplay)
	}

	logs := s.auditSvc.ListFiltered(
		"tenant_demo_jakarta",
		"enterprise_hris_webhook_execution_replayed",
		enterpriseHRISWebhookExecutionReplayAuditSource,
		10,
	)
	if len(logs) != 1 {
		t.Fatalf("expected one execution replay audit log, got %d", len(logs))
	}
}

func TestReplayEnterpriseHRISWebhookExecutionQueuedReceiptFlowRejectsDuplicateReplay(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "execution-replay-duplicate.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType: "talenta.employee.detail.created",
			RequestID: "talenta-execution-replay-duplicate-001",
			RawPayload: `{
				"event_type":"talenta.employee.detail.created",
				"employee":{
					"employment":{
						"employee_id":"EMP-EXEC-REPLAY-DUP-001",
						"employee_number":"TAL-EXEC-REPLAY-DUP-001",
						"organization_name":"Operations",
						"job_position":"Coordinator",
						"branch":"Jakarta",
						"join_date":"2026-04-20"
					},
					"personal":{
						"first_name":"Execution",
						"last_name":"ReplayDuplicate",
						"email":"execution.replay.duplicate@execution-replay-duplicate.local",
						"mobile_phone":"+628110001101"
					}
				}
			}`,
		},
	)
	if err != nil {
		t.Fatalf("create replay duplicate receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", receipt.ID, errors.New("seed receipt failure")); err != nil {
		t.Fatalf("mark replay duplicate receipt failed should succeed: %v", err)
	}

	sourceExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      receipt.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "failed",
		RequestedBy:   "qa@example.com",
	})
	if err != nil {
		t.Fatalf("create source execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", sourceExecution.ID); err != nil {
		t.Fatalf("mark source execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		sourceExecution.ID,
		"failed",
		errors.New("seed execution failure"),
	); err != nil {
		t.Fatalf("mark source execution failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal duplicate replay request should succeed: %v", err)
	}

	firstRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstRequest = withURLParam(firstRequest, "executionID", sourceExecution.ID)
	firstRequest = withAuthUser(firstRequest, auth.User{
		Role:  "super_admin",
		Email: "qa@example.com",
	})
	firstRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from first duplicate replay request, got %d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}
	var firstPayload struct {
		ExecutionID string `json:"execution_id"`
	}
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &firstPayload); err != nil {
		t.Fatalf("expected valid first duplicate replay payload: %v body=%s", err, firstRecorder.Body.String())
	}
	if firstPayload.ExecutionID == "" {
		t.Fatalf("expected first duplicate replay payload to include execution_id")
	}

	secondRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest = withURLParam(secondRequest, "executionID", sourceExecution.ID)
	secondRequest = withAuthUser(secondRequest, auth.User{
		Role:  "super_admin",
		Email: "qa@example.com",
	})
	secondRecorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from duplicate replay request, got %d body=%s", secondRecorder.Code, secondRecorder.Body.String())
	}
	var secondPayload struct {
		Error               string                          `json:"error"`
		ExistingExecutionID string                          `json:"existing_execution_id"`
		ExistingExecution   enterprise.HRISWebhookExecution `json:"existing_execution"`
	}
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &secondPayload); err != nil {
		t.Fatalf("expected valid duplicate replay conflict payload: %v body=%s", err, secondRecorder.Body.String())
	}
	if secondPayload.ExistingExecutionID != firstPayload.ExecutionID ||
		secondPayload.ExistingExecution.ID != firstPayload.ExecutionID ||
		secondPayload.ExistingExecution.Status != enterprise.HRISWebhookExecutionStatusQueued {
		t.Fatalf("unexpected duplicate replay conflict payload: %+v first=%s", secondPayload, firstPayload.ExecutionID)
	}
	if !strings.Contains(secondPayload.Error, "already queued or running") {
		t.Fatalf("unexpected duplicate replay conflict error payload: %+v", secondPayload)
	}
	executions := s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")
	if len(executions) != 2 {
		t.Fatalf("expected duplicate replay protection to keep one child execution, got %+v", executions)
	}
}

func TestProcessBatchEnterpriseHRISWebhookReceiptsQueuedFlowRejectsRequireWorkerWithoutWorker(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerMaxAttempts:       3,
			EnterpriseHRISWebhookReceiptWorkerRetryCooldown:     5 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff:   15 * time.Minute,
			EnterpriseHRISWebhookReceiptWorkerProcessingTimeout: 10 * time.Minute,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.created",
			RequestID:  "receipt-batch-queued-require-worker",
			RawPayload: `{"event_type":"talenta.employee.detail.created"}`,
		},
	)
	if err != nil {
		t.Fatalf("create queued receipt should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"receipt_ids":    []string{receipt.ID},
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal queued process-batch request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-receipts/process-batch",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.processBatchEnterpriseHRISWebhookReceipts(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from queued receipt process batch without worker, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), errEnterpriseHRISWebhookQueuedReceiptWorkerRequired.Error()) {
		t.Fatalf("expected require_worker conflict message, got body=%s", recorder.Body.String())
	}

	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "received" || updatedReceipt.AttemptCount != 0 {
		t.Fatalf("expected batch receipt to remain unclaimed after require_worker conflict, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 0 {
		t.Fatalf("expected no execution record after batch require_worker conflict")
	}
}

func TestReplayEnterpriseHRISWebhookExecutionQueuedReceiptFlowRejectsRequireWorkerWithoutWorker(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	receipt, err := s.enterpriseSvc.ReceiveHRISWebhookReceipt(
		connector.ID,
		enterprise.HRISWebhookReceiptInput{
			EventType:  "talenta.employee.detail.created",
			RequestID:  "receipt-execution-replay-require-worker",
			RawPayload: `{"event_type":"talenta.employee.detail.created"}`,
		},
	)
	if err != nil {
		t.Fatalf("create replay receipt should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookReceiptFailed("tenant_demo_jakarta", receipt.ID, errors.New("seed receipt failure")); err != nil {
		t.Fatalf("mark replay receipt failed should succeed: %v", err)
	}

	sourceExecution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      receipt.ID,
		ReceiptID:     receipt.ID,
		ConnectorID:   connector.ID,
		Vendor:        "talenta",
		RequestID:     receipt.RequestID,
		EventType:     receipt.EventType,
		AuditSource:   "enterprise_sync",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeGoroutineFallback,
		TargetStatus:  "failed",
	})
	if err != nil {
		t.Fatalf("create source execution should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionRunning("tenant_demo_jakarta", sourceExecution.ID); err != nil {
		t.Fatalf("mark source execution running should succeed: %v", err)
	}
	if _, err := s.enterpriseSvc.MarkHRISWebhookExecutionFailed(
		"tenant_demo_jakarta",
		sourceExecution.ID,
		"failed",
		errors.New("seed execution failure"),
	); err != nil {
		t.Fatalf("mark source execution failed should succeed: %v", err)
	}

	requestBody, err := json.Marshal(map[string]any{
		"tenant_id":      "tenant_demo_jakarta",
		"execution_mode": "queued",
		"require_worker": true,
	})
	if err != nil {
		t.Fatalf("marshal execution replay request should succeed: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook-executions/"+sourceExecution.ID+"/replay",
		bytes.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "executionID", sourceExecution.ID)
	request = withAuthUser(request, auth.User{Role: "super_admin"})
	recorder := httptest.NewRecorder()

	s.replayEnterpriseHRISWebhookExecution(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 from queued receipt execution replay without worker, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), errEnterpriseHRISWebhookQueuedReceiptWorkerRequired.Error()) {
		t.Fatalf("expected require_worker conflict message, got body=%s", recorder.Body.String())
	}

	updatedReceipt, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", receipt.ID)
	if err != nil {
		t.Fatalf("lookup receipt should succeed: %v", err)
	}
	if updatedReceipt.Status != "failed" {
		t.Fatalf("expected receipt to remain failed after replay require_worker conflict, got %+v", updatedReceipt)
	}
	if len(s.enterpriseSvc.ListAllHRISWebhookExecutions("tenant_demo_jakarta")) != 1 {
		t.Fatalf("expected replay conflict to avoid creating a new execution")
	}
}

func TestReceiveEnterpriseHRISWebhookInactiveConnectorConflict(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"inactive",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(`{"event_type":"employee.updated"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReceiveEnterpriseHRISWebhookPayloadTooLarge(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"linovhr",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(strings.Repeat("a", enterpriseHRISWebhookMaxPayloadBytes+1)),
	)
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReceiveEnterpriseHRISWebhookProcessesSupportedTalentaEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
			"employment":{
				"employee_id":"EMP-001",
				"employee_number":"TAL-001",
				"organization_name":"IT Division",
				"job_position":"Staff IT",
				"branch":"Pusat",
				"join_date":"2026-04-20"
			},
			"personal":{
				"first_name":"SDET",
				"last_name":"Superadmin",
				"email":"sdet.superadmin@talenta-sync.local",
				"mobile_phone":"+628110000111",
				"avatar":"https://cdn.example.com/photos/sdet-superadmin.jpg"
			},
			"leave_info":{
				"status":"approved",
				"type":"Annual Leave"
			},
			"payroll_info":{
				"cost_center_name":"CC-OPS-01"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount+1 {
		t.Fatalf("expected employee count to increase by 1, before=%d after=%d", initialEmployeeCount, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].Email != "sdet.superadmin@talenta-sync.local" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected synced employee to be present")
	}
	if syncedEmployee.ExternalID != "EMP-001" {
		t.Fatalf("unexpected external_id: %s", syncedEmployee.ExternalID)
	}
	if syncedEmployee.Department != "IT Division" {
		t.Fatalf("unexpected department: %s", syncedEmployee.Department)
	}
	if syncedEmployee.JoinDate != "2026-04-20" {
		t.Fatalf("unexpected join_date: %s", syncedEmployee.JoinDate)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("unexpected leave_status: %s", syncedEmployee.LeaveStatus)
	}
	if syncedEmployee.CostCenter != "CC-OPS-01" {
		t.Fatalf("unexpected cost_center: %s", syncedEmployee.CostCenter)
	}
	if syncedEmployee.PhotoURL != "https://cdn.example.com/photos/sdet-superadmin.jpg" {
		t.Fatalf("unexpected photo_url: %s", syncedEmployee.PhotoURL)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected access user count to increase by 1, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "sdet.superadmin@talenta-sync.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected synced access user to be present")
	}

	record, err := s.enterpriseSvc.GetSyncRequestRecord("tenant_demo_jakarta", "mekari-evt-hris-001")
	if err != nil {
		t.Fatalf("expected sync request record to be stored: %v", err)
	}
	if record.ConnectorID != connector.ID {
		t.Fatalf("connector_id mismatch: %s", record.ConnectorID)
	}
	if !record.AccessApplied {
		t.Fatalf("expected access sync to be applied")
	}
	if record.RawPayloadRef == "" {
		t.Fatalf("expected raw_payload_ref to be stored")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_processed", "enterprise_sync", 10)
	if len(logs) == 0 {
		t.Fatalf("expected webhook processed audit log")
	}
}

func TestReceiveEnterpriseHRISWebhookProcessesTalentaUpdatedEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	seedTalentaUpdatableEmployee(t, s.enterpriseSvc)
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.updated",
		"employee":{
			"employment":{
				"employee_id":"EMP-UPDATED-001",
				"employee_number":"TAL-UPDATED-001",
				"organization_name":"Security",
				"job_position":"Security Lead",
				"branch":"Bandung",
				"join_date":"2024-06-01"
			},
			"personal":{
				"first_name":"Updated",
				"last_name":"User",
				"email":"updated.user@talenta-sync.local",
				"mobile_phone":"+628110000333",
				"avatar":"https://cdn.example.com/photos/updated-user-v2.jpg"
			},
			"leave_info":{
				"status":"approved",
				"type":"Sick Leave"
			},
			"payroll_info":{
				"cost_center_name":"CC-SEC-02"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-updated-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.updated")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount+1 {
		t.Fatalf("expected updated event to modify existing employee only, before=%d after=%d", initialEmployeeCount+1, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-UPDATED-001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected updated employee to be present")
	}
	if syncedEmployee.Department != "Security" {
		t.Fatalf("unexpected department: %s", syncedEmployee.Department)
	}
	if syncedEmployee.JobTitle != "Security Lead" {
		t.Fatalf("unexpected job_title: %s", syncedEmployee.JobTitle)
	}
	if syncedEmployee.Location != "Bandung" {
		t.Fatalf("unexpected location: %s", syncedEmployee.Location)
	}
	if syncedEmployee.JoinDate != "2024-06-01" {
		t.Fatalf("unexpected join_date: %s", syncedEmployee.JoinDate)
	}
	if syncedEmployee.LeaveStatus != "sick_leave" {
		t.Fatalf("unexpected leave_status: %s", syncedEmployee.LeaveStatus)
	}
	if syncedEmployee.CostCenter != "CC-SEC-02" {
		t.Fatalf("unexpected cost_center: %s", syncedEmployee.CostCenter)
	}
	if syncedEmployee.PhotoURL != "https://cdn.example.com/photos/updated-user-v2.jpg" {
		t.Fatalf("unexpected photo_url: %s", syncedEmployee.PhotoURL)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected updated event to upsert one access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "updated.user@talenta-sync.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected updated access user to be present")
	}

	record, err := s.enterpriseSvc.GetSyncRequestRecord("tenant_demo_jakarta", "mekari-evt-hris-updated-001")
	if err != nil {
		t.Fatalf("expected sync request record to be stored: %v", err)
	}
	if record.ConnectorID != connector.ID {
		t.Fatalf("connector_id mismatch: %s", record.ConnectorID)
	}
}

func TestReceiveEnterpriseHRISWebhookProcessesTalentaTransferApprovedEmploymentOnlyEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	seedTalentaLifecycleEmployee(t, s.enterpriseSvc, "EMP-TRANSFER-001", "transfer.user@talenta-sync.local", "Transfer User")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.transfer.approved",
		"old_employment":{
			"employee_id":"EMP-TRANSFER-001",
			"organization_name":"Operations",
			"job_position":"Ops Specialist",
			"branch":"Jakarta"
		},
		"new_employment":{
			"employee_id":"EMP-TRANSFER-001",
			"organization_name":"Security",
			"job_position":"Security Lead",
			"branch":"Bandung",
			"transfer_date":"2026-05-02"
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-transfer-approved-001")
	request.Header.Set("X-Event-Type", "talenta.employee.transfer.approved")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount {
		t.Fatalf("expected transfer approved to update existing employee only, before=%d after=%d", initialEmployeeCount, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-TRANSFER-001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected transfer approved employee to be present")
	}
	if syncedEmployee.Email != "transfer.user@talenta-sync.local" {
		t.Fatalf("expected transfer approved to preserve email, got %s", syncedEmployee.Email)
	}
	if syncedEmployee.Department != "Security" {
		t.Fatalf("unexpected department: %s", syncedEmployee.Department)
	}
	if syncedEmployee.JobTitle != "Security Lead" {
		t.Fatalf("unexpected job_title: %s", syncedEmployee.JobTitle)
	}
	if syncedEmployee.Location != "Bandung" {
		t.Fatalf("unexpected location: %s", syncedEmployee.Location)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("expected transfer approved to preserve leave_status, got %s", syncedEmployee.LeaveStatus)
	}
	if syncedEmployee.CostCenter != "CC-OPS-01" {
		t.Fatalf("expected transfer approved to preserve cost_center, got %s", syncedEmployee.CostCenter)
	}
	if syncedEmployee.PhotoURL != "https://cdn.example.com/photos/transfer-user-v1.jpg" {
		t.Fatalf("expected transfer approved to preserve photo_url, got %s", syncedEmployee.PhotoURL)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected transfer approved to upsert one access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
}

func TestReceiveEnterpriseHRISWebhookProcessesTalentaTransferCancelledEmploymentOnlyEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	seedTalentaTransferredEmployee(t, s.enterpriseSvc)
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.transfer.cancelled",
		"employment":{
			"employee_id":"EMP-TRANSFER-CANCELLED-001",
			"organization_name":"Operations",
			"job_position":"Ops Specialist",
			"branch":"Jakarta",
			"transfer_date":"2026-05-06"
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-transfer-cancelled-001")
	request.Header.Set("X-Event-Type", "talenta.employee.transfer.cancelled")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount {
		t.Fatalf("expected transfer cancelled to update existing employee only, before=%d after=%d", initialEmployeeCount, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-TRANSFER-CANCELLED-001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected transfer cancelled employee to be present")
	}
	if syncedEmployee.Department != "Operations" {
		t.Fatalf("unexpected department: %s", syncedEmployee.Department)
	}
	if syncedEmployee.JobTitle != "Ops Specialist" {
		t.Fatalf("unexpected job_title: %s", syncedEmployee.JobTitle)
	}
	if syncedEmployee.Location != "Jakarta" {
		t.Fatalf("unexpected location: %s", syncedEmployee.Location)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("expected transfer cancelled to preserve leave_status, got %s", syncedEmployee.LeaveStatus)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected transfer cancelled to upsert one access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
}

func TestReceiveEnterpriseHRISWebhookQueuesForAsyncWorkerAndProcessesOnTick(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
			"employment":{
				"employee_id":"EMP-ASYNC-001",
				"employee_number":"TAL-ASYNC-001",
				"organization_name":"IT Division",
				"job_position":"Staff IT",
				"branch":"Pusat",
				"join_date":"2026-04-20"
			},
			"personal":{
				"first_name":"Async",
				"last_name":"Worker",
				"email":"async.worker@talenta-sync.local",
				"mobile_phone":"+628110000222",
				"avatar":"https://cdn.example.com/photos/async-worker.jpg"
			},
			"leave_info":{
				"status":"approved",
				"type":"Annual Leave"
			},
			"payroll_info":{
				"cost_center_name":"CC-ASYNC-01"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-async-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected async path to avoid immediate employee sync")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != initialAccessUserCount {
		t.Fatalf("expected async path to avoid immediate access sync")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}
	queued, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected queued receipt lookup success: %v", err)
	}
	if queued.Status != "received" {
		t.Fatalf("expected queued receipt status received, got %s", queued.Status)
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount+1 {
		t.Fatalf("expected worker tick to sync employee, before=%d after=%d", initialEmployeeCount, len(employees))
	}
	foundEmployee := false
	for i := range employees {
		if employees[i].Email != "async.worker@talenta-sync.local" {
			continue
		}
		foundEmployee = true
		if employees[i].JoinDate != "2026-04-20" {
			t.Fatalf("expected async worker synced join_date 2026-04-20, got %s", employees[i].JoinDate)
		}
		if employees[i].LeaveStatus != "annual_leave" {
			t.Fatalf("expected async worker synced leave_status annual_leave, got %s", employees[i].LeaveStatus)
		}
		if employees[i].CostCenter != "CC-ASYNC-01" {
			t.Fatalf("expected async worker synced cost_center CC-ASYNC-01, got %s", employees[i].CostCenter)
		}
		if employees[i].PhotoURL != "https://cdn.example.com/photos/async-worker.jpg" {
			t.Fatalf("expected async worker synced photo_url, got %s", employees[i].PhotoURL)
		}
	}
	if !foundEmployee {
		t.Fatalf("expected async worker synced employee to be present")
	}
	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected worker tick to sync access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}

	processed, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if processed.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %s", processed.Status)
	}
	if processed.ProcessedAt == nil {
		t.Fatalf("expected processed receipt processed_at to be set")
	}
	if processed.LastError != "" {
		t.Fatalf("expected processed receipt last_error empty, got %s", processed.LastError)
	}
	if len(s.enterpriseSvc.ListPendingHRISWebhookReceipts("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("expected processed receipt to be removed from pending queue")
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerProcessesTalentaUpdatedEvent(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	seedTalentaUpdatableEmployee(t, s.enterpriseSvc)
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.updated",
		"employee":{
			"employment":{
				"employee_id":"EMP-UPDATED-001",
				"employee_number":"TAL-UPDATED-001",
				"organization_name":"Risk",
				"job_position":"Risk Manager",
				"branch":"Surabaya",
				"join_date":"2024-07-01"
			},
			"personal":{
				"first_name":"Updated",
				"last_name":"User",
				"email":"updated.user@talenta-sync.local",
				"mobile_phone":"+628110000444",
				"avatar":"https://cdn.example.com/photos/updated-user-v3.jpg"
			},
			"leave_info":{
				"status":"approved",
				"type":"Personal Leave"
			},
			"payroll_info":{
				"cost_center_name":"CC-RISK-03"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-updated-async-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.updated")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount+1 {
		t.Fatalf("expected async path to avoid immediate employee sync")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount+1 {
		t.Fatalf("expected async updated event to modify existing employee only, before=%d after=%d", initialEmployeeCount+1, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-UPDATED-001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected async updated employee to be present")
	}
	if syncedEmployee.Department != "Risk" {
		t.Fatalf("unexpected department: %s", syncedEmployee.Department)
	}
	if syncedEmployee.JobTitle != "Risk Manager" {
		t.Fatalf("unexpected job_title: %s", syncedEmployee.JobTitle)
	}
	if syncedEmployee.Location != "Surabaya" {
		t.Fatalf("unexpected location: %s", syncedEmployee.Location)
	}
	if syncedEmployee.JoinDate != "2024-07-01" {
		t.Fatalf("unexpected join_date: %s", syncedEmployee.JoinDate)
	}
	if syncedEmployee.LeaveStatus != "personal_leave" {
		t.Fatalf("unexpected leave_status: %s", syncedEmployee.LeaveStatus)
	}
	if syncedEmployee.CostCenter != "CC-RISK-03" {
		t.Fatalf("unexpected cost_center: %s", syncedEmployee.CostCenter)
	}
	if syncedEmployee.PhotoURL != "https://cdn.example.com/photos/updated-user-v3.jpg" {
		t.Fatalf("unexpected photo_url: %s", syncedEmployee.PhotoURL)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected async updated event to upsert one access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}

	processed, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if processed.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %s", processed.Status)
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerProcessesTalentaResignationCreatedEmploymentOnlyEvent(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	seedTalentaLifecycleEmployee(t, s.enterpriseSvc, "EMP-RESIGN-001", "resign.user@talenta-sync.local", "Resign User")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.resignation.created",
		"employment":{
			"employee_id":"EMP-RESIGN-001",
			"organization_name":"Operations",
			"job_position":"Operator",
			"branch":"Jakarta",
			"resign_date":"2026-05-12",
			"status":"resigned"
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-resignation-created-001")
	request.Header.Set("X-Event-Type", "talenta.employee.resignation.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected async resignation path to avoid immediate employee sync")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount {
		t.Fatalf("expected resignation created to update existing employee only, before=%d after=%d", initialEmployeeCount, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-RESIGN-001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected resignation employee to be present")
	}
	if syncedEmployee.Email != "resign.user@talenta-sync.local" {
		t.Fatalf("expected resignation created to preserve email, got %s", syncedEmployee.Email)
	}
	if syncedEmployee.ResignDate != "2026-05-12" {
		t.Fatalf("unexpected resign_date: %s", syncedEmployee.ResignDate)
	}
	if syncedEmployee.EmploymentStatus != "terminated" {
		t.Fatalf("unexpected employment_status: %s", syncedEmployee.EmploymentStatus)
	}
	if syncedEmployee.Status != "inactive" {
		t.Fatalf("unexpected status: %s", syncedEmployee.Status)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("expected resignation created to preserve leave_status, got %s", syncedEmployee.LeaveStatus)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected resignation created to upsert one access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}

	processed, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if processed.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %s", processed.Status)
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerProcessesTalentaResignationCancelledEmploymentOnlyEvent(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	seedTalentaResignedEmployee(t, s.enterpriseSvc)
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.resignation.cancelled",
		"employment":{
			"employee_id":"EMP-RESIGN-CANCELLED-001",
			"organization_name":"Operations",
			"job_position":"Operator",
			"branch":"Jakarta",
			"resign_date":"",
			"status":"active"
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-resignation-cancelled-001")
	request.Header.Set("X-Event-Type", "talenta.employee.resignation.cancelled")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected async resignation cancelled path to avoid immediate employee sync")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	if len(employees) != initialEmployeeCount {
		t.Fatalf("expected resignation cancelled to update existing employee only, before=%d after=%d", initialEmployeeCount, len(employees))
	}
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "EMP-RESIGN-CANCELLED-001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected resignation cancelled employee to be present")
	}
	if syncedEmployee.Email != "resign.cancelled@talenta-sync.local" {
		t.Fatalf("expected resignation cancelled to preserve email, got %s", syncedEmployee.Email)
	}
	if syncedEmployee.ResignDate != "" {
		t.Fatalf("expected resignation cancelled to clear resign_date, got %s", syncedEmployee.ResignDate)
	}
	if syncedEmployee.EmploymentStatus != "active" {
		t.Fatalf("unexpected employment_status: %s", syncedEmployee.EmploymentStatus)
	}
	if syncedEmployee.Status != "active" {
		t.Fatalf("unexpected status: %s", syncedEmployee.Status)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("expected resignation cancelled to preserve leave_status, got %s", syncedEmployee.LeaveStatus)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected resignation cancelled to upsert one access user, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}

	processed, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if processed.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %s", processed.Status)
	}
}

func TestRunQueuedEnterpriseHRISWebhookReceiptExecutionsReenqueuesCooldownExternalQueueCandidate(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_cooldown_queue_001",
		ReceiptID:     "whr_exec_cooldown_queue_001",
		ConnectorID:   "connector-talenta-cooldown-queue",
		Vendor:        "talenta",
		RequestID:     "talenta-exec-cooldown-queue-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}

	retryAt := time.Now().UTC().Add(5 * time.Minute)
	if _, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		"tenant_demo_jakarta",
		execution.ID,
		"processing",
		retryAt,
		nil,
	); err != nil {
		t.Fatalf("requeue queued receipt execution should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookReceiptExecutionQueue, execution.ID); err != nil {
		t.Fatalf("seed receipt execution external queue should succeed: %v", err)
	}

	processed := s.runQueuedEnterpriseHRISWebhookReceiptExecutions(
		10,
		3,
		5*time.Minute,
		15*time.Minute,
		10*time.Minute,
	)
	if processed != 0 {
		t.Fatalf("expected cooldown receipt execution to remain deferred, got processed=%d", processed)
	}
	if queueStore.dequeueCalls != 1 {
		t.Fatalf("expected one external queue dequeue call, got %d", queueStore.dequeueCalls)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected one seed enqueue call, got %d", queueStore.enqueueCalls)
	}
	if queueStore.requeueCalls != 1 {
		t.Fatalf("expected cooldown receipt execution to be requeued once, got %d requeue calls", queueStore.requeueCalls)
	}
	items := queueStore.itemsByQueue[enterpriseHRISWebhookReceiptExecutionQueue]
	if len(items) != 1 || items[0] != execution.ID {
		t.Fatalf("expected cooldown receipt execution to remain in external queue, got %v", items)
	}

	updatedExecution, err := s.enterpriseSvc.GetHRISWebhookExecution("tenant_demo_jakarta", execution.ID)
	if err != nil {
		t.Fatalf("lookup cooldown receipt execution should succeed: %v", err)
	}
	if updatedExecution.Status != enterprise.HRISWebhookExecutionStatusQueued {
		t.Fatalf("expected cooldown receipt execution to stay queued, got %+v", updatedExecution)
	}
	if !updatedExecution.QueuedAt.Equal(retryAt) {
		t.Fatalf("expected cooldown receipt execution queued_at=%s, got %s", retryAt, updatedExecution.QueuedAt)
	}
}

func TestAcknowledgeEnterpriseHRISWebhookExecutionQueueClaimIgnoresStaleClaim(t *testing.T) {
	queueName := enterpriseHRISWebhookReceiptExecutionQueue
	executionID := "whr_exec_stale_ack_001"
	queueStore := &stubWorkerQueueStore{
		indexByQueue: map[string]map[string]struct{}{
			queueName: {
				executionID: {},
			},
		},
		claimsByQueue: map[string]map[string]redistore.WorkerQueueClaim{
			queueName: {
				executionID: {
					ItemID:     executionID,
					ClaimToken: "fresh-claim",
				},
			},
		},
		deadlinesByQueue: map[string]map[string]time.Time{
			queueName: {
				executionID: time.Now().UTC().Add(time.Minute),
			},
		},
	}
	s := &server{workerQueueStore: queueStore}

	applied := s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
		queueName,
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		redistore.WorkerQueueClaim{
			ItemID:     executionID,
			ClaimToken: "stale-claim",
		},
	)
	if applied {
		t.Fatalf("expected stale ack claim to be ignored")
	}
	if queueStore.ackCalls != 1 {
		t.Fatalf("expected one ack call, got %d", queueStore.ackCalls)
	}
	claim, ok := queueStore.claimsByQueue[queueName][executionID]
	if !ok {
		t.Fatalf("expected fresh claim to remain in queue store")
	}
	if claim.ClaimToken != "fresh-claim" {
		t.Fatalf("expected fresh claim token to remain intact, got %+v", claim)
	}
}

func TestRequeueQueuedEnterpriseHRISWebhookExecutionFallsBackToEnqueueOnStaleClaim(t *testing.T) {
	queueName := enterpriseHRISWebhookReceiptExecutionQueue
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: &stubWorkerQueueStore{},
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_stale_requeue_001",
		ReceiptID:     "whr_exec_stale_requeue_001",
		ConnectorID:   "connector-talenta-stale-requeue",
		Vendor:        "talenta",
		RequestID:     "talenta-exec-stale-requeue-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "processing",
	})
	if err != nil {
		t.Fatalf("create queued execution should succeed: %v", err)
	}

	queueStore := s.workerQueueStore.(*stubWorkerQueueStore)
	if err := queueStore.EnqueueWorkerQueue(queueName, execution.ID); err != nil {
		t.Fatalf("seed recovered queued execution should succeed: %v", err)
	}
	queueStore.enqueueCalls = 0
	s.requeueQueuedEnterpriseHRISWebhookExecution(
		enterpriseHRISWebhookQueuedExecution{
			Execution: execution,
			QueueName: queueName,
			QueueClaim: &redistore.WorkerQueueClaim{
				ItemID:     execution.ID,
				ClaimToken: "stale-claim",
			},
		},
		execution,
	)

	if queueStore.requeueCalls != 1 {
		t.Fatalf("expected one stale requeue attempt, got %d", queueStore.requeueCalls)
	}
	if queueStore.enqueueCalls != 1 {
		t.Fatalf("expected stale requeue to fall back to enqueue once, got %d", queueStore.enqueueCalls)
	}
	items := queueStore.itemsByQueue[queueName]
	if len(items) != 1 || items[0] != execution.ID {
		t.Fatalf("expected recovered queued execution to remain queued once, got %v", items)
	}
}

func TestReenqueueEnterpriseHRISWebhookExecutionRepairsExternalQueueIndexDrift(t *testing.T) {
	queueName := enterpriseHRISWebhookReceiptExecutionQueue
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_reenqueue_drift_001",
		ReceiptID:     "whr_exec_reenqueue_drift_001",
		ConnectorID:   "connector-talenta-reenqueue-drift",
		Vendor:        "talenta",
		RequestID:     "talenta-reenqueue-drift-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}
	queueStore.indexByQueue = map[string]map[string]struct{}{
		queueName: {
			execution.ID: {},
		},
	}

	s.reenqueueEnterpriseHRISWebhookExecution(execution)
	s.reenqueueEnterpriseHRISWebhookExecution(execution)

	items := queueStore.itemsByQueue[queueName]
	if len(items) != 1 || items[0] != execution.ID {
		t.Fatalf("expected reenqueue to repair index-only drift without duplicating queue item, got %v", items)
	}
	if _, ok := queueStore.indexByQueue[queueName][execution.ID]; !ok {
		t.Fatalf("expected repaired queue index to retain execution %s", execution.ID)
	}
}

func TestListQueuedEnterpriseHRISWebhookExecutionsRecoversExpiredExternalQueueClaim(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_exec_visibility_recovery_001",
		ReceiptID:     "whr_exec_visibility_recovery_001",
		ConnectorID:   "connector-talenta-visibility-recovery",
		Vendor:        "talenta",
		RequestID:     "talenta-exec-visibility-recovery-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}
	if err := queueStore.EnqueueWorkerQueue(enterpriseHRISWebhookReceiptExecutionQueue, execution.ID); err != nil {
		t.Fatalf("seed receipt execution external queue should succeed: %v", err)
	}

	first := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		1,
		20*time.Millisecond,
		time.Now().UTC(),
	)
	if len(first) != 1 || first[0].Execution.ID != execution.ID {
		t.Fatalf("expected first claim to return queued execution, got %+v", first)
	}
	if first[0].QueueClaim == nil {
		t.Fatalf("expected first claim to carry external queue claim metadata")
	}
	if len(queueStore.claimsByQueue[enterpriseHRISWebhookReceiptExecutionQueue]) != 1 {
		t.Fatalf("expected one in-flight queue claim, got %+v", queueStore.claimsByQueue[enterpriseHRISWebhookReceiptExecutionQueue])
	}

	time.Sleep(30 * time.Millisecond)

	second := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		1,
		20*time.Millisecond,
		time.Now().UTC(),
	)
	if len(second) != 1 || second[0].Execution.ID != execution.ID {
		t.Fatalf("expected expired queue claim to be recovered, got %+v", second)
	}
	if second[0].QueueClaim == nil {
		t.Fatalf("expected recovered execution to carry refreshed queue claim metadata")
	}
	if second[0].QueueClaim.ClaimToken == first[0].QueueClaim.ClaimToken {
		t.Fatalf("expected recovered queue claim to rotate claim token")
	}
}

func TestListQueuedEnterpriseHRISWebhookExecutionsDeduplicatesDuplicateExternalQueueItems(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_receipt_duplicate_claim_001",
		ReceiptID:     "whr_receipt_duplicate_claim_001",
		ConnectorID:   "connector-talenta-duplicate-claim",
		Vendor:        "talenta",
		RequestID:     "talenta-duplicate-claim-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}
	queueStore.itemsByQueue = map[string][]string{
		enterpriseHRISWebhookReceiptExecutionQueue: {
			execution.ID,
			execution.ID,
		},
	}

	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		10,
		5*time.Minute,
		time.Now().UTC(),
	)
	if len(items) != 1 || items[0].Execution.ID != execution.ID {
		t.Fatalf("expected duplicate queue entries to collapse into one claimed execution, got %+v", items)
	}
	if items[0].QueueClaim == nil {
		t.Fatalf("expected claimed execution to carry queue claim metadata")
	}
	if len(queueStore.claimsByQueue[enterpriseHRISWebhookReceiptExecutionQueue]) != 1 {
		t.Fatalf("expected one active claim after duplicate compression, got %+v", queueStore.claimsByQueue[enterpriseHRISWebhookReceiptExecutionQueue])
	}
	if len(queueStore.itemsByQueue[enterpriseHRISWebhookReceiptExecutionQueue]) != 0 {
		t.Fatalf("expected duplicate pending entries to be compacted after claim, got %+v", queueStore.itemsByQueue[enterpriseHRISWebhookReceiptExecutionQueue])
	}
}

func TestListQueuedEnterpriseHRISWebhookExecutionsSkipsIndexedFallbackWhenExternalQueueClaimed(t *testing.T) {
	now := time.Now().UTC()
	queueStore := &stubWorkerQueueStore{
		claimsByQueue: map[string]map[string]redistore.WorkerQueueClaim{
			enterpriseHRISWebhookReceiptExecutionQueue: {
				"hwe_receipt_claimed_elsewhere_001": {
					ItemID:     "hwe_receipt_claimed_elsewhere_001",
					ClaimToken: "active-claim-token",
				},
			},
		},
		deadlinesByQueue: map[string]map[string]time.Time{
			enterpriseHRISWebhookReceiptExecutionQueue: {
				"hwe_receipt_claimed_elsewhere_001": now.Add(5 * time.Minute),
			},
		},
	}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	if _, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_receipt_claimed_elsewhere_001",
		ReceiptID:     "whr_receipt_claimed_elsewhere_001",
		ConnectorID:   "connector-talenta-claimed-elsewhere",
		Vendor:        "talenta",
		RequestID:     "talenta-receipt-claimed-elsewhere-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	}); err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}

	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		10,
		5*time.Minute,
		now,
	)
	if len(items) != 0 {
		t.Fatalf("expected claimed external queue receipt execution to be skipped from indexed fallback, got %+v", items)
	}
}

func TestListQueuedEnterpriseHRISWebhookDLQExecutionsSkipsIndexedFallbackWhenExternalQueueClaimed(t *testing.T) {
	now := time.Now().UTC()
	queueStore := &stubWorkerQueueStore{
		claimsByQueue: map[string]map[string]redistore.WorkerQueueClaim{
			enterpriseHRISWebhookDLQExecutionQueue: {
				"hwe_dlq_claimed_elsewhere_001": {
					ItemID:     "hwe_dlq_claimed_elsewhere_001",
					ClaimToken: "active-claim-token",
				},
			},
		},
		deadlinesByQueue: map[string]map[string]time.Time{
			enterpriseHRISWebhookDLQExecutionQueue: {
				"hwe_dlq_claimed_elsewhere_001": now.Add(5 * time.Minute),
			},
		},
	}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	if _, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      "hdlq_claimed_elsewhere_001",
		ReceiptID:     "whr_dlq_claimed_elsewhere_001",
		ConnectorID:   "connector-talenta-dlq-claimed-elsewhere",
		Vendor:        "talenta",
		RequestID:     "talenta-dlq-claimed-elsewhere-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "merge",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "replaying",
	}); err != nil {
		t.Fatalf("create queued dlq execution should succeed: %v", err)
	}

	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindDLQReplay,
		10,
		5*time.Minute,
		now,
	)
	if len(items) != 0 {
		t.Fatalf("expected claimed external queue dlq execution to be skipped from indexed fallback, got %+v", items)
	}
}

func TestListQueuedEnterpriseHRISWebhookExecutionsTreatsIndexOnlyExternalQueueDriftAsMissing(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindReceiptProcess,
		TargetID:      "whr_receipt_index_only_drift_001",
		ReceiptID:     "whr_receipt_index_only_drift_001",
		ConnectorID:   "connector-talenta-index-only-drift",
		Vendor:        "talenta",
		RequestID:     "talenta-index-only-drift-001",
		EventType:     "talenta.employee.detail.updated",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "received",
	})
	if err != nil {
		t.Fatalf("create queued receipt execution should succeed: %v", err)
	}
	queueStore.indexByQueue = map[string]map[string]struct{}{
		enterpriseHRISWebhookReceiptExecutionQueue: {
			execution.ID: {},
		},
	}

	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		10,
		5*time.Minute,
		time.Now().UTC(),
	)
	if len(items) != 1 || items[0].Execution.ID != execution.ID {
		t.Fatalf("expected indexed drifted receipt execution to remain claimable via fallback, got %+v", items)
	}
	if items[0].QueueClaim != nil {
		t.Fatalf("expected indexed drifted receipt execution to come from fallback without queue claim, got %+v", items[0])
	}
}

func TestListQueuedEnterpriseHRISWebhookDLQExecutionsTreatsIndexOnlyExternalQueueDriftAsMissing(t *testing.T) {
	queueStore := &stubWorkerQueueStore{}
	s := &server{
		enterpriseSvc:    enterprise.NewService(),
		workerQueueStore: queueStore,
	}

	execution, err := s.enterpriseSvc.CreateHRISWebhookExecution(enterprise.HRISWebhookExecutionInput{
		TenantID:      "tenant_demo_jakarta",
		Kind:          enterprise.HRISWebhookExecutionKindDLQReplay,
		TargetID:      "hdlq_index_only_drift_001",
		ReceiptID:     "whr_dlq_index_only_drift_001",
		ConnectorID:   "connector-talenta-dlq-index-only-drift",
		Vendor:        "talenta",
		RequestID:     "talenta-dlq-index-only-drift-001",
		EventType:     "talenta.employee.detail.updated",
		FailureStage:  "merge",
		ExecutionMode: "queued",
		DispatchMode:  enterprise.HRISWebhookExecutionDispatchModeWorkerTick,
		TargetStatus:  "replaying",
	})
	if err != nil {
		t.Fatalf("create queued dlq execution should succeed: %v", err)
	}
	queueStore.indexByQueue = map[string]map[string]struct{}{
		enterpriseHRISWebhookDLQExecutionQueue: {
			execution.ID: {},
		},
	}

	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindDLQReplay,
		10,
		5*time.Minute,
		time.Now().UTC(),
	)
	if len(items) != 1 || items[0].Execution.ID != execution.ID {
		t.Fatalf("expected indexed drifted dlq execution to remain claimable via fallback, got %+v", items)
	}
	if items[0].QueueClaim != nil {
		t.Fatalf("expected indexed drifted dlq execution to come from fallback without queue claim, got %+v", items[0])
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerTickWithLeaseRunsWhenAcquired(t *testing.T) {
	leaseStore := &stubWorkerLeaseStore{acquireOK: true}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerLeaseStore:       leaseStore,
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
			"employment":{
				"employee_id":"EMP-LEASE-001",
				"employee_number":"TAL-LEASE-001",
				"organization_name":"IT Division",
				"job_position":"Staff IT",
				"branch":"Pusat",
				"join_date":"2026-04-20"
			},
			"personal":{
				"first_name":"Lease",
				"last_name":"Worker",
				"email":"lease.worker@talenta-sync.local",
				"mobile_phone":"+628110000333"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-lease-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTickWithLease(10, 5, 30*time.Second, time.Minute, 3, 10*time.Minute)

	processed, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if processed.Status != "processed" {
		t.Fatalf("expected leased receipt worker to process receipt, got %s", processed.Status)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount+1 {
		t.Fatalf("expected leased receipt worker to sync employee")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != initialAccessUserCount+1 {
		t.Fatalf("expected leased receipt worker to sync access user")
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 1 {
		t.Fatalf("expected one lease acquire/release, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseHRISWebhookReceiptLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	if leaseStore.lastTTL != 10*time.Minute {
		t.Fatalf("unexpected lease ttl: %s", leaseStore.lastTTL)
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerTickWithLeaseSkipsWhenUnavailable(t *testing.T) {
	leaseStore := &stubWorkerLeaseStore{acquireOK: false}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
		workerLeaseStore:       leaseStore,
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
			"employment":{
				"employee_id":"EMP-LEASE-SKIP-001",
				"employee_number":"TAL-LEASE-SKIP-001",
				"organization_name":"IT Division",
				"job_position":"Staff IT",
				"branch":"Pusat",
				"join_date":"2026-04-20"
			},
			"personal":{
				"first_name":"Lease",
				"last_name":"Skip",
				"email":"lease.skip@talenta-sync.local",
				"mobile_phone":"+628110000444"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-lease-skip-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTickWithLease(10, 5, 30*time.Second, time.Minute, 3, 10*time.Minute)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected queued receipt lookup success: %v", err)
	}
	if record.Status != "received" {
		t.Fatalf("expected lease miss to preserve queued receipt, got %s", record.Status)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected lease miss to avoid syncing employee")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != initialAccessUserCount {
		t.Fatalf("expected lease miss to avoid syncing access user")
	}
	if leaseStore.acquireCalls != 1 || leaseStore.releaseCalls != 0 {
		t.Fatalf("expected one lease acquire and no release on lease miss, got acquire=%d release=%d", leaseStore.acquireCalls, leaseStore.releaseCalls)
	}
	if leaseStore.lastKey != enterpriseHRISWebhookReceiptLeaseKey {
		t.Fatalf("unexpected lease key: %s", leaseStore.lastKey)
	}
	if leaseStore.lastTTL != 10*time.Minute {
		t.Fatalf("unexpected lease ttl: %s", leaseStore.lastTTL)
	}
}

func TestReceiveEnterpriseHRISWebhookQueuesForAsyncWorkerAndMarksReceiptFailed(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-evt-async-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 0 {
		t.Fatalf("expected queue path to avoid immediate dlq append")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 30*time.Second, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after worker failure: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected dlq receipt status after final handoff, got %s", record.Status)
	}
	if record.ProcessedAt == nil {
		t.Fatalf("expected failed receipt processed_at to be set")
	}
	if !strings.Contains(record.LastError, "forced webhook normalization failure") {
		t.Fatalf("unexpected failed receipt last_error: %s", record.LastError)
	}

	entries := s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)
	if len(entries) != 1 {
		t.Fatalf("expected one dlq entry after worker failure, got %d", len(entries))
	}
	if entries[0].ReceiptID != response.ReceiptID {
		t.Fatalf("expected dlq receipt_id %s, got %s", response.ReceiptID, entries[0].ReceiptID)
	}
	if entries[0].FailureStage != "normalize" {
		t.Fatalf("expected normalize failure_stage, got %s", entries[0].FailureStage)
	}

	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) == 0 {
		t.Fatalf("expected receipt worker alert audit log")
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerRetriesAfterCooldown(t *testing.T) {
	normalizer := &flakyGadjianNormalizer{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "retry-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-RETRY-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-evt-retry-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 3, 20*time.Millisecond, time.Minute, 2)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected failed receipt lookup success after first attempt: %v", err)
	}
	if record.Status != "failed" {
		t.Fatalf("expected failed receipt status after first attempt, got %s", record.Status)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1 after first attempt, got %d", record.AttemptCount)
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 0 {
		t.Fatalf("expected intermediate failure to stay in receipt queue before DLQ")
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 3, time.Hour, time.Minute, 2)

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success during cooldown: %v", err)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected cooldown tick to avoid incrementing attempt_count, got %d", record.AttemptCount)
	}
	if normalizer.calls != 1 {
		t.Fatalf("expected cooldown tick to skip second normalize attempt, calls=%d", normalizer.calls)
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 0 {
		t.Fatalf("expected cooldown receipt to stay out of worker alerts once it is not claimable, got %d", len(alertLogs))
	}

	time.Sleep(25 * time.Millisecond)
	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 3, 20*time.Millisecond, time.Minute, 2)

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success after retry: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected processed receipt status after retry, got %s", record.Status)
	}
	if record.AttemptCount != 2 {
		t.Fatalf("expected second attempt to increment attempt_count to 2, got %d", record.AttemptCount)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != 1 {
		t.Fatalf("expected retry path to sync one enterprise employee")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != 1 {
		t.Fatalf("expected retry path to sync one access user")
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 0 {
		t.Fatalf("expected successful retry to avoid DLQ entries")
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerWithRetryBackoffHonorsMaxBackoff(t *testing.T) {
	normalizer := &failNTimesGadjianNormalizer{failUntil: 2}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "replay-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-RECEIPT-BACKOFF-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-receipt-backoff-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	baseBackoff := 25 * time.Millisecond
	maxBackoff := 100 * time.Millisecond
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup after first failure: %v", err)
	}
	if record.Status != "failed" || record.AttemptCount != 1 {
		t.Fatalf("unexpected receipt after first failure: %+v", record)
	}
	if normalizer.calls != 1 {
		t.Fatalf("expected one normalizer call after first attempt, got %d", normalizer.calls)
	}

	time.Sleep(baseBackoff + 10*time.Millisecond)
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup after second failure: %v", err)
	}
	if record.Status != "failed" || record.AttemptCount != 2 {
		t.Fatalf("unexpected receipt after second failure: %+v", record)
	}
	if normalizer.calls != 2 {
		t.Fatalf("expected two normalizer calls after second attempt, got %d", normalizer.calls)
	}

	time.Sleep(baseBackoff + 10*time.Millisecond)
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup during exponential cooldown: %v", err)
	}
	if record.AttemptCount != 2 {
		t.Fatalf("expected exponential cooldown to preserve attempt_count=2, got %d", record.AttemptCount)
	}
	if normalizer.calls != 2 {
		t.Fatalf("expected exponential cooldown to skip third attempt, calls=%d", normalizer.calls)
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 2 {
		t.Fatalf("expected exponential cooldown tick to avoid appending a new receipt worker alert, got %d", len(alertLogs))
	}

	time.Sleep(baseBackoff)
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(10, 5, baseBackoff, maxBackoff, time.Minute, 1)

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup after exponential cooldown expiry: %v", err)
	}
	if record.Status != "processed" || record.AttemptCount != 3 {
		t.Fatalf("unexpected receipt after exponential cooldown expiry: %+v", record)
	}
	if normalizer.calls != 3 {
		t.Fatalf("expected third processing attempt after exponential cooldown expiry, calls=%d", normalizer.calls)
	}
	foundEmployee := false
	for _, employee := range s.enterpriseSvc.ListEmployees("tenant_demo_jakarta") {
		if employee.Email == "fail.ntimes@replay-sync.local" {
			foundEmployee = true
			break
		}
	}
	if !foundEmployee {
		t.Fatalf("expected exponential retry path to sync fail.ntimes@replay-sync.local")
	}
	foundAccessUser := false
	for _, user := range s.accessSvc.ListUsers("tenant_demo_jakarta") {
		if user.Email == "fail.ntimes@replay-sync.local" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected exponential retry path to sync access user fail.ntimes@replay-sync.local")
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerSkipsAttemptLimit(t *testing.T) {
	normalizer := &alwaysFailGadjianNormalizer{}
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(normalizer),
	}

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"gadjian",
		"active",
		"webhook",
		"",
		"",
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"employee.updated","employee":{"id":"GADJIAN-EMP-LIMIT-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "gadjian-evt-limit-001")
	request = withURLParam(request, "connectorID", connector.ID)
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 0, time.Minute, 1)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected failed receipt lookup success after first attempt: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected dlq receipt status after final handoff, got %s", record.Status)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1 after failed final attempt, got %d", record.AttemptCount)
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 1 {
		t.Fatalf("expected final failed attempt to create one dlq entry")
	}
	if normalizer.calls != 1 {
		t.Fatalf("expected one normalize attempt before reaching limit, got %d", normalizer.calls)
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 1, 0, time.Minute, 1)

	record, err = s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt lookup success after exhausted retry: %v", err)
	}
	if record.Status != "dlq" {
		t.Fatalf("expected exhausted receipt to stay in dlq status, got %s", record.Status)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected exhausted retry tick to keep attempt_count=1, got %d", record.AttemptCount)
	}
	if normalizer.calls != 1 {
		t.Fatalf("expected exhausted retry tick to avoid reprocessing, got calls=%d", normalizer.calls)
	}
	if len(s.enterpriseSvc.ListRetryableHRISWebhookReceipts("tenant_demo_jakarta", 10)) != 0 {
		t.Fatalf("expected exhausted receipt to be removed from retryable queue after dlq handoff")
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 1 {
		t.Fatalf("expected exhausted retry tick to avoid duplicate dlq entries")
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 1 {
		t.Fatalf("expected exhausted receipt to stop producing extra receipt worker alerts, got %d", len(alertLogs))
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerRecoversStaleProcessingReceipt(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
			"employment":{
				"employee_id":"EMP-PROCESSING-RECOVER-001",
				"employee_number":"TAL-PROCESSING-RECOVER-001",
				"organization_name":"IT Division",
				"job_position":"Staff IT",
				"branch":"Pusat",
				"join_date":"2026-04-20"
			},
			"personal":{
				"first_name":"Recover",
				"last_name":"Processing",
				"email":"recover.processing@talenta-sync.local",
				"mobile_phone":"+628110000555"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-processing-recover-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	started, err := s.enterpriseSvc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt mark started to succeed: %v", err)
	}
	if started.Status != "processing" {
		t.Fatalf("expected started receipt to be processing, got %s", started.Status)
	}

	time.Sleep(25 * time.Millisecond)
	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, 20*time.Millisecond, 3)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected recovered receipt lookup success: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected stale processing receipt to be recovered and processed, got %s", record.Status)
	}
	if record.AttemptCount != 2 {
		t.Fatalf("expected recovered processing receipt attempt_count=2, got %d", record.AttemptCount)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount+1 {
		t.Fatalf("expected recovered processing receipt to sync employee")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != initialAccessUserCount+1 {
		t.Fatalf("expected recovered processing receipt to sync access user")
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerSkipsFreshProcessingReceipt(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	_, err := s.enterpriseSvc.CreateDomainMapping("tenant_demo_jakarta", "talenta-sync.local", "active")
	if err != nil {
		t.Fatalf("create domain mapping should succeed: %v", err)
	}
	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.employee.detail.created",
		"employee":{
			"employment":{
				"employee_id":"EMP-PROCESSING-FRESH-001",
				"employee_number":"TAL-PROCESSING-FRESH-001",
				"organization_name":"IT Division",
				"job_position":"Staff IT",
				"branch":"Pusat",
				"join_date":"2026-04-20"
			},
			"personal":{
				"first_name":"Fresh",
				"last_name":"Processing",
				"email":"fresh.processing@talenta-sync.local",
				"mobile_phone":"+628110000556"
			}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-processing-fresh-001")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	started, err := s.enterpriseSvc.MarkHRISWebhookReceiptStarted("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected receipt mark started to succeed: %v", err)
	}
	if started.Status != "processing" {
		t.Fatalf("expected started receipt to be processing, got %s", started.Status)
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Hour, 3)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected fresh processing receipt lookup success: %v", err)
	}
	if record.Status != "processing" {
		t.Fatalf("expected fresh processing receipt to remain processing, got %s", record.Status)
	}
	if record.AttemptCount != 1 {
		t.Fatalf("expected fresh processing receipt attempt_count=1, got %d", record.AttemptCount)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected fresh processing receipt to avoid syncing employee")
	}
	if len(s.accessSvc.ListUsers("tenant_demo_jakarta")) != initialAccessUserCount {
		t.Fatalf("expected fresh processing receipt to avoid syncing access user")
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 0 {
		t.Fatalf("expected fresh processing skip to avoid receipt worker alert audit, got %d", len(alertLogs))
	}
}

func TestReceiveEnterpriseHRISWebhookProcessesTalentaShiftChangeEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	seedTalentaSparseMergeEmployee(t, s.enterpriseSvc)

	body := `{
		"event_type":"talenta.attendance.scheduler.changeshift",
		"changes":[
			{
				"employee_id":"hris-jkt-1001",
				"employee_name":"Arief Putra",
				"change_date":"2026-04-22",
				"new_shift":{
					"name":"SHIFT-B",
					"schedule_in":"10:00",
					"schedule_out":"19:00"
				}
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-shift-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeshift")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected sparse shift event to update existing employee only")
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "hris-jkt-1001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected seeded employee to remain present")
	}
	if syncedEmployee.Email != "arief.putra@sudirman.co" {
		t.Fatalf("expected sparse webhook to preserve existing email, got %s", syncedEmployee.Email)
	}
	if syncedEmployee.ShiftCode != "SHIFT-B" {
		t.Fatalf("shift_code mismatch: %s", syncedEmployee.ShiftCode)
	}
	if syncedEmployee.ScheduleWindow != "10:00-19:00" {
		t.Fatalf("schedule_window mismatch: %s", syncedEmployee.ScheduleWindow)
	}
	if syncedEmployee.JoinDate != "2024-01-15" {
		t.Fatalf("expected sparse webhook to preserve join_date, got %s", syncedEmployee.JoinDate)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("expected sparse webhook to preserve leave_status, got %s", syncedEmployee.LeaveStatus)
	}
	if syncedEmployee.CostCenter != "CC-FIN-01" {
		t.Fatalf("expected sparse webhook to preserve cost_center, got %s", syncedEmployee.CostCenter)
	}
	if syncedEmployee.PhotoURL != "https://cdn.example.com/photos/arief-putra.jpg" {
		t.Fatalf("expected sparse webhook to preserve photo_url, got %s", syncedEmployee.PhotoURL)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected access user count to increase by 1, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "arief.putra@sudirman.co" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected merged employee access user to be upserted")
	}

	record, err := s.enterpriseSvc.GetSyncRequestRecord("tenant_demo_jakarta", "mekari-evt-hris-shift-001")
	if err != nil {
		t.Fatalf("expected sync request record to be stored: %v", err)
	}
	if record.Result.Job.Updated != 1 || record.Result.Job.Created != 0 {
		t.Fatalf("unexpected sync counters: created=%d updated=%d", record.Result.Job.Created, record.Result.Job.Updated)
	}
}

func TestReceiveEnterpriseHRISWebhookProcessesTalentaScheduleChangeEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	seedTalentaSparseMergeEmployee(t, s.enterpriseSvc)

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"hris-jkt-1001",
				"full_name":"Arief Putra",
				"shifts":[
					{
						"date":"2026-04-24",
						"name":"SHIFT-C",
						"schedule_in":"08:00",
						"schedule_out":"17:00"
					},
					{
						"date":"2026-04-25",
						"name":"SHIFT-C",
						"schedule_in":"08:00",
						"schedule_out":"17:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-schedule-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected sparse schedule event to update existing employee only")
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "hris-jkt-1001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected seeded employee to remain present")
	}
	if syncedEmployee.Email != "arief.putra@sudirman.co" {
		t.Fatalf("expected sparse webhook to preserve existing email, got %s", syncedEmployee.Email)
	}
	if syncedEmployee.ShiftCode != "SHIFT-C" {
		t.Fatalf("shift_code mismatch: %s", syncedEmployee.ShiftCode)
	}
	if syncedEmployee.ScheduleWindow != "2026-04-24:08:00-17:00;2026-04-25:08:00-17:00" {
		t.Fatalf("schedule_window mismatch: %s", syncedEmployee.ScheduleWindow)
	}
	if syncedEmployee.JoinDate != "2024-01-15" {
		t.Fatalf("expected sparse webhook to preserve join_date, got %s", syncedEmployee.JoinDate)
	}
	if syncedEmployee.LeaveStatus != "annual_leave" {
		t.Fatalf("expected sparse webhook to preserve leave_status, got %s", syncedEmployee.LeaveStatus)
	}
	if syncedEmployee.CostCenter != "CC-FIN-01" {
		t.Fatalf("expected sparse webhook to preserve cost_center, got %s", syncedEmployee.CostCenter)
	}
	if syncedEmployee.PhotoURL != "https://cdn.example.com/photos/arief-putra.jpg" {
		t.Fatalf("expected sparse webhook to preserve photo_url, got %s", syncedEmployee.PhotoURL)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected access user count to increase by 1, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
	foundAccessUser := false
	for i := range accessUsers {
		if accessUsers[i].Email == "arief.putra@sudirman.co" {
			foundAccessUser = true
			break
		}
	}
	if !foundAccessUser {
		t.Fatalf("expected merged employee access user to be upserted")
	}

	record, err := s.enterpriseSvc.GetSyncRequestRecord("tenant_demo_jakarta", "mekari-evt-hris-schedule-001")
	if err != nil {
		t.Fatalf("expected sync request record to be stored: %v", err)
	}
	if record.ConnectorID != connector.ID {
		t.Fatalf("connector_id mismatch: %s", record.ConnectorID)
	}
	if record.Result.Job.Updated != 1 || record.Result.Job.Created != 0 {
		t.Fatalf("unexpected sync counters: created=%d updated=%d", record.Result.Job.Created, record.Result.Job.Updated)
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerProcessesTalentaShiftChangeEvent(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	seedTalentaSparseMergeEmployee(t, s.enterpriseSvc)

	body := `{
		"event_type":"talenta.attendance.scheduler.changeshift",
		"changes":[
			{
				"employee_id":"hris-jkt-1001",
				"employee_name":"Arief Putra",
				"change_date":"2026-04-22",
				"new_shift":{
					"name":"SHIFT-B",
					"schedule_in":"10:00",
					"schedule_out":"19:00"
				}
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-shift-worker-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeshift")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected async sparse shift event to stay queued before worker sync")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %s", record.Status)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "hris-jkt-1001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected seeded employee to remain present")
	}
	if syncedEmployee.ShiftCode != "SHIFT-B" {
		t.Fatalf("shift_code mismatch: %s", syncedEmployee.ShiftCode)
	}
	if syncedEmployee.ScheduleWindow != "10:00-19:00" {
		t.Fatalf("schedule_window mismatch: %s", syncedEmployee.ScheduleWindow)
	}
	if syncedEmployee.JoinDate != "2024-01-15" ||
		syncedEmployee.LeaveStatus != "annual_leave" ||
		syncedEmployee.CostCenter != "CC-FIN-01" ||
		syncedEmployee.PhotoURL != "https://cdn.example.com/photos/arief-putra.jpg" {
		t.Fatalf("expected async sparse shift merge to preserve extended fields, got %+v", syncedEmployee)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected access user count to increase by 1, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerProcessesTalentaScheduleChangeEvent(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))
	initialAccessUserCount := len(s.accessSvc.ListUsers("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}
	seedTalentaSparseMergeEmployee(t, s.enterpriseSvc)

	body := `{
		"event_type":"talenta.attendance.scheduler.changeschedule",
		"changes":[
			{
				"employee_id":"hris-jkt-1001",
				"full_name":"Arief Putra",
				"shifts":[
					{
						"date":"2026-04-24",
						"name":"SHIFT-C",
						"schedule_in":"08:00",
						"schedule_out":"17:00"
					},
					{
						"date":"2026-04-25",
						"name":"SHIFT-C",
						"schedule_in":"08:00",
						"schedule_out":"17:00"
					}
				]
			}
		]
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "mekari-evt-hris-schedule-worker-001")
	request.Header.Set("X-Event-Type", "talenta.attendance.scheduler.changeschedule")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected async sparse schedule event to stay queued before worker sync")
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected processed receipt lookup success: %v", err)
	}
	if record.Status != "processed" {
		t.Fatalf("expected processed receipt status, got %s", record.Status)
	}

	employees := s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")
	var syncedEmployee enterprise.EnterpriseEmployee
	foundEmployee := false
	for i := range employees {
		if employees[i].ExternalID != "hris-jkt-1001" {
			continue
		}
		syncedEmployee = employees[i]
		foundEmployee = true
		break
	}
	if !foundEmployee {
		t.Fatalf("expected seeded employee to remain present")
	}
	if syncedEmployee.ShiftCode != "SHIFT-C" {
		t.Fatalf("shift_code mismatch: %s", syncedEmployee.ShiftCode)
	}
	if syncedEmployee.ScheduleWindow != "2026-04-24:08:00-17:00;2026-04-25:08:00-17:00" {
		t.Fatalf("schedule_window mismatch: %s", syncedEmployee.ScheduleWindow)
	}
	if syncedEmployee.JoinDate != "2024-01-15" ||
		syncedEmployee.LeaveStatus != "annual_leave" ||
		syncedEmployee.CostCenter != "CC-FIN-01" ||
		syncedEmployee.PhotoURL != "https://cdn.example.com/photos/arief-putra.jpg" {
		t.Fatalf("expected async sparse schedule merge to preserve extended fields, got %+v", syncedEmployee)
	}

	accessUsers := s.accessSvc.ListUsers("tenant_demo_jakarta")
	if len(accessUsers) != initialAccessUserCount+1 {
		t.Fatalf("expected access user count to increase by 1, before=%d after=%d", initialAccessUserCount, len(accessUsers))
	}
}

func TestReceiveEnterpriseHRISWebhookDefersTalentaLiveAttendanceEvent(t *testing.T) {
	s := &server{
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.liveattendance",
		"employee":{
			"employment":{"employee_id":"EMP-ATT-001"},
			"personal":{"email":"attendance.user@sudirman.co"}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Event-Type", "talenta.attendance.liveattendance")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected deferred event to skip employee sync")
	}

	receipts := s.enterpriseSvc.ListHRISWebhookReceipts("tenant_demo_jakarta", connector.ID, 10)
	if len(receipts) != 1 {
		t.Fatalf("expected one receipt, got %d", len(receipts))
	}
	if receipts[0].Status != "skipped" {
		t.Fatalf("expected deferred event receipt status skipped, got %s", receipts[0].Status)
	}
	if receipts[0].LastError != hris.ErrDeferredWebhookEvent.Error() {
		t.Fatalf("expected deferred reason %q, got %q", hris.ErrDeferredWebhookEvent.Error(), receipts[0].LastError)
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_processing_deferred", "enterprise_webhook", 10)
	if len(logs) == 0 {
		t.Fatalf("expected webhook deferred audit log")
	}
}

func TestReceiveEnterpriseHRISWebhookReceiptWorkerTickDefersTalentaLiveAttendanceEvent(t *testing.T) {
	s := &server{
		cfg: config.Config{
			EnterpriseHRISWebhookReceiptWorkerEnabled: true,
		},
		enterpriseSvc:          enterprise.NewService(),
		accessSvc:              access.NewService(),
		auditSvc:               audit.NewService(),
		hrisVaultSvc:           hris.NewVaultService("vault-master-key-001"),
		hrisDLQSvc:             hris.NewDLQService(),
		hrisNormalizerRegistry: hris.NewRegistry(talenta.NewNormalizer()),
	}
	credentialRef, webhookSecretRef, clientID, clientSecret := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")
	initialEmployeeCount := len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta"))

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{
		"event_type":"talenta.attendance.liveattendance",
		"employee":{
			"employment":{"employee_id":"EMP-ATT-WORKER-001"},
			"personal":{"email":"attendance.worker@sudirman.co"}
		}
	}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Event-Type", "talenta.attendance.liveattendance")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, clientSecret, time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid webhook ack json: %v body=%s", err, recorder.Body.String())
	}

	s.runEnterpriseHRISWebhookReceiptWorkerTick(10, 5, 30*time.Second, time.Minute, 3)

	record, err := s.enterpriseSvc.GetHRISWebhookReceipt("tenant_demo_jakarta", response.ReceiptID)
	if err != nil {
		t.Fatalf("expected deferred receipt lookup success: %v", err)
	}
	if record.Status != "skipped" {
		t.Fatalf("expected deferred worker event receipt status skipped, got %s", record.Status)
	}
	if record.LastError != hris.ErrDeferredWebhookEvent.Error() {
		t.Fatalf("expected deferred reason %q, got %q", hris.ErrDeferredWebhookEvent.Error(), record.LastError)
	}
	if len(s.enterpriseSvc.ListEmployees("tenant_demo_jakarta")) != initialEmployeeCount {
		t.Fatalf("expected deferred worker event to avoid employee sync")
	}
	if len(s.hrisDLQSvc.ListEntries("tenant_demo_jakarta", connector.ID, 10)) != 0 {
		t.Fatalf("expected deferred worker event to avoid dlq append")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_processing_deferred", "enterprise_webhook", 10)
	if len(logs) == 0 {
		t.Fatalf("expected deferred worker audit log")
	}
	alertLogs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_receipt_worker_alert", "enterprise_sync_worker", 10)
	if len(alertLogs) != 0 {
		t.Fatalf("expected no receipt worker alert for deferred event, got %d", len(alertLogs))
	}
}

func TestReceiveEnterpriseHRISWebhookRejectsInvalidTalentaSignature(t *testing.T) {
	s := &server{
		enterpriseSvc: enterprise.NewService(),
		auditSvc:      audit.NewService(),
		hrisVaultSvc:  hris.NewVaultService("vault-master-key-001"),
	}
	credentialRef, webhookSecretRef, clientID, _ := seedTalentaWebhookSecrets(t, s.hrisVaultSvc, "tenant_demo_jakarta")

	connector, err := s.enterpriseSvc.CreateHRISConnector(
		"tenant_demo_jakarta",
		"talenta",
		"active",
		"webhook",
		credentialRef,
		webhookSecretRef,
		"qa",
	)
	if err != nil {
		t.Fatalf("create connector should succeed: %v", err)
	}

	body := `{"event_type":"talenta.employee.detail.created","employee":{"id":"EMP-001"}}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/enterprise/hris-webhook/"+connector.ID,
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Event-Type", "talenta.employee.detail.created")
	request = withURLParam(request, "connectorID", connector.ID)
	applyTalentaWebhookSignature(request, body, clientID, "wrong-secret", time.Now().UTC())
	recorder := httptest.NewRecorder()

	s.receiveEnterpriseHRISWebhook(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(s.enterpriseSvc.ListHRISWebhookReceipts("tenant_demo_jakarta", connector.ID, 10)) != 0 {
		t.Fatalf("expected rejected webhook to avoid receipt persistence")
	}

	logs := s.auditSvc.ListFiltered("tenant_demo_jakarta", "enterprise_hris_webhook_rejected", "enterprise_webhook", 10)
	if len(logs) == 0 {
		t.Fatalf("expected webhook rejected audit log")
	}
}

func seedTalentaWebhookSecrets(
	t *testing.T,
	vault *hris.VaultService,
	tenantID string,
) (credentialRef, webhookSecretRef, clientID, clientSecret string) {
	t.Helper()

	clientID = "mekari-client-id-001"
	clientSecret = "mekari-client-secret-001"

	credential, err := vault.UpsertSecret(
		tenantID,
		"hris/talenta/client_id",
		"connector_credential",
		clientID,
		"qa",
	)
	if err != nil {
		t.Fatalf("seed talenta client_id should succeed: %v", err)
	}
	webhookSecret, err := vault.UpsertSecret(
		tenantID,
		"hris/talenta/webhook_secret",
		"webhook_secret",
		clientSecret,
		"qa",
	)
	if err != nil {
		t.Fatalf("seed talenta webhook secret should succeed: %v", err)
	}
	return credential.Ref, webhookSecret.Ref, clientID, clientSecret
}

func seedTalentaSparseMergeEmployee(t *testing.T, svc *enterprise.Service) {
	t.Helper()

	if svc == nil {
		t.Fatalf("enterprise service is required")
	}
	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       "hris-jkt-1001",
				EmployeeNumber:   "TAL-MERGE-1001",
				Email:            "arief.putra@sudirman.co",
				FullName:         "Arief Putra",
				Department:       "Finance",
				JobTitle:         "Finance Manager",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				JoinDate:         "2024-01-15",
				LeaveStatus:      "annual_leave",
				CostCenter:       "CC-FIN-01",
				PhotoURL:         "https://cdn.example.com/photos/arief-putra.jpg",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected sparse merge seed sync success: %v", err)
	}

	employee, err := svc.GetEmployeeByEmail("tenant_demo_jakarta", "arief.putra@sudirman.co")
	if err != nil {
		t.Fatalf("expected sparse merge seed employee lookup success: %v", err)
	}
	if employee.JoinDate != "2024-01-15" ||
		employee.LeaveStatus != "annual_leave" ||
		employee.CostCenter != "CC-FIN-01" ||
		employee.PhotoURL != "https://cdn.example.com/photos/arief-putra.jpg" {
		t.Fatalf("expected sparse merge seed employee extended fields to be stored, got %+v", employee)
	}
}

func seedTalentaUpdatableEmployee(t *testing.T, svc *enterprise.Service) {
	t.Helper()

	if svc == nil {
		t.Fatalf("enterprise service is required")
	}
	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       "EMP-UPDATED-001",
				EmployeeNumber:   "TAL-UPDATED-001",
				Email:            "updated.user@talenta-sync.local",
				FullName:         "Updated User",
				Department:       "Operations",
				JobTitle:         "Ops Specialist",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				JoinDate:         "2024-01-15",
				LeaveStatus:      "annual_leave",
				CostCenter:       "CC-OPS-01",
				PhotoURL:         "https://cdn.example.com/photos/updated-user-v1.jpg",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected updatable employee seed sync success: %v", err)
	}

	employee, err := svc.GetEmployeeByEmail("tenant_demo_jakarta", "updated.user@talenta-sync.local")
	if err != nil {
		t.Fatalf("expected updatable employee lookup success: %v", err)
	}
	if employee.Department != "Operations" ||
		employee.JoinDate != "2024-01-15" ||
		employee.LeaveStatus != "annual_leave" ||
		employee.CostCenter != "CC-OPS-01" ||
		employee.PhotoURL != "https://cdn.example.com/photos/updated-user-v1.jpg" {
		t.Fatalf("expected updatable employee seed fields to be stored, got %+v", employee)
	}
}

func seedTalentaLifecycleEmployee(t *testing.T, svc *enterprise.Service, externalID, email, fullName string) {
	t.Helper()

	if svc == nil {
		t.Fatalf("enterprise service is required")
	}
	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       externalID,
				EmployeeNumber:   "TAL-" + externalID,
				Email:            email,
				FullName:         fullName,
				Department:       "Operations",
				JobTitle:         "Ops Specialist",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				JoinDate:         "2024-01-15",
				LeaveStatus:      "annual_leave",
				CostCenter:       "CC-OPS-01",
				PhotoURL:         "https://cdn.example.com/photos/transfer-user-v1.jpg",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected lifecycle employee seed sync success: %v", err)
	}
}

func seedTalentaTransferredEmployee(t *testing.T, svc *enterprise.Service) {
	t.Helper()

	if svc == nil {
		t.Fatalf("enterprise service is required")
	}
	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       "EMP-TRANSFER-CANCELLED-001",
				EmployeeNumber:   "TAL-EMP-TRANSFER-CANCELLED-001",
				Email:            "transfer.cancelled@talenta-sync.local",
				FullName:         "Transfer Cancelled User",
				Department:       "Security",
				JobTitle:         "Security Lead",
				Location:         "Bandung",
				EmploymentStatus: "active",
				JoinDate:         "2024-01-15",
				LeaveStatus:      "annual_leave",
				CostCenter:       "CC-OPS-01",
				PhotoURL:         "https://cdn.example.com/photos/transfer-user-v1.jpg",
				Status:           "active",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected transferred employee seed sync success: %v", err)
	}
}

func seedTalentaResignedEmployee(t *testing.T, svc *enterprise.Service) {
	t.Helper()

	if svc == nil {
		t.Fatalf("enterprise service is required")
	}
	_, err := svc.SyncEmployees(
		"tenant_demo_jakarta",
		"hris_talenta",
		"seed",
		[]enterprise.EmployeeSyncInput{
			{
				ExternalID:       "EMP-RESIGN-CANCELLED-001",
				EmployeeNumber:   "TAL-EMP-RESIGN-CANCELLED-001",
				Email:            "resign.cancelled@talenta-sync.local",
				FullName:         "Resign Cancelled User",
				Department:       "Operations",
				JobTitle:         "Operator",
				Location:         "Jakarta",
				EmploymentStatus: "terminated",
				JoinDate:         "2024-01-15",
				ResignDate:       "2026-05-12",
				LeaveStatus:      "annual_leave",
				CostCenter:       "CC-OPS-01",
				PhotoURL:         "https://cdn.example.com/photos/transfer-user-v1.jpg",
				Status:           "inactive",
			},
		},
	)
	if err != nil {
		t.Fatalf("expected resigned employee seed sync success: %v", err)
	}
}

func applyTalentaWebhookSignature(
	request *http.Request,
	body string,
	clientID string,
	clientSecret string,
	now time.Time,
) {
	dateHeader := now.UTC().Format(http.TimeFormat)
	request.Header.Set("Date", dateHeader)

	digestSum := sha256.Sum256([]byte(body))
	request.Header.Set("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(digestSum[:]))

	proto := request.Proto
	if strings.TrimSpace(proto) == "" {
		proto = "HTTP/1.1"
	}
	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte("date: " + dateHeader + "\n" + request.Method + " " + request.URL.RequestURI() + " " + proto))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	request.Header.Set(
		"Authorization",
		`hmac username="`+clientID+`", algorithm="hmac-sha256", headers="date request-line", signature="`+signature+`"`,
	)
}

type alwaysFailGadjianNormalizer struct {
	calls int
}

func (n *alwaysFailGadjianNormalizer) Vendor() string {
	return "gadjian"
}

func (n *alwaysFailGadjianNormalizer) NormalizeWebhook(receipt enterprise.HRISWebhookReceipt) (hris.NormalizedSyncRequest, error) {
	n.calls++
	return hris.NormalizedSyncRequest{}, errors.New("forced persistent webhook normalization failure")
}

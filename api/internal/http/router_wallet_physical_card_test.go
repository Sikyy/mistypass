package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestWalletPhysicalCardInventoryRoutes(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "wallet-physical-card-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	vendorsRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/wallet/physical-card-vendors?tenant_id=tenant_demo_jakarta", token, nil)
	if vendorsRecorder.Code != http.StatusOK {
		t.Fatalf("expected vendors 200, got %d body=%s", vendorsRecorder.Code, vendorsRecorder.Body.String())
	}
	if !strings.Contains(vendorsRecorder.Body.String(), "NusaCard Fulfillment") {
		t.Fatalf("expected seeded vendor, body=%s", vendorsRecorder.Body.String())
	}

	inventoryBody := []byte(`{"tenant_id":"tenant_demo_jakarta","card_number":"CARD-HTTP-9001","uid":"UID-HTTP-9001","vendor_id":"wpcv_internal_demo"}`)
	createInventoryRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-inventory", token, inventoryBody)
	if createInventoryRecorder.Code != http.StatusCreated {
		t.Fatalf("expected inventory create 201, got %d body=%s", createInventoryRecorder.Code, createInventoryRecorder.Body.String())
	}
	var createdInventory struct {
		ID         string `json:"id"`
		CardNumber string `json:"card_number"`
		Status     string `json:"status"`
		VendorName string `json:"vendor_name"`
	}
	if err := json.Unmarshal(createInventoryRecorder.Body.Bytes(), &createdInventory); err != nil {
		t.Fatalf("decode inventory: %v", err)
	}
	if createdInventory.ID == "" || createdInventory.CardNumber != "CARD-HTTP-9001" || createdInventory.Status != "available" {
		t.Fatalf("unexpected created inventory: %+v", createdInventory)
	}

	scanBody := []byte(`{"tenant_id":"tenant_demo_jakarta","uid":"UID-SCAN-HTTP-9004","card_number":"CARD-HTTP-9004","reader_id":"reader_demo_001","vendor_id":"wpcv_internal_demo"}`)
	scanRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-inventory/scan", token, scanBody)
	if scanRecorder.Code != http.StatusCreated {
		t.Fatalf("expected inventory scan 201, got %d body=%s", scanRecorder.Code, scanRecorder.Body.String())
	}
	var scannedInventory struct {
		ID         string `json:"id"`
		CardNumber string `json:"card_number"`
		UID        string `json:"uid"`
		Source     string `json:"source"`
		ReaderID   string `json:"reader_id"`
		Status     string `json:"status"`
		ScannedAt  string `json:"scanned_at"`
	}
	if err := json.Unmarshal(scanRecorder.Body.Bytes(), &scannedInventory); err != nil {
		t.Fatalf("decode scanned inventory: %v", err)
	}
	if scannedInventory.ID == "" ||
		scannedInventory.CardNumber != "CARD-HTTP-9004" ||
		scannedInventory.UID != "UID-SCAN-HTTP-9004" ||
		scannedInventory.Source != "reader_scan" ||
		scannedInventory.ReaderID != "reader_demo_001" ||
		scannedInventory.Status != "available" ||
		scannedInventory.ScannedAt == "" {
		t.Fatalf("unexpected scanned inventory: %+v body=%s", scannedInventory, scanRecorder.Body.String())
	}

	rescanBody := []byte(`{"tenant_id":"tenant_demo_jakarta","uid":"UID-SCAN-HTTP-9004","reader_id":"reader_demo_002"}`)
	rescanRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-inventory/scan", token, rescanBody)
	if rescanRecorder.Code != http.StatusCreated {
		t.Fatalf("expected inventory rescan 201, got %d body=%s", rescanRecorder.Code, rescanRecorder.Body.String())
	}
	if !strings.Contains(rescanRecorder.Body.String(), `"id":"`+scannedInventory.ID+`"`) ||
		!strings.Contains(rescanRecorder.Body.String(), `"card_number":"CARD-HTTP-9004"`) ||
		!strings.Contains(rescanRecorder.Body.String(), `"reader_id":"reader_demo_002"`) {
		t.Fatalf("expected rescan to update existing inventory item, body=%s", rescanRecorder.Body.String())
	}

	importCSVBody := []byte(`{"tenant_id":"tenant_demo_jakarta","csv_content":"card_number,uid,vendor_id,status\nCARD-HTTP-9002,UID-HTTP-9002,wpcv_internal_demo,available\nCARD-HTTP-9003,UID-HTTP-9003,wpcv_nusacard_demo,available"}`)
	importCSVRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-inventory/import-csv", token, importCSVBody)
	if importCSVRecorder.Code != http.StatusCreated {
		t.Fatalf("expected inventory csv import 201, got %d body=%s", importCSVRecorder.Code, importCSVRecorder.Body.String())
	}
	if !strings.Contains(importCSVRecorder.Body.String(), "CARD-HTTP-9002") || !strings.Contains(importCSVRecorder.Body.String(), "NusaCard Fulfillment") {
		t.Fatalf("expected imported csv inventory items, body=%s", importCSVRecorder.Body.String())
	}
	var importResponse struct {
		Items []struct {
			ID         string `json:"id"`
			CardNumber string `json:"card_number"`
			Status     string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(importCSVRecorder.Body.Bytes(), &importResponse); err != nil {
		t.Fatalf("decode imported inventory: %v", err)
	}
	if len(importResponse.Items) < 2 {
		t.Fatalf("expected imported inventory items, got %+v", importResponse.Items)
	}

	batchFreezeBody := []byte(`{"tenant_id":"tenant_demo_jakarta","inventory_ids":["` + importResponse.Items[0].ID + `","` + importResponse.Items[0].ID + `","` + importResponse.Items[1].ID + `"],"status":"frozen","reason":"supplier QA hold"}`)
	batchFreezeRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/wallet/physical-card-inventory/batch-status", token, batchFreezeBody)
	if batchFreezeRecorder.Code != http.StatusOK {
		t.Fatalf("expected inventory batch freeze 200, got %d body=%s", batchFreezeRecorder.Code, batchFreezeRecorder.Body.String())
	}
	if strings.Count(batchFreezeRecorder.Body.String(), `"status":"frozen"`) != 2 {
		t.Fatalf("expected two frozen inventory items, body=%s", batchFreezeRecorder.Body.String())
	}

	frozenTaskBody := []byte(`{"tenant_id":"tenant_demo_jakarta","pass_id":"wps_demo_1001","task_type":"reissue","inventory_id":"` + importResponse.Items[0].ID + `","note":"should be blocked while frozen"}`)
	frozenTaskRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-tasks", token, frozenTaskBody)
	if frozenTaskRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected frozen inventory task create 404, got %d body=%s", frozenTaskRecorder.Code, frozenTaskRecorder.Body.String())
	}

	releaseBody := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"available","reason":"QA release"}`)
	releaseRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/wallet/physical-card-inventory/"+importResponse.Items[0].ID+"/status", token, releaseBody)
	if releaseRecorder.Code != http.StatusOK {
		t.Fatalf("expected inventory release 200, got %d body=%s", releaseRecorder.Code, releaseRecorder.Body.String())
	}
	if !strings.Contains(releaseRecorder.Body.String(), `"status":"available"`) {
		t.Fatalf("expected released inventory to be available, body=%s", releaseRecorder.Body.String())
	}

	taskBody := []byte(`{"tenant_id":"tenant_demo_jakarta","pass_id":"wps_demo_1001","task_type":"reissue","inventory_id":"` + createdInventory.ID + `","note":"HTTP route coverage"}`)
	createTaskRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/wallet/physical-card-tasks", token, taskBody)
	if createTaskRecorder.Code != http.StatusCreated {
		t.Fatalf("expected task create 201, got %d body=%s", createTaskRecorder.Code, createTaskRecorder.Body.String())
	}
	var createdTask struct {
		ID          string `json:"id"`
		InventoryID string `json:"inventory_id"`
		CardNumber  string `json:"card_number"`
		VendorName  string `json:"vendor_name"`
	}
	if err := json.Unmarshal(createTaskRecorder.Body.Bytes(), &createdTask); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if createdTask.InventoryID != createdInventory.ID || createdTask.CardNumber != createdInventory.CardNumber || createdTask.VendorName == "" {
		t.Fatalf("expected task to bind inventory/vendor, got %+v", createdTask)
	}

	reservedRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/wallet/physical-card-inventory?tenant_id=tenant_demo_jakarta&status=reserved", token, nil)
	if reservedRecorder.Code != http.StatusOK {
		t.Fatalf("expected reserved inventory 200, got %d body=%s", reservedRecorder.Code, reservedRecorder.Body.String())
	}
	if !strings.Contains(reservedRecorder.Body.String(), createdTask.ID) {
		t.Fatalf("expected reserved inventory to reference task, body=%s", reservedRecorder.Body.String())
	}

	reservedScrapBody := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"scrapped","reason":"bypass task"}`)
	reservedScrapRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/wallet/physical-card-inventory/"+createdInventory.ID+"/status", token, reservedScrapBody)
	if reservedScrapRecorder.Code != http.StatusConflict {
		t.Fatalf("expected reserved inventory direct scrap 409, got %d body=%s", reservedScrapRecorder.Code, reservedScrapRecorder.Body.String())
	}

	scrapBody := []byte(`{"tenant_id":"tenant_demo_jakarta","status":"scrapped","reason":"bad print stock"}`)
	scrapRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/wallet/physical-card-inventory/"+importResponse.Items[1].ID+"/status", token, scrapBody)
	if scrapRecorder.Code != http.StatusOK {
		t.Fatalf("expected frozen inventory scrap 200, got %d body=%s", scrapRecorder.Code, scrapRecorder.Body.String())
	}
	if !strings.Contains(scrapRecorder.Body.String(), `"status":"scrapped"`) {
		t.Fatalf("expected scrapped inventory status, body=%s", scrapRecorder.Body.String())
	}

	assertReferenceAuditLog(t, router, token, "physical_card_inventory_status_batch_update", "count=2", "status=frozen")
	assertReferenceAuditLog(t, router, token, "physical_card_inventory_status_update", "inventory_id="+importResponse.Items[1].ID, "status=scrapped")
}

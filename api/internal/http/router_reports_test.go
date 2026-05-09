package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestReferenceReportsListDetailDownloadAndScope(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "reference-reports-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	listRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/reports?tenant_id=tenant_demo_jakarta", token, nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected reports list status 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Items []struct {
			ID           string `json:"id"`
			ResourceType string `json:"resource_type"`
			TenantID     string `json:"tenant_id"`
			Category     string `json:"category"`
			DownloadURL  string `json:"download_url"`
			Metrics      []struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
			} `json:"metrics"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode reports list: %v", err)
	}
	reportIDs := map[string]bool{}
	for _, item := range listResponse.Items {
		reportIDs[item.ID] = true
		if item.ResourceType != "Report" || item.TenantID != "tenant_demo_jakarta" || item.DownloadURL == "" || len(item.Metrics) == 0 {
			t.Fatalf("expected populated report item, got %#v body=%s", item, listRecorder.Body.String())
		}
	}
	for _, reportID := range []string{referenceReportAccessActivityID, referenceReportDoorAlarmID, referenceReportAuditActivityID} {
		if !reportIDs[reportID] {
			t.Fatalf("expected report %s in list, got %#v body=%s", reportID, reportIDs, listRecorder.Body.String())
		}
	}

	detailRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/reports/"+referenceReportDoorAlarmID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected report detail status 200, got %d body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
	if !strings.Contains(detailRecorder.Body.String(), `"category":"doors"`) || !strings.Contains(detailRecorder.Body.String(), `"high_priority"`) {
		t.Fatalf("expected door report detail metrics, body=%s", detailRecorder.Body.String())
	}

	downloadRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/reports/"+referenceReportAccessActivityID+"/download?tenant_id=tenant_demo_jakarta", token, nil)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected report download status 200, got %d body=%s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if !strings.Contains(downloadRecorder.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(downloadRecorder.Header().Get("Content-Disposition"), referenceReportAccessActivityID+".csv") {
		t.Fatalf("expected CSV download headers, got content-type=%q disposition=%q", downloadRecorder.Header().Get("Content-Type"), downloadRecorder.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(downloadRecorder.Body.String(), "door_jkt_001") || !strings.Contains(downloadRecorder.Body.String(), "door_jkt_014") {
		t.Fatalf("expected organization CSV to include Jakarta access rows, body=%s", downloadRecorder.Body.String())
	}

	placeToken := referenceAPILogin(t, router, "place.admin.sudirman@mistypass.local")
	scopedListRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/reports?tenant_id=tenant_demo_jakarta", placeToken, nil)
	if scopedListRecorder.Code != http.StatusOK {
		t.Fatalf("expected scoped reports list status 200, got %d body=%s", scopedListRecorder.Code, scopedListRecorder.Body.String())
	}
	if strings.Contains(scopedListRecorder.Body.String(), referenceReportAuditActivityID) {
		t.Fatalf("expected scoped reports to hide organization audit report, body=%s", scopedListRecorder.Body.String())
	}
	scopedDownloadRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/reports/"+referenceReportAccessActivityID+"/download?tenant_id=tenant_demo_jakarta", placeToken, nil)
	if scopedDownloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected scoped report download status 200, got %d body=%s", scopedDownloadRecorder.Code, scopedDownloadRecorder.Body.String())
	}
	if !strings.Contains(scopedDownloadRecorder.Body.String(), "door_jkt_001") || strings.Contains(scopedDownloadRecorder.Body.String(), "door_jkt_014") {
		t.Fatalf("expected scoped CSV to include only assigned place rows, body=%s", scopedDownloadRecorder.Body.String())
	}
	crossPlaceRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/reports?tenant_id=tenant_demo_jakarta&place_id=building_demo_002", placeToken, nil)
	if crossPlaceRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-place reports status 403, got %d body=%s", crossPlaceRecorder.Code, crossPlaceRecorder.Body.String())
	}
}

func TestReferenceScheduledReportsCRUD(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "reference-scheduled-reports-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createBody := []byte(`{"scheduled_report":{"tenant_id":"tenant_demo_jakarta","report_id":"` + referenceReportAccessActivityID + `","name":"Weekly Sudirman Access","cadence":"weekly","format":"csv","place_id":"building_demo_001","recipients":["ops@example.test","ops@example.test","security@example.test"]}}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/scheduled_reports", token, createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected scheduled report create status 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		ID         string   `json:"id"`
		ReportID   string   `json:"report_id"`
		Name       string   `json:"name"`
		Cadence    string   `json:"cadence"`
		Status     string   `json:"status"`
		PlaceID    string   `json:"place_id"`
		Recipients []string `json:"recipients"`
		NextRunAt  string   `json:"next_run_at"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode scheduled report create: %v", err)
	}
	if created.ID == "" || created.ReportID != referenceReportAccessActivityID || created.Cadence != "weekly" || created.Status != "active" ||
		created.PlaceID != "building_demo_001" || len(created.Recipients) != 2 || created.NextRunAt == "" {
		t.Fatalf("unexpected scheduled report create response: %#v body=%s", created, createRecorder.Body.String())
	}

	getRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/scheduled_reports/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected scheduled report detail status 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	if !strings.Contains(getRecorder.Body.String(), `"name":"Weekly Sudirman Access"`) {
		t.Fatalf("expected scheduled report detail, body=%s", getRecorder.Body.String())
	}

	updateBody := []byte(`{"scheduled_report":{"tenant_id":"tenant_demo_jakarta","status":"paused","cadence":"monthly","name":"Monthly Sudirman Access"}}`)
	updateRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/scheduled_reports/"+created.ID, token, updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected scheduled report update status 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if !strings.Contains(updateRecorder.Body.String(), `"status":"paused"`) ||
		!strings.Contains(updateRecorder.Body.String(), `"cadence":"monthly"`) ||
		!strings.Contains(updateRecorder.Body.String(), `"name":"Monthly Sudirman Access"`) {
		t.Fatalf("expected updated scheduled report, body=%s", updateRecorder.Body.String())
	}

	listRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/scheduled_reports?tenant_id=tenant_demo_jakarta&status=paused", token, nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected scheduled reports list status 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	if !strings.Contains(listRecorder.Body.String(), created.ID) || strings.Contains(listRecorder.Body.String(), "scheduled_report_daily_access_demo") {
		t.Fatalf("expected paused list to include only updated scheduled report, body=%s", listRecorder.Body.String())
	}

	deleteRecorder := referenceAPIRequest(t, router, http.MethodDelete, "/api/v1/scheduled_reports/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected scheduled report delete status 204, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	getDeletedRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/scheduled_reports/"+created.ID+"?tenant_id=tenant_demo_jakarta", token, nil)
	if getDeletedRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted scheduled report status 404, got %d body=%s", getDeletedRecorder.Code, getDeletedRecorder.Body.String())
	}
	assertReferenceAuditLog(t, router, token, "reference_scheduled_report_created", "scheduled_report_id="+created.ID, "report_id="+referenceReportAccessActivityID)
	assertReferenceAuditLog(t, router, token, "reference_scheduled_report_updated", "scheduled_report_id="+created.ID, "status=paused", "cadence=monthly")
	assertReferenceAuditLog(t, router, token, "reference_scheduled_report_deleted", "scheduled_report_id="+created.ID, "report_id="+referenceReportAccessActivityID)
}

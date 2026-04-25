package talenta

import (
	"errors"
	"testing"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
)

func TestNormalizeWebhookCreated(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		ID:          "whr_talenta_001",
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeDetailCreated,
		RequestID:   "mekari-evt-001",
		RawPayload: `{
			"event_type":"talenta.employee.detail.created",
			"employee":{
				"employment":{
					"employee_id":"EMP-001",
					"employee_number":"NIP-001",
					"organization_name":"IT Division",
					"job_position":"Staff IT",
					"approval_line_employee_id":"EMP-MGR-001",
					"branch":"Jakarta HQ",
					"employment_status":"active",
					"join_date":"2024-01-15"
				},
				"personal":{
					"first_name":"Arief",
					"last_name":"Putra",
					"email":"arief.putra@sudirman.co",
					"mobile_phone":"+6281234567890",
					"avatar":"https://cdn.example.com/photos/emp-001.jpg"
				},
				"leave_info":{
					"status":"approved",
					"type":"Annual Leave"
				},
				"payroll_info":{
					"cost_center_name":"CC-OPS-01"
				}
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected normalize webhook to succeed: %v", err)
	}
	if result.Source != "hris_talenta" {
		t.Fatalf("expected source hris_talenta, got %s", result.Source)
	}
	if result.RequestID != "mekari-evt-001" {
		t.Fatalf("request_id mismatch: %s", result.RequestID)
	}
	if result.RawPayloadRef != "hris_webhook_receipt:whr_talenta_001" {
		t.Fatalf("raw_payload_ref mismatch: %s", result.RawPayloadRef)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}

	employee := result.Employees[0]
	if employee.ExternalID != "EMP-001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.EmployeeNumber != "NIP-001" {
		t.Fatalf("employee_number mismatch: %s", employee.EmployeeNumber)
	}
	if employee.FullName != "Arief Putra" {
		t.Fatalf("full_name mismatch: %s", employee.FullName)
	}
	if employee.Email != "arief.putra@sudirman.co" {
		t.Fatalf("email mismatch: %s", employee.Email)
	}
	if employee.Department != "IT Division" {
		t.Fatalf("department mismatch: %s", employee.Department)
	}
	if employee.JobTitle != "Staff IT" {
		t.Fatalf("job_title mismatch: %s", employee.JobTitle)
	}
	if employee.Location != "Jakarta HQ" {
		t.Fatalf("location mismatch: %s", employee.Location)
	}
	if employee.ManagerExternalID != "EMP-MGR-001" {
		t.Fatalf("manager_external_id mismatch: %s", employee.ManagerExternalID)
	}
	if employee.JoinDate != "2024-01-15" {
		t.Fatalf("join_date mismatch: %s", employee.JoinDate)
	}
	if employee.LeaveStatus != "annual_leave" {
		t.Fatalf("leave_status mismatch: %s", employee.LeaveStatus)
	}
	if employee.CostCenter != "CC-OPS-01" {
		t.Fatalf("cost_center mismatch: %s", employee.CostCenter)
	}
	if employee.PhotoURL != "https://cdn.example.com/photos/emp-001.jpg" {
		t.Fatalf("photo_url mismatch: %s", employee.PhotoURL)
	}
	if employee.EmploymentStatus != "active" || employee.Status != "active" {
		t.Fatalf("expected active employment and status, got employment_status=%s status=%s", employee.EmploymentStatus, employee.Status)
	}
}

func TestNormalizeWebhookUpdatedUsesStableFallbackRequestID(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		ID:          "whr_talenta_001_updated",
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeDetailUpdated,
		RawPayload: `{
			"event_type":"talenta.employee.detail.updated",
			"employee":{
				"employment":{
					"employee_id":"EMP-001",
					"employee_number":"NIP-001",
					"organization_name":"Security",
					"job_position":"Security Lead",
					"branch":"Bandung",
					"join_date":"2024-06-01"
				},
				"personal":{
					"first_name":"Arief",
					"last_name":"Updated",
					"email":"arief.updated@sudirman.co",
					"mobile_phone":"+6281234567000",
					"avatar":"https://cdn.example.com/photos/emp-001-updated.jpg"
				},
				"leave_info":{
					"status":"approved",
					"type":"Sick Leave"
				},
				"payroll_info":{
					"cost_center_name":"CC-SEC-02"
				}
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected normalize webhook to succeed: %v", err)
	}
	expectedRequestID := "talenta:tenant_demo_jakarta:talenta.employee.detail.updated:emp-001:2024-06-01"
	if result.RequestID != expectedRequestID {
		t.Fatalf("expected request_id %s, got %s", expectedRequestID, result.RequestID)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}

	employee := result.Employees[0]
	if employee.ExternalID != "EMP-001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.Email != "arief.updated@sudirman.co" {
		t.Fatalf("email mismatch: %s", employee.Email)
	}
	if employee.FullName != "Arief Updated" {
		t.Fatalf("full_name mismatch: %s", employee.FullName)
	}
	if employee.Department != "Security" {
		t.Fatalf("department mismatch: %s", employee.Department)
	}
	if employee.JobTitle != "Security Lead" {
		t.Fatalf("job_title mismatch: %s", employee.JobTitle)
	}
	if employee.Location != "Bandung" {
		t.Fatalf("location mismatch: %s", employee.Location)
	}
	if employee.JoinDate != "2024-06-01" {
		t.Fatalf("join_date mismatch: %s", employee.JoinDate)
	}
	if employee.LeaveStatus != "sick_leave" {
		t.Fatalf("leave_status mismatch: %s", employee.LeaveStatus)
	}
	if employee.CostCenter != "CC-SEC-02" {
		t.Fatalf("cost_center mismatch: %s", employee.CostCenter)
	}
	if employee.PhotoURL != "https://cdn.example.com/photos/emp-001-updated.jpg" {
		t.Fatalf("photo_url mismatch: %s", employee.PhotoURL)
	}
	if employee.EmploymentStatus != "active" || employee.Status != "active" {
		t.Fatalf("expected active employment and status, got employment_status=%s status=%s", employee.EmploymentStatus, employee.Status)
	}
}

func TestNormalizeWebhookDeletedUsesStableFallbackRequestID(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		ID:          "whr_talenta_002",
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeDetailDeleted,
		RawPayload: `{
			"event_type":"talenta.employee.detail.deleted",
			"employee":{
				"employment":{
					"employee_id":"EMP-002",
					"resign_date":"2026-04-30"
				},
				"personal":{
					"email":"siti.nuraini@sudirman.co"
				}
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected normalize webhook to succeed: %v", err)
	}
	expectedRequestID := "talenta:tenant_demo_jakarta:talenta.employee.detail.deleted:emp-002:2026-04-30"
	if result.RequestID != expectedRequestID {
		t.Fatalf("expected request_id %s, got %s", expectedRequestID, result.RequestID)
	}
	employee := result.Employees[0]
	if employee.EmploymentStatus != "inactive" {
		t.Fatalf("expected employment_status inactive, got %s", employee.EmploymentStatus)
	}
	if employee.Status != "inactive" {
		t.Fatalf("expected status inactive, got %s", employee.Status)
	}
	if employee.FullName != "siti.nuraini@sudirman.co" {
		t.Fatalf("expected email fallback full_name, got %s", employee.FullName)
	}
}

func TestNormalizeWebhookShiftChange(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventAttendanceSchedulerShift,
		RawPayload: `{
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
		}`,
	})
	if err != nil {
		t.Fatalf("expected shift change normalize to succeed: %v", err)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
	employee := result.Employees[0]
	if employee.ExternalID != "hris-jkt-1001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.FullName != "Arief Putra" {
		t.Fatalf("full_name mismatch: %s", employee.FullName)
	}
	if employee.ShiftCode != "SHIFT-B" {
		t.Fatalf("shift_code mismatch: %s", employee.ShiftCode)
	}
	if employee.ScheduleWindow != "10:00-19:00" {
		t.Fatalf("schedule_window mismatch: %s", employee.ScheduleWindow)
	}
	if employee.Email != "" {
		t.Fatalf("expected sparse webhook email to remain empty before merge, got %s", employee.Email)
	}
}

func TestNormalizeWebhookScheduleChange(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventAttendanceSchedulerSchedule,
		RawPayload: `{
			"event_type":"talenta.attendance.scheduler.changeschedule",
			"changes":[
				{
					"employee_id":"hris-jkt-1001",
					"full_name":"Arief Putra",
					"shifts":[
						{
							"date":"2026-04-22",
							"name":"SHIFT-B",
							"schedule_in":"10:00",
							"schedule_out":"19:00"
						},
						{
							"date":"2026-04-23",
							"name":"SHIFT-B",
							"schedule_in":"10:00",
							"schedule_out":"19:00"
						}
					]
				}
			]
		}`,
	})
	if err != nil {
		t.Fatalf("expected schedule change normalize to succeed: %v", err)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
	employee := result.Employees[0]
	if employee.ExternalID != "hris-jkt-1001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.ShiftCode != "SHIFT-B" {
		t.Fatalf("shift_code mismatch: %s", employee.ShiftCode)
	}
	if employee.ScheduleWindow != "2026-04-22:10:00-19:00;2026-04-23:10:00-19:00" {
		t.Fatalf("schedule_window mismatch: %s", employee.ScheduleWindow)
	}
}

func TestNormalizeWebhookTransferApprovedEmploymentOnlyPayload(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeTransferApproved,
		RawPayload: `{
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
		}`,
	})
	if err != nil {
		t.Fatalf("expected transfer approved normalize to succeed: %v", err)
	}
	expectedRequestID := "talenta:tenant_demo_jakarta:talenta.employee.transfer.approved:emp-transfer-001:2026-05-02"
	if result.RequestID != expectedRequestID {
		t.Fatalf("expected request_id %s, got %s", expectedRequestID, result.RequestID)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
	employee := result.Employees[0]
	if employee.ExternalID != "EMP-TRANSFER-001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.Email != "" {
		t.Fatalf("expected transfer approved email empty before merge, got %s", employee.Email)
	}
	if employee.FullName != "" {
		t.Fatalf("expected transfer approved full_name empty before merge, got %s", employee.FullName)
	}
	if employee.Department != "Security" {
		t.Fatalf("department mismatch: %s", employee.Department)
	}
	if employee.JobTitle != "Security Lead" {
		t.Fatalf("job_title mismatch: %s", employee.JobTitle)
	}
	if employee.Location != "Bandung" {
		t.Fatalf("location mismatch: %s", employee.Location)
	}
}

func TestNormalizeWebhookResignationCreatedEmploymentOnlyPayload(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeResignationCreated,
		RawPayload: `{
			"event_type":"talenta.employee.resignation.created",
			"employment":{
				"employee_id":"EMP-RESIGN-001",
				"organization_name":"Operations",
				"job_position":"Operator",
				"branch":"Jakarta",
				"resign_date":"2026-05-12",
				"status":"resigned"
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected resignation created normalize to succeed: %v", err)
	}
	expectedRequestID := "talenta:tenant_demo_jakarta:talenta.employee.resignation.created:emp-resign-001:2026-05-12"
	if result.RequestID != expectedRequestID {
		t.Fatalf("expected request_id %s, got %s", expectedRequestID, result.RequestID)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
	employee := result.Employees[0]
	if employee.ExternalID != "EMP-RESIGN-001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.Email != "" {
		t.Fatalf("expected resignation created email empty before merge, got %s", employee.Email)
	}
	if employee.ResignDate != "2026-05-12" {
		t.Fatalf("resign_date mismatch: %s", employee.ResignDate)
	}
	if employee.EmploymentStatus != "terminated" {
		t.Fatalf("employment_status mismatch: %s", employee.EmploymentStatus)
	}
	if employee.Status != "inactive" {
		t.Fatalf("status mismatch: %s", employee.Status)
	}
}

func TestNormalizeWebhookTransferCancelledEmploymentOnlyPayload(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeTransferCancelled,
		RawPayload: `{
			"event_type":"talenta.employee.transfer.cancelled",
			"employment":{
				"employee_id":"EMP-TRANSFER-CANCELLED-001",
				"organization_name":"Operations",
				"job_position":"Ops Specialist",
				"branch":"Jakarta",
				"transfer_date":"2026-05-06"
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected transfer cancelled normalize to succeed: %v", err)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
	employee := result.Employees[0]
	if employee.ExternalID != "EMP-TRANSFER-CANCELLED-001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.Email != "" {
		t.Fatalf("expected transfer cancelled email empty before merge, got %s", employee.Email)
	}
	if employee.Department != "Operations" {
		t.Fatalf("department mismatch: %s", employee.Department)
	}
	if employee.JobTitle != "Ops Specialist" {
		t.Fatalf("job_title mismatch: %s", employee.JobTitle)
	}
	if employee.Location != "Jakarta" {
		t.Fatalf("location mismatch: %s", employee.Location)
	}
}

func TestNormalizeWebhookResignationCancelledEmploymentOnlyPayload(t *testing.T) {
	normalizer := NewNormalizer()

	result, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		TenantID:    "tenant_demo_jakarta",
		ConnectorID: "hrc_talenta_001",
		Vendor:      "talenta",
		EventType:   EventEmployeeResignationCancelled,
		RawPayload: `{
			"event_type":"talenta.employee.resignation.cancelled",
			"employment":{
				"employee_id":"EMP-RESIGN-CANCELLED-001",
				"organization_name":"Operations",
				"job_position":"Operator",
				"branch":"Jakarta",
				"resign_date":"",
				"status":"active"
			}
		}`,
	})
	if err != nil {
		t.Fatalf("expected resignation cancelled normalize to succeed: %v", err)
	}
	if len(result.Employees) != 1 {
		t.Fatalf("expected one employee, got %d", len(result.Employees))
	}
	employee := result.Employees[0]
	if employee.ExternalID != "EMP-RESIGN-CANCELLED-001" {
		t.Fatalf("external_id mismatch: %s", employee.ExternalID)
	}
	if employee.Email != "" {
		t.Fatalf("expected resignation cancelled email empty before merge, got %s", employee.Email)
	}
	if employee.ResignDate != "" {
		t.Fatalf("expected resignation cancelled resign_date empty, got %s", employee.ResignDate)
	}
	if employee.EmploymentStatus != "active" {
		t.Fatalf("employment_status mismatch: %s", employee.EmploymentStatus)
	}
	if employee.Status != "active" {
		t.Fatalf("status mismatch: %s", employee.Status)
	}
}

func TestNormalizeWebhookUnsupportedEvent(t *testing.T) {
	normalizer := NewNormalizer()

	_, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		Vendor:    "talenta",
		EventType: EventAttendanceLiveAttendance,
		RawPayload: `{
			"event_type":"talenta.attendance.liveattendance"
		}`,
	})
	if !errors.Is(err, hris.ErrDeferredWebhookEvent) {
		t.Fatalf("expected ErrDeferredWebhookEvent, got %v", err)
	}
}

func TestNormalizeWebhookRejectsMissingEmail(t *testing.T) {
	normalizer := NewNormalizer()

	_, err := normalizer.NormalizeWebhook(enterprise.HRISWebhookReceipt{
		Vendor:    "talenta",
		EventType: EventEmployeeDetailUpdated,
		RawPayload: `{
			"event_type":"talenta.employee.detail.updated",
			"employee":{
				"employment":{"employee_id":"EMP-003"}
			}
		}`,
	})
	if !errors.Is(err, hris.ErrInvalidWebhookPayload) {
		t.Fatalf("expected ErrInvalidWebhookPayload, got %v", err)
	}
}

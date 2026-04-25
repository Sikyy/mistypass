package talenta

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/hris"
)

type Normalizer struct{}

func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

func (n *Normalizer) Vendor() string {
	return "talenta"
}

func (n *Normalizer) NormalizeWebhook(receipt enterprise.HRISWebhookReceipt) (hris.NormalizedSyncRequest, error) {
	eventType := NormalizeEventType(receipt.EventType)
	if eventType == "" {
		eventType = NormalizeEventType(extractPayloadEventType(receipt.RawPayload))
	}
	if IsDeferredEvent(eventType) {
		return hris.NormalizedSyncRequest{}, hris.ErrDeferredWebhookEvent
	}
	if !SupportsEmployeeSync(eventType) {
		return hris.NormalizedSyncRequest{}, hris.ErrUnsupportedWebhookEvent
	}

	root, err := decodePayload(receipt.RawPayload)
	if err != nil {
		return hris.NormalizedSyncRequest{}, err
	}
	inputs := make([]enterprise.EmployeeSyncInput, 0, 1)
	employeeKey := ""
	effectiveAt := ""
	if IsSparseEmployeeEvent(eventType) {
		inputs, employeeKey, effectiveAt, err = normalizeSparseEmployeeInputs(eventType, root)
		if err != nil {
			return hris.NormalizedSyncRequest{}, err
		}
	} else {
		record := resolveEmployeeRecord(eventType, root)
		if record == nil {
			return hris.NormalizedSyncRequest{}, fmt.Errorf("%w: talenta employee record not found", hris.ErrInvalidWebhookPayload)
		}

		input, normalizedEmployeeKey, normalizedEffectiveAt, normalizeErr := normalizeEmployeeInput(eventType, record, root)
		if normalizeErr != nil {
			return hris.NormalizedSyncRequest{}, normalizeErr
		}
		inputs = append(inputs, input)
		employeeKey = normalizedEmployeeKey
		effectiveAt = normalizedEffectiveAt
	}

	request, err := hris.NormalizeSyncRequest(hris.NormalizedSyncRequest{
		TenantID:      receipt.TenantID,
		Source:        hris.SyncSourceForVendor(receipt.Vendor),
		Actor:         hris.SyncActor,
		RequestID:     hris.StableRequestID(receipt, employeeKey, effectiveAt),
		ConnectorID:   receipt.ConnectorID,
		RawPayloadRef: hris.RawPayloadRef(receipt),
		EventType:     eventType,
		Employees:     inputs,
	})
	if err != nil {
		return hris.NormalizedSyncRequest{}, err
	}
	return request, nil
}

func decodePayload(raw string) (map[string]any, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return nil, fmt.Errorf("%w: empty payload", hris.ErrInvalidWebhookPayload)
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return nil, fmt.Errorf("%w: %v", hris.ErrInvalidWebhookPayload, err)
	}
	return root, nil
}

func extractPayloadEventType(raw string) string {
	root, err := decodePayload(raw)
	if err != nil {
		return ""
	}
	return firstNonEmptyString(
		stringAt(root, "event_type"),
		stringAt(root, "event"),
		stringAt(root, "type"),
	)
}

func resolveEmployeeRecord(eventType string, root map[string]any) map[string]any {
	switch NormalizeEventType(eventType) {
	case EventEmployeeTransferApproved:
		if employment := mapAt(root, "new_employment"); len(employment) > 0 {
			return map[string]any{"employment": employment}
		}
	case EventEmployeeTransferCancelled,
		EventEmployeeResignationCreated,
		EventEmployeeResignationCancelled:
		if employment := mapAt(root, "employment"); len(employment) > 0 {
			return map[string]any{"employment": employment}
		}
	}

	candidates := []map[string]any{
		root,
		mapAt(root, "employee"),
		mapAt(root, "data"),
		mapAt(mapAt(root, "data"), "employee"),
		mapAt(root, "payload"),
		mapAt(mapAt(root, "payload"), "employee"),
	}

	for i := range candidates {
		if hasEmployeeSections(candidates[i]) {
			return candidates[i]
		}
	}
	for i := range candidates {
		if len(candidates[i]) > 0 {
			return candidates[i]
		}
	}
	return nil
}

func resolveChangeRecords(root map[string]any) []map[string]any {
	candidates := [][]map[string]any{
		mapsAt(root, "changes"),
		mapsAt(mapAt(root, "data"), "changes"),
		mapsAt(mapAt(root, "payload"), "changes"),
	}
	for i := range candidates {
		if len(candidates[i]) > 0 {
			return candidates[i]
		}
	}
	return nil
}

func resolveShiftRecord(change map[string]any) map[string]any {
	candidates := []map[string]any{
		mapAt(change, "new_shift"),
		mapAt(change, "shift"),
		mapAt(mapAt(change, "data"), "new_shift"),
	}
	for i := range candidates {
		if len(candidates[i]) > 0 {
			return candidates[i]
		}
	}
	return nil
}

func resolveScheduleShiftRecords(change map[string]any) []map[string]any {
	candidates := [][]map[string]any{
		mapsAt(change, "shifts"),
		mapsAt(mapAt(change, "schedule"), "shifts"),
		mapsAt(mapAt(change, "new_schedule"), "shifts"),
	}
	for i := range candidates {
		if len(candidates[i]) > 0 {
			return candidates[i]
		}
	}
	return nil
}

func hasEmployeeSections(record map[string]any) bool {
	if len(record) == 0 {
		return false
	}
	if len(mapAt(record, "employment")) > 0 || len(mapAt(record, "personal")) > 0 || len(mapAt(record, "payroll_info")) > 0 {
		return true
	}
	return firstNonEmptyString(
		stringAt(record, "employee_id"),
		stringAt(record, "id"),
		stringAt(record, "email"),
	) != ""
}

func normalizeEmployeeInput(eventType string, record map[string]any, root map[string]any) (enterprise.EmployeeSyncInput, string, string, error) {
	employment := mapAt(record, "employment")
	personal := mapAt(record, "personal")
	payrollInfo := mapAt(record, "payroll_info")

	externalID := firstNonEmptyString(
		stringAt(employment, "employee_id"),
		stringAt(record, "employee_id"),
		stringAt(record, "id"),
	)
	if externalID == "" {
		return enterprise.EmployeeSyncInput{}, "", "", fmt.Errorf("%w: talenta employee_id is required", hris.ErrInvalidWebhookPayload)
	}

	email := normalizeEmail(firstNonEmptyString(
		stringAt(personal, "email"),
		stringAt(record, "email"),
	))
	if email == "" && !RequiresExistingEmployeeMerge(eventType) {
		return enterprise.EmployeeSyncInput{}, "", "", fmt.Errorf("%w: talenta employee email is required", hris.ErrInvalidWebhookPayload)
	}

	fullName := buildFullName(
		firstNonEmptyString(stringAt(personal, "first_name"), stringAt(record, "first_name")),
		firstNonEmptyString(stringAt(personal, "last_name"), stringAt(record, "last_name")),
		firstNonEmptyString(stringAt(personal, "full_name"), stringAt(record, "full_name"), stringAt(record, "name")),
		email,
		externalID,
		RequiresExistingEmployeeMerge(eventType),
	)

	joinDate := firstNonEmptyString(
		stringAt(employment, "join_date"),
		stringAt(record, "join_date"),
		stringAt(root, "join_date"),
	)
	resignDate := firstNonEmptyString(
		stringAt(employment, "resign_date"),
		stringAt(record, "resign_date"),
		stringAt(root, "resign_date"),
	)
	effectiveAt := firstNonEmptyString(
		resignDate,
		joinDate,
		stringAt(employment, "transfer_date"),
		stringAt(record, "transfer_date"),
		stringAt(root, "transfer_date"),
		stringAt(record, "effective_at"),
		stringAt(record, "created_at"),
		stringAt(root, "effective_at"),
		stringAt(root, "created_at"),
		stringAt(root, "timestamp"),
	)

	employmentStatus := normalizeEmploymentStatus(
		eventType,
		firstNonEmptyString(
			stringAt(employment, "employment_status"),
			stringAt(employment, "status"),
			stringAt(record, "employment_status"),
			stringAt(record, "status"),
		),
	)

	status := "active"
	if enterprise.EmploymentStatusBlocksSession(employmentStatus) || eventType == EventEmployeeDetailDeleted {
		status = "inactive"
	}

	return enterprise.EmployeeSyncInput{
		ExternalID:        externalID,
		EmployeeNumber:    firstNonEmptyString(stringAt(employment, "employee_number"), stringAt(record, "employee_number"), stringAt(record, "employee_no")),
		Email:             email,
		FullName:          fullName,
		Department:        firstNonEmptyString(stringAt(employment, "organization_name"), stringAt(record, "organization_name")),
		JobTitle:          firstNonEmptyString(stringAt(employment, "job_position"), stringAt(record, "job_position")),
		Location:          firstNonEmptyString(stringAt(employment, "branch"), stringAt(record, "branch")),
		Phone:             firstNonEmptyString(stringAt(personal, "mobile_phone"), stringAt(personal, "phone"), stringAt(record, "mobile_phone"), stringAt(record, "phone")),
		ManagerExternalID: firstNonEmptyString(stringAt(employment, "approval_line_employee_id"), stringAt(record, "approval_line_employee_id")),
		EmploymentStatus:  employmentStatus,
		JoinDate:          joinDate,
		ResignDate:        resignDate,
		LeaveStatus:       resolveLeaveStatus(record, root),
		CostCenter:        firstNonEmptyString(stringAt(payrollInfo, "cost_center_name"), stringAt(record, "cost_center_name")),
		PhotoURL:          firstNonEmptyString(stringAt(personal, "avatar"), stringAt(record, "avatar")),
		Status:            status,
	}, externalID, effectiveAt, nil
}

func normalizeSparseEmployeeInputs(
	eventType string,
	root map[string]any,
) ([]enterprise.EmployeeSyncInput, string, string, error) {
	changes := resolveChangeRecords(root)
	if len(changes) == 0 {
		return nil, "", "", fmt.Errorf("%w: talenta change records not found", hris.ErrInvalidWebhookPayload)
	}

	items := make([]enterprise.EmployeeSyncInput, 0, len(changes))
	employeeKey := ""
	effectiveAt := ""
	for i := range changes {
		input, nextEmployeeKey, nextEffectiveAt, err := normalizeSparseEmployeeInput(eventType, changes[i], root)
		if err != nil {
			return nil, "", "", err
		}
		if employeeKey == "" {
			employeeKey = nextEmployeeKey
		}
		if effectiveAt == "" {
			effectiveAt = nextEffectiveAt
		}
		items = append(items, input)
	}
	return items, employeeKey, effectiveAt, nil
}

func normalizeSparseEmployeeInput(
	eventType string,
	change map[string]any,
	root map[string]any,
) (enterprise.EmployeeSyncInput, string, string, error) {
	employee := mapAt(change, "employee")
	externalID := firstNonEmptyString(
		stringAt(change, "employee_id"),
		stringAt(employee, "employee_id"),
		stringAt(employee, "id"),
		stringAt(change, "id"),
	)
	if externalID == "" {
		return enterprise.EmployeeSyncInput{}, "", "", fmt.Errorf("%w: talenta employee_id is required", hris.ErrInvalidWebhookPayload)
	}

	email := normalizeEmail(firstNonEmptyString(
		stringAt(employee, "email"),
		stringAt(change, "email"),
	))
	fullName := strings.TrimSpace(firstNonEmptyString(
		stringAt(employee, "full_name"),
		stringAt(employee, "name"),
		stringAt(change, "full_name"),
		stringAt(change, "employee_name"),
		stringAt(change, "name"),
	))
	effectiveAt := firstNonEmptyString(
		stringAt(change, "effective_at"),
		stringAt(change, "updated_at"),
		stringAt(change, "change_date"),
		stringAt(change, "date"),
		stringAt(root, "effective_at"),
		stringAt(root, "created_at"),
		stringAt(root, "timestamp"),
	)

	switch NormalizeEventType(eventType) {
	case EventAttendanceSchedulerShift:
		newShift := resolveShiftRecord(change)
		if len(newShift) == 0 {
			return enterprise.EmployeeSyncInput{}, "", "", fmt.Errorf("%w: talenta new_shift is required", hris.ErrInvalidWebhookPayload)
		}
		return enterprise.EmployeeSyncInput{
			ExternalID:     externalID,
			Email:          email,
			FullName:       fullName,
			ShiftCode:      shiftCodeFromRecord(newShift),
			ScheduleWindow: scheduleWindowFromShiftRecord(newShift, ""),
		}, externalID, effectiveAt, nil
	case EventAttendanceSchedulerSchedule:
		shifts := resolveScheduleShiftRecords(change)
		if len(shifts) == 0 {
			return enterprise.EmployeeSyncInput{}, "", "", fmt.Errorf("%w: talenta schedule shifts are required", hris.ErrInvalidWebhookPayload)
		}
		if effectiveAt == "" {
			effectiveAt = scheduleEffectiveAt(shifts)
		}
		return enterprise.EmployeeSyncInput{
			ExternalID:     externalID,
			Email:          email,
			FullName:       fullName,
			ShiftCode:      shiftCodeFromScheduleRecords(shifts),
			ScheduleWindow: scheduleWindowFromScheduleRecords(shifts),
		}, externalID, effectiveAt, nil
	default:
		if IsDeferredEvent(eventType) {
			return enterprise.EmployeeSyncInput{}, "", "", hris.ErrDeferredWebhookEvent
		}
		return enterprise.EmployeeSyncInput{}, "", "", hris.ErrUnsupportedWebhookEvent
	}
}

func normalizeEmploymentStatus(eventType, rawStatus string) string {
	switch NormalizeEventType(eventType) {
	case EventEmployeeDetailDeleted:
		return "inactive"
	case EventEmployeeResignationCreated:
		if next := normalizeTalentaStatus(rawStatus); next != "" {
			return next
		}
		return "terminated"
	case EventEmployeeResignationCancelled:
		if next := normalizeTalentaStatus(rawStatus); next != "" {
			return next
		}
		return "active"
	default:
		if next := normalizeTalentaStatus(rawStatus); next != "" {
			return next
		}
		return "active"
	}
}

func normalizeTalentaStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "":
		return ""
	case "active", "aktif":
		return "active"
	case "inactive", "nonaktif", "deleted", "disabled", "deactivated":
		return "inactive"
	case "resigned", "resign", "terminated", "terminate", "termination":
		return "terminated"
	default:
		next := enterprise.NormalizeEmploymentStatus(status)
		if next == "" {
			return ""
		}
		switch next {
		case "resigned", "resign":
			return "terminated"
		default:
			return next
		}
	}
}

func resolveLeaveStatus(record map[string]any, root map[string]any) string {
	if next := normalizeLeaveStatusValue(
		firstNonEmptyString(
			stringAt(record, "leave_status"),
			stringAt(root, "leave_status"),
		),
	); next != "" {
		return next
	}

	sections := []map[string]any{
		mapAt(record, "leave"),
		mapAt(record, "leave_info"),
		mapAt(record, "leave_information"),
		mapAt(record, "time_off"),
		mapAt(record, "time_off_info"),
		mapAt(record, "attendance"),
		mapAt(root, "leave"),
		mapAt(root, "leave_info"),
		mapAt(root, "leave_information"),
		mapAt(root, "time_off"),
		mapAt(root, "time_off_info"),
	}
	for i := range sections {
		section := sections[i]
		if len(section) == 0 {
			continue
		}
		if next := normalizeLeaveStatusValue(
			firstNonEmptyString(
				stringAt(section, "leave_status"),
				stringAt(section, "leave_status_name"),
				stringAt(section, "status_name"),
			),
		); next != "" {
			return next
		}

		status := firstNonEmptyString(
			stringAt(section, "status"),
			stringAt(section, "approval_status"),
			stringAt(section, "state"),
		)
		leaveType := firstNonEmptyString(
			stringAt(section, "leave_type"),
			stringAt(section, "type"),
			stringAt(section, "name"),
			stringAt(section, "category"),
		)
		if leaveType != "" && leaveStatusImpliesOnLeave(status) {
			return normalizeLeaveStatusValue(leaveType)
		}
		if next := normalizeLeaveStatusValue(status); next != "" {
			return next
		}
		if next := normalizeLeaveStatusValue(leaveType); next != "" {
			return next
		}
	}
	return ""
}

func leaveStatusImpliesOnLeave(status string) bool {
	switch normalizeLeaveStatusValue(status) {
	case "on_leave", "approved":
		return true
	default:
		return false
	}
}

func normalizeLeaveStatusValue(status string) string {
	next := strings.ToLower(strings.TrimSpace(status))
	switch next {
	case "":
		return ""
	case "none", "no_leave", "not_on_leave", "working", "active", "available":
		return "none"
	case "approved", "onleave", "on_leave", "on leave", "leave":
		return "on_leave"
	case "annual", "annualleave", "annual_leave", "annual leave", "cuti":
		return "annual_leave"
	case "sick", "sickleave", "sick_leave", "sick leave", "sakit":
		return "sick_leave"
	case "maternity", "maternityleave", "maternity_leave", "maternity leave", "melahirkan":
		return "maternity_leave"
	case "personal", "personalleave", "personal_leave", "personal leave", "izin":
		return "personal_leave"
	default:
		tokens := strings.FieldsFunc(next, func(r rune) bool {
			return r == ' ' || r == '-' || r == '/' || r == ':'
		})
		if len(tokens) == 0 {
			return ""
		}
		return strings.Join(tokens, "_")
	}
}

func buildFullName(firstName, lastName, fallbackName, email, externalID string, mergeOnly bool) string {
	fullName := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(firstName),
		strings.TrimSpace(lastName),
	}, " "))
	if fullName != "" {
		return fullName
	}
	if strings.TrimSpace(fallbackName) != "" {
		return strings.TrimSpace(fallbackName)
	}
	if mergeOnly {
		return ""
	}
	if strings.TrimSpace(email) != "" {
		return strings.TrimSpace(email)
	}
	return strings.TrimSpace(externalID)
}

func shiftCodeFromRecord(record map[string]any) string {
	return firstNonEmptyString(
		stringAt(record, "code"),
		stringAt(record, "shift_code"),
		stringAt(record, "shift_name"),
		stringAt(record, "name"),
	)
}

func scheduleWindowFromShiftRecord(record map[string]any, fallbackDate string) string {
	window := timeWindow(
		firstNonEmptyString(
			stringAt(record, "schedule_in"),
			stringAt(record, "clock_in"),
			stringAt(record, "start_time"),
			stringAt(record, "time_in"),
		),
		firstNonEmptyString(
			stringAt(record, "schedule_out"),
			stringAt(record, "clock_out"),
			stringAt(record, "end_time"),
			stringAt(record, "time_out"),
		),
	)
	return datedWindow(
		firstNonEmptyString(
			stringAt(record, "date"),
			stringAt(record, "shift_date"),
			stringAt(record, "work_date"),
			fallbackDate,
		),
		window,
	)
}

func shiftCodeFromScheduleRecords(records []map[string]any) string {
	unique := ""
	for i := range records {
		next := shiftCodeFromRecord(records[i])
		if next == "" {
			continue
		}
		if unique == "" {
			unique = next
			continue
		}
		if unique != next {
			return ""
		}
	}
	return unique
}

func scheduleWindowFromScheduleRecords(records []map[string]any) string {
	if len(records) == 0 {
		return ""
	}

	items := make([]string, 0, len(records))
	for i := range records {
		item := scheduleWindowFromShiftRecord(records[i], "")
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return strings.Join(items, ";")
}

func scheduleEffectiveAt(records []map[string]any) string {
	for i := range records {
		next := firstNonEmptyString(
			stringAt(records[i], "date"),
			stringAt(records[i], "shift_date"),
			stringAt(records[i], "work_date"),
		)
		if next != "" {
			return next
		}
	}
	return ""
}

func mapAt(input map[string]any, key string) map[string]any {
	if len(input) == 0 {
		return nil
	}
	value, ok := input[key]
	if !ok {
		return nil
	}
	output, ok := value.(map[string]any)
	if ok {
		return output
	}
	return nil
}

func mapsAt(input map[string]any, key string) []map[string]any {
	if len(input) == 0 {
		return nil
	}
	value, ok := input[key]
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	output := make([]map[string]any, 0, len(items))
	for i := range items {
		record, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		output = append(output, record)
	}
	return output
}

func stringAt(input map[string]any, key string) string {
	if len(input) == 0 {
		return ""
	}
	value, ok := input[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func timeWindow(start, end string) string {
	nextStart := strings.TrimSpace(start)
	nextEnd := strings.TrimSpace(end)
	switch {
	case nextStart != "" && nextEnd != "":
		return nextStart + "-" + nextEnd
	case nextStart != "":
		return nextStart
	default:
		return nextEnd
	}
}

func datedWindow(dateValue, window string) string {
	nextDate := strings.TrimSpace(dateValue)
	nextWindow := strings.TrimSpace(window)
	switch {
	case nextDate != "" && nextWindow != "":
		return nextDate + ":" + nextWindow
	case nextDate != "":
		return nextDate
	default:
		return nextWindow
	}
}

func firstNonEmptyString(items ...string) string {
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next != "" {
			return next
		}
	}
	return ""
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

package talenta

import "strings"

const (
	EventEmployeeDetailCreated        = "talenta.employee.detail.created"
	EventEmployeeDetailUpdated        = "talenta.employee.detail.updated"
	EventEmployeeDetailDeleted        = "talenta.employee.detail.deleted"
	EventEmployeeTransferApproved     = "talenta.employee.transfer.approved"
	EventEmployeeTransferCancelled    = "talenta.employee.transfer.cancelled"
	EventEmployeeResignationCreated   = "talenta.employee.resignation.created"
	EventEmployeeResignationCancelled = "talenta.employee.resignation.cancelled"
	EventAttendanceLiveAttendance     = "talenta.attendance.liveattendance"
	EventAttendanceSchedulerShift     = "talenta.attendance.scheduler.changeshift"
	EventAttendanceSchedulerSchedule  = "talenta.attendance.scheduler.changeschedule"
)

func NormalizeEventType(eventType string) string {
	return strings.ToLower(strings.TrimSpace(eventType))
}

func IsSparseEmployeeEvent(eventType string) bool {
	switch NormalizeEventType(eventType) {
	case EventAttendanceSchedulerShift,
		EventAttendanceSchedulerSchedule:
		return true
	default:
		return false
	}
}

func IsDeferredEvent(eventType string) bool {
	switch NormalizeEventType(eventType) {
	case EventAttendanceLiveAttendance:
		return true
	default:
		return false
	}
}

func SupportsEmployeeSync(eventType string) bool {
	switch NormalizeEventType(eventType) {
	case EventEmployeeDetailCreated,
		EventEmployeeDetailUpdated,
		EventEmployeeDetailDeleted,
		EventEmployeeTransferApproved,
		EventEmployeeTransferCancelled,
		EventEmployeeResignationCreated,
		EventEmployeeResignationCancelled:
		return true
	default:
		return IsSparseEmployeeEvent(eventType)
	}
}

func RequiresExistingEmployeeMerge(eventType string) bool {
	switch NormalizeEventType(eventType) {
	case EventAttendanceSchedulerShift,
		EventAttendanceSchedulerSchedule,
		EventEmployeeTransferApproved,
		EventEmployeeTransferCancelled,
		EventEmployeeResignationCreated,
		EventEmployeeResignationCancelled:
		return true
	default:
		return false
	}
}

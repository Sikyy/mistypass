package wallet

import (
	"strings"
	"time"
)

type PhysicalCardTask struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	PassID      string     `json:"pass_id"`
	TemplateID  string     `json:"template_id"`
	TargetType  string     `json:"target_type"`
	TargetID    string     `json:"target_id"`
	TaskType    string     `json:"task_type"`
	Status      string     `json:"status"`
	CardNumber  string     `json:"card_number,omitempty"`
	Note        string     `json:"note,omitempty"`
	PassStatus  string     `json:"pass_status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedBy   string     `json:"created_by"`
	UpdatedBy   string     `json:"updated_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func normalizePhysicalCardTaskType(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "issue", "reissue", "loss_report":
		return strings.ToLower(strings.TrimSpace(raw)), nil
	default:
		return "", ErrInvalidPhysicalCardTaskType
	}
}

func normalizePhysicalCardTaskStatus(taskType, raw string) (string, error) {
	next := strings.ToLower(strings.TrimSpace(raw))
	if next == "" {
		next = "queued"
	}
	switch taskType {
	case "issue", "reissue":
		switch next {
		case "queued", "printing", "ready", "issued", "cancelled":
			return next, nil
		}
	case "loss_report":
		switch next {
		case "queued", "reported_lost", "cancelled":
			return next, nil
		}
	}
	return "", ErrInvalidPhysicalCardTaskStatus
}

func canTransitPhysicalCardTaskStatus(taskType, current, next string) bool {
	if current == next {
		return true
	}
	switch taskType {
	case "issue", "reissue":
		switch current {
		case "queued":
			return next == "printing" || next == "ready" || next == "issued" || next == "cancelled"
		case "printing":
			return next == "ready" || next == "issued" || next == "cancelled"
		case "ready":
			return next == "issued" || next == "cancelled"
		default:
			return false
		}
	case "loss_report":
		return current == "queued" && (next == "reported_lost" || next == "cancelled")
	default:
		return false
	}
}

func clonePhysicalCardTask(input PhysicalCardTask) PhysicalCardTask {
	output := input
	if input.CompletedAt != nil {
		value := *input.CompletedAt
		output.CompletedAt = &value
	}
	return output
}

func clonePhysicalCardTasks(items []PhysicalCardTask) []PhysicalCardTask {
	output := make([]PhysicalCardTask, 0, len(items))
	for i := range items {
		output = append(output, clonePhysicalCardTask(items[i]))
	}
	return output
}

func (s *Service) ListPhysicalCardTasks(tenantID string) []PhysicalCardTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]PhysicalCardTask, 0, len(s.physicalCardTasks))
	for i := range s.physicalCardTasks {
		if s.physicalCardTasks[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, clonePhysicalCardTask(s.physicalCardTasks[i]))
	}
	return items
}

func (s *Service) CreatePhysicalCardTask(tenantID, passID, taskType, cardNumber, note, actor string) (PhysicalCardTask, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PhysicalCardTask{}, ErrPassIDRequired
	}

	nextTaskType, err := normalizePhysicalCardTaskType(taskType)
	if err != nil {
		return PhysicalCardTask{}, err
	}
	nextStatus, err := normalizePhysicalCardTaskStatus(nextTaskType, "")
	if err != nil {
		return PhysicalCardTask{}, err
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	id, err := walletID("wpc_")
	if err != nil {
		return PhysicalCardTask{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	passIndex := -1
	for i := range s.passes {
		if s.passes[i].ID == nextPassID {
			passIndex = i
			break
		}
	}
	if passIndex < 0 || s.passes[passIndex].TenantID != nextTenantID {
		return PhysicalCardTask{}, ErrPassNotFound
	}
	if s.passes[passIndex].TargetType != "user" {
		return PhysicalCardTask{}, ErrPhysicalCardTaskEmployeePassRequired
	}

	task := PhysicalCardTask{
		ID:         id,
		TenantID:   nextTenantID,
		PassID:     s.passes[passIndex].ID,
		TemplateID: s.passes[passIndex].TemplateID,
		TargetType: s.passes[passIndex].TargetType,
		TargetID:   s.passes[passIndex].TargetID,
		TaskType:   nextTaskType,
		Status:     nextStatus,
		CardNumber: strings.TrimSpace(cardNumber),
		Note:       strings.TrimSpace(note),
		PassStatus: s.passes[passIndex].Status,
		CreatedBy:  nextActor,
		UpdatedBy:  nextActor,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.physicalCardTasks = append([]PhysicalCardTask{task}, s.physicalCardTasks...)
	s.appendAuditLocked(nextTenantID, "wallet.physical_card_task.create", nextActor, task.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PhysicalCardTask{}, err
	}
	return clonePhysicalCardTask(task), nil
}

func (s *Service) UpdatePhysicalCardTaskStatus(tenantID, taskID, status, cardNumber, note, actor string) (PhysicalCardTask, error) {
	nextTaskID := strings.TrimSpace(taskID)
	if nextTaskID == "" {
		return PhysicalCardTask{}, ErrPhysicalCardTaskNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.physicalCardTasks {
		if s.physicalCardTasks[i].ID != nextTaskID {
			continue
		}
		if s.physicalCardTasks[i].TenantID != nextTenantID {
			return PhysicalCardTask{}, ErrPhysicalCardTaskNotFound
		}

		nextStatus, err := normalizePhysicalCardTaskStatus(s.physicalCardTasks[i].TaskType, status)
		if err != nil {
			return PhysicalCardTask{}, err
		}
		if !canTransitPhysicalCardTaskStatus(s.physicalCardTasks[i].TaskType, s.physicalCardTasks[i].Status, nextStatus) {
			return PhysicalCardTask{}, ErrInvalidPhysicalCardTaskTransition
		}

		passIndex := -1
		for j := range s.passes {
			if s.passes[j].ID == s.physicalCardTasks[i].PassID {
				passIndex = j
				break
			}
		}
		if passIndex < 0 || s.passes[passIndex].TenantID != nextTenantID {
			return PhysicalCardTask{}, ErrPassNotFound
		}

		s.physicalCardTasks[i].Status = nextStatus
		s.physicalCardTasks[i].UpdatedBy = nextActor
		s.physicalCardTasks[i].UpdatedAt = now
		if trimmed := strings.TrimSpace(cardNumber); trimmed != "" {
			s.physicalCardTasks[i].CardNumber = trimmed
		}
		if trimmed := strings.TrimSpace(note); trimmed != "" {
			s.physicalCardTasks[i].Note = trimmed
		}
		if nextStatus == "issued" || nextStatus == "reported_lost" || nextStatus == "cancelled" {
			value := now
			s.physicalCardTasks[i].CompletedAt = &value
		}

		if nextStatus == "reported_lost" && canTransitPassStatus(s.passes[passIndex].Status, "suspended") {
			s.passes[passIndex].Status = "suspended"
			s.passes[passIndex].UpdatedBy = nextActor
			s.passes[passIndex].UpdatedAt = now
			s.appendAuditLocked(nextTenantID, "wallet.pass.suspended", nextActor, s.passes[passIndex].ID, "success")
		}
		if nextStatus == "issued" && s.physicalCardTasks[i].TaskType == "reissue" && canTransitPassStatus(s.passes[passIndex].Status, "active") {
			s.passes[passIndex].Status = "active"
			s.passes[passIndex].UpdatedBy = nextActor
			s.passes[passIndex].UpdatedAt = now
			value := now
			s.passes[passIndex].ActivatedAt = &value
			s.appendAuditLocked(nextTenantID, "wallet.pass.active", nextActor, s.passes[passIndex].ID, "success")
		}
		s.physicalCardTasks[i].PassStatus = s.passes[passIndex].Status

		s.appendAuditLocked(nextTenantID, "wallet.physical_card_task.status", nextActor, s.physicalCardTasks[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PhysicalCardTask{}, err
		}
		return clonePhysicalCardTask(s.physicalCardTasks[i]), nil
	}

	return PhysicalCardTask{}, ErrPhysicalCardTaskNotFound
}

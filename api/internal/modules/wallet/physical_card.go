package wallet

import (
	"strconv"
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
	InventoryID string     `json:"inventory_id,omitempty"`
	VendorID    string     `json:"vendor_id,omitempty"`
	VendorName  string     `json:"vendor_name,omitempty"`
	Note        string     `json:"note,omitempty"`
	PassStatus  string     `json:"pass_status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedBy   string     `json:"created_by"`
	UpdatedBy   string     `json:"updated_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type PhysicalCardVendor struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PhysicalCardInventoryItem struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	CardNumber     string     `json:"card_number"`
	UID            string     `json:"uid,omitempty"`
	VendorID       string     `json:"vendor_id,omitempty"`
	VendorName     string     `json:"vendor_name,omitempty"`
	Source         string     `json:"source,omitempty"`
	ReaderID       string     `json:"reader_id,omitempty"`
	Status         string     `json:"status"`
	AssignedPassID string     `json:"assigned_pass_id,omitempty"`
	ActiveTaskID   string     `json:"active_task_id,omitempty"`
	ScannedAt      *time.Time `json:"scanned_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PhysicalCardInventoryImportItem struct {
	CardNumber string `json:"card_number"`
	UID        string `json:"uid"`
	VendorID   string `json:"vendor_id"`
	Status     string `json:"status"`
}

func normalizePhysicalCardInventoryStatus(raw string) (string, error) {
	next := strings.ToLower(strings.TrimSpace(raw))
	if next == "" {
		next = "available"
	}
	switch next {
	case "available", "reserved", "issued", "lost", "frozen", "scrapped":
		return next, nil
	default:
		return "", ErrInvalidPhysicalCardInventoryStatus
	}
}

func canTransitPhysicalCardInventoryStatus(current, next string) bool {
	if current == next {
		return true
	}
	switch current {
	case "available":
		return next == "frozen" || next == "scrapped"
	case "frozen":
		return next == "available" || next == "scrapped"
	case "lost":
		return next == "scrapped"
	default:
		return false
	}
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

func clonePhysicalCardVendors(items []PhysicalCardVendor) []PhysicalCardVendor {
	output := make([]PhysicalCardVendor, len(items))
	copy(output, items)
	return output
}

func clonePhysicalCardInventory(items []PhysicalCardInventoryItem) []PhysicalCardInventoryItem {
	output := make([]PhysicalCardInventoryItem, 0, len(items))
	for i := range items {
		item := items[i]
		if item.ScannedAt != nil {
			value := *item.ScannedAt
			item.ScannedAt = &value
		}
		output = append(output, item)
	}
	return output
}

func findPhysicalCardVendorLocked(items []PhysicalCardVendor, tenantID, vendorID string) (PhysicalCardVendor, bool) {
	nextVendorID := strings.TrimSpace(vendorID)
	if nextVendorID == "" {
		return PhysicalCardVendor{}, false
	}
	for i := range items {
		if items[i].TenantID == tenantID && items[i].ID == nextVendorID {
			return items[i], true
		}
	}
	return PhysicalCardVendor{}, false
}

func findPhysicalCardInventoryIndexLocked(items []PhysicalCardInventoryItem, tenantID, inventoryID, cardNumber string) int {
	nextInventoryID := strings.TrimSpace(inventoryID)
	nextCardNumber := strings.TrimSpace(cardNumber)
	for i := range items {
		if items[i].TenantID != tenantID {
			continue
		}
		if nextInventoryID != "" && items[i].ID == nextInventoryID {
			return i
		}
		if nextInventoryID == "" && nextCardNumber != "" && strings.EqualFold(items[i].CardNumber, nextCardNumber) {
			return i
		}
	}
	return -1
}

func findPhysicalCardInventoryIndexByUIDLocked(items []PhysicalCardInventoryItem, tenantID, uid string) int {
	nextUID := strings.TrimSpace(uid)
	if nextUID == "" {
		return -1
	}
	for i := range items {
		if items[i].TenantID != tenantID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(items[i].UID), nextUID) {
			return i
		}
	}
	return -1
}

func (s *Service) ListPhysicalCardVendors(tenantID string) []PhysicalCardVendor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]PhysicalCardVendor, 0, len(s.physicalCardVendors))
	for i := range s.physicalCardVendors {
		if s.physicalCardVendors[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.physicalCardVendors[i])
	}
	return clonePhysicalCardVendors(items)
}

func (s *Service) ListPhysicalCardInventory(tenantID, status string) []PhysicalCardInventoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	nextStatus := strings.ToLower(strings.TrimSpace(status))
	items := make([]PhysicalCardInventoryItem, 0, len(s.physicalCardInventory))
	for i := range s.physicalCardInventory {
		if s.physicalCardInventory[i].TenantID != nextTenantID {
			continue
		}
		if nextStatus != "" && s.physicalCardInventory[i].Status != nextStatus {
			continue
		}
		items = append(items, s.physicalCardInventory[i])
	}
	return clonePhysicalCardInventory(items)
}

func (s *Service) CreatePhysicalCardInventoryItem(tenantID, cardNumber, uid, vendorID, status, actor string) (PhysicalCardInventoryItem, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextCardNumber := strings.TrimSpace(cardNumber)
	if nextCardNumber == "" {
		return PhysicalCardInventoryItem{}, ErrPhysicalCardInventoryCardNumberRequired
	}
	nextStatus, err := normalizePhysicalCardInventoryStatus(status)
	if err != nil {
		return PhysicalCardInventoryItem{}, err
	}
	now := time.Now().UTC()
	id, err := walletID("wpci_")
	if err != nil {
		return PhysicalCardInventoryItem{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if findPhysicalCardInventoryIndexLocked(s.physicalCardInventory, nextTenantID, "", nextCardNumber) >= 0 {
		return PhysicalCardInventoryItem{}, ErrPhysicalCardInventoryAlreadyExists
	}

	var vendor PhysicalCardVendor
	if strings.TrimSpace(vendorID) != "" {
		var ok bool
		vendor, ok = findPhysicalCardVendorLocked(s.physicalCardVendors, nextTenantID, vendorID)
		if !ok {
			return PhysicalCardInventoryItem{}, ErrPhysicalCardVendorNotFound
		}
	}

	item := PhysicalCardInventoryItem{
		ID:         id,
		TenantID:   nextTenantID,
		CardNumber: nextCardNumber,
		UID:        strings.TrimSpace(uid),
		VendorID:   vendor.ID,
		VendorName: vendor.Name,
		Source:     "manual",
		Status:     nextStatus,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.physicalCardInventory = append([]PhysicalCardInventoryItem{item}, s.physicalCardInventory...)
	s.appendAuditLocked(nextTenantID, "wallet.physical_card_inventory.create", normalizeActor(actor), item.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PhysicalCardInventoryItem{}, err
	}
	return item, nil
}

func (s *Service) UpdatePhysicalCardInventoryStatus(tenantID, inventoryID, status, reason, actor string) (PhysicalCardInventoryItem, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextInventoryID := strings.TrimSpace(inventoryID)
	if nextInventoryID == "" {
		return PhysicalCardInventoryItem{}, ErrPhysicalCardInventoryNotFound
	}
	nextStatus, err := normalizePhysicalCardInventoryStatus(status)
	if err != nil {
		return PhysicalCardInventoryItem{}, err
	}
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	index := findPhysicalCardInventoryIndexLocked(s.physicalCardInventory, nextTenantID, nextInventoryID, "")
	if index < 0 {
		return PhysicalCardInventoryItem{}, ErrPhysicalCardInventoryNotFound
	}
	if err := applyPhysicalCardInventoryStatusTransition(&s.physicalCardInventory[index], nextStatus, now); err != nil {
		return PhysicalCardInventoryItem{}, err
	}
	s.appendAuditLocked(nextTenantID, "wallet.physical_card_inventory.status", nextActor, s.physicalCardInventory[index].ID, "status="+nextStatus+",reason="+strings.TrimSpace(reason))
	if err := s.persistLocked(); err != nil {
		return PhysicalCardInventoryItem{}, err
	}
	return clonePhysicalCardInventory([]PhysicalCardInventoryItem{s.physicalCardInventory[index]})[0], nil
}

func (s *Service) BatchUpdatePhysicalCardInventoryStatus(tenantID string, inventoryIDs []string, status, reason, actor string) ([]PhysicalCardInventoryItem, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextStatus, err := normalizePhysicalCardInventoryStatus(status)
	if err != nil {
		return nil, err
	}

	normalizedIDs := make([]string, 0, len(inventoryIDs))
	seen := map[string]struct{}{}
	for i := range inventoryIDs {
		id := strings.TrimSpace(inventoryIDs[i])
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalizedIDs = append(normalizedIDs, id)
	}
	if len(normalizedIDs) == 0 {
		return nil, ErrPhysicalCardInventoryIDsRequired
	}

	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	indexes := make([]int, 0, len(normalizedIDs))
	for i := range normalizedIDs {
		index := findPhysicalCardInventoryIndexLocked(s.physicalCardInventory, nextTenantID, normalizedIDs[i], "")
		if index < 0 {
			return nil, ErrPhysicalCardInventoryNotFound
		}
		currentStatus := strings.TrimSpace(s.physicalCardInventory[index].Status)
		if currentStatus == "" {
			currentStatus = "available"
		}
		if !canTransitPhysicalCardInventoryStatus(currentStatus, nextStatus) {
			return nil, ErrInvalidPhysicalCardInventoryTransition
		}
		indexes = append(indexes, index)
	}
	for i := range indexes {
		if err := applyPhysicalCardInventoryStatusTransition(&s.physicalCardInventory[indexes[i]], nextStatus, now); err != nil {
			return nil, err
		}
	}

	updated := make([]PhysicalCardInventoryItem, 0, len(indexes))
	for i := range indexes {
		updated = append(updated, s.physicalCardInventory[indexes[i]])
	}
	s.appendAuditLocked(nextTenantID, "wallet.physical_card_inventory.status_batch", nextActor, "count="+strconv.Itoa(len(updated)), "status="+nextStatus+",reason="+strings.TrimSpace(reason))
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return clonePhysicalCardInventory(updated), nil
}

func (s *Service) ImportPhysicalCardInventory(tenantID string, records []PhysicalCardInventoryImportItem, actor string) ([]PhysicalCardInventoryItem, error) {
	nextTenantID := normalizeTenantID(tenantID)
	if len(records) == 0 {
		return nil, ErrPhysicalCardInventoryRecordsRequired
	}

	now := time.Now().UTC()
	createdOrUpdated := make([]PhysicalCardInventoryItem, 0, len(records))

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range records {
		nextCardNumber := strings.TrimSpace(records[i].CardNumber)
		if nextCardNumber == "" {
			return nil, ErrPhysicalCardInventoryCardNumberRequired
		}
		nextStatus, err := normalizePhysicalCardInventoryStatus(records[i].Status)
		if err != nil {
			return nil, err
		}

		var vendor PhysicalCardVendor
		if strings.TrimSpace(records[i].VendorID) != "" {
			var ok bool
			vendor, ok = findPhysicalCardVendorLocked(s.physicalCardVendors, nextTenantID, records[i].VendorID)
			if !ok {
				return nil, ErrPhysicalCardVendorNotFound
			}
		}

		existingIndex := findPhysicalCardInventoryIndexLocked(s.physicalCardInventory, nextTenantID, "", nextCardNumber)
		if existingIndex >= 0 {
			if s.physicalCardInventory[existingIndex].Status != "available" {
				createdOrUpdated = append(createdOrUpdated, s.physicalCardInventory[existingIndex])
				continue
			}
			s.physicalCardInventory[existingIndex].UID = strings.TrimSpace(records[i].UID)
			s.physicalCardInventory[existingIndex].VendorID = vendor.ID
			s.physicalCardInventory[existingIndex].VendorName = vendor.Name
			s.physicalCardInventory[existingIndex].Source = firstNonEmpty(s.physicalCardInventory[existingIndex].Source, "import")
			s.physicalCardInventory[existingIndex].Status = nextStatus
			s.physicalCardInventory[existingIndex].UpdatedAt = now
			createdOrUpdated = append(createdOrUpdated, s.physicalCardInventory[existingIndex])
			continue
		}

		id, err := walletID("wpci_")
		if err != nil {
			return nil, err
		}
		item := PhysicalCardInventoryItem{
			ID:         id,
			TenantID:   nextTenantID,
			CardNumber: nextCardNumber,
			UID:        strings.TrimSpace(records[i].UID),
			VendorID:   vendor.ID,
			VendorName: vendor.Name,
			Source:     "import",
			Status:     nextStatus,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		s.physicalCardInventory = append([]PhysicalCardInventoryItem{item}, s.physicalCardInventory...)
		createdOrUpdated = append(createdOrUpdated, item)
	}

	s.appendAuditLocked(nextTenantID, "wallet.physical_card_inventory.import", normalizeActor(actor), "count="+strconv.Itoa(len(createdOrUpdated)), "success")
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return clonePhysicalCardInventory(createdOrUpdated), nil
}

func (s *Service) ScanPhysicalCardInventory(tenantID, uid, cardNumber, readerID, vendorID, actor string) (PhysicalCardInventoryItem, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextUID := strings.TrimSpace(uid)
	if nextUID == "" {
		return PhysicalCardInventoryItem{}, ErrPhysicalCardInventoryUIDRequired
	}
	requestedCardNumber := strings.TrimSpace(cardNumber)
	nextCardNumber := requestedCardNumber
	if nextCardNumber == "" {
		nextCardNumber = nextUID
	}
	nextReaderID := strings.TrimSpace(readerID)
	now := time.Now().UTC()
	id, err := walletID("wpci_")
	if err != nil {
		return PhysicalCardInventoryItem{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var vendor PhysicalCardVendor
	if strings.TrimSpace(vendorID) != "" {
		var ok bool
		vendor, ok = findPhysicalCardVendorLocked(s.physicalCardVendors, nextTenantID, vendorID)
		if !ok {
			return PhysicalCardInventoryItem{}, ErrPhysicalCardVendorNotFound
		}
	}

	existingIndex := findPhysicalCardInventoryIndexByUIDLocked(s.physicalCardInventory, nextTenantID, nextUID)
	if existingIndex < 0 {
		existingIndex = findPhysicalCardInventoryIndexLocked(s.physicalCardInventory, nextTenantID, "", nextCardNumber)
	}
	if existingIndex >= 0 {
		s.physicalCardInventory[existingIndex].UID = nextUID
		if requestedCardNumber != "" && s.physicalCardInventory[existingIndex].Status == "available" {
			s.physicalCardInventory[existingIndex].CardNumber = requestedCardNumber
		}
		if vendor.ID != "" && s.physicalCardInventory[existingIndex].Status == "available" {
			s.physicalCardInventory[existingIndex].VendorID = vendor.ID
			s.physicalCardInventory[existingIndex].VendorName = vendor.Name
		}
		s.physicalCardInventory[existingIndex].Source = "reader_scan"
		s.physicalCardInventory[existingIndex].ReaderID = nextReaderID
		s.physicalCardInventory[existingIndex].ScannedAt = &now
		s.physicalCardInventory[existingIndex].UpdatedAt = now
		s.appendAuditLocked(nextTenantID, "wallet.physical_card_inventory.scan", normalizeActor(actor), s.physicalCardInventory[existingIndex].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PhysicalCardInventoryItem{}, err
		}
		return clonePhysicalCardInventory([]PhysicalCardInventoryItem{s.physicalCardInventory[existingIndex]})[0], nil
	}

	item := PhysicalCardInventoryItem{
		ID:         id,
		TenantID:   nextTenantID,
		CardNumber: nextCardNumber,
		UID:        nextUID,
		VendorID:   vendor.ID,
		VendorName: vendor.Name,
		Source:     "reader_scan",
		ReaderID:   nextReaderID,
		Status:     "available",
		ScannedAt:  &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.physicalCardInventory = append([]PhysicalCardInventoryItem{item}, s.physicalCardInventory...)
	s.appendAuditLocked(nextTenantID, "wallet.physical_card_inventory.scan", normalizeActor(actor), item.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PhysicalCardInventoryItem{}, err
	}
	return clonePhysicalCardInventory([]PhysicalCardInventoryItem{item})[0], nil
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

func (s *Service) CreatePhysicalCardTask(tenantID, passID, taskType, cardNumber, inventoryID, vendorID, note, actor string) (PhysicalCardTask, error) {
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

	nextCardNumber := strings.TrimSpace(cardNumber)
	nextInventoryID := strings.TrimSpace(inventoryID)
	nextVendorID := strings.TrimSpace(vendorID)
	var vendor PhysicalCardVendor
	inventoryIndex := findPhysicalCardInventoryIndexLocked(s.physicalCardInventory, nextTenantID, nextInventoryID, nextCardNumber)
	if inventoryIndex >= 0 {
		if s.physicalCardInventory[inventoryIndex].Status != "available" {
			return PhysicalCardTask{}, ErrPhysicalCardInventoryNotFound
		}
		nextInventoryID = s.physicalCardInventory[inventoryIndex].ID
		nextCardNumber = s.physicalCardInventory[inventoryIndex].CardNumber
		nextVendorID = s.physicalCardInventory[inventoryIndex].VendorID
		vendor.Name = s.physicalCardInventory[inventoryIndex].VendorName
	}
	if nextVendorID != "" && vendor.Name == "" {
		var ok bool
		vendor, ok = findPhysicalCardVendorLocked(s.physicalCardVendors, nextTenantID, nextVendorID)
		if !ok {
			return PhysicalCardTask{}, ErrPhysicalCardVendorNotFound
		}
	}

	task := PhysicalCardTask{
		ID:          id,
		TenantID:    nextTenantID,
		PassID:      s.passes[passIndex].ID,
		TemplateID:  s.passes[passIndex].TemplateID,
		TargetType:  s.passes[passIndex].TargetType,
		TargetID:    s.passes[passIndex].TargetID,
		TaskType:    nextTaskType,
		Status:      nextStatus,
		CardNumber:  nextCardNumber,
		InventoryID: nextInventoryID,
		VendorID:    nextVendorID,
		VendorName:  vendor.Name,
		Note:        strings.TrimSpace(note),
		PassStatus:  s.passes[passIndex].Status,
		CreatedBy:   nextActor,
		UpdatedBy:   nextActor,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if inventoryIndex >= 0 {
		s.physicalCardInventory[inventoryIndex].Status = "reserved"
		s.physicalCardInventory[inventoryIndex].AssignedPassID = s.passes[passIndex].ID
		s.physicalCardInventory[inventoryIndex].ActiveTaskID = task.ID
		s.physicalCardInventory[inventoryIndex].UpdatedAt = now
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

		inventoryIndex := findPhysicalCardInventoryIndexLocked(
			s.physicalCardInventory,
			nextTenantID,
			s.physicalCardTasks[i].InventoryID,
			s.physicalCardTasks[i].CardNumber,
		)
		if inventoryIndex >= 0 {
			switch nextStatus {
			case "issued":
				s.physicalCardInventory[inventoryIndex].Status = "issued"
				s.physicalCardInventory[inventoryIndex].AssignedPassID = s.physicalCardTasks[i].PassID
				s.physicalCardInventory[inventoryIndex].ActiveTaskID = s.physicalCardTasks[i].ID
			case "reported_lost":
				s.physicalCardInventory[inventoryIndex].Status = "lost"
				s.physicalCardInventory[inventoryIndex].AssignedPassID = s.physicalCardTasks[i].PassID
				s.physicalCardInventory[inventoryIndex].ActiveTaskID = s.physicalCardTasks[i].ID
			case "cancelled":
				s.physicalCardInventory[inventoryIndex].Status = "available"
				s.physicalCardInventory[inventoryIndex].AssignedPassID = ""
				s.physicalCardInventory[inventoryIndex].ActiveTaskID = ""
			default:
				if s.physicalCardInventory[inventoryIndex].Status == "available" {
					s.physicalCardInventory[inventoryIndex].Status = "reserved"
					s.physicalCardInventory[inventoryIndex].AssignedPassID = s.physicalCardTasks[i].PassID
					s.physicalCardInventory[inventoryIndex].ActiveTaskID = s.physicalCardTasks[i].ID
				}
			}
			s.physicalCardInventory[inventoryIndex].UpdatedAt = now
		}

		s.appendAuditLocked(nextTenantID, "wallet.physical_card_task.status", nextActor, s.physicalCardTasks[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PhysicalCardTask{}, err
		}
		return clonePhysicalCardTask(s.physicalCardTasks[i]), nil
	}

	return PhysicalCardTask{}, ErrPhysicalCardTaskNotFound
}

func applyPhysicalCardInventoryStatusTransition(item *PhysicalCardInventoryItem, nextStatus string, now time.Time) error {
	currentStatus := strings.TrimSpace(item.Status)
	if currentStatus == "" {
		currentStatus = "available"
	}
	if currentStatus == nextStatus {
		item.UpdatedAt = now
		return nil
	}
	if !canTransitPhysicalCardInventoryStatus(currentStatus, nextStatus) {
		return ErrInvalidPhysicalCardInventoryTransition
	}

	item.Status = nextStatus
	switch nextStatus {
	case "available", "frozen", "scrapped":
		item.AssignedPassID = ""
		item.ActiveTaskID = ""
	}
	item.UpdatedAt = now
	return nil
}

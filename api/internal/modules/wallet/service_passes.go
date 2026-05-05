package wallet

import (
	"fmt"
	"strings"
	"time"
)

func (s *Service) ListTemplates(tenantID string) []PassTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]PassTemplate, 0, len(s.templates))
	for i := range s.templates {
		if s.templates[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, cloneTemplate(s.templates[i]))
	}
	return items
}

func (s *Service) CreateTemplate(tenantID, passType, classID, name string, styleConfig map[string]string, status, actor string) (PassTemplate, error) {
	nextPassType, err := normalizePassType(passType)
	if err != nil {
		return PassTemplate{}, err
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return PassTemplate{}, ErrTemplateNameRequired
	}

	nextStatus, err := normalizeTemplateStatus(status)
	if err != nil {
		return PassTemplate{}, err
	}

	nextClassID := strings.TrimSpace(classID)
	if nextClassID == "" {
		nextClassID = fmt.Sprintf("mistypass.%s.class", nextPassType)
	}

	id, err := walletID("wpt_")
	if err != nil {
		return PassTemplate{}, err
	}

	now := time.Now().UTC()
	record := PassTemplate{
		ID:          id,
		TenantID:    normalizeTenantID(tenantID),
		Provider:    "google",
		PassType:    nextPassType,
		ClassID:     nextClassID,
		Name:        nextName,
		StyleConfig: cloneStringMap(styleConfig),
		Status:      nextStatus,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	s.templates = append([]PassTemplate{record}, s.templates...)
	s.appendAuditLocked(record.TenantID, "wallet.template.create", normalizeActor(actor), record.ID, "success")
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return PassTemplate{}, err
	}
	s.mu.Unlock()

	return cloneTemplate(record), nil
}

func (s *Service) UpdateTemplateStatus(tenantID, templateID, status, actor string) (PassTemplate, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return PassTemplate{}, ErrTemplateIDRequired
	}

	nextStatus, err := normalizeTemplateStatus(status)
	if err != nil {
		return PassTemplate{}, err
	}

	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.templates {
		if s.templates[i].ID != nextTemplateID {
			continue
		}
		if s.templates[i].TenantID != nextTenantID {
			return PassTemplate{}, ErrTemplateNotFound
		}

		s.templates[i].Status = nextStatus
		s.templates[i].UpdatedAt = now
		s.appendAuditLocked(s.templates[i].TenantID, "wallet.template.status", normalizeActor(actor), s.templates[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassTemplate{}, err
		}
		return cloneTemplate(s.templates[i]), nil
	}

	return PassTemplate{}, ErrTemplateNotFound
}

func (s *Service) ListPasses(tenantID string) []PassInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]PassInstance, 0, len(s.passes))
	for i := range s.passes {
		if s.passes[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.passes[i])
	}
	return items
}

func (s *Service) GetPass(tenantID, passID string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.passes {
		if s.passes[i].ID == nextPassID {
			if s.passes[i].TenantID != nextTenantID {
				return PassInstance{}, ErrPassNotFound
			}
			return s.passes[i], nil
		}
	}
	return PassInstance{}, ErrPassNotFound
}

func (s *Service) IssuePass(tenantID, templateID, targetType, targetID, expiresAt, actor string) (PassInstance, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return PassInstance{}, ErrTemplateIDRequired
	}

	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return PassInstance{}, err
	}

	nextTargetID := strings.TrimSpace(targetID)
	if nextTargetID == "" {
		return PassInstance{}, ErrTargetIDRequired
	}

	now := time.Now().UTC()
	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return PassInstance{}, ErrTemplateInactive
	}

	record, err := s.createPassRecord(nextTenantID, template, nextTargetType, nextTargetID, "", "", strings.TrimSpace(expiresAt), nextActor, now)
	if err != nil {
		return PassInstance{}, err
	}

	s.passes = append([]PassInstance{record}, s.passes...)
	s.appendAuditLocked(nextTenantID, "wallet.pass.issue", nextActor, record.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PassInstance{}, err
	}

	return record, nil
}

func (s *Service) CreateUnassignedCard(tenantID, templateID, token, uid, cardNumber, cardType, expiresAt, actor string) (PassInstance, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return PassInstance{}, ErrTemplateIDRequired
	}

	now := time.Now().UTC()
	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return PassInstance{}, ErrTemplateInactive
	}

	id, err := walletID("wps_")
	if err != nil {
		return PassInstance{}, err
	}
	objectID := firstNonEmpty(strings.TrimSpace(token), strings.TrimSpace(uid), strings.TrimSpace(cardNumber))
	if objectID == "" {
		objectID = fmt.Sprintf("%s.%s", template.ClassID, id)
	}
	provider, credentialKind := normalizeCardCredentialProvider(cardType, token, uid, cardNumber, template.Provider)
	saveLink := ""
	if credentialKind == "google_wallet" {
		saveLink = fmt.Sprintf("https://pay.google.com/gp/v/save/%s", id)
	}
	record := PassInstance{
		ID:             id,
		TenantID:       nextTenantID,
		Provider:       provider,
		CredentialKind: credentialKind,
		TemplateID:     nextTemplateID,
		ObjectID:       objectID,
		Token:          strings.TrimSpace(token),
		UID:            strings.TrimSpace(uid),
		CardNumber:     strings.TrimSpace(cardNumber),
		Status:         "issued",
		SaveLink:       saveLink,
		ExpiresAt:      strings.TrimSpace(expiresAt),
		IssuedAt:       now,
		CreatedBy:      nextActor,
		UpdatedBy:      nextActor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.passes = append([]PassInstance{record}, s.passes...)
	s.appendAuditLocked(nextTenantID, "wallet.card.create", nextActor, record.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PassInstance{}, err
	}

	return record, nil
}

func (s *Service) EnrollApplePass(tenantID, userID, deviceID, passSerial, expiresAt, actor string) (PassInstance, error) {
	nextTargetID := strings.TrimSpace(userID)
	if nextTargetID == "" {
		return PassInstance{}, ErrTargetIDRequired
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextDeviceID := strings.TrimSpace(deviceID)
	nextPassSerial := strings.TrimSpace(passSerial)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].TenantID != nextTenantID || s.passes[i].TargetType != "user" || s.passes[i].TargetID != nextTargetID {
			continue
		}
		if credentialKindForProvider(s.passes[i].Provider, s.passes[i].TargetType) != "apple_wallet" && strings.ToLower(strings.TrimSpace(s.passes[i].CredentialKind)) != "apple_wallet" {
			continue
		}
		if s.passes[i].Status == "revoked" {
			continue
		}
		if nextPassSerial != "" && s.passes[i].ObjectID != nextPassSerial {
			continue
		}
		if nextDeviceID != "" && s.passes[i].Token != nextDeviceID {
			continue
		}
		if nextPassSerial == "" && nextDeviceID == "" {
			return s.passes[i], nil
		}
		if nextPassSerial != "" || nextDeviceID != "" {
			return s.passes[i], nil
		}
	}

	id, err := walletID("wps_")
	if err != nil {
		return PassInstance{}, err
	}
	objectID := nextPassSerial
	if objectID == "" {
		objectID = fmt.Sprintf("apple.%s.%s", nextTargetID, id)
	}
	record := PassInstance{
		ID:             id,
		TenantID:       nextTenantID,
		Provider:       "apple",
		CredentialKind: "apple_wallet",
		TemplateID:     "wpt_apple_pass_self",
		TargetType:     "user",
		TargetID:       nextTargetID,
		ObjectID:       objectID,
		Token:          nextDeviceID,
		Status:         "active",
		SaveLink:       "",
		ExpiresAt:      strings.TrimSpace(expiresAt),
		IssuedAt:       now,
		ActivatedAt:    &now,
		CreatedBy:      nextActor,
		UpdatedBy:      nextActor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.passes = append([]PassInstance{record}, s.passes...)
	s.appendAuditLocked(nextTenantID, "wallet.apple_pass.enroll", nextActor, record.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PassInstance{}, err
	}

	return record, nil
}

func (s *Service) AssignPass(tenantID, passID, targetType, targetID, actor string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}
	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return PassInstance{}, err
	}
	nextTargetID := strings.TrimSpace(targetID)
	if nextTargetID == "" {
		return PassInstance{}, ErrTargetIDRequired
	}
	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].ID != nextPassID {
			continue
		}
		if s.passes[i].TenantID != nextTenantID {
			return PassInstance{}, ErrPassNotFound
		}
		if s.passes[i].Status == "revoked" {
			return PassInstance{}, ErrInvalidPassTransition
		}
		s.passes[i].TargetType = nextTargetType
		s.passes[i].TargetID = nextTargetID
		s.passes[i].Status = "active"
		s.passes[i].UpdatedBy = nextActor
		s.passes[i].UpdatedAt = now
		value := now
		s.passes[i].ActivatedAt = &value
		s.passes[i].RevokedAt = nil

		s.appendAuditLocked(nextTenantID, "wallet.card.assign", nextActor, s.passes[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassInstance{}, err
		}

		return s.passes[i], nil
	}

	return PassInstance{}, ErrPassNotFound
}

func (s *Service) DeassignPass(tenantID, passID, actor string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}
	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].ID != nextPassID {
			continue
		}
		if s.passes[i].TenantID != nextTenantID {
			return PassInstance{}, ErrPassNotFound
		}
		if s.passes[i].Status == "revoked" {
			return PassInstance{}, ErrInvalidPassTransition
		}
		s.passes[i].TargetType = ""
		s.passes[i].TargetID = ""
		s.passes[i].Status = "issued"
		s.passes[i].UpdatedBy = nextActor
		s.passes[i].UpdatedAt = now
		s.passes[i].ActivatedAt = nil

		s.appendAuditLocked(nextTenantID, "wallet.card.deassign", nextActor, s.passes[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassInstance{}, err
		}

		return s.passes[i], nil
	}

	return PassInstance{}, ErrPassNotFound
}

func (s *Service) IssuePassBatch(tenantID, templateID, targetType string, targetIDs []string, expiresAt, actor string) ([]IssueJob, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return nil, ErrTemplateIDRequired
	}

	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return nil, err
	}

	if len(targetIDs) == 0 {
		return nil, ErrTargetIDsRequired
	}

	batchID, err := walletID("wbt_")
	if err != nil {
		return nil, err
	}

	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return nil, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return nil, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return nil, ErrTemplateInactive
	}

	jobs := make([]IssueJob, 0, len(targetIDs))
	for _, rawTargetID := range targetIDs {
		targetID := strings.TrimSpace(rawTargetID)
		jobID, idErr := walletID("wjb_")
		if idErr != nil {
			return nil, idErr
		}

		job := IssueJob{
			ID:         jobID,
			TenantID:   nextTenantID,
			Provider:   firstNonEmpty(template.Provider, "google"),
			BatchID:    batchID,
			TemplateID: nextTemplateID,
			TargetType: nextTargetType,
			TargetID:   targetID,
			ExpiresAt:  strings.TrimSpace(expiresAt),
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if targetID == "" {
			job.Status = "failed"
			job.ErrorCode = "target_id_required"
			job.ErrorMessage = ErrTargetIDRequired.Error()
			s.jobs = append([]IssueJob{job}, s.jobs...)
			jobs = append(jobs, job)
			continue
		}

		record, issueErr := s.createPassRecord(nextTenantID, template, nextTargetType, targetID, "", "", strings.TrimSpace(expiresAt), nextActor, now)
		if issueErr != nil {
			job.Status = "failed"
			job.ErrorCode = "issue_failed"
			job.ErrorMessage = issueErr.Error()
			s.jobs = append([]IssueJob{job}, s.jobs...)
			jobs = append(jobs, job)
			continue
		}

		s.passes = append([]PassInstance{record}, s.passes...)
		job.Status = "success"
		job.PassID = record.ID
		s.jobs = append([]IssueJob{job}, s.jobs...)
		jobs = append(jobs, job)
	}

	s.appendAuditLocked(nextTenantID, "wallet.pass.issue_batch", nextActor, batchID, "success")
	if err := s.persistLocked(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *Service) IssuePassBatchQueued(tenantID, templateID, targetType string, targetIDs []string, expiresAt, actor string) ([]IssueJob, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return nil, ErrTemplateIDRequired
	}

	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return nil, err
	}

	if len(targetIDs) == 0 {
		return nil, ErrTargetIDsRequired
	}

	batchID, err := walletID("wbt_")
	if err != nil {
		return nil, err
	}

	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return nil, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return nil, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return nil, ErrTemplateInactive
	}

	jobs := make([]IssueJob, 0, len(targetIDs))
	for _, rawTargetID := range targetIDs {
		targetID := strings.TrimSpace(rawTargetID)
		jobID, idErr := walletID("wjb_")
		if idErr != nil {
			return nil, idErr
		}

		job := IssueJob{
			ID:         jobID,
			TenantID:   nextTenantID,
			Provider:   firstNonEmpty(template.Provider, "google"),
			BatchID:    batchID,
			TemplateID: nextTemplateID,
			TargetType: nextTargetType,
			TargetID:   targetID,
			ExpiresAt:  strings.TrimSpace(expiresAt),
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if targetID == "" {
			job.Status = "failed"
			job.ErrorCode = "target_id_required"
			job.ErrorMessage = ErrTargetIDRequired.Error()
		}

		s.jobs = append([]IssueJob{job}, s.jobs...)
		jobs = append(jobs, job)
	}

	s.appendAuditLocked(nextTenantID, "wallet.pass.issue_batch_queued", nextActor, batchID, "success")
	if err := s.persistLocked(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *Service) GetSaveLink(tenantID, passID string) (string, error) {
	record, err := s.GetPass(tenantID, passID)
	if err != nil {
		return "", err
	}
	return record.SaveLink, nil
}

func (s *Service) SuspendPass(tenantID, passID, actor string) (PassInstance, error) {
	return s.updatePassStatus(tenantID, passID, "suspended", actor)
}

func (s *Service) ActivatePass(tenantID, passID, actor string) (PassInstance, error) {
	return s.updatePassStatus(tenantID, passID, "active", actor)
}

func (s *Service) RevokePass(tenantID, passID, actor string) (PassInstance, error) {
	return s.updatePassStatus(tenantID, passID, "revoked", actor)
}


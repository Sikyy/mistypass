package camera

import (
	"context"
	"errors"
	"testing"
)

func TestServiceCreateCamera(t *testing.T) {
	svc := NewService(nil)
	cam, err := svc.Create(CameraCreateRequest{
		TenantID: "t1",
		PlaceID:  "p1",
		Name:     "Lobby Camera",
		Provider: "hikvision",
		Host:     "192.168.1.100",
		Username: "admin",
		Password: "pass123",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cam.Name != "Lobby Camera" {
		t.Fatalf("expected name Lobby Camera, got %q", cam.Name)
	}
	if cam.Provider != "hikvision" {
		t.Fatalf("expected provider hikvision, got %q", cam.Provider)
	}
	if cam.Status != "active" {
		t.Fatalf("expected status active, got %q", cam.Status)
	}
	if cam.Port != 80 {
		t.Fatalf("expected default port 80, got %d", cam.Port)
	}
	if cam.Channel != 1 {
		t.Fatalf("expected default channel 1, got %d", cam.Channel)
	}
}

func TestServiceCreateCameraValidation(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.Create(CameraCreateRequest{Provider: "hikvision", Host: "x"}); !errors.Is(err, ErrCameraNameRequired) {
		t.Fatalf("expected name required, got %v", err)
	}
	if _, err := svc.Create(CameraCreateRequest{Name: "x", Provider: "hikvision"}); !errors.Is(err, ErrCameraHostRequired) {
		t.Fatalf("expected host required, got %v", err)
	}
	if _, err := svc.Create(CameraCreateRequest{Name: "x", Host: "x", Provider: "invalid"}); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("expected invalid provider, got %v", err)
	}
}

func TestServiceListCameras(t *testing.T) {
	svc := NewService(nil)
	svc.Create(CameraCreateRequest{TenantID: "t1", Name: "Cam1", Provider: "onvif", Host: "1.1.1.1"})
	svc.Create(CameraCreateRequest{TenantID: "t2", Name: "Cam2", Provider: "dahua", Host: "2.2.2.2"})

	all := svc.List("")
	if len(all) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(all))
	}
	t1Only := svc.List("t1")
	if len(t1Only) != 1 {
		t.Fatalf("expected 1 camera for t1, got %d", len(t1Only))
	}
}

func TestServiceGetCamera(t *testing.T) {
	svc := NewService(nil)
	created, _ := svc.Create(CameraCreateRequest{TenantID: "t1", Name: "Test", Provider: "vivotek", Host: "x"})
	cam, err := svc.Get("t1", created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cam.ID != created.ID {
		t.Fatalf("ID mismatch")
	}
	if _, err := svc.Get("t1", "nonexistent"); !errors.Is(err, ErrCameraNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestServiceUpdateCamera(t *testing.T) {
	svc := NewService(nil)
	created, _ := svc.Create(CameraCreateRequest{TenantID: "t1", Name: "Old", Provider: "hikvision", Host: "1.1.1.1"})
	newName := "New Name"
	updated, err := svc.Update("t1", created.ID, CameraUpdateRequest{Name: &newName})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Fatalf("expected New Name, got %q", updated.Name)
	}
}

func TestServiceDeleteCamera(t *testing.T) {
	svc := NewService(nil)
	created, _ := svc.Create(CameraCreateRequest{TenantID: "t1", Name: "Del", Provider: "onvif", Host: "x"})
	if err := svc.Delete("t1", created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get("t1", created.ID); !errors.Is(err, ErrCameraNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestServiceAllProviders(t *testing.T) {
	svc := NewService(nil)
	providers := []string{"onvif", "hikvision", "dahua", "zkteco", "vivotek"}
	for _, p := range providers {
		caps, err := svc.GetProviderCapabilities(p)
		if err != nil {
			t.Fatalf("provider %s: %v", p, err)
		}
		if !caps.SupportsSnapshot {
			t.Fatalf("provider %s should support snapshot", p)
		}
		if !caps.SupportsRTSP {
			t.Fatalf("provider %s should support RTSP", p)
		}
	}
}

func TestServiceTriggerEventSnapshotNoMatch(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()
	results := svc.TriggerEventSnapshot(ctx, "t1", "door-999", "evt-1", "unlock")
	if len(results) != 0 {
		t.Fatalf("expected 0 snapshots for unmatched door, got %d", len(results))
	}
}

func TestServiceVideoLinkFormat(t *testing.T) {
	svc := NewService(nil)
	cam, _ := svc.Create(CameraCreateRequest{
		TenantID: "t1", Name: "Test", Provider: "hikvision",
		Host: "192.168.1.50", Username: "admin", Password: "pass",
	})
	ctx := context.Background()
	link, err := svc.GetVideoLink(ctx, "t1", cam.ID)
	if err != nil {
		t.Fatalf("VideoLink: %v", err)
	}
	if link == "" {
		t.Fatal("expected non-empty video link")
	}
}

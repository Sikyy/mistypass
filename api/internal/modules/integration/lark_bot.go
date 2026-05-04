package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LarkBotSender sends messages to Lark groups via Custom Bot Webhook.
// Each group has its own webhook URL configured in the system.
type LarkBotSender struct {
	client *http.Client
}

func NewLarkBotSender() *LarkBotSender {
	return &LarkBotSender{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// SendText sends a plain text message to a Lark group.
func (s *LarkBotSender) SendText(ctx context.Context, webhookURL, text string) error {
	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": text},
	}
	return s.send(ctx, webhookURL, payload)
}

// SendAccessAlert sends a formatted access control alert card.
func (s *LarkBotSender) SendAccessAlert(ctx context.Context, webhookURL string, alert AccessAlertMessage) error {
	card := buildAccessAlertCard(alert)
	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}
	return s.send(ctx, webhookURL, payload)
}

// SendVisitorNotification sends a visitor arrival notification.
func (s *LarkBotSender) SendVisitorNotification(ctx context.Context, webhookURL string, visitor VisitorMessage) error {
	card := buildVisitorCard(visitor)
	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}
	return s.send(ctx, webhookURL, payload)
}

func (s *LarkBotSender) send(ctx context.Context, webhookURL string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("lark bot send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lark bot: status %d", resp.StatusCode)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 0 {
		return fmt.Errorf("lark bot error: code=%d msg=%s", result.Code, result.Msg)
	}
	return nil
}

// --- Message types ---

type AccessAlertMessage struct {
	EventType string // "access_granted", "access_denied", "door_held_open", "alarm"
	DoorName  string
	UserName  string
	UserEmail string
	Time      string
	Reason    string
}

type VisitorMessage struct {
	VisitorName  string
	HostName     string
	Purpose      string
	ArrivalTime  string
	BuildingName string
}

// --- Card builders ---

func buildAccessAlertCard(alert AccessAlertMessage) map[string]any {
	headerColor := "green"
	headerTitle := "Access Granted"
	switch alert.EventType {
	case "access_denied":
		headerColor = "red"
		headerTitle = "Access Denied"
	case "door_held_open":
		headerColor = "orange"
		headerTitle = "Door Held Open"
	case "alarm":
		headerColor = "red"
		headerTitle = "Security Alarm"
	}

	content := fmt.Sprintf("**Door:** %s\n**User:** %s\n**Time:** %s",
		alert.DoorName, alert.UserName, alert.Time)
	if alert.Reason != "" {
		content += fmt.Sprintf("\n**Reason:** %s", alert.Reason)
	}

	return map[string]any{
		"header": map[string]any{
			"title":    map[string]string{"tag": "plain_text", "content": headerTitle},
			"template": headerColor,
		},
		"elements": []map[string]any{
			{
				"tag":  "div",
				"text": map[string]string{"tag": "lark_md", "content": content},
			},
		},
	}
}

func buildVisitorCard(visitor VisitorMessage) map[string]any {
	content := fmt.Sprintf("**Visitor:** %s\n**Host:** %s\n**Building:** %s\n**Time:** %s",
		visitor.VisitorName, visitor.HostName, visitor.BuildingName, visitor.ArrivalTime)
	if visitor.Purpose != "" {
		content += fmt.Sprintf("\n**Purpose:** %s", visitor.Purpose)
	}

	return map[string]any{
		"header": map[string]any{
			"title":    map[string]string{"tag": "plain_text", "content": "Visitor Arrived"},
			"template": "blue",
		},
		"elements": []map[string]any{
			{
				"tag":  "div",
				"text": map[string]string{"tag": "lark_md", "content": content},
			},
		},
	}
}

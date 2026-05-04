package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// larkBotSender sends plain text messages to a Lark group webhook.
// This is a lightweight sender embedded in the wallet package to avoid
// a circular dependency with the integration package.
type larkBotSender struct {
	client *http.Client
}

var sharedLarkBotSender *larkBotSender

func larkBotSenderInstance() *larkBotSender {
	if sharedLarkBotSender == nil {
		sharedLarkBotSender = &larkBotSender{
			client: &http.Client{Timeout: 10 * time.Second},
		}
	}
	return sharedLarkBotSender
}

func (s *larkBotSender) SendText(ctx context.Context, webhookURL, text string) error {
	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": text},
	}
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
		return fmt.Errorf("lark bot: %w", err)
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
		return fmt.Errorf("lark bot: code=%d msg=%s", result.Code, result.Msg)
	}
	return nil
}

// Ensure strings import is used
var _ = strings.TrimSpace

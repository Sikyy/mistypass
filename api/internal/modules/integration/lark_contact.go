package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// LarkUser represents an employee from Lark's Contact API.
type LarkUser struct {
	UserID       string `json:"user_id"`
	OpenID       string `json:"open_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Mobile       string `json:"mobile"`
	DepartmentID string `json:"department_id"`
	Status       struct {
		IsActivated bool `json:"is_activated"`
		IsFrozen    bool `json:"is_frozen"`
		IsResigned  bool `json:"is_resigned"`
	} `json:"status"`
	JoinTime int64 `json:"join_time"`
}

// LarkContactSync handles employee directory synchronization from Lark.
type LarkContactSync struct {
	client *LarkClient
}

func NewLarkContactSync(client *LarkClient) *LarkContactSync {
	return &LarkContactSync{client: client}
}

// ListAllUsers fetches the complete employee directory from Lark.
// Uses pagination to handle large organizations.
func (s *LarkContactSync) ListAllUsers(ctx context.Context) ([]LarkUser, error) {
	var allUsers []LarkUser
	pageToken := ""

	for {
		path := "/contact/v3/users?department_id=0&page_size=50&user_id_type=user_id"
		if pageToken != "" {
			path += "&page_token=" + pageToken
		}

		respBody, err := s.client.DoRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, fmt.Errorf("lark list users: %w", err)
		}

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items     []LarkUser `json:"items"`
				PageToken string     `json:"page_token"`
				HasMore   bool       `json:"has_more"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("lark decode users: %w", err)
		}
		if result.Code != 0 {
			return nil, fmt.Errorf("lark users error: code=%d msg=%s", result.Code, result.Msg)
		}

		allUsers = append(allUsers, result.Data.Items...)

		if !result.Data.HasMore || result.Data.PageToken == "" {
			break
		}
		pageToken = result.Data.PageToken
	}

	return allUsers, nil
}

// GetUser fetches a single user by user_id.
func (s *LarkContactSync) GetUser(ctx context.Context, userID string) (*LarkUser, error) {
	path := fmt.Sprintf("/contact/v3/users/%s?user_id_type=user_id", userID)

	respBody, err := s.client.DoRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			User LarkUser `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("lark get user: code=%d msg=%s", result.Code, result.Msg)
	}
	return &result.Data.User, nil
}

// --- Sync result types ---

// SyncAction represents what should happen to an employee in MistyPass.
type SyncAction struct {
	Action    string   `json:"action"` // "create", "update", "deactivate", "reactivate"
	LarkUser  LarkUser `json:"lark_user"`
	Reason    string   `json:"reason,omitempty"`
}

// ComputeSyncActions compares Lark users with existing MistyPass users
// and returns the actions needed to bring them in sync.
func ComputeSyncActions(larkUsers []LarkUser, existingEmails map[string]string) []SyncAction {
	var actions []SyncAction
	larkEmailSet := make(map[string]struct{})

	for _, lu := range larkUsers {
		if lu.Email == "" {
			continue
		}
		larkEmailSet[lu.Email] = struct{}{}

		_, existsInMistyPass := existingEmails[lu.Email]

		if lu.Status.IsResigned {
			if existsInMistyPass {
				actions = append(actions, SyncAction{
					Action:   "deactivate",
					LarkUser: lu,
					Reason:   "resigned in Lark",
				})
			}
			continue
		}

		if lu.Status.IsFrozen {
			if existsInMistyPass {
				actions = append(actions, SyncAction{
					Action:   "deactivate",
					LarkUser: lu,
					Reason:   "frozen in Lark",
				})
			}
			continue
		}

		if !existsInMistyPass && lu.Status.IsActivated {
			actions = append(actions, SyncAction{
				Action:   "create",
				LarkUser: lu,
				Reason:   "new employee in Lark",
			})
		} else if existsInMistyPass && lu.Status.IsActivated {
			actions = append(actions, SyncAction{
				Action:   "update",
				LarkUser: lu,
			})
		}
	}

	// Employees in MistyPass but not in Lark → deactivate
	for email := range existingEmails {
		if _, inLark := larkEmailSet[email]; !inLark {
			actions = append(actions, SyncAction{
				Action: "deactivate",
				LarkUser: LarkUser{Email: email},
				Reason: "not found in Lark directory",
			})
		}
	}

	return actions
}

// --- Event subscription types ---

// LarkEvent represents a callback event from Lark Event Subscription.
type LarkEvent struct {
	Schema string          `json:"schema"`
	Header LarkEventHeader `json:"header"`
	Event  json.RawMessage `json:"event"`
}

type LarkEventHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	AppID     string `json:"app_id"`
	TenantKey string `json:"tenant_key"`
	Token     string `json:"token"`
}

// LarkUserCreatedEvent is the payload for contact.user.created_v3
type LarkUserCreatedEvent struct {
	Object LarkUser `json:"object"`
}

// LarkUserDeletedEvent is the payload for contact.user.deleted_v3
type LarkUserDeletedEvent struct {
	Object struct {
		UserID string `json:"user_id"`
		OpenID string `json:"open_id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
	} `json:"object"`
}

// Placeholder to use time package
var _ = time.Now

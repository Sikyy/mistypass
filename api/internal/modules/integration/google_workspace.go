package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
)

// GoogleWorkspaceConfig holds configuration for Google Workspace Directory API.
type GoogleWorkspaceConfig struct {
	ServiceAccountJSON []byte // JSON key file content
	DelegatedAdmin     string // admin email for domain-wide delegation
	Domain             string // company domain (e.g. "company.com")
}

// GoogleWorkspaceClient manages Directory API access.
type GoogleWorkspaceClient struct {
	config     GoogleWorkspaceConfig
	httpClient *http.Client
}

// NewGoogleWorkspaceClient creates a client with service account auth.
func NewGoogleWorkspaceClient(cfg GoogleWorkspaceConfig) (*GoogleWorkspaceClient, error) {
	jwtConfig, err := google.JWTConfigFromJSON(
		cfg.ServiceAccountJSON,
		"https://www.googleapis.com/auth/admin.directory.user.readonly",
	)
	if err != nil {
		return nil, fmt.Errorf("google workspace: parse service account: %w", err)
	}
	jwtConfig.Subject = cfg.DelegatedAdmin

	return &GoogleWorkspaceClient{
		config:     cfg,
		httpClient: jwtConfig.Client(context.Background()),
	}, nil
}

// GoogleWorkspaceUser represents a user from Google Workspace Directory.
type GoogleWorkspaceUser struct {
	ID             string `json:"id"`
	PrimaryEmail   string `json:"primaryEmail"`
	FullName       string `json:"fullName"`
	GivenName      string `json:"givenName"`
	FamilyName     string `json:"familyName"`
	OrgUnitPath    string `json:"orgUnitPath"`
	Suspended      bool   `json:"suspended"`
	CreationTime   string `json:"creationTime"`
	LastLoginTime  string `json:"lastLoginTime"`
}

// ListAllUsers fetches the complete employee directory.
func (c *GoogleWorkspaceClient) ListAllUsers(ctx context.Context) ([]GoogleWorkspaceUser, error) {
	var allUsers []GoogleWorkspaceUser
	pageToken := ""

	for {
		url := fmt.Sprintf(
			"https://admin.googleapis.com/admin/directory/v1/users?domain=%s&maxResults=100&orderBy=email",
			c.config.Domain,
		)
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("google directory: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("google directory: status %d body=%s", resp.StatusCode, string(body))
		}

		var result struct {
			Users []struct {
				ID           string `json:"id"`
				PrimaryEmail string `json:"primaryEmail"`
				Name         struct {
					FullName   string `json:"fullName"`
					GivenName  string `json:"givenName"`
					FamilyName string `json:"familyName"`
				} `json:"name"`
				OrgUnitPath   string `json:"orgUnitPath"`
				Suspended     bool   `json:"suspended"`
				CreationTime  string `json:"creationTime"`
				LastLoginTime string `json:"lastLoginTime"`
			} `json:"users"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, u := range result.Users {
			allUsers = append(allUsers, GoogleWorkspaceUser{
				ID:            u.ID,
				PrimaryEmail:  u.PrimaryEmail,
				FullName:      u.Name.FullName,
				GivenName:     u.Name.GivenName,
				FamilyName:    u.Name.FamilyName,
				OrgUnitPath:   u.OrgUnitPath,
				Suspended:     u.Suspended,
				CreationTime:  u.CreationTime,
				LastLoginTime: u.LastLoginTime,
			})
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return allUsers, nil
}

// ComputeGWSyncActions compares Google Workspace users with existing MistyPass users.
func ComputeGWSyncActions(gwsUsers []GoogleWorkspaceUser, existingEmails map[string]string) []SyncAction {
	var actions []SyncAction
	gwsEmailSet := make(map[string]struct{})

	for _, gu := range gwsUsers {
		email := strings.ToLower(gu.PrimaryEmail)
		gwsEmailSet[email] = struct{}{}

		_, existsInMistyPass := existingEmails[email]

		if gu.Suspended {
			if existsInMistyPass {
				actions = append(actions, SyncAction{
					Action: "deactivate",
					LarkUser: LarkUser{
						Email: gu.PrimaryEmail,
						Name:  gu.FullName,
					},
					Reason: "suspended in Google Workspace",
				})
			}
			continue
		}

		if !existsInMistyPass {
			actions = append(actions, SyncAction{
				Action: "create",
				LarkUser: LarkUser{
					Email: gu.PrimaryEmail,
					Name:  gu.FullName,
				},
				Reason: "new employee in Google Workspace",
			})
		}
	}

	for email := range existingEmails {
		if _, inGWS := gwsEmailSet[strings.ToLower(email)]; !inGWS {
			actions = append(actions, SyncAction{
				Action:   "deactivate",
				LarkUser: LarkUser{Email: email},
				Reason:   "not found in Google Workspace directory",
			})
		}
	}

	return actions
}

// Ensure imports are used
var _ = jwt.Config{}
var _ = time.Now

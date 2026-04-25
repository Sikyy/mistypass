package talenta

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

var ErrWebhookSecretRequired = errors.New("talenta webhook secret is required")
var ErrWebhookAuthorizationHeaderRequired = errors.New("talenta webhook authorization header is required")
var ErrWebhookDateHeaderRequired = errors.New("talenta webhook date header is required")
var ErrWebhookDigestHeaderRequired = errors.New("talenta webhook digest header is required")
var ErrInvalidWebhookAuthorization = errors.New("invalid talenta webhook authorization header")
var ErrInvalidWebhookDate = errors.New("invalid talenta webhook date header")
var ErrInvalidWebhookDigest = errors.New("invalid talenta webhook digest header")
var ErrWebhookClientIDMismatch = errors.New("talenta webhook client_id mismatch")
var ErrWebhookDateSkewExceeded = errors.New("talenta webhook date skew exceeds 300 seconds")
var ErrWebhookDigestMismatch = errors.New("talenta webhook digest mismatch")
var ErrWebhookSignatureMismatch = errors.New("talenta webhook signature mismatch")

const (
	hmacAuthorizationScheme   = "hmac"
	hmacAuthorizationAlgo     = "hmac-sha256"
	hmacSignedHeadersValue    = "date request-line"
	hmacDigestAlgorithm       = "SHA-256"
	maxWebhookDateClockSkew   = 300 * time.Second
	defaultWebhookHTTPVersion = "HTTP/1.1"
)

type WebhookSignatureInput struct {
	Authorization string
	Date          string
	Digest        string
	Method        string
	RequestURI    string
	Proto         string
	Body          []byte
	ClientID      string
	ClientSecret  string
	Now           time.Time
}

func VerifyWebhookSignature(input WebhookSignatureInput) error {
	clientSecret := strings.TrimSpace(input.ClientSecret)
	if clientSecret == "" {
		return ErrWebhookSecretRequired
	}

	authorization := strings.TrimSpace(input.Authorization)
	if authorization == "" {
		return ErrWebhookAuthorizationHeaderRequired
	}

	dateHeader := strings.TrimSpace(input.Date)
	if dateHeader == "" {
		return ErrWebhookDateHeaderRequired
	}
	parsedDate, err := http.ParseTime(dateHeader)
	if err != nil {
		return ErrInvalidWebhookDate
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if absDuration(now.Sub(parsedDate)) > maxWebhookDateClockSkew {
		return ErrWebhookDateSkewExceeded
	}

	digestHeader := strings.TrimSpace(input.Digest)
	if requiresWebhookDigest(input.Method, input.Body) && digestHeader == "" {
		return ErrWebhookDigestHeaderRequired
	}
	if digestHeader != "" {
		if err := verifyWebhookDigest(input.Body, digestHeader); err != nil {
			return err
		}
	}

	params, err := parseWebhookAuthorization(authorization)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(params["algorithm"]), hmacAuthorizationAlgo) {
		return ErrInvalidWebhookAuthorization
	}
	if normalizeSignedHeaders(params["headers"]) != hmacSignedHeadersValue {
		return ErrInvalidWebhookAuthorization
	}

	expectedClientID := strings.TrimSpace(input.ClientID)
	if expectedClientID != "" && strings.TrimSpace(params["username"]) != expectedClientID {
		return ErrWebhookClientIDMismatch
	}

	signature := strings.TrimSpace(params["signature"])
	if signature == "" {
		return ErrInvalidWebhookAuthorization
	}

	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(webhookStringToSign(dateHeader, input.Method, input.RequestURI, input.Proto)))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) != 1 {
		return ErrWebhookSignatureMismatch
	}
	return nil
}

func verifyWebhookDigest(body []byte, digestHeader string) error {
	parts := strings.SplitN(strings.TrimSpace(digestHeader), "=", 2)
	if len(parts) != 2 {
		return ErrInvalidWebhookDigest
	}
	if !strings.EqualFold(strings.TrimSpace(parts[0]), hmacDigestAlgorithm) {
		return ErrInvalidWebhookDigest
	}
	expected := webhookDigest(body)
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(parts[1])), []byte(expected)) != 1 {
		return ErrWebhookDigestMismatch
	}
	return nil
}

func webhookDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func webhookStringToSign(dateHeader, method, requestURI, proto string) string {
	nextMethod := strings.ToUpper(strings.TrimSpace(method))
	if nextMethod == "" {
		nextMethod = http.MethodPost
	}
	nextRequestURI := strings.TrimSpace(requestURI)
	if nextRequestURI == "" {
		nextRequestURI = "/"
	}
	nextProto := strings.TrimSpace(proto)
	if nextProto == "" {
		nextProto = defaultWebhookHTTPVersion
	}
	return "date: " + strings.TrimSpace(dateHeader) + "\n" + nextMethod + " " + nextRequestURI + " " + nextProto
}

func parseWebhookAuthorization(header string) (map[string]string, error) {
	nextHeader := strings.TrimSpace(header)
	if nextHeader == "" {
		return nil, ErrWebhookAuthorizationHeaderRequired
	}

	parts := strings.SplitN(nextHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), hmacAuthorizationScheme) {
		return nil, ErrInvalidWebhookAuthorization
	}

	items := splitAuthorizationParams(parts[1])
	if len(items) == 0 {
		return nil, ErrInvalidWebhookAuthorization
	}

	params := make(map[string]string, len(items))
	for i := range items {
		item := strings.TrimSpace(items[i])
		if item == "" {
			continue
		}
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return nil, ErrInvalidWebhookAuthorization
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		if key == "" || value == "" {
			return nil, ErrInvalidWebhookAuthorization
		}
		params[key] = value
	}

	if params["username"] == "" || params["algorithm"] == "" || params["headers"] == "" || params["signature"] == "" {
		return nil, ErrInvalidWebhookAuthorization
	}
	return params, nil
}

func splitAuthorizationParams(input string) []string {
	items := make([]string, 0, 4)
	var current strings.Builder
	inQuotes := false

	for _, char := range input {
		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case ',':
			if inQuotes {
				current.WriteRune(char)
				continue
			}
			items = append(items, current.String())
			current.Reset()
		default:
			current.WriteRune(char)
		}
	}
	if current.Len() > 0 {
		items = append(items, current.String())
	}
	return items
}

func normalizeSignedHeaders(input string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(input)))
	return strings.Join(fields, " ")
}

func requiresWebhookDigest(method string, body []byte) bool {
	nextMethod := strings.ToUpper(strings.TrimSpace(method))
	if len(body) == 0 {
		return nextMethod == http.MethodPost || nextMethod == http.MethodPut || nextMethod == http.MethodPatch
	}
	switch nextMethod {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type referenceAlertPolicyConditionPreviewRequest struct {
	AlertPolicy *struct {
		TenantID  string         `json:"tenant_id"`
		PolicyID  string         `json:"policy_id"`
		Condition string         `json:"condition_expression"`
		Event     map[string]any `json:"event"`
	} `json:"alert_policy"`
	TenantID  string         `json:"tenant_id"`
	PolicyID  string         `json:"policy_id"`
	Condition string         `json:"condition_expression"`
	Event     map[string]any `json:"event"`
}

type referenceAlertPolicyConditionPreviewPayload struct {
	TenantID  string
	PolicyID  string
	Condition string
	Event     map[string]any
}

type referenceAlertPolicyConditionPreviewResponse struct {
	PolicyID  string         `json:"policy_id,omitempty"`
	Condition string         `json:"condition_expression"`
	Matched   bool           `json:"matched"`
	Event     map[string]any `json:"event"`
}

type referenceAlertPolicyEventEvaluationRequest struct {
	AlertPolicy *struct {
		TenantID  string         `json:"tenant_id"`
		PolicyIDs []string       `json:"policy_ids"`
		Event     map[string]any `json:"event"`
	} `json:"alert_policy"`
	TenantID  string         `json:"tenant_id"`
	PolicyIDs []string       `json:"policy_ids"`
	Event     map[string]any `json:"event"`
}

type referenceAlertPolicyEventEvaluationPayload struct {
	TenantID  string
	PolicyIDs []string
	Event     map[string]any
}

type referenceAlertPolicyEventEvaluationMatch struct {
	PolicyID            string                       `json:"policy_id"`
	Name                string                       `json:"name"`
	Trigger             string                       `json:"trigger"`
	Severity            string                       `json:"severity"`
	Condition           string                       `json:"condition_expression,omitempty"`
	Channels            referenceAlertPolicyChannels `json:"channels"`
	ReceiverGroups      []string                     `json:"receiver_groups"`
	WindowSeconds       int64                        `json:"window_seconds"`
	CooldownSeconds     int64                        `json:"cooldown_seconds"`
	NotificationSummary string                       `json:"notification_summary"`
}

type referenceAlertPolicyEventEvaluationError struct {
	PolicyID string `json:"policy_id"`
	Error    string `json:"error"`
}

type referenceAlertPolicyEventEvaluationResponse struct {
	TenantID       string                                     `json:"tenant_id"`
	Event          map[string]any                             `json:"event"`
	EvaluatedCount int                                        `json:"evaluated_count"`
	MatchedCount   int                                        `json:"matched_count"`
	Matches        []referenceAlertPolicyEventEvaluationMatch `json:"matches"`
	Errors         []referenceAlertPolicyEventEvaluationError `json:"errors,omitempty"`
}

func (request referenceAlertPolicyConditionPreviewRequest) payload() referenceAlertPolicyConditionPreviewPayload {
	if request.AlertPolicy != nil {
		return referenceAlertPolicyConditionPreviewPayload{
			TenantID:  request.AlertPolicy.TenantID,
			PolicyID:  request.AlertPolicy.PolicyID,
			Condition: request.AlertPolicy.Condition,
			Event:     request.AlertPolicy.Event,
		}
	}
	return referenceAlertPolicyConditionPreviewPayload{
		TenantID:  request.TenantID,
		PolicyID:  request.PolicyID,
		Condition: request.Condition,
		Event:     request.Event,
	}
}

func (request referenceAlertPolicyEventEvaluationRequest) payload() referenceAlertPolicyEventEvaluationPayload {
	if request.AlertPolicy != nil {
		return referenceAlertPolicyEventEvaluationPayload{
			TenantID:  request.AlertPolicy.TenantID,
			PolicyIDs: request.AlertPolicy.PolicyIDs,
			Event:     request.AlertPolicy.Event,
		}
	}
	return referenceAlertPolicyEventEvaluationPayload{
		TenantID:  request.TenantID,
		PolicyIDs: request.PolicyIDs,
		Event:     request.Event,
	}
}

func (s *server) previewReferenceAlertPolicyCondition(w http.ResponseWriter, r *http.Request) {
	var request referenceAlertPolicyConditionPreviewRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	policyID := strings.TrimSpace(payload.PolicyID)
	condition := strings.TrimSpace(payload.Condition)
	tenantID := strings.TrimSpace(payload.TenantID)
	if policyID != "" {
		policy, exists := s.referenceCustomAlertPolicyByID(policyID)
		if !exists {
			writeError(w, http.StatusNotFound, "alert policy not found")
			return
		}
		if tenantID != "" && tenantID != policy.TenantID {
			writeError(w, http.StatusForbidden, "tenant scope forbidden")
			return
		}
		if tenantID == "" {
			tenantID = policy.TenantID
		}
		if condition == "" {
			condition = policy.Condition
		}
	}
	if _, ok := s.resolveTenantID(w, r, tenantID); !ok {
		return
	}
	event := payload.Event
	if len(event) == 0 {
		event = defaultReferenceAlertPolicyPreviewEvent()
	}
	matched, err := evaluateReferenceAlertPolicyCondition(condition, event)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, referenceAlertPolicyConditionPreviewResponse{
		PolicyID:  policyID,
		Condition: condition,
		Matched:   matched,
		Event:     event,
	})
}

func (s *server) evaluateReferenceAlertPoliciesForEvent(w http.ResponseWriter, r *http.Request) {
	var request referenceAlertPolicyEventEvaluationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	event := payload.Event
	if len(event) == 0 {
		writeError(w, http.StatusBadRequest, "event is required")
		return
	}

	policyFilter := make(map[string]struct{}, len(payload.PolicyIDs))
	for _, policyID := range payload.PolicyIDs {
		if nextID := strings.TrimSpace(policyID); nextID != "" {
			policyFilter[nextID] = struct{}{}
		}
	}
	matches := []referenceAlertPolicyEventEvaluationMatch{}
	evaluationErrors := []referenceAlertPolicyEventEvaluationError{}
	evaluatedCount := 0
	selectedCount := 0
	for _, policy := range s.referenceCustomAlertPolicies(tenantID) {
		if len(policyFilter) > 0 {
			if _, exists := policyFilter[policy.ID]; !exists {
				continue
			}
		}
		selectedCount++
		if !policy.Enabled || !strings.EqualFold(policy.Status, "active") {
			continue
		}
		evaluatedCount++
		matched, err := evaluateReferenceAlertPolicyCondition(policy.Condition, event)
		if err != nil {
			evaluationErrors = append(evaluationErrors, referenceAlertPolicyEventEvaluationError{
				PolicyID: policy.ID,
				Error:    err.Error(),
			})
			continue
		}
		if !matched {
			continue
		}
		matches = append(matches, referenceAlertPolicyEventEvaluationMatch{
			PolicyID:            policy.ID,
			Name:                policy.Name,
			Trigger:             policy.Trigger,
			Severity:            policy.Severity,
			Condition:           policy.Condition,
			Channels:            policy.Channels,
			ReceiverGroups:      append([]string(nil), policy.ReceiverGroups...),
			WindowSeconds:       policy.WindowSeconds,
			CooldownSeconds:     policy.CooldownSeconds,
			NotificationSummary: referenceAlertPolicyNotificationSummary(policy),
		})
	}
	if len(policyFilter) > 0 && selectedCount == 0 {
		writeError(w, http.StatusNotFound, "alert policy not found")
		return
	}
	s.appendAuditLog(
		r,
		tenantID,
		"reference_custom_alert_policy_event_evaluated",
		fmt.Sprintf("evaluated=%d matched=%d", evaluatedCount, len(matches)),
		"alert_policy",
	)
	writeJSON(w, http.StatusOK, referenceAlertPolicyEventEvaluationResponse{
		TenantID:       tenantID,
		Event:          event,
		EvaluatedCount: evaluatedCount,
		MatchedCount:   len(matches),
		Matches:        matches,
		Errors:         evaluationErrors,
	})
}

func defaultReferenceAlertPolicyPreviewEvent() map[string]any {
	return map[string]any{
		"type":    "access_denied",
		"result":  "denied",
		"hour":    float64(20),
		"source":  "preview",
		"trigger": "access_denied_after_hours",
	}
}

func validateReferenceAlertPolicyCondition(condition string) error {
	if strings.TrimSpace(condition) == "" {
		return nil
	}
	_, err := parseReferenceAlertPolicyCondition(condition)
	return err
}

func evaluateReferenceAlertPolicyCondition(condition string, event map[string]any) (bool, error) {
	if strings.TrimSpace(condition) == "" {
		return true, nil
	}
	expr, err := parseReferenceAlertPolicyCondition(condition)
	if err != nil {
		return false, err
	}
	value, err := expr.eval(map[string]any{"event": event})
	if err != nil {
		return false, err
	}
	matched, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("condition expression must evaluate to a boolean")
	}
	return matched, nil
}

func referenceAlertPolicyNotificationSummary(policy referenceAlertPolicy) string {
	channels := []string{}
	if policy.Channels.Email {
		channels = append(channels, "email")
	}
	if policy.Channels.WhatsApp {
		channels = append(channels, "whatsapp")
	}
	if len(channels) == 0 {
		channels = append(channels, "none")
	}
	receivers := policy.ReceiverGroups
	if len(receivers) == 0 {
		receivers = []string{"security"}
	}
	return strings.Join(channels, "+") + " to " + strings.Join(receivers, ",")
}

type referenceAlertPolicyConditionTokenKind int

const (
	referenceAlertPolicyConditionTokenEOF referenceAlertPolicyConditionTokenKind = iota
	referenceAlertPolicyConditionTokenIdentifier
	referenceAlertPolicyConditionTokenString
	referenceAlertPolicyConditionTokenNumber
	referenceAlertPolicyConditionTokenBool
	referenceAlertPolicyConditionTokenOperator
	referenceAlertPolicyConditionTokenLParen
	referenceAlertPolicyConditionTokenRParen
)

type referenceAlertPolicyConditionToken struct {
	kind  referenceAlertPolicyConditionTokenKind
	value string
}

type referenceAlertPolicyConditionExpression interface {
	eval(context map[string]any) (any, error)
}

type referenceAlertPolicyConditionLiteral struct {
	value any
}

func (literal referenceAlertPolicyConditionLiteral) eval(map[string]any) (any, error) {
	return literal.value, nil
}

type referenceAlertPolicyConditionIdentifier struct {
	path []string
}

func (identifier referenceAlertPolicyConditionIdentifier) eval(context map[string]any) (any, error) {
	var current any = context
	for _, part := range identifier.path {
		values, ok := current.(map[string]any)
		if !ok {
			return nil, nil
		}
		next, exists := values[part]
		if !exists {
			return nil, nil
		}
		current = next
	}
	return current, nil
}

type referenceAlertPolicyConditionUnary struct {
	op    string
	right referenceAlertPolicyConditionExpression
}

func (unary referenceAlertPolicyConditionUnary) eval(context map[string]any) (any, error) {
	value, err := unary.right.eval(context)
	if err != nil {
		return nil, err
	}
	switch unary.op {
	case "!":
		boolValue, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("operator ! requires a boolean operand")
		}
		return !boolValue, nil
	default:
		return nil, fmt.Errorf("unsupported operator %q", unary.op)
	}
}

type referenceAlertPolicyConditionBinary struct {
	left  referenceAlertPolicyConditionExpression
	op    string
	right referenceAlertPolicyConditionExpression
}

func (binary referenceAlertPolicyConditionBinary) eval(context map[string]any) (any, error) {
	left, err := binary.left.eval(context)
	if err != nil {
		return nil, err
	}
	switch binary.op {
	case "&&":
		leftBool, ok := left.(bool)
		if !ok {
			return nil, fmt.Errorf("operator && requires boolean operands")
		}
		if !leftBool {
			return false, nil
		}
		right, err := binary.right.eval(context)
		if err != nil {
			return nil, err
		}
		rightBool, ok := right.(bool)
		if !ok {
			return nil, fmt.Errorf("operator && requires boolean operands")
		}
		return rightBool, nil
	case "||":
		leftBool, ok := left.(bool)
		if !ok {
			return nil, fmt.Errorf("operator || requires boolean operands")
		}
		if leftBool {
			return true, nil
		}
		right, err := binary.right.eval(context)
		if err != nil {
			return nil, err
		}
		rightBool, ok := right.(bool)
		if !ok {
			return nil, fmt.Errorf("operator || requires boolean operands")
		}
		return rightBool, nil
	default:
		right, err := binary.right.eval(context)
		if err != nil {
			return nil, err
		}
		return compareReferenceAlertPolicyConditionValues(left, binary.op, right)
	}
}

type referenceAlertPolicyConditionParser struct {
	tokens []referenceAlertPolicyConditionToken
	index  int
}

func parseReferenceAlertPolicyCondition(condition string) (referenceAlertPolicyConditionExpression, error) {
	tokens, err := tokenizeReferenceAlertPolicyCondition(condition)
	if err != nil {
		return nil, err
	}
	parser := referenceAlertPolicyConditionParser{tokens: tokens}
	expr, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if parser.peek().kind != referenceAlertPolicyConditionTokenEOF {
		return nil, fmt.Errorf("unexpected token %q", parser.peek().value)
	}
	return expr, nil
}

func (parser *referenceAlertPolicyConditionParser) parseOr() (referenceAlertPolicyConditionExpression, error) {
	expr, err := parser.parseAnd()
	if err != nil {
		return nil, err
	}
	for parser.matchOperator("||") {
		right, err := parser.parseAnd()
		if err != nil {
			return nil, err
		}
		expr = referenceAlertPolicyConditionBinary{left: expr, op: "||", right: right}
	}
	return expr, nil
}

func (parser *referenceAlertPolicyConditionParser) parseAnd() (referenceAlertPolicyConditionExpression, error) {
	expr, err := parser.parseComparison()
	if err != nil {
		return nil, err
	}
	for parser.matchOperator("&&") {
		right, err := parser.parseComparison()
		if err != nil {
			return nil, err
		}
		expr = referenceAlertPolicyConditionBinary{left: expr, op: "&&", right: right}
	}
	return expr, nil
}

func (parser *referenceAlertPolicyConditionParser) parseComparison() (referenceAlertPolicyConditionExpression, error) {
	expr, err := parser.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op := parser.peek().value
		switch op {
		case "==", "!=", ">", ">=", "<", "<=":
			parser.advance()
			right, err := parser.parseUnary()
			if err != nil {
				return nil, err
			}
			expr = referenceAlertPolicyConditionBinary{left: expr, op: op, right: right}
		default:
			return expr, nil
		}
	}
}

func (parser *referenceAlertPolicyConditionParser) parseUnary() (referenceAlertPolicyConditionExpression, error) {
	if parser.matchOperator("!") {
		right, err := parser.parseUnary()
		if err != nil {
			return nil, err
		}
		return referenceAlertPolicyConditionUnary{op: "!", right: right}, nil
	}
	return parser.parsePrimary()
}

func (parser *referenceAlertPolicyConditionParser) parsePrimary() (referenceAlertPolicyConditionExpression, error) {
	token := parser.advance()
	switch token.kind {
	case referenceAlertPolicyConditionTokenIdentifier:
		return referenceAlertPolicyConditionIdentifier{path: strings.Split(token.value, ".")}, nil
	case referenceAlertPolicyConditionTokenString:
		return referenceAlertPolicyConditionLiteral{value: token.value}, nil
	case referenceAlertPolicyConditionTokenNumber:
		value, err := strconv.ParseFloat(token.value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", token.value)
		}
		return referenceAlertPolicyConditionLiteral{value: value}, nil
	case referenceAlertPolicyConditionTokenBool:
		return referenceAlertPolicyConditionLiteral{value: token.value == "true"}, nil
	case referenceAlertPolicyConditionTokenLParen:
		expr, err := parser.parseOr()
		if err != nil {
			return nil, err
		}
		if parser.peek().kind != referenceAlertPolicyConditionTokenRParen {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		parser.advance()
		return expr, nil
	default:
		return nil, fmt.Errorf("unexpected token %q", token.value)
	}
}

func (parser *referenceAlertPolicyConditionParser) matchOperator(op string) bool {
	if parser.peek().kind == referenceAlertPolicyConditionTokenOperator && parser.peek().value == op {
		parser.advance()
		return true
	}
	return false
}

func (parser *referenceAlertPolicyConditionParser) peek() referenceAlertPolicyConditionToken {
	if parser.index >= len(parser.tokens) {
		return referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenEOF}
	}
	return parser.tokens[parser.index]
}

func (parser *referenceAlertPolicyConditionParser) advance() referenceAlertPolicyConditionToken {
	token := parser.peek()
	if parser.index < len(parser.tokens) {
		parser.index++
	}
	return token
}

func tokenizeReferenceAlertPolicyCondition(condition string) ([]referenceAlertPolicyConditionToken, error) {
	tokens := []referenceAlertPolicyConditionToken{}
	runes := []rune(condition)
	for i := 0; i < len(runes); {
		ch := runes[i]
		if unicode.IsSpace(ch) {
			i++
			continue
		}
		if isReferenceAlertPolicyConditionIdentifierStart(ch) {
			start := i
			i++
			for i < len(runes) && isReferenceAlertPolicyConditionIdentifierPart(runes[i]) {
				i++
			}
			value := string(runes[start:i])
			switch value {
			case "true", "false":
				tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenBool, value: value})
			default:
				tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenIdentifier, value: value})
			}
			continue
		}
		if unicode.IsDigit(ch) {
			start := i
			i++
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenNumber, value: string(runes[start:i])})
			continue
		}
		if ch == '\'' || ch == '"' {
			value, next, err := readReferenceAlertPolicyConditionString(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenString, value: value})
			i = next
			continue
		}
		if ch == '(' {
			tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenLParen, value: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenRParen, value: ")"})
			i++
			continue
		}
		if i+1 < len(runes) {
			op := string(runes[i : i+2])
			switch op {
			case "==", "!=", ">=", "<=", "&&", "||":
				tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenOperator, value: op})
				i += 2
				continue
			}
		}
		switch ch {
		case '>', '<', '!':
			tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenOperator, value: string(ch)})
			i++
		default:
			return nil, fmt.Errorf("unexpected character %q", ch)
		}
	}
	tokens = append(tokens, referenceAlertPolicyConditionToken{kind: referenceAlertPolicyConditionTokenEOF})
	return tokens, nil
}

func readReferenceAlertPolicyConditionString(runes []rune, start int) (string, int, error) {
	quote := runes[start]
	var builder strings.Builder
	for i := start + 1; i < len(runes); i++ {
		ch := runes[i]
		if ch == quote {
			return builder.String(), i + 1, nil
		}
		if ch == '\\' {
			if i+1 >= len(runes) {
				return "", 0, fmt.Errorf("invalid string escape")
			}
			i++
			switch runes[i] {
			case '\\', '\'', '"':
				builder.WriteRune(runes[i])
			case 'n':
				builder.WriteRune('\n')
			case 't':
				builder.WriteRune('\t')
			default:
				return "", 0, fmt.Errorf("unsupported string escape")
			}
			continue
		}
		builder.WriteRune(ch)
	}
	return "", 0, fmt.Errorf("unterminated string literal")
}

func isReferenceAlertPolicyConditionIdentifierStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isReferenceAlertPolicyConditionIdentifierPart(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '.'
}

func compareReferenceAlertPolicyConditionValues(left any, op string, right any) (bool, error) {
	switch op {
	case "==":
		return referenceAlertPolicyConditionValuesEqual(left, right), nil
	case "!=":
		return !referenceAlertPolicyConditionValuesEqual(left, right), nil
	case ">", ">=", "<", "<=":
		leftNumber, leftOK := referenceAlertPolicyConditionNumber(left)
		rightNumber, rightOK := referenceAlertPolicyConditionNumber(right)
		if !leftOK || !rightOK {
			return false, fmt.Errorf("operator %s requires numeric operands", op)
		}
		switch op {
		case ">":
			return leftNumber > rightNumber, nil
		case ">=":
			return leftNumber >= rightNumber, nil
		case "<":
			return leftNumber < rightNumber, nil
		default:
			return leftNumber <= rightNumber, nil
		}
	default:
		return false, fmt.Errorf("unsupported operator %q", op)
	}
}

func referenceAlertPolicyConditionValuesEqual(left any, right any) bool {
	if leftNumber, leftOK := referenceAlertPolicyConditionNumber(left); leftOK {
		if rightNumber, rightOK := referenceAlertPolicyConditionNumber(right); rightOK {
			return leftNumber == rightNumber
		}
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func referenceAlertPolicyConditionNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case json.Number:
		next, err := strconv.ParseFloat(typed.String(), 64)
		return next, err == nil
	default:
		return 0, false
	}
}

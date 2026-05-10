package httpx

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/x509"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
)

func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := allowedCORSOrigin(s.cfg.CORSOrigin, r.Header.Get("Origin"), s.cfg.AppEnv); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if origin != "*" {
				w.Header().Add("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "X-Collection-Range, Deprecation, Link, X-MistyPass-Replacement")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func allowedCORSOrigin(configuredOrigin, requestOrigin, appEnv string) string {
	configuredOrigin = strings.TrimSpace(configuredOrigin)
	if configuredOrigin == "" {
		return ""
	}
	requestOrigin = strings.TrimSpace(requestOrigin)
	fallbackOrigin := ""
	for _, candidate := range strings.Split(configuredOrigin, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if fallbackOrigin == "" {
			fallbackOrigin = candidate
		}
		if candidate == "*" {
			if appEnv == "production" {
				continue // reject wildcard CORS in production
			}
			return candidate
		}
		if requestOrigin != "" && candidate == requestOrigin {
			return requestOrigin
		}
	}
	if requestOrigin == "" {
		return fallbackOrigin
	}
	return ""
}

func (s *server) withLoginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.DisableLoginRateLimit {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := s.clientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowLoginAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many login attempts, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) withGlobalAPIRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := s.clientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowGlobalAPIAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many api requests, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) withEnterprisePublicRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := s.clientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowEnterprisePublicAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many enterprise public requests, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) withEnterpriseWebhookRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := s.clientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowEnterpriseWebhookAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many enterprise webhook requests, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestClientIP(r *http.Request) string {
	return resolveClientIP(r, nil)
}

func (s *server) clientIP(r *http.Request) string {
	return resolveClientIP(r, s.trustedProxies)
}

func resolveClientIP(r *http.Request, trustedProxies []*net.IPNet) string {
	if r == nil {
		return ""
	}

	remoteIP := extractRemoteIP(r)

	if len(trustedProxies) > 0 && isFromTrustedProxy(remoteIP, trustedProxies) {
		xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
		if xff != "" {
			parts := strings.Split(xff, ",")
			first := strings.TrimSpace(parts[0])
			if ip := net.ParseIP(first); ip != nil {
				return ip.String()
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if ip := net.ParseIP(xri); ip != nil {
				return ip.String()
			}
		}
	}

	if remoteIP != "" {
		return remoteIP
	}
	return ""
}

func extractRemoteIP(r *http.Request) string {
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	if ip := net.ParseIP(remote); ip != nil {
		return ip.String()
	}
	return ""
}

func isFromTrustedProxy(remoteIP string, trustedProxies []*net.IPNet) bool {
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}
	for _, cidr := range trustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func parseTrustedProxyCIDRs(raw []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range raw {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			if ip := net.ParseIP(cidr); ip != nil {
				mask := net.CIDRMask(32, 32)
				if ip.To4() == nil {
					mask = net.CIDRMask(128, 128)
				}
				ipNet = &net.IPNet{IP: ip, Mask: mask}
			} else {
				continue
			}
		}
		nets = append(nets, ipNet)
	}
	return nets
}

func (s *server) allowLoginAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"login",
			key,
			now,
			loginRateLimitWindow,
			loginRateLimitMaxAttempts,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("login", key, err)
	}

	s.loginRateLimitMu.Lock()
	defer s.loginRateLimitMu.Unlock()

	s.compactLoginRateBuckets(now)
	bucket, found := s.loginRateLimitBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= loginRateLimitWindow {
		s.loginRateLimitBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= loginRateLimitMaxAttempts {
		retryAfter := loginRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.loginRateLimitBuckets[key] = bucket
	return true, 0
}

func (s *server) compactLoginRateBuckets(now time.Time) {
	if len(s.loginRateLimitBuckets) == 0 {
		return
	}

	for key, bucket := range s.loginRateLimitBuckets {
		if now.Sub(bucket.WindowStart) > loginRateLimitBucketTTL {
			delete(s.loginRateLimitBuckets, key)
		}
	}

	if len(s.loginRateLimitBuckets) <= loginRateLimitBucketMaxKeys {
		return
	}

	for key := range s.loginRateLimitBuckets {
		delete(s.loginRateLimitBuckets, key)
		if len(s.loginRateLimitBuckets) <= loginRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) allowGlobalAPIAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"global_api",
			key,
			now,
			apiRateLimitWindow,
			apiRateLimitMaxRequests,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("global_api", key, err)
	}

	s.apiRateLimitMu.Lock()
	defer s.apiRateLimitMu.Unlock()

	s.compactGlobalAPIRateBuckets(now)
	bucket, found := s.apiRateLimitBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= apiRateLimitWindow {
		s.apiRateLimitBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= apiRateLimitMaxRequests {
		retryAfter := apiRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.apiRateLimitBuckets[key] = bucket
	return true, 0
}

func (s *server) allowEnterprisePublicAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"enterprise_public",
			key,
			now,
			enterprisePublicRateLimitWindow,
			enterprisePublicRateLimitMaxRequests,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("enterprise_public", key, err)
	}

	s.enterprisePublicRateLimitMu.Lock()
	defer s.enterprisePublicRateLimitMu.Unlock()

	s.compactEnterprisePublicRateBuckets(now)
	bucket, found := s.enterprisePublicRateBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= enterprisePublicRateLimitWindow {
		s.enterprisePublicRateBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= enterprisePublicRateLimitMaxRequests {
		retryAfter := enterprisePublicRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.enterprisePublicRateBuckets[key] = bucket
	return true, 0
}

func (s *server) allowEnterpriseWebhookAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"enterprise_webhook",
			key,
			now,
			enterpriseWebhookRateLimitWindow,
			enterpriseWebhookRateLimitMaxRequests,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("enterprise_webhook", key, err)
	}

	s.enterpriseWebhookRateLimitMu.Lock()
	defer s.enterpriseWebhookRateLimitMu.Unlock()

	s.compactEnterpriseWebhookRateBuckets(now)
	bucket, found := s.enterpriseWebhookRateBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= enterpriseWebhookRateLimitWindow {
		s.enterpriseWebhookRateBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= enterpriseWebhookRateLimitMaxRequests {
		retryAfter := enterpriseWebhookRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.enterpriseWebhookRateBuckets[key] = bucket
	return true, 0
}

func (s *server) compactGlobalAPIRateBuckets(now time.Time) {
	if len(s.apiRateLimitBuckets) == 0 {
		return
	}

	for key, bucket := range s.apiRateLimitBuckets {
		if now.Sub(bucket.WindowStart) > apiRateLimitBucketTTL {
			delete(s.apiRateLimitBuckets, key)
		}
	}

	if len(s.apiRateLimitBuckets) <= apiRateLimitBucketMaxKeys {
		return
	}

	for key := range s.apiRateLimitBuckets {
		delete(s.apiRateLimitBuckets, key)
		if len(s.apiRateLimitBuckets) <= apiRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) compactEnterprisePublicRateBuckets(now time.Time) {
	if len(s.enterprisePublicRateBuckets) == 0 {
		return
	}

	for key, bucket := range s.enterprisePublicRateBuckets {
		if now.Sub(bucket.WindowStart) > enterprisePublicRateLimitBucketTTL {
			delete(s.enterprisePublicRateBuckets, key)
		}
	}

	if len(s.enterprisePublicRateBuckets) <= enterprisePublicRateLimitBucketMaxKeys {
		return
	}

	for key := range s.enterprisePublicRateBuckets {
		delete(s.enterprisePublicRateBuckets, key)
		if len(s.enterprisePublicRateBuckets) <= enterprisePublicRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) compactEnterpriseWebhookRateBuckets(now time.Time) {
	if len(s.enterpriseWebhookRateBuckets) == 0 {
		return
	}

	for key, bucket := range s.enterpriseWebhookRateBuckets {
		if now.Sub(bucket.WindowStart) > enterpriseWebhookRateLimitBucketTTL {
			delete(s.enterpriseWebhookRateBuckets, key)
		}
	}

	if len(s.enterpriseWebhookRateBuckets) <= enterpriseWebhookRateLimitBucketMaxKeys {
		return
	}

	for key := range s.enterpriseWebhookRateBuckets {
		delete(s.enterpriseWebhookRateBuckets, key)
		if len(s.enterpriseWebhookRateBuckets) <= enterpriseWebhookRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) warnRateLimitFallback(scope, key string, err error) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("redis rate limit fallback to in-memory buckets", "scope", scope, "key", key, "err", err)
}

func (s *server) withBearerToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		user, err := s.authService.VerifyAccessToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid access token")
			return
		}

		ctx := context.WithValue(r.Context(), authUserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(authHeader string) (string, error) {
	header := strings.TrimSpace(authHeader)
	if !strings.HasPrefix(header, "Bearer ") {
		return "", errors.New("missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return "", errors.New("missing bearer token")
	}

	return token, nil
}

func authenticatedUser(r *http.Request) (auth.User, bool) {
	value := r.Context().Value(authUserContextKey)
	user, ok := value.(auth.User)
	return user, ok
}

func firstNonEmptyString(items ...string) string {
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next != "" {
			return next
		}
	}
	return ""
}

func firstNonZeroDuration(items ...time.Duration) time.Duration {
	for i := range items {
		if items[i] > 0 {
			return items[i]
		}
	}
	return 0
}

func (s *server) requireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for i := range roles {
		role := strings.ToLower(strings.TrimSpace(roles[i]))
		if role == "" {
			continue
		}
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authenticatedUser(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "invalid access token")
				return
			}
			role := strings.ToLower(strings.TrimSpace(user.Role))
			if _, exists := allowed[role]; !exists {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *server) resolveTenantID(w http.ResponseWriter, r *http.Request, requestedTenantID string) (string, bool) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return "", false
	}

	requested := strings.TrimSpace(requestedTenantID)
	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role == "super_admin" {
		return requested, true
	}

	tokenTenantID := strings.TrimSpace(user.TenantID)
	if tokenTenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return "", false
	}
	if requested != "" && requested != tokenTenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return "", false
	}

	return tokenTenantID, true
}

func (s *server) buildingScopeForRequest(w http.ResponseWriter, r *http.Request) (map[string]struct{}, bool) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return nil, false
	}

	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role != "building_admin" {
		return nil, true
	}

	scope := normalizeBuildingScope(user.BuildingIDs)
	if scope == nil {
		scope = map[string]struct{}{}
	}
	for buildingID := range s.buildingScopeFromRoleAssignments(user) {
		scope[buildingID] = struct{}{}
	}
	if len(scope) == 0 {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return nil, false
	}

	return scope, true
}

func (s *server) buildingScopeFromRoleAssignments(user auth.User) map[string]struct{} {
	scope := map[string]struct{}{}
	if s == nil || s.accessSvc == nil {
		return scope
	}
	tenantID := strings.TrimSpace(user.TenantID)
	if tenantID == "" {
		return scope
	}

	userID := strings.TrimSpace(user.ID)
	userEmail := strings.ToLower(strings.TrimSpace(user.Email))
	userTeamIDs := map[string]struct{}{}
	for _, membership := range s.accessSvc.ListTeamMemberships(tenantID) {
		if strings.EqualFold(strings.TrimSpace(membership.MemberType), "User") &&
			((userID != "" && strings.TrimSpace(membership.MemberID) == userID) ||
				(userEmail != "" && strings.EqualFold(strings.TrimSpace(membership.MemberEmail), userEmail))) {
			if teamID := strings.TrimSpace(membership.TeamID); teamID != "" {
				userTeamIDs[teamID] = struct{}{}
			}
		}
	}

	for _, assignment := range s.accessSvc.ListRoleAssignments(tenantID) {
		if strings.TrimSpace(assignment.RoleID) != "role_place_admin" ||
			!strings.EqualFold(strings.TrimSpace(assignment.AppliesToType), "Place") {
			continue
		}
		placeID := strings.TrimSpace(assignment.AppliesToID)
		if placeID == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(assignment.AssigneeType)) {
		case "user":
			if (userID != "" && strings.TrimSpace(assignment.AssigneeID) == userID) ||
				(userEmail != "" && strings.EqualFold(strings.TrimSpace(assignment.AssigneeEmail), userEmail)) {
				scope[placeID] = struct{}{}
			}
		case "team":
			if _, exists := userTeamIDs[strings.TrimSpace(assignment.AssigneeID)]; exists {
				scope[placeID] = struct{}{}
			}
		}
	}

	return scope
}

func (s *server) requireBuildingScope(w http.ResponseWriter, scope map[string]struct{}, buildingID string) bool {
	if scope == nil {
		return true
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	if nextBuildingID == "" {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	if _, exists := scope[nextBuildingID]; !exists {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}

	return true
}

func normalizeBuildingScope(buildingIDs []string) map[string]struct{} {
	scope := make(map[string]struct{}, len(buildingIDs))
	for i := range buildingIDs {
		nextBuildingID := strings.TrimSpace(buildingIDs[i])
		if nextBuildingID == "" {
			continue
		}
		scope[nextBuildingID] = struct{}{}
	}
	return scope
}

func filterTopologyByBuildingScope(input space.TenantTopology, scope map[string]struct{}) space.TenantTopology {
	return space.TenantTopology{
		TenantID:  input.TenantID,
		Buildings: filterBuildingsByScope(input.Buildings, scope),
		Floors:    filterFloorsByScope(input.Floors, scope),
		Areas:     filterAreasByScope(input.Areas, scope),
		Doors:     filterDoorsByScope(input.Doors, scope),
	}
}

func filterBuildingsByScope(items []space.Building, scope map[string]struct{}) []space.Building {
	filtered := make([]space.Building, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].ID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterFloorsByScope(items []space.Floor, scope map[string]struct{}) []space.Floor {
	filtered := make([]space.Floor, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterAreasByScope(items []space.Area, scope map[string]struct{}) []space.Area {
	filtered := make([]space.Area, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterDoorsByScope(items []space.Door, scope map[string]struct{}) []space.Door {
	filtered := make([]space.Door, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func allowedDoorIDsByBuildingScope(doors []space.Door, scope map[string]struct{}) map[string]struct{} {
	allowed := make(map[string]struct{}, len(doors))
	for i := range doors {
		if _, exists := scope[doors[i].BuildingID]; !exists {
			continue
		}
		allowed[doors[i].ID] = struct{}{}
	}
	return allowed
}

func filterDoorGroupsByScope(items []space.DoorGroup, allowedDoorIDs map[string]struct{}) []space.DoorGroup {
	filtered := make([]space.DoorGroup, 0, len(items))
	for i := range items {
		nextDoorIDs := make([]string, 0, len(items[i].DoorIDs))
		for j := range items[i].DoorIDs {
			if _, exists := allowedDoorIDs[items[i].DoorIDs[j]]; !exists {
				continue
			}
			nextDoorIDs = append(nextDoorIDs, items[i].DoorIDs[j])
		}
		if len(nextDoorIDs) == 0 {
			continue
		}
		record := items[i]
		record.DoorIDs = nextDoorIDs
		filtered = append(filtered, record)
	}
	return filtered
}

func filterGatewaysByScope(items []gateway.Gateway, scope map[string]struct{}) []gateway.Gateway {
	filtered := make([]gateway.Gateway, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterPoliciesByScope(items []access.Policy, scope map[string]struct{}) []access.Policy {
	filtered := make([]access.Policy, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterTemporaryAccessByScope(items []access.TemporaryAccess, scope map[string]struct{}) []access.TemporaryAccess {
	filtered := make([]access.TemporaryAccess, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterUsersByScope(items []access.AccessUser, scope map[string]struct{}) []access.AccessUser {
	filtered := make([]access.AccessUser, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterUserGroupsByScope(items []access.UserGroup, scope map[string]struct{}) []access.UserGroup {
	filtered := make([]access.UserGroup, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterVisitorPassesByScope(items []access.VisitorPass, scope map[string]struct{}) []access.VisitorPass {
	filtered := make([]access.VisitorPass, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterAccessEventsByScope(items []event.AccessEvent, scope map[string]struct{}) []event.AccessEvent {
	filtered := make([]event.AccessEvent, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterDeviceEventsByScope(items []event.DeviceEvent, scope map[string]struct{}) []event.DeviceEvent {
	filtered := make([]event.DeviceEvent, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterAlarmsByScope(items []alarm.Alarm, scope map[string]struct{}) []alarm.Alarm {
	filtered := make([]alarm.Alarm, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func (s *server) appendAuditLog(r *http.Request, tenantID, action, target, source string) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}

	actor := "system"
	role := "system"
	if r != nil {
		if user, ok := authenticatedUser(r); ok {
			if strings.TrimSpace(user.Email) != "" {
				actor = strings.TrimSpace(user.Email)
			}
			if strings.TrimSpace(user.Role) != "" {
				role = strings.TrimSpace(user.Role)
			}
		}
	}

	record, err := s.auditSvc.Append(
		nextTenantID,
		actor,
		role,
		strings.TrimSpace(action),
		strings.TrimSpace(target),
		strings.TrimSpace(source),
	)
	if err != nil {
		return
	}
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	s.publishInternalEvent(
		ctx,
		"audit.log.appended",
		map[string]any{
			"tenant_id": nextTenantID,
			"log":       record,
		},
		map[string]string{
			"tenant_id": nextTenantID,
			"action":    strings.TrimSpace(action),
			"source":    strings.TrimSpace(source),
		},
	)
}

func (s *server) publishInternalEvent(
	ctx context.Context,
	subject string,
	payload any,
	headers map[string]string,
) {
	if s.messageBus == nil {
		return
	}
	if err := s.messageBus.PublishJSON(ctx, subject, payload, headers); err != nil {
		s.loggerOrDefault().Warn(
			"publish internal event failed",
			"subject", strings.TrimSpace(subject),
			"err", err,
		)
	}
}

func (s *server) findGatewayByTenant(tenantID, gatewayID string) (gateway.Gateway, bool) {
	items := s.gatewaySvc.List(strings.TrimSpace(tenantID))
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return gateway.Gateway{}, false
	}

	for i := range items {
		if items[i].ID == nextGatewayID {
			return items[i], true
		}
	}
	return gateway.Gateway{}, false
}

func (s *server) setGatewayDeviceToken(gatewayID, deviceToken string) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextDeviceToken := strings.TrimSpace(deviceToken)
	if nextGatewayID == "" || nextDeviceToken == "" {
		return
	}
	if s.gatewayTokenStore != nil {
		if err := s.gatewayTokenStore.UpsertGatewayDeviceToken(nextGatewayID, nextDeviceToken); err != nil {
			s.loggerOrDefault().Error(
				"gateway bootstrap token persist failed",
				"gateway_id", nextGatewayID,
				"err", err,
			)
		}
		return
	}
	s.gatewayTokenMu.Lock()
	defer s.gatewayTokenMu.Unlock()
	if s.gatewayDeviceTokens == nil {
		s.gatewayDeviceTokens = map[string]string{}
	}
	s.gatewayDeviceTokens[nextGatewayID] = nextDeviceToken
	if err := s.persistGatewayBootstrapStateLocked(); err != nil {
		s.loggerOrDefault().Error(
			"gateway bootstrap token persist failed",
			"gateway_id", nextGatewayID,
			"err", err,
		)
	}
}

func (s *server) setGatewayAuthzCacheAckVersion(gatewayID, version string) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextVersion := strings.TrimSpace(version)
	if nextGatewayID == "" || nextVersion == "" {
		return
	}
	s.gatewayAuthzAckMu.Lock()
	defer s.gatewayAuthzAckMu.Unlock()
	if s.gatewayAuthzAckVersion == nil {
		s.gatewayAuthzAckVersion = map[string]string{}
	}
	s.gatewayAuthzAckVersion[nextGatewayID] = nextVersion
}

func (s *server) gatewayAuthzCacheAckVersion(gatewayID string) string {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return ""
	}
	s.gatewayAuthzAckMu.RLock()
	defer s.gatewayAuthzAckMu.RUnlock()
	if s.gatewayAuthzAckVersion == nil {
		return ""
	}
	return strings.TrimSpace(s.gatewayAuthzAckVersion[nextGatewayID])
}

func (s *server) authorizeGatewayDeviceToken(w http.ResponseWriter, r *http.Request, gatewayID string) bool {
	if s.gatewayIdentityStatusRevoked(gatewayID) {
		recordGatewayMTLSAuth("gateway_revoked")
		writeError(w, http.StatusUnauthorized, "gateway revoked")
		return false
	}
	if s.authorizeGatewayClientCertificate(r, gatewayID) {
		return true
	}
	if gatewayMTLSRequired(r) {
		recordGatewayMTLSAuth("required_missing")
		writeError(w, http.StatusUnauthorized, "valid gateway client certificate required")
		return false
	}

	provided := strings.TrimSpace(r.Header.Get("X-Device-Token"))
	if provided == "" {
		if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
			provided = token
		}
	}
	if provided == "" {
		recordGatewayMTLSAuth("token_missing")
		writeError(w, http.StatusUnauthorized, "missing device token")
		return false
	}
	if s.gatewayTokenStore != nil {
		exists, matched, err := s.gatewayTokenStore.VerifyGatewayDeviceToken(strings.TrimSpace(gatewayID), provided)
		if err != nil {
			recordGatewayMTLSAuth("token_store_error")
			writeError(w, http.StatusInternalServerError, "device token verification failed")
			return false
		}
		if exists && matched {
			recordGatewayMTLSAuth("token_fallback")
			return true
		}
		if !exists {
			recordGatewayMTLSAuth("token_unregistered")
			writeError(w, http.StatusUnauthorized, "device not registered")
		} else {
			recordGatewayMTLSAuth("token_invalid")
			writeError(w, http.StatusUnauthorized, "invalid device token")
		}
		return false
	}

	nextGatewayID := strings.TrimSpace(gatewayID)
	s.gatewayTokenMu.RLock()
	expected, exists := s.gatewayDeviceTokens[nextGatewayID]
	s.gatewayTokenMu.RUnlock()
	if exists && strings.TrimSpace(expected) != "" {
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1 {
			recordGatewayMTLSAuth("token_fallback")
			return true
		}
	}

	if !exists || strings.TrimSpace(expected) == "" {
		recordGatewayMTLSAuth("token_unregistered")
		writeError(w, http.StatusUnauthorized, "device not registered")
	} else {
		recordGatewayMTLSAuth("token_invalid")
		writeError(w, http.StatusUnauthorized, "invalid device token")
	}
	return false
}

func (s *server) authorizeGatewayClientCertificate(r *http.Request, gatewayID string) bool {
	cert := verifiedGatewayClientCertificate(r)
	if cert == nil {
		return false
	}
	if s.gatewayClientCertificateRevoked(cert) {
		recordGatewayMTLSAuth("revoked_serial")
		return false
	}

	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" || strings.TrimSpace(cert.Subject.CommonName) != nextGatewayID {
		recordGatewayMTLSAuth("subject_mismatch")
		return false
	}

	if s.gatewaySvc == nil {
		recordGatewayMTLSAuth("accepted")
		return true
	}
	record, ok := s.gatewaySvc.FindGatewayByID("", nextGatewayID)
	if !ok {
		recordGatewayMTLSAuth("gateway_missing")
		return false
	}
	if gatewayCertificateStatusRevoked(record.Status) {
		recordGatewayMTLSAuth("gateway_revoked")
		return false
	}
	if len(cert.Subject.Organization) == 0 {
		recordGatewayMTLSAuth("tenant_missing")
		return false
	}
	if strings.TrimSpace(cert.Subject.Organization[0]) != strings.TrimSpace(record.TenantID) {
		recordGatewayMTLSAuth("tenant_mismatch")
		return false
	}
	recordGatewayMTLSAuth("accepted")
	return true
}

func (s *server) gatewayClientCertificateRevoked(cert *x509.Certificate) bool {
	if s == nil || cert == nil || cert.SerialNumber == nil {
		return false
	}
	return s.gatewayClientSerialRevoked(cert.SerialNumber.Text(16))
}

func (s *server) gatewayClientSerialRevoked(serialNumber string) bool {
	if s == nil {
		return false
	}
	serial := gateway.NormalizeCertificateSerial(serialNumber)
	if serial == "" {
		return false
	}
	for i := range s.cfg.GatewayMTLSRevokedSerials {
		if gateway.NormalizeCertificateSerial(s.cfg.GatewayMTLSRevokedSerials[i]) == serial {
			return true
		}
	}
	if s.gatewaySvc != nil && s.gatewaySvc.IsCertificateSerialRevoked(serial) {
		return true
	}
	return false
}

func gatewayCertificateSerialFromRequest(r *http.Request) string {
	cert := verifiedGatewayClientCertificate(r)
	if cert == nil || cert.SerialNumber == nil {
		return ""
	}
	return gateway.NormalizeCertificateSerial(cert.SerialNumber.Text(16))
}

func (s *server) gatewayIdentityStatusRevoked(gatewayID string) bool {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if s == nil || s.gatewaySvc == nil || nextGatewayID == "" {
		return false
	}
	record, ok := s.gatewaySvc.FindGatewayByID("", nextGatewayID)
	return ok && gatewayCertificateStatusRevoked(record.Status)
}

func gatewayCertificateStatusRevoked(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "disabled", "revoked":
		return true
	default:
		return false
	}
}

func verifiedGatewayClientCertificate(r *http.Request) *x509.Certificate {
	if r == nil || r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil
	}
	if len(r.TLS.VerifiedChains) == 0 {
		return nil
	}
	leaf := r.TLS.PeerCertificates[0]
	for _, chain := range r.TLS.VerifiedChains {
		if len(chain) > 0 && chain[0].Equal(leaf) {
			return leaf
		}
	}
	return nil
}

func (s *server) authorizeGatewayBootstrapToken(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(s.cfg.GatewayBootstrapToken)
	if expected == "" {
		writeError(w, http.StatusServiceUnavailable, "gateway bootstrap token is not configured")
		return false
	}

	provided := strings.TrimSpace(r.Header.Get("X-Bootstrap-Token"))
	if provided == "" {
		if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
			provided = token
		}
	}
	if provided == "" {
		writeError(w, http.StatusUnauthorized, "missing gateway bootstrap token")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid gateway bootstrap token")
		return false
	}

	return true
}

// validateGatewayRequestNonce checks the X-Request-Nonce and X-Request-Timestamp
// headers for gateway HTTP replay protection.
func (s *server) validateGatewayRequestNonce(w http.ResponseWriter, r *http.Request, gatewayID string) bool {
	nonce := strings.TrimSpace(r.Header.Get("X-Request-Nonce"))
	tsRaw := strings.TrimSpace(r.Header.Get("X-Request-Timestamp"))

	if nonce == "" && tsRaw == "" {
		if s.cfg.GatewayRequireRequestNonce {
			recordGatewayNonceValidation("missing")
			writeError(w, http.StatusUnauthorized, "gateway request nonce required")
			return false
		}
		return true
	}

	if nonce == "" || tsRaw == "" {
		recordGatewayNonceValidation("partial")
		writeError(w, http.StatusBadRequest, "X-Request-Nonce and X-Request-Timestamp must both be present")
		return false
	}

	// Validate timestamp within 5-minute window
	ts, err := time.Parse(time.RFC3339, tsRaw)
	if err != nil {
		recordGatewayNonceValidation("invalid_timestamp")
		writeError(w, http.StatusBadRequest, "invalid X-Request-Timestamp format")
		return false
	}
	const nonceWindow = 5 * time.Minute
	now := time.Now().UTC()
	if now.Sub(ts) > nonceWindow || ts.Sub(now) > nonceWindow {
		recordGatewayNonceValidation("timestamp_window")
		writeError(w, http.StatusUnauthorized, "request timestamp outside acceptable window")
		return false
	}

	if s.gatewayNonceStore != nil {
		inserted, err := s.gatewayNonceStore.MarkGatewayRequestNonce(gatewayID, nonce, nonceWindow)
		if err != nil {
			s.loggerOrDefault().Warn("gateway request nonce store failed", "gateway_id", gatewayID, "err", err)
			recordGatewayNonceValidation("store_error")
			writeError(w, http.StatusServiceUnavailable, "gateway request nonce store unavailable")
			return false
		}
		if !inserted {
			recordGatewayNonceValidation("duplicate")
			writeError(w, http.StatusConflict, "duplicate request nonce")
			return false
		}
		recordGatewayNonceValidation("accepted_redis")
		return true
	}

	s.gatewayNonceMu.Lock()
	if s.gatewayNonces == nil {
		s.gatewayNonces = map[string]time.Time{}
	}
	for k, exp := range s.gatewayNonces {
		if now.After(exp) {
			delete(s.gatewayNonces, k)
		}
	}
	nonceKey := strings.TrimSpace(gatewayID) + ":" + nonce
	if _, exists := s.gatewayNonces[nonceKey]; exists {
		s.gatewayNonceMu.Unlock()
		recordGatewayNonceValidation("duplicate")
		writeError(w, http.StatusConflict, "duplicate request nonce")
		return false
	}
	s.gatewayNonces[nonceKey] = now.Add(nonceWindow)
	s.gatewayNonceMu.Unlock()

	recordGatewayNonceValidation("accepted_memory")
	return true
}

func randomHexID(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func resolveHRISVaultMasterKey(raw string) (string, error) {
	next := strings.TrimSpace(raw)
	if next != "" {
		return next, nil
	}
	return randomHexID(32)
}

func parseSerialInventoryCSV(content string) ([]gateway.SerialInventoryImportItem, error) {
	nextContent := strings.TrimSpace(content)
	if nextContent == "" {
		return nil, gateway.ErrSerialInventoryRecordsRequired
	}

	reader := csv.NewReader(strings.NewReader(nextContent))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gateway.ErrSerialInventoryRecordsRequired
	}

	start := 0
	if isSerialInventoryCSVHeader(rows[0]) {
		start = 1
	}

	records := make([]gateway.SerialInventoryImportItem, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		serialNumber := strings.TrimSpace(row[0])
		productType := ""
		batchCode := ""
		source := ""

		if len(row) > 1 {
			productType = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			batchCode = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			source = strings.TrimSpace(row[3])
		}

		if serialNumber == "" && productType == "" {
			continue
		}

		records = append(records, gateway.SerialInventoryImportItem{
			SerialNumber: serialNumber,
			ProductType:  productType,
			BatchCode:    batchCode,
			Source:       source,
		})
	}

	if len(records) == 0 {
		return nil, gateway.ErrSerialInventoryRecordsRequired
	}
	return records, nil
}

func isSerialInventoryCSVHeader(row []string) bool {
	if len(row) < 2 {
		return false
	}
	column0 := strings.ToLower(strings.TrimSpace(row[0]))
	column1 := strings.ToLower(strings.TrimSpace(row[1]))
	return column0 == "serial_number" && column1 == "product_type"
}

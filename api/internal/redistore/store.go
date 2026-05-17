package redistore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Options struct {
	Addr         string
	Password     string
	DB           int
	KeyPrefix    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type Store struct {
	client    *redis.Client
	keyPrefix string
}

type WorkerQueueClaim struct {
	ItemID     string
	ClaimToken string
}

type WorkerQueueItemState struct {
	State                string
	VisibilityDeadlineAt *time.Time
}

type WorkerQueueTelemetry struct {
	PendingCount int
	ClaimedCount int
	Items        map[string]WorkerQueueItemState
}

const (
	WorkerQueueStateQueued  = "queued"
	WorkerQueueStateClaimed = "claimed"
	WorkerQueueStateMissing = "missing"
)

type refreshSessionPayload struct {
	UserID    string `json:"user_id"`
	ExpiresAt int64  `json:"expires_at"`
}

func New(options Options) (*Store, error) {
	addr := strings.TrimSpace(options.Addr)
	if addr == "" {
		return nil, errors.New("redis addr is required")
	}

	keyPrefix := strings.TrimSpace(options.KeyPrefix)
	if keyPrefix == "" {
		keyPrefix = "mistypass"
	}

	dialTimeout := options.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 3 * time.Second
	}
	readTimeout := options.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 3 * time.Second
	}
	writeTimeout := options.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 3 * time.Second
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     options.Password,
		DB:           options.DB,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Store{client: client, keyPrefix: keyPrefix}, nil
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) UpsertAuthRefreshSession(sessionID, userID string, expiresAt time.Time) error {
	nextSessionID := strings.TrimSpace(sessionID)
	nextUserID := strings.TrimSpace(userID)
	nextExpiresAt := expiresAt.UTC()
	if nextSessionID == "" || nextUserID == "" || nextExpiresAt.IsZero() {
		return errors.New("invalid refresh session")
	}
	ttl := time.Until(nextExpiresAt)
	if ttl <= 0 {
		return errors.New("refresh session already expired")
	}

	payload, err := json.Marshal(refreshSessionPayload{
		UserID:    nextUserID,
		ExpiresAt: nextExpiresAt.Unix(),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.authRefreshSessionKey(nextSessionID), payload, ttl)
	userSessionKey := s.authRefreshUserIndexKey(nextUserID)
	pipe.SAdd(ctx, userSessionKey, nextSessionID)
	pipe.Expire(ctx, userSessionKey, ttl+24*time.Hour)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) FindAuthRefreshSession(sessionID string) (string, time.Time, bool, error) {
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return "", time.Time{}, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := s.client.Get(ctx, s.authRefreshSessionKey(nextSessionID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}

	session, ok := decodeRefreshSession(value)
	if !ok {
		_, _ = s.client.Del(ctx, s.authRefreshSessionKey(nextSessionID)).Result()
		return "", time.Time{}, false, nil
	}
	now := time.Now().UTC()
	if !session.ExpiresAt.After(now) {
		_, _ = s.client.Del(ctx, s.authRefreshSessionKey(nextSessionID)).Result()
		_, _ = s.client.SRem(ctx, s.authRefreshUserIndexKey(session.UserID), nextSessionID).Result()
		return "", time.Time{}, false, nil
	}

	return session.UserID, session.ExpiresAt, true, nil
}

func (s *Store) DeleteAuthRefreshSession(sessionID string) error {
	nextSessionID := strings.TrimSpace(sessionID)
	if nextSessionID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	value, err := s.client.Get(ctx, s.authRefreshSessionKey(nextSessionID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	session, ok := decodeRefreshSession(value)
	_, err = s.client.Del(ctx, s.authRefreshSessionKey(nextSessionID)).Result()
	if err != nil {
		return err
	}
	if ok {
		_, _ = s.client.SRem(ctx, s.authRefreshUserIndexKey(session.UserID), nextSessionID).Result()
	}
	return nil
}

func (s *Store) DeleteAuthRefreshSessionsByUserID(userID string) (int, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	indexKey := s.authRefreshUserIndexKey(nextUserID)
	sessionIDs, err := s.client.SMembers(ctx, indexKey).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	deleted := 0
	for i := range sessionIDs {
		nextSessionID := strings.TrimSpace(sessionIDs[i])
		if nextSessionID == "" {
			continue
		}
		affected, delErr := s.client.Del(ctx, s.authRefreshSessionKey(nextSessionID)).Result()
		if delErr != nil {
			return deleted, delErr
		}
		if affected > 0 {
			deleted++
		}
		_, _ = s.client.SRem(ctx, indexKey, nextSessionID).Result()
	}
	_, _ = s.client.Del(ctx, indexKey).Result()
	return deleted, nil
}

func (s *Store) UpsertAuthRevokedAccessToken(tokenID string, expiresAt time.Time) error {
	nextTokenID := strings.TrimSpace(tokenID)
	nextExpiresAt := expiresAt.UTC()
	if nextTokenID == "" || nextExpiresAt.IsZero() {
		return errors.New("invalid revoked access token")
	}
	ttl := time.Until(nextExpiresAt)
	if ttl <= 0 {
		return errors.New("revoked access token already expired")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.authRevokedAccessTokenKey(nextTokenID), "1", ttl).Err()
}

func (s *Store) IsAuthAccessTokenRevoked(tokenID string, _ time.Time) (bool, error) {
	nextTokenID := strings.TrimSpace(tokenID)
	if nextTokenID == "" {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	exists, err := s.client.Exists(ctx, s.authRevokedAccessTokenKey(nextTokenID)).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (s *Store) AllowRateLimit(scope, key string, now time.Time, window time.Duration, maxAttempts int) (bool, time.Duration, error) {
	nextScope := strings.TrimSpace(scope)
	nextKey := strings.TrimSpace(key)
	nextNow := now.UTC()
	if nextScope == "" || nextKey == "" {
		return false, 0, errors.New("scope/key are required")
	}
	if window <= 0 {
		return false, 0, errors.New("window must be positive")
	}
	if maxAttempts < 1 {
		return false, 0, errors.New("max attempts must be >= 1")
	}

	windowSeconds := int64(window / time.Second)
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	bucket := nextNow.Unix() / windowSeconds
	bucketKey := s.rateLimitCounterKey(nextScope, nextKey, bucket)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pipe := s.client.TxPipeline()
	countCmd := pipe.Incr(ctx, bucketKey)
	ttlCmd := pipe.TTL(ctx, bucketKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}

	count := countCmd.Val()
	ttl := ttlCmd.Val()
	if count == 1 || ttl <= 0 {
		if expErr := s.client.Expire(ctx, bucketKey, window).Err(); expErr != nil {
			return false, 0, expErr
		}
		ttl = window
	}

	if count <= int64(maxAttempts) {
		return true, 0, nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	return false, ttl, nil
}

func (s *Store) MarkGatewayRequestNonce(gatewayID, nonce string, ttl time.Duration) (bool, error) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextNonce := strings.TrimSpace(nonce)
	if nextGatewayID == "" || nextNonce == "" {
		return false, errors.New("gateway id/nonce are required")
	}
	if ttl <= 0 {
		return false, errors.New("nonce ttl must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.SetNX(ctx, s.gatewayRequestNonceKey(nextGatewayID, nextNonce), "1", ttl).Result()
}

func (s *Store) TryAcquireLease(key, token string, ttl time.Duration) (bool, error) {
	nextKey := strings.TrimSpace(key)
	nextToken := strings.TrimSpace(token)
	if nextKey == "" || nextToken == "" {
		return false, errors.New("lease key/token are required")
	}
	if ttl <= 0 {
		return false, errors.New("lease ttl must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.SetNX(ctx, s.workerLeaseKey(nextKey), nextToken, ttl).Result()
}

func (s *Store) ReleaseLease(key, token string) error {
	nextKey := strings.TrimSpace(key)
	nextToken := strings.TrimSpace(token)
	if nextKey == "" || nextToken == "" {
		return errors.New("lease key/token are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Eval(
		ctx,
		`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`,
		[]string{s.workerLeaseKey(nextKey)},
		nextToken,
	).Err()
}

func (s *Store) EnqueueWorkerQueue(queueName, itemID string) error {
	nextQueueName := strings.TrimSpace(queueName)
	nextItemID := strings.TrimSpace(itemID)
	if nextQueueName == "" || nextItemID == "" {
		return errors.New("worker queue name/item id are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Eval(
		ctx,
		`if redis.call("ZSCORE", KEYS[3], ARGV[1]) or redis.call("HEXISTS", KEYS[4], ARGV[1]) == 1 then
			redis.call("SADD", KEYS[2], ARGV[1])
			redis.call("LREM", KEYS[1], 0, ARGV[1])
			return 0
		end
		if redis.call("LPOS", KEYS[1], ARGV[1]) then
			redis.call("SADD", KEYS[2], ARGV[1])
			while redis.call("LPOS", KEYS[1], ARGV[1], "RANK", 2) do
				redis.call("LREM", KEYS[1], -1, ARGV[1])
			end
			return 0
		end
		redis.call("SADD", KEYS[2], ARGV[1])
		return redis.call("RPUSH", KEYS[1], ARGV[1])`,
		[]string{
			s.workerQueueKey(nextQueueName),
			s.workerQueueIndexKey(nextQueueName),
			s.workerQueueProcessingKey(nextQueueName),
			s.workerQueueClaimKey(nextQueueName),
		},
		nextItemID,
	).Err()
}

func (s *Store) DequeueWorkerQueueBatch(queueName string, batchSize int) ([]string, error) {
	nextQueueName := strings.TrimSpace(queueName)
	if nextQueueName == "" {
		return nil, errors.New("worker queue name is required")
	}
	if batchSize <= 0 {
		return nil, errors.New("worker queue batch size must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.client.Eval(
		ctx,
		`local batch = tonumber(ARGV[1]) or 0
		if batch <= 0 then
			return {}
		end
		local items = redis.call("LRANGE", KEYS[1], 0, batch - 1)
		local count = #items
		if count == 0 then
			return {}
		end
		redis.call("LTRIM", KEYS[1], count, -1)
		for i = 1, count do
			redis.call("SREM", KEYS[2], items[i])
		end
		return items`,
		[]string{s.workerQueueKey(nextQueueName), s.workerQueueIndexKey(nextQueueName)},
		strconv.Itoa(batchSize),
	).Result()
	if err != nil {
		return nil, err
	}

	rawItems, ok := result.([]any)
	if !ok {
		return nil, errors.New("worker queue dequeue returned unexpected payload")
	}
	items := make([]string, 0, len(rawItems))
	for i := range rawItems {
		value, ok := rawItems[i].(string)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		items = append(items, value)
	}
	return items, nil
}

func (s *Store) ClaimWorkerQueueBatch(
	queueName string,
	batchSize int,
	visibilityTimeout time.Duration,
) ([]WorkerQueueClaim, error) {
	nextQueueName := strings.TrimSpace(queueName)
	if nextQueueName == "" {
		return nil, errors.New("worker queue name is required")
	}
	if batchSize <= 0 {
		return nil, errors.New("worker queue batch size must be positive")
	}
	if visibilityTimeout <= 0 {
		return nil, errors.New("worker queue visibility timeout must be positive")
	}

	now := time.Now().UTC()
	visibilityMillis := visibilityTimeout.Milliseconds()
	if visibilityMillis <= 0 {
		visibilityMillis = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.client.Eval(
		ctx,
		`local batch = tonumber(ARGV[1]) or 0
		local visibility_ms = tonumber(ARGV[2]) or 0
		local now_ms = tonumber(ARGV[3]) or 0
		if batch <= 0 or visibility_ms <= 0 then
			return {}
		end
		local expired = redis.call("ZRANGEBYSCORE", KEYS[3], "-inf", now_ms, "LIMIT", 0, batch)
		for i = 1, #expired do
			local item = expired[i]
			redis.call("ZREM", KEYS[3], item)
			redis.call("HDEL", KEYS[4], item)
			redis.call("SADD", KEYS[2], item)
			redis.call("LREM", KEYS[1], 0, item)
			redis.call("RPUSH", KEYS[1], item)
		end
		local items = {}
		local seen = {}
		local skipped_claimed = {}
		while #items < batch do
			local item = redis.call("LPOP", KEYS[1])
			if not item then
				break
			end
			if seen[item] then
			elseif redis.call("ZSCORE", KEYS[3], item) or redis.call("HEXISTS", KEYS[4], item) == 1 then
				redis.call("SADD", KEYS[2], item)
				skipped_claimed[item] = 1
			else
				seen[item] = 1
				items[#items + 1] = item
			end
		end
		for item, _ in pairs(skipped_claimed) do
			redis.call("LREM", KEYS[1], 0, item)
		end
		local count = #items
		if count == 0 then
			return {}
		end
		local deadline = now_ms + visibility_ms
		local claims = {}
		for i = 1, count do
			local item = items[i]
			redis.call("LREM", KEYS[1], 0, item)
			local seq = redis.call("INCR", KEYS[5])
			local token = redis.sha1hex(item .. ":" .. tostring(now_ms) .. ":" .. tostring(seq))
			redis.call("ZADD", KEYS[3], deadline, item)
			redis.call("HSET", KEYS[4], item, token)
			claims[#claims + 1] = item
			claims[#claims + 1] = token
		end
		return claims`,
		[]string{
			s.workerQueueKey(nextQueueName),
			s.workerQueueIndexKey(nextQueueName),
			s.workerQueueProcessingKey(nextQueueName),
			s.workerQueueClaimKey(nextQueueName),
			s.workerQueueClaimSeqKey(nextQueueName),
		},
		strconv.Itoa(batchSize),
		strconv.FormatInt(visibilityMillis, 10),
		strconv.FormatInt(now.UnixMilli(), 10),
	).Result()
	if err != nil {
		return nil, err
	}

	rawClaims, ok := result.([]any)
	if !ok {
		return nil, errors.New("worker queue claim returned unexpected payload")
	}
	claims := make([]WorkerQueueClaim, 0, len(rawClaims)/2)
	for i := 0; i+1 < len(rawClaims); i += 2 {
		itemID, ok := rawClaims[i].(string)
		if !ok {
			continue
		}
		claimToken, ok := rawClaims[i+1].(string)
		if !ok {
			continue
		}
		itemID = strings.TrimSpace(itemID)
		claimToken = strings.TrimSpace(claimToken)
		if itemID == "" || claimToken == "" {
			continue
		}
		claims = append(claims, WorkerQueueClaim{
			ItemID:     itemID,
			ClaimToken: claimToken,
		})
	}
	return claims, nil
}

func (s *Store) AckWorkerQueue(queueName, itemID, claimToken string) (bool, error) {
	nextQueueName := strings.TrimSpace(queueName)
	nextItemID := strings.TrimSpace(itemID)
	nextClaimToken := strings.TrimSpace(claimToken)
	if nextQueueName == "" || nextItemID == "" || nextClaimToken == "" {
		return false, errors.New("worker queue name/item id/claim token are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.client.Eval(
		ctx,
		`if redis.call("HGET", KEYS[4], ARGV[1]) ~= ARGV[2] then
			return 0
		end
		redis.call("ZREM", KEYS[3], ARGV[1])
		redis.call("HDEL", KEYS[4], ARGV[1])
		redis.call("LREM", KEYS[1], 0, ARGV[1])
		redis.call("SREM", KEYS[2], ARGV[1])
		return 1`,
		[]string{
			s.workerQueueKey(nextQueueName),
			s.workerQueueIndexKey(nextQueueName),
			s.workerQueueProcessingKey(nextQueueName),
			s.workerQueueClaimKey(nextQueueName),
		},
		nextItemID,
		nextClaimToken,
	).Result()
	if err != nil {
		return false, err
	}
	applied, err := parseWorkerQueueMutationApplied(result)
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (s *Store) RequeueWorkerQueue(queueName, itemID, claimToken string) (bool, error) {
	nextQueueName := strings.TrimSpace(queueName)
	nextItemID := strings.TrimSpace(itemID)
	nextClaimToken := strings.TrimSpace(claimToken)
	if nextQueueName == "" || nextItemID == "" || nextClaimToken == "" {
		return false, errors.New("worker queue name/item id/claim token are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.client.Eval(
		ctx,
		`if redis.call("HGET", KEYS[4], ARGV[1]) ~= ARGV[2] then
			return 0
		end
		redis.call("ZREM", KEYS[3], ARGV[1])
		redis.call("HDEL", KEYS[4], ARGV[1])
		redis.call("SADD", KEYS[2], ARGV[1])
		redis.call("LREM", KEYS[1], 0, ARGV[1])
		redis.call("RPUSH", KEYS[1], ARGV[1])
		return 1`,
		[]string{
			s.workerQueueKey(nextQueueName),
			s.workerQueueIndexKey(nextQueueName),
			s.workerQueueProcessingKey(nextQueueName),
			s.workerQueueClaimKey(nextQueueName),
		},
		nextItemID,
		nextClaimToken,
	).Result()
	if err != nil {
		return false, err
	}
	applied, err := parseWorkerQueueMutationApplied(result)
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (s *Store) DescribeWorkerQueue(
	queueName string,
	itemIDs []string,
) (WorkerQueueTelemetry, error) {
	nextQueueName := strings.TrimSpace(queueName)
	if nextQueueName == "" {
		return WorkerQueueTelemetry{}, errors.New("worker queue name is required")
	}

	normalizedIDs := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for i := range itemIDs {
		nextItemID := strings.TrimSpace(itemIDs[i])
		if nextItemID == "" {
			continue
		}
		if _, ok := seen[nextItemID]; ok {
			continue
		}
		seen[nextItemID] = struct{}{}
		normalizedIDs = append(normalizedIDs, nextItemID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := make([]string, 0, len(normalizedIDs))
	args = append(args, normalizedIDs...)
	result, err := s.client.Eval(
		ctx,
		`local pending_items = redis.call("LRANGE", KEYS[1], 0, -1)
		local queued = {}
		local pending_count = 0
		for i = 1, #pending_items do
			local item = pending_items[i]
			if not queued[item] and not redis.call("ZSCORE", KEYS[3], item) and redis.call("HEXISTS", KEYS[4], item) == 0 then
				queued[item] = 1
				pending_count = pending_count + 1
			end
		end
		local result = {
			pending_count,
			redis.call("ZCARD", KEYS[3]),
		}
		for i = 1, #ARGV do
			local item = ARGV[i]
			local score = redis.call("ZSCORE", KEYS[3], item)
			result[#result + 1] = item
			if score or redis.call("HEXISTS", KEYS[4], item) == 1 then
				result[#result + 1] = "claimed"
				if score then
					result[#result + 1] = tostring(score)
				else
					result[#result + 1] = ""
				end
			elseif queued[item] then
				result[#result + 1] = "queued"
				result[#result + 1] = ""
			else
				result[#result + 1] = "missing"
				result[#result + 1] = ""
			end
		end
		return result`,
		[]string{
			s.workerQueueKey(nextQueueName),
			s.workerQueueIndexKey(nextQueueName),
			s.workerQueueProcessingKey(nextQueueName),
			s.workerQueueClaimKey(nextQueueName),
		},
		args,
	).Result()
	if err != nil {
		return WorkerQueueTelemetry{}, err
	}

	rawValues, ok := result.([]any)
	if !ok || len(rawValues) < 2 {
		return WorkerQueueTelemetry{}, errors.New("worker queue describe returned unexpected payload")
	}
	pendingCount, err := parseWorkerQueueTelemetryInt(rawValues[0])
	if err != nil {
		return WorkerQueueTelemetry{}, err
	}
	claimedCount, err := parseWorkerQueueTelemetryInt(rawValues[1])
	if err != nil {
		return WorkerQueueTelemetry{}, err
	}

	items := make(map[string]WorkerQueueItemState, len(normalizedIDs))
	for i := 2; i+2 < len(rawValues); i += 3 {
		itemID, ok := rawValues[i].(string)
		if !ok {
			continue
		}
		state, ok := rawValues[i+1].(string)
		if !ok {
			continue
		}
		itemID = strings.TrimSpace(itemID)
		state = strings.TrimSpace(state)
		if itemID == "" || state == "" {
			continue
		}
		itemState := WorkerQueueItemState{State: state}
		if visibilityAt, ok := parseWorkerQueueVisibilityDeadline(rawValues[i+2]); ok {
			itemState.VisibilityDeadlineAt = &visibilityAt
		}
		items[itemID] = itemState
	}

	return WorkerQueueTelemetry{
		PendingCount: pendingCount,
		ClaimedCount: claimedCount,
		Items:        items,
	}, nil
}

func (s *Store) SetHikConnectAccessToken(token string, ttl time.Duration) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("hikconnect access token is empty")
	}
	if ttl <= 0 {
		return errors.New("hikconnect access token ttl must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.hikConnectAccessTokenKey(), token, ttl).Err()
}

func (s *Store) GetHikConnectAccessToken() (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := s.client.Get(ctx, s.hikConnectAccessTokenKey()).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) DeleteHikConnectAccessToken() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Del(ctx, s.hikConnectAccessTokenKey()).Err()
}

func (s *Store) SetHikConnectDeviceToken(serial, token string, ttl time.Duration) error {
	nextSerial := strings.TrimSpace(serial)
	if nextSerial == "" || strings.TrimSpace(token) == "" {
		return errors.New("hikconnect device serial/token are required")
	}
	if ttl <= 0 {
		return errors.New("hikconnect device token ttl must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.client.Set(ctx, s.hikConnectDeviceTokenKey(nextSerial), token, ttl).Err()
}

func (s *Store) GetHikConnectDeviceToken(serial string) (string, bool, error) {
	nextSerial := strings.TrimSpace(serial)
	if nextSerial == "" {
		return "", false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	value, err := s.client.Get(ctx, s.hikConnectDeviceTokenKey(nextSerial)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) hikConnectAccessTokenKey() string {
	return s.key("hikconnect", "access_token")
}

func (s *Store) hikConnectDeviceTokenKey(serial string) string {
	return s.key("hikconnect", "device_token", serial)
}

func (s *Store) authRefreshSessionKey(sessionID string) string {
	return s.key("auth", "refresh", "session", sessionID)
}

func (s *Store) authRefreshUserIndexKey(userID string) string {
	return s.key("auth", "refresh", "user", userID)
}

func (s *Store) authRevokedAccessTokenKey(tokenID string) string {
	return s.key("auth", "revoked", "access", tokenID)
}

func (s *Store) rateLimitCounterKey(scope, key string, bucket int64) string {
	return s.key(
		"ratelimit",
		scope,
		strconv.FormatInt(bucket, 10),
		hashValue(key),
	)
}

func (s *Store) gatewayRequestNonceKey(gatewayID, nonce string) string {
	return s.key("gateway", "nonce", gatewayID, hashValue(nonce))
}

func (s *Store) workerLeaseKey(key string) string {
	return s.key("worker", "lease", key)
}

func (s *Store) workerQueueKey(queueName string) string {
	return s.key("worker", "queue", queueName)
}

func (s *Store) workerQueueIndexKey(queueName string) string {
	return s.key("worker", "queue", "index", queueName)
}

func (s *Store) workerQueueProcessingKey(queueName string) string {
	return s.key("worker", "queue", "processing", queueName)
}

func (s *Store) workerQueueClaimKey(queueName string) string {
	return s.key("worker", "queue", "claim", queueName)
}

func (s *Store) workerQueueClaimSeqKey(queueName string) string {
	return s.key("worker", "queue", "claim-seq", queueName)
}

func (s *Store) key(parts ...string) string {
	items := make([]string, 0, len(parts)+1)
	if s.keyPrefix != "" {
		items = append(items, s.keyPrefix)
	}
	for i := range parts {
		next := strings.TrimSpace(parts[i])
		if next == "" {
			continue
		}
		items = append(items, next)
	}
	return strings.Join(items, ":")
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func decodeRefreshSession(raw string) (struct {
	UserID    string
	ExpiresAt time.Time
}, bool) {
	payload := refreshSessionPayload{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return struct {
			UserID    string
			ExpiresAt time.Time
		}{}, false
	}
	nextUserID := strings.TrimSpace(payload.UserID)
	nextExpiresAt := time.Unix(payload.ExpiresAt, 0).UTC()
	if nextUserID == "" || nextExpiresAt.IsZero() {
		return struct {
			UserID    string
			ExpiresAt time.Time
		}{}, false
	}
	return struct {
		UserID    string
		ExpiresAt time.Time
	}{
		UserID:    nextUserID,
		ExpiresAt: nextExpiresAt,
	}, true
}

func parseWorkerQueueTelemetryInt(value any) (int, error) {
	switch typed := value.(type) {
	case int64:
		return int(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		return int(parsed), nil
	default:
		return 0, errors.New("worker queue telemetry returned unexpected count payload")
	}
}

func parseWorkerQueueMutationApplied(value any) (bool, error) {
	switch typed := value.(type) {
	case int64:
		return typed > 0, nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return false, err
		}
		return parsed > 0, nil
	default:
		return false, errors.New("worker queue mutation returned unexpected payload")
	}
}

func parseWorkerQueueVisibilityDeadline(value any) (time.Time, bool) {
	raw, ok := value.(string)
	if !ok {
		return time.Time{}, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	score, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return time.Time{}, false
	}
	deadline := time.UnixMilli(int64(score)).UTC()
	if deadline.IsZero() {
		return time.Time{}, false
	}
	return deadline, true
}

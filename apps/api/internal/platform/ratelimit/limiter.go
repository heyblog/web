package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix       = "heyblog:ratelimit:v1:"
	minimumTTL      = time.Second
	maximumTTL      = 24 * time.Hour
	ttlRefillCycles = 2
)

var validPolicyName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

var tokenBucketScript = redis.NewScript(`
local now_parts = redis.call('TIME')
local now_ms = (tonumber(now_parts[1]) * 1000) + math.floor(tonumber(now_parts[2]) / 1000)
local capacity = tonumber(ARGV[1])
local refill_per_ms = tonumber(ARGV[2])
local ttl_ms = tonumber(ARGV[3])

local state = redis.call('HMGET', KEYS[1], 'tokens', 'last_ms')
local tokens = tonumber(state[1]) or capacity
local last_ms = tonumber(state[2]) or now_ms
if now_ms > last_ms then
  tokens = math.min(capacity, tokens + ((now_ms - last_ms) * refill_per_ms))
end

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

local retry_ms = 0
if allowed == 0 then
  retry_ms = math.ceil((1 - tokens) / refill_per_ms)
end
local reset_ms = math.ceil((capacity - tokens) / refill_per_ms)

redis.call('HSET', KEYS[1], 'tokens', tokens, 'last_ms', now_ms)
redis.call('PEXPIRE', KEYS[1], ttl_ms)
return {allowed, math.floor(tokens), retry_ms, reset_ms}
`)

type Policy struct {
	Name           string
	Capacity       int64
	RefillTokens   int64
	RefillInterval time.Duration
}

type Decision struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}

type evalFunc func(context.Context, []string, ...any) ([]any, error)

type Limiter struct {
	eval evalFunc
}

func New(client redis.Scripter) *Limiter {
	return newWithEval(func(ctx context.Context, keys []string, arguments ...any) ([]any, error) {
		return tokenBucketScript.Run(ctx, client, keys, arguments...).Slice()
	})
}

func newWithEval(eval evalFunc) *Limiter {
	return &Limiter{eval: eval}
}

func (limiter *Limiter) Allow(ctx context.Context, identifier string, policy Policy) (Decision, error) {
	if err := policy.validate(); err != nil {
		return Decision{}, err
	}
	if identifier == "" {
		return Decision{}, fmt.Errorf("rate limit identifier is required")
	}

	intervalMilliseconds := policy.RefillInterval.Milliseconds()
	refillPerMillisecond := float64(policy.RefillTokens) / float64(intervalMilliseconds)
	ttlMilliseconds := float64(policy.Capacity) / refillPerMillisecond * ttlRefillCycles
	ttl := maximumTTL
	if ttlMilliseconds < float64(maximumTTL.Milliseconds()) {
		ttl = time.Duration(math.Ceil(ttlMilliseconds)) * time.Millisecond
	}
	if ttl < minimumTTL {
		ttl = minimumTTL
	}

	result, err := limiter.eval(
		ctx,
		[]string{redisKey(policy.Name, identifier)},
		policy.Capacity,
		strconv.FormatFloat(refillPerMillisecond, 'g', -1, 64),
		ttl.Milliseconds(),
	)
	if err != nil {
		return Decision{}, fmt.Errorf("execute Redis token bucket: %w", err)
	}
	if len(result) != 4 {
		return Decision{}, fmt.Errorf("decode Redis token bucket: expected four values")
	}
	allowed, err := integerResult(result[0])
	if err != nil {
		return Decision{}, fmt.Errorf("decode Redis token bucket allowed value: %w", err)
	}
	remaining, err := integerResult(result[1])
	if err != nil {
		return Decision{}, fmt.Errorf("decode Redis token bucket remaining value: %w", err)
	}
	retryMilliseconds, err := integerResult(result[2])
	if err != nil {
		return Decision{}, fmt.Errorf("decode Redis token bucket retry value: %w", err)
	}
	resetMilliseconds, err := integerResult(result[3])
	if err != nil {
		return Decision{}, fmt.Errorf("decode Redis token bucket reset value: %w", err)
	}
	return Decision{
		Allowed:    allowed == 1,
		Limit:      policy.Capacity,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMilliseconds) * time.Millisecond,
		ResetAfter: time.Duration(resetMilliseconds) * time.Millisecond,
	}, nil
}

func (policy Policy) validate() error {
	if !validPolicyName.MatchString(policy.Name) {
		return fmt.Errorf("rate limit policy name is invalid")
	}
	if policy.Capacity < 1 || policy.RefillTokens < 1 {
		return fmt.Errorf("rate limit capacity and refill tokens must be positive")
	}
	if policy.RefillInterval < time.Millisecond {
		return fmt.Errorf("rate limit refill interval must be at least one millisecond")
	}
	return nil
}

func redisKey(policyName, identifier string) string {
	digest := sha256.Sum256([]byte(identifier))
	return keyPrefix + policyName + ":" + hex.EncodeToString(digest[:])
}

func integerResult(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}

package sluggeneration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"heyblog-api/internal/platform/apperror"
	"heyblog-api/internal/platform/config"
)

const guardPrefix = "heyblog:{slug-generation}:"

type RedisGuard struct {
	client *redis.Client
	limits config.AILimits
	lease  time.Duration
}

type limitedError struct {
	cause      error
	RetryAfter int64
}

func (err *limitedError) Error() string { return err.cause.Error() }
func (err *limitedError) Unwrap() error { return err.cause }

func NewRedisGuard(client *redis.Client, configuration config.AIConfig) *RedisGuard {
	return &RedisGuard{client: client, limits: configuration.Limits, lease: configuration.Timeout + 5*time.Second}
}

func digestIdentity(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func rateFailure(code string, retry int64) error {
	return &limitedError{cause: apperror.New(apperror.KindRateLimited, code, "slug generation limit exceeded"), RetryAfter: max(1, retry)}
}

func (guard *RedisGuard) Allow(ctx context.Context, identity Identity) error {
	if guard.client == nil {
		return unavailable()
	}
	ipHash := identity.IPHash
	if ipHash == "" {
		ipHash = digestIdentity(identity.IP)
	}
	values, err := requestScript.Run(ctx, guard.client, []string{
		guardPrefix + "user:" + digestIdentity(identity.UserID), guardPrefix + "ip:" + ipHash,
	}, guard.limits.UserPerMinute, guard.limits.IPPerMinute).Int64Slice()
	if err != nil || len(values) != 2 {
		return unavailable()
	}
	if values[0] != 0 {
		return rateFailure("slug_rate_limited", values[1])
	}
	return nil
}

func (guard *RedisGuard) Acquire(ctx context.Context, attempt Attempt) (string, error) {
	if guard.client == nil {
		return "", unavailable()
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", unavailable()
	}
	owner := hex.EncodeToString(bytes)
	keys := []string{guardPrefix + "attempt:" + attempt.Key, guardPrefix + "active"}
	values, err := acquireScript.Run(ctx, guard.client, keys, owner, guard.lease.Milliseconds(), guard.limits.Concurrent).Int64Slice()
	if err != nil || len(values) != 2 {
		return "", unavailable()
	}
	switch values[0] {
	case 0:
		return owner + ":" + attempt.Key, nil
	case 1:
		return "", apperror.New(apperror.KindConflict, "generation_in_progress", "a matching slug generation is already in progress")
	case 2:
		return "", rateFailure("slug_concurrency_limited", values[1])
	default:
		return "", unavailable()
	}
}

func (guard *RedisGuard) Charge(ctx context.Context, userID string) error {
	if guard.client == nil {
		return unavailable()
	}
	values, err := budgetScript.Run(ctx, guard.client, []string{guardPrefix + "daily-user:" + digestIdentity(userID), guardPrefix + "daily-global"}, guard.limits.UserPerDay, guard.limits.GlobalPerDay).Int64Slice()
	if err != nil || len(values) != 2 {
		return unavailable()
	}
	if values[0] != 0 {
		return rateFailure("slug_daily_limit", values[1])
	}
	return nil
}

func (guard *RedisGuard) Release(ctx context.Context, lease string) error {
	owner, key, ok := strings.Cut(lease, ":")
	if !ok {
		return errors.New("invalid slug generation lease")
	}
	return releaseScript.Run(ctx, guard.client, []string{guardPrefix + "attempt:" + key, guardPrefix + "active"}, owner).Err()
}

func retryAfter(err error) string {
	var limited *limitedError
	if errors.As(err, &limited) {
		return strconv.FormatInt(limited.RetryAfter, 10)
	}
	return ""
}

var requestScript = redis.NewScript(`
local now = tonumber(redis.call('TIME')[1])
local period = math.floor(now / 60)
local ttl = 60 - now % 60
local u = KEYS[1] .. ':' .. period
local ip = KEYS[2] .. ':' .. period
if tonumber(redis.call('GET', u) or '0') >= tonumber(ARGV[1]) or tonumber(redis.call('GET', ip) or '0') >= tonumber(ARGV[2]) then
 return {1, ttl}
end
redis.call('INCR', u)
redis.call('EXPIRE', u, ttl)
redis.call('INCR', ip)
redis.call('EXPIRE', ip, ttl)
return {0, 0}
`)

var acquireScript = redis.NewScript(`
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
if redis.call('EXISTS', KEYS[1]) == 1 then return {1, 1} end
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', now)
if redis.call('ZCARD', KEYS[2]) >= tonumber(ARGV[3]) then return {2, math.ceil(tonumber(ARGV[2])/1000)} end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('ZADD', KEYS[2], now + tonumber(ARGV[2]), ARGV[1])
redis.call('PEXPIRE', KEYS[2], ARGV[2])
return {0, 0}
`)

var releaseScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then redis.call('DEL', KEYS[1]) end
return redis.call('ZREM', KEYS[2], ARGV[1])
`)

var budgetScript = redis.NewScript(`
local seconds = tonumber(redis.call('TIME')[1])
local day = math.floor(seconds / 86400)
local ttl = 86400 - seconds % 86400
local userday = KEYS[1] .. ':' .. day
local globalday = KEYS[2] .. ':' .. day
if tonumber(redis.call('GET', userday) or '0') >= tonumber(ARGV[1]) or tonumber(redis.call('GET', globalday) or '0') >= tonumber(ARGV[2]) then return {1, ttl} end
redis.call('INCR', userday)
redis.call('EXPIRE', userday, ttl)
redis.call('INCR', globalday)
redis.call('EXPIRE', globalday, ttl)
return {0, 0}
`)

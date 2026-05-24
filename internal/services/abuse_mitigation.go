package services

import (
	"context"
	"errors"
	"time"

	"github.com/AnshRaj112/serenify-backend/internal/database"
	"github.com/redis/go-redis/v9"
)

// CheckReportRateLimit executes atomic evaluation of IP reporting thresholds using a Redis token bucket.
func CheckReportRateLimit(ctx context.Context, ipAddress string) error {
	if database.RedisClient == nil {
		// Fallback if Redis is not configured in local environment
		return nil
	}

	key := "limit_ip:" + ipAddress
	now := time.Now().Unix()

	// Redis Lua script to atomically implement the Token Bucket rate limit
	luaScript := `
		local key = KEYS[1]
		local max_tokens = 5
		local fill_rate = 720 -- 1 token every 12 minutes (5 per hour)
		local now = tonumber(ARGV[1])

		local data = redis.call("HMGET", key, "tokens", "last_fill")
		local tokens = tonumber(data[1])
		local last_fill = tonumber(data[2])

		if not tokens then
			tokens = max_tokens
			last_fill = now
		else
			local elapsed = now - last_fill
			local add = math.floor(elapsed / fill_rate)
			if add > 0 then
				tokens = math.min(max_tokens, tokens + add)
				last_fill = last_fill + (add * fill_rate)
			end
		end

		if tokens > 0 then
			redis.call("HMSET", key, "tokens", tokens - 1, "last_fill", last_fill)
			redis.call("EXPIRE", key, 3600)
			return 1
		else
			return 0
		end
	`

	res, err := database.RedisClient.Eval(ctx, luaScript, []string{key}, now).Int()
	if err != nil {
		return err
	}
	if res == 0 {
		return errors.New("rate limit exceeded: max 5 reports per hour allowed")
	}
	return nil
}

// PrioritizeSevereCrisis flags severe-risk reports (self-harm, threats) to expedite moderation queues in Redis.
func PrioritizeSevereCrisis(ctx context.Context, reportID string, category string) {
	if database.RedisClient == nil {
		return
	}

	score := 0.0
	if category == "self_harm" || category == "threats" || category == "violent threats" {
		score = 1.0 // High Priority
	}

	_ = database.RedisClient.ZAdd(ctx, "moderation:priority:set", redis.Z{
		Score:  score,
		Member: reportID,
	}).Err()
}

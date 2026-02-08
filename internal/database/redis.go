package database

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

// ConnectRedis connects to Redis database
func ConnectRedis(redisURI string) error {
	opt, err := redis.ParseURL(redisURI)
	if err != nil {
		return err
	}

	RedisClient = redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return err
	}

	log.Println("✅ Connected to Redis")
	return nil
}

// DisconnectRedis closes the Redis connection
func DisconnectRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}


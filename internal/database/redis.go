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

	// Configure connection pool and timeouts for better resilience
	opt.PoolSize = 10                     // Connection pool size
	opt.MinIdleConns = 5                  // Minimum idle connections
	opt.MaxRetries = 3                    // Retry failed commands up to 3 times
	opt.DialTimeout = 5 * time.Second     // Timeout for establishing connection
	opt.ReadTimeout = 3 * time.Second     // Timeout for read operations
	opt.WriteTimeout = 3 * time.Second    // Timeout for write operations
	opt.PoolTimeout = 4 * time.Second     // Timeout for getting connection from pool
	opt.ConnMaxIdleTime = 5 * time.Minute // Close idle connections after 5 minutes

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

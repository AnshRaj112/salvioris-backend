# Rate Limiting Documentation

## Overview

The Serenify backend implements a comprehensive rate limiting system using Redis to protect APIs from abuse, DDoS attacks, and excessive usage. The rate limiting middleware automatically tracks requests per IP address and blocks abusive IPs.

## Features

- **Time Window**: 120 seconds (2 minutes)
- **Request Limit**: 25 requests per window per IP address
- **IP Blocking**: Automatic blocking for 24 hours after exceeding limit
- **Redis-Based**: Uses Redis for fast, distributed rate limiting
- **Automatic Cleanup**: Blocked IPs automatically unblock after 24 hours

## How It Works

### Rate Limiting Flow

1. **Request Arrives**: Every API request (except `/health`) goes through the rate limiting middleware
2. **IP Extraction**: The middleware extracts the client's IP address from:
   - `X-Forwarded-For` header (first IP if multiple)
   - `X-Real-IP` header
   - `RemoteAddr` (fallback)
3. **Check Blocked Status**: First checks if the IP is already blocked
4. **Count Requests**: Increments a counter in Redis for the IP address
5. **Evaluate Limit**: 
   - If under limit: Request proceeds normally
   - If limit exceeded: IP is blocked and request is rejected

### Rate Limit Window

- **Window Duration**: 120 seconds (2 minutes)
- **Max Requests**: 25 requests per window
- **Sliding Window**: Uses Redis TTL to create a sliding window effect
- **Counter Reset**: Counter automatically expires after 120 seconds

### IP Blocking

When an IP exceeds 25 requests in a 120-second window:

1. **Immediate Block**: IP is added to blocked list
2. **Block Duration**: 24 hours
3. **Automatic Unblock**: Block expires automatically after 24 hours
4. **Response**: Returns HTTP 429 (Too Many Requests) with error message

## Configuration

### Environment Variables

No additional configuration needed. The rate limiting uses the same Redis connection as sessions and caching:

```bash
REDIS_URI=redis://localhost:6379/0
```

### Redis Keys

The middleware uses the following Redis key patterns:

- **Rate Limit Counter**: `ratelimit:{ip_address}`
  - Value: Request count (integer)
  - TTL: 120 seconds
  
- **Blocked IP**: `blocked_ip:{ip_address}`
  - Value: "1"
  - TTL: 24 hours

## API Response Headers

When rate limiting is active, the following headers are included in responses:

### Normal Requests (Under Limit)

```
X-RateLimit-Limit: 25
X-RateLimit-Remaining: 20
X-RateLimit-Reset: 1234567890
```

- `X-RateLimit-Limit`: Maximum requests allowed in the window
- `X-RateLimit-Remaining`: Number of requests remaining in current window
- `X-RateLimit-Reset`: Unix timestamp when the rate limit window resets

### Blocked Requests

```
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
```

Response Body:
```json
{
  "success": false,
  "message": "Rate limit exceeded. Your IP has been temporarily blocked. Please try again later.",
  "retry_after": 120
}
```

## Implementation Details

### Middleware Location

File: `internal/middleware/ratelimit.go`

### Applied Routes

The rate limiting middleware is applied globally to all routes except:
- `GET /health` - Health check endpoint (no rate limit)

All other routes are protected:
- `/api/auth/*` - Authentication endpoints
- `/api/vent` - Vent/message endpoints
- `/api/therapist/*` - Therapist endpoints
- `/api/admin/*` - Admin endpoints
- `/api/contact` - Contact form
- `/api/feedback` - Feedback endpoints

### Code Example

```go
// Rate limiting is applied in main.go
r.Use(middleware.RateLimitMiddleware)

// Health check (no rate limit)
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("OK"))
})

// All other routes are rate limited
routes.SetupRoutes(r)
```

## Testing Rate Limits

### Test Normal Usage

```bash
# Make 24 requests (under limit)
for i in {1..24}; do
  curl -X POST http://localhost:8080/api/vent \
    -H "Content-Type: application/json" \
    -d '{"message":"test"}'
done
```

### Test Rate Limit Exceeded

```bash
# Make 26 requests (exceeds limit of 25)
for i in {1..26}; do
  curl -X POST http://localhost:8080/api/vent \
    -H "Content-Type: application/json" \
    -d '{"message":"test"}'
done

# 26th request will return 429
```

### Check Rate Limit Headers

```bash
curl -v -X POST http://localhost:8080/api/vent \
  -H "Content-Type: application/json" \
  -d '{"message":"test"}'

# Response headers will include:
# X-RateLimit-Limit: 25
# X-RateLimit-Remaining: 24
# X-RateLimit-Reset: 1234567890
```

## Admin Functions

### Unblock an IP Address

If you need to manually unblock an IP address, you can use the admin endpoint:

```bash
PUT /api/admin/unblock-ip
```

Or directly in code:

```go
import "github.com/AnshRaj112/serenify-backend/internal/middleware"

// Unblock an IP
err := middleware.UnblockIP("192.168.1.100")
```

### Check if IP is Blocked

```go
import "github.com/AnshRaj112/serenify-backend/internal/middleware"

// Check block status
isBlocked, err := middleware.IsIPBlocked("192.168.1.100")
```

## Best Practices

### For Frontend Developers

1. **Handle 429 Responses**: Implement retry logic with exponential backoff
2. **Monitor Headers**: Use `X-RateLimit-Remaining` to show users their remaining requests
3. **User Feedback**: Display rate limit warnings when `X-RateLimit-Remaining` is low
4. **Retry After**: Use `retry_after` value from error response for retry timing

### For Backend Developers

1. **Monitor Redis**: Ensure Redis is healthy for rate limiting to work
2. **Fail Open**: If Redis is unavailable, requests are allowed (fail open strategy)
3. **Logging**: Consider logging blocked IPs for security monitoring
4. **Adjust Limits**: Modify constants in `ratelimit.go` if needed:
   - `RateLimitWindow`: Time window duration
   - `RateLimitMaxRequests`: Maximum requests per window
   - `BlockedIPDuration`: How long IPs stay blocked

## Troubleshooting

### Rate Limiting Not Working

1. **Check Redis Connection**: Ensure Redis is running and accessible
2. **Verify Middleware**: Confirm middleware is applied in `main.go`
3. **Check IP Extraction**: Verify IP is being extracted correctly (check logs)

### IPs Getting Blocked Too Easily

1. **Increase Limit**: Modify `RateLimitMaxRequests` in `ratelimit.go`
2. **Increase Window**: Modify `RateLimitWindow` to allow more time
3. **Review Logs**: Check if legitimate users are being affected

### IPs Not Getting Blocked

1. **Check Redis**: Ensure Redis is working and keys are being set
2. **Verify Counter**: Check Redis for `ratelimit:{ip}` keys
3. **Test Manually**: Make 26+ requests and verify blocking

## Security Considerations

1. **IP Spoofing**: Rate limiting relies on IP addresses, which can be spoofed. Consider additional authentication for sensitive endpoints.

2. **Distributed Attacks**: If using multiple servers, ensure they share the same Redis instance for consistent rate limiting.

3. **Legitimate High Traffic**: Legitimate users with high traffic may be blocked. Consider:
   - Whitelisting certain IPs
   - Higher limits for authenticated users
   - Different limits per endpoint

4. **Redis Security**: Ensure Redis is properly secured:
   - Use authentication (password)
   - Use TLS for remote connections
   - Restrict network access

## Monitoring

### Redis Keys to Monitor

```bash
# Check rate limit counters
redis-cli KEYS "ratelimit:*"

# Check blocked IPs
redis-cli KEYS "blocked_ip:*"

# Get count of blocked IPs
redis-cli --scan --pattern "blocked_ip:*" | wc -l
```

### Metrics to Track

- Number of blocked IPs
- Rate limit hit rate
- Average requests per IP
- Redis memory usage for rate limiting keys

## Future Enhancements

Potential improvements to consider:

1. **Per-Endpoint Limits**: Different limits for different endpoints
2. **User-Based Limits**: Higher limits for authenticated users
3. **Whitelist/Blacklist**: Manual IP management
4. **Rate Limit Logging**: Log all rate limit events
5. **Metrics Dashboard**: Real-time rate limit monitoring
6. **Adaptive Limits**: Adjust limits based on server load

## Related Documentation

- [Session Management](./SESSION_MANAGEMENT.md) - 7-day session system
- [Caching](./CACHING.md) - Redis caching for performance
- [Privacy Architecture](./PRIVACY_ARCHITECTURE.md) - Overall system architecture


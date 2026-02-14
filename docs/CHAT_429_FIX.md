# Chat History 429 Fix – Production Implementation

## Summary

HTTP 429 on `GET /api/chat/history` was caused by:

1. **Frontend `useEffect` dependency loop** – `loadHistory` depended on `messages`, so each load updated `messages` → recreated `loadHistory` → triggered `useEffect` again → repeated requests.
2. **Strict global rate limit** – Production used 1 req/s, burst 10 for all routes, so rapid history calls quickly hit 429.
3. **No Redis cache** – Every history request hit MongoDB, increasing load and request frequency under the same limit.

## 1. Frontend Audit (Next.js)

### Root cause

```tsx
// BEFORE (bug)
const loadHistory = useCallback(async (...) => { ... }, [activeGroup, messages]);
useEffect(() => { if (activeGroup) loadHistory(false); }, [activeGroup, loadHistory]);
```

- `loadHistory` depended on `messages`.
- Each call to `loadHistory` did `setMessages(...)`.
- That changed `messages` → new `loadHistory` → `useEffect` ran again → another `loadHistory` call.
- Result: repeated requests until rate limit was hit.

### Fixed pattern

```tsx
// AFTER
const loadHistory = useCallback(async (appendTop: boolean) => {
  const group = activeGroupRef.current;
  if (!group) return;
  if (historyRequestLockRef.current) return;  // Deduplication
  historyRequestLockRef.current = true;
  const reqId = ++historyRequestIdRef.current;
  // ... fetch, then setMessages with functional update for appendTop
}, []); // No messages in deps

useEffect(() => {
  const groupId = activeGroup?.id ?? null;
  if (!groupId) { /* clear */ return; }
  if (lastLoadedGroupIdRef.current === groupId) return;  // Same group, skip
  lastLoadedGroupIdRef.current = groupId;
  loadHistory(false);
}, [activeGroup?.id, loadHistory]);  // Only group id, not full object
```

### Guards

| Guard | Purpose |
|-------|---------|
| `historyRequestLockRef` | Blocks concurrent / repeated in-flight requests |
| `lastLoadedGroupIdRef` | Avoids reload when switching back to the same group |
| `historyRequestIdRef` | Cancels stale responses |
| `oldestMessageRef` | Provides `before` for “load older” without `messages` in deps |

### Reconnect-safe behavior

- WebSocket reconnect is separate from `activeGroup?.id`.
- `useEffect` depends only on `activeGroup?.id`.
- Reconnect does not change `activeGroup?.id`, so history is not reloaded.

### Multi-tab

- Each tab has its own state and refs.
- No shared `BroadcastChannel` is needed for correctness.
- Each tab makes one initial load when opening a group.

---

## 2. Backend Rate Limiting (Golang)

### Chat history middleware

- File: `internal/middleware/chat_ratelimit.go`
- Uses `golang.org/x/time/rate` (token bucket)
- Auth vs anonymous:
  - **Authenticated** (Bearer token): 30/min, burst 20
  - **Anonymous**: 10/min, burst 5

### Global rate limit change

- `GlobalRateLimit` in `security.go` skips `GET /api/chat/history`.
- `ChatHistoryRateLimit` is used only for that path.

### Headers

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`

### Middleware chain

1. CORS
2. ProductionSecurity (or Redis rate limit in dev)
3. `ChatHistoryRateLimit` (acts only on `/api/chat/history`)

---

## 3. Redis History Caching

### Layout

| Key | Description |
|-----|-------------|
| `chat:group:<groupID>:recent` | List of last 50 messages (newest at head) |
| TTL | 1 hour per group |

### Flow

**On new message**

1. Mongo: `SaveChatMessageAsync(msg)`
2. Redis: `PushMessageToRecentCache(msg)` → LPUSH + LTRIM 50 + EXPIRE

**On history request (initial load, no `before`)**

1. Try `GetRecentMessagesFromCache(ctx, groupID)`
2. On hit: return cached, no Mongo call
3. On miss: fetch from Mongo, `WarmRecentCache`, return

**“Load older” (with `before`)**

- Always goes to Mongo (no cache).
- Cache is used only for the most recent page.

### Files

- `internal/services/chat_cache.go` – cache helpers
- `internal/handlers/chat_ws.go` – `PushMessageToRecentCache` on new message
- `internal/handlers/chat_http.go` – uses `LoadChatMessagesWithCache` instead of `LoadChatMessages`

---

## 4. Scalability Analysis

### Why 429 still happens with WebSockets

- Real-time messaging uses WebSockets; HTTP `GET /api/chat/history` is separate.
- On group switch, reconnect, or “load older”, the frontend calls the history API.
- With the old `useEffect` loop, many requests were sent in a short time.
- WebSockets reduce polling but not misconfigured HTTP usage.

### History endpoint as bottleneck

- Every group switch / “load older” hits Mongo.
- No cache meant all reads went to Mongo.
- Mongo was stressed, and under strict rate limits the client often saw 429.

### Effect of Redis read cache

- Most history requests are initial loads (no `before`).
- These are served from Redis.
- Mongo is only used for:
  - “Load older” pagination
  - Cache misses (first access after a while)
- This can remove most Mongo reads for history.

### Scaling to 10k+ users

1. **Redis cache** – already implemented.
2. **Horizontal scaling** – stateless API; Redis and Mongo are shared.
3. **Mongo indexes** – `idx_group_timestamp` on `(group_id, timestamp)` for history.
4. **Rate limiting** – per IP/identity, not shared across instances when using in-memory limiters. For multiple instances, use a Redis-based limiter for chat history.

### Horizontal scaling across instances

- Current chat limiter is in-memory per process.
- With multiple app instances, limits are per instance, not global.
- For consistent limits across instances, use Redis (similar to the existing Redis rate limiter for other routes).

---

## 5. Final Checklist

- [x] Fixed Next.js history loading logic (no `messages` in deps, ref-based guards)
- [x] Go rate limiter middleware for chat history
- [x] Redis caching for recent history
- [x] Mongo query unchanged and still optimized via `idx_group_timestamp`
- [x] `getAuthHeaders()` added to chat API calls
- [x] Exempted `/api/chat/history` from global rate limit
- [x] Applied chat-specific rate limits (auth vs anonymous)

### Why this removes 429

1. Frontend loop is fixed, so requests are no longer repeated.
2. Chat history has its own, higher limits (30/min auth, 10/min anon).
3. Redis cache reduces Mongo load and slows how often the endpoint is needed.
4. Request deduplication and group-id guards prevent redundant calls.

---

## Production notes

1. Ensure Redis is available; cache gracefully degrades if Redis is down.
2. Monitor `X-RateLimit-Remaining` for debugging.
3. For multi-instance deployments, consider a Redis-based chat history limiter.
4. Watch Mongo read metrics to confirm cache effectiveness.

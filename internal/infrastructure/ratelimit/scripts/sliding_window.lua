local key = KEYS[1]

local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local request_id = ARGV[4]

local min_score = now - window

redis.call("ZREMRANGEBYSCORE", key, "-inf", min_score)

local current = redis.call("ZCARD", key)

if current >= limit then
    return {0, limit - current, min_score + window}
end

redis.call("ZADD", key, now, request_id)

redis.call("PEXPIRE", key, window)

return {1, limit - current - 1, now + window}
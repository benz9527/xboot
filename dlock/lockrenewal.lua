-- MGET key1 key2 ...
-- Fetch locked keys' values then update TTL if all matched
-- to target value.
-- Redis Lua5.1 only support unpack() function,
-- so we can't use table.unpack() here.
local lockedValues = redis.call("MGET", unpack(KEYS))
for i, _ in ipairs(KEYS) do
    if lockedValues[i] ~= ARGV[1] then
        return redis.error_reply("dlock token mismatch, unable to refresh")
    end
end

-- PEXPIRE key milliseconds
-- 1: OK
-- 0: Not exist or set failed.
local function updateLockTTL(ttl)
    for _, k in ipairs(KEYS) do
        redis.call("PEXPIRE", k, ttl)
    end
end

updateLockTTL(ARGV[2])
return redis.status_reply("OK")

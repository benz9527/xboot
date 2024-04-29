-- MGET key1 key2 ...
-- Fetch locked keys' values then update TTL if all matched
-- to target value.
local lockedValues = redis.call("MGET", table.unpack(KEYS))
for i, _ in ipairs(KEYS) do
    if lockedValues[i] ~= ARGV[1] then
        return false
    end
end

-- PEXIRE key milliseconds
-- 1: OK
-- 0: Not exist or set failed.
local function updateLockTTL(ttl)
    for _, k in ipairs(KEYS) do
        redis.call("PEXIRE", k, ttl)
    end
end

updateLockTTL(ARGV[2])
return redis.status_reply("OK")

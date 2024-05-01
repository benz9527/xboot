-- MGET key1 key2 ...
-- Fetch locked keys' values then get min TTL if all matched
-- to target value.
-- Redis Lua5.1 only support unpack() function,
-- so we can't use table.unpack() here.
local lockerValues = redis.call("MGET", unpack(KEYS))
for i, _ in ipairs(lockerValues) do
    if lockerValues[i] ~= ARGV[1] then
        return redis.error_reply("dlock token mismatch, unable to fetch ttl")
    end
end

-- PTTL key
-- -2: Not exist.
-- -1: Exists but no TTL.
local minTTL = 0
for _, k in ipairs(KEYS) do
    local ttl = redis.call("PTTL", k)
    if ttl > 0 and (minTTL == 0 or ttl < minTTL) then
        minTTL = ttl
    end
end
-- ttl lower or equal to 0 probably means the keys
-- no longer exists.
return minTTL

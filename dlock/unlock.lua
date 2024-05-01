-- MGET key1 key2 ...
-- Fetch locked keys' values then unlock if all matched
-- to target value.
-- Redis Lua5.1 only support unpack() function,
-- so we can't use table.unpack() here.
local lockedValues = redis.call("MGET", unpack(KEYS))
for i, _ in ipairs(KEYS) do
    if lockedValues[i] ~= ARGV[1] then
        return redis.error_reply("dlock token mismatch, unable to unlock")
    end
end

-- DEL key1 key2 ...
-- Really delete keys (i.e. unlock).
-- Redis Lua5.1 only support unpack() function,
-- so we can't use table.unpack() here.
redis.call("DEL", unpack(KEYS))
return redis.status_reply("OK")

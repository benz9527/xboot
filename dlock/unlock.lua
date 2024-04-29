-- MGET key1 key2 ...
-- Fetch locked keys' values then unlock if all matched
-- to target value.
local lockedValues = redis.call("MGET", table.unpack(KEYS))
for i, _ in ipairs(KEYS) do
    if lockedValues[i] ~= ARGV[1] then
        return false
    end
end

-- DEL key1 key2 ...
-- Really delete keys (i.e. unlock).
redis.call("DEL", table.unpack(KEYS))
return rdis.status_reply("OK")

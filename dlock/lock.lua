-- References:
-- https://www.redisio.com/en/redis-lua.html
--
-- Try to set keys with values and time to live (in seconds) if they don't exist. They will be used as a lock.
-- lock.lua value tokenLength ttl

-- PEXIRE key milliseconds
-- 1: OK
-- 0: Not exist or set failed.
local function updateLockTTL(ttl)
    for _, k in ipairs(KEYS) do
        redis.call("PEXPIRE", k, ttl)
    end
end

-- GETRANGE key start end
-- If start/end is negative, it means the start position is the end of the string.
local function isReentrant()
    local offset = tonumber(ARGV[2])
    for _, k in ipairs(KEYS) do
        if redis.call("GETRANGE", k, 0, offset - 1) ~= string.sub(ARGV[1], 1, offset) then
            return false
        end
    end
    return true
end


-- Start to lock keys as a distributed lock.
local argvSet = {}
for _, k in ipairs(KEYS) do
    table.insert(argvSet, k)
    table.insert(argvSet, ARGV[1])
end

-- MSETNX key1 value1 key2 value2 ...
-- 1: OK
-- 0: One of the keys exist or set failed.
--
-- MSET key1 value1 key2 value2 ...
-- Always return OK.
--
-- Check the lock if it has been occupied.
-- Lua scripts is atomic and can't be interrupted.
-- So the set key and set expire operation divded
-- into two steps is fine.
-- Redis Lua5.1 only support unpack() function,
-- so we can't use table.unpack() here.
if redis.call("MSETNX", unpack(argvSet)) == 0 then
    return redis.error_reply("dlock occupied")
end
if not isReentrant() then
    return redis.error_reply("dlock reentrant failed")
end
-- Really acquires a lock.
redis.call("MSET", unpack(argvSet))
updateLockTTL(ARGV[3])
return redis.status_reply("OK")

_G.JSON = {
    escape = "\\",
    comma = ",",
    colon = ":",
    null = "null",
    quotes = '"',
    left_brace = '{',
    right_brace = '}',
    left_square_bracket = '[',
    right_square_bracket = ']'
}
JSON._trim = function(target) return target and string.gsub(target, "^%s*(.-)%s*$", "%1") end
-- parse json key or value from stringify json
-- @return string(metadata),string(rest string)
JSON._parse = function(str)
    local chStack, index, lastCh = {}, 1
    while index <= #str do
        local ch = string.sub(str, index, index)
        if JSON.quotes == ch then
            if ch == lastCh then
                table.remove(chStack, #chStack)
                lastCh = #chStack > 0 and chStack[#chStack] or nil
            else
                lastCh = ch
                table.insert(chStack, lastCh)
            end
        elseif JSON.escape == ch then
            str = string.sub(str, 1, index - 1) .. string.sub(str, index + 1)
        end
        if JSON.quotes ~= lastCh then
            if JSON.left_brace == ch then
                table.insert(chStack, JSON.right_brace)
                lastCh = ch
            elseif JSON.left_square_bracket == ch then
                table.insert(chStack, JSON.right_square_bracket)
                lastCh = ch
            elseif JSON.right_brace == ch or JSON.right_square_bracket == ch then
                assert(lastCh == ch, str .. " : " .. index .. " unexpected " .. ch .. "<->" .. lastCh)
                table.remove(chStack, #chStack)
                lastCh = #chStack > 0 and chStack[#chStack] or nil
            elseif JSON.comma == ch or JSON.colon == ch then
                if not lastCh then return string.sub(str, 1, index - 1), string.sub(str, index + 1) end
            end
        end
        index = index + 1
    end
    return string.sub(str, 1, index - 1), string.sub(str, index + 1)
end
-- stringify json to lua table
JSON.toJSON = function(str)
    str = JSON._trim(str)
    -- handle string
    -- return plain string, not stringify json
    if JSON.quotes == string.sub(str, 1, 1) and JSON.quotes == string.sub(str, -1, -1) then
        return string.sub(
            JSON._parse(str), 2, -2)
    end
    if 4 == #str then
        -- handle boolean and nil
        local lower = string.lower(str)
        if "true" == lower then
            return true
        elseif "false" == lower then
            return false
        elseif JSON.null == lower then
            return nil
        end
    end
    -- handle number
    local n = tonumber(str)
    if n then return n end
    -- handle array
    if JSON.left_square_bracket == string.sub(str, 1, 1) and JSON.right_square_bracket == string.sub(str, -1, -1) then
        local rest = string.gsub(str, "[\r\n]+", "")
        rest = string.sub(rest, 2, -2)
        local arr, index, val = {}, 1
        while #rest > 0 do
            val, rest = JSON._parse(rest)
            if val then
                val = JSON.toJSON(val)
                arr[index] = val
                index = index + 1
            end
        end
        return arr
    end
    -- handle table
    if JSON.left_brace == string.sub(str, 1, 1) and JSON.right_brace == string.sub(str, -1, -1) then
        local rest = string.gsub(str, "[\r\n]+", "")
        rest = string.sub(rest, 2, -2)
        local key, val
        local tbl = {}
        while #rest > 0 do
            key, rest = JSON._parse(rest)
            val, rest = JSON._parse(rest)
            if key and #key > 0 and val then
                key = JSON.toJSON(key)
                val = JSON.toJSON(val)
                if key and val then tbl[key] = val end
            end
        end
        return tbl
    end
    -- parse error
    return nil
end
-- https://docs.fluentbit.io/manual/pipeline/filters/lua
-- Maybe you have to update the following function to match your real log format.
-- Log example: 
-- 2023-03-31T15:35:48.917244985+00:00 stdout F {"level":"INFO","timestamp":"2023-03-31T15:35:48.917Z","caller":"logger/console.go:138","message":"ignored proxy module [auth]","service":"auth","request-trace-id":"__internal__","pod-info":{"ip":"10.128.2.66","uid":"368a8103-2c54-4e3b-8b37-627aa06544a9","name":"omc-auth-ss-0","namespace":"xxx","node":"worker3.dev","node-ip":"172.29.13.164","software":{"build-date":"2023-03-28 07:11:01","app-version":"1.0.0-dev","dev-kit-version":"go1.20.2","git-commit":"8bd57509da035bc4e19fab5e2fe9a2d960373de4","mode":"dev/linux/amd64"}}}
-- fluentbit log match and extract regexpr:
-- ^(?<time>[^ ]+) (?<stream>stdout|stderr) (?<logtag>[^ ]*) (?<log>.*)$
function nest_to_json(tag, timestamp, record)
    local str_json = record["log"]
    local json = JSON.toJSON(str_json)
    if not json then
        return 0, timestamp, record
    end
    local tbl = {}
    for k, v in pairs(json) do
        if k and "pod-info" == k and "table" == type(v) then
            for k1, v1 in pairs(v) do
                tbl[k1] = v1
            end
        elseif k and "timestamp" == k or "@timestamp" == k then
            tbl["app@ts"] = v
        else
            tbl[k] = v
        end
    end
    -- crio log flag
    tbl["stream"] = record["stream"]
    return 2, timestamp, tbl
end

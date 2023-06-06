local json = require "json"
local utils = require "utils"

local sensitive_json = {}

function sensitive_json.sensitive(json_data, sensitive_fields)
    local mask_str = string.rep("*", 4)

    local function do_mask(data, field)
        if field == nil or data == nil or type(data) ~= "table" then
            return
        end

        if string.sub(field, 1, 1) == "{" and string.sub(field, -1) == "}" then
            field = string.sub(field, 2, -2)
            local sub_fields = utils.split(field, ",")
            for _, v in ipairs(sub_fields) do
                do_mask(data, v)
            end
            
            return
        end

        -- mask the target field
        if field == "[]" then
            for i = 1, #data do
                data[i] = mask_str
            end
        else
            data[field] = mask_str
        end
    end

    local function recursive_mask(data, field)
        local fields = utils.split(field, ".")
        if fields == nil or data == nil or type(data) ~= "table" then
            return
        end
        -- go to the target field
        for i = 1, #fields - 1 do
            if data == nil then
                return
            end

            local f = fields[i]
            if f == nil or f == "" then
                return
            end

            if f == "[]" then -- loop array
                for _, v in ipairs(data) do
                    recursive_mask(v, table.concat(fields, ".", i + 1))
                end
            end

            if type(data) == "table" then
                data = data[f]
            end
        end

        do_mask(data, fields[#fields])
    end

    for _, field in ipairs(sensitive_fields) do
        recursive_mask(json_data, field)
    end

    return json.encode(json_data)
end

return sensitive_json

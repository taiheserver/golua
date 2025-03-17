


local g_div = div
local g_div_panic = divPanic

local function error_handle(err) 
    print("error: " , err)
    print(debug.traceback())
    return err
end

local function div(a, b)
    if b == 0 then
        error("Division by zero")
    end
    return a / b
end

local function div_err(a, b)
    return a / b
end


local function pcall_fn(fn, a, b)
    print("pcall calling", fn.name, a, b)
    local status, result = pcall(fn.fn, a, b)
    if status then
        print("\tresult: ", result)
    else
        print("\terror: ", result)
    end
    print()
end

local function xpcall_fn(fn, a, b)
    print("xpcall calling", fn.name, a, b)
    local status, result = xpcall(fn.fn, error_handle, a, b)
    if status then
        print("\tresult: ", result)
    else
        print("\terror: ", result)
    end
    print()
end


local fn_table = {
    { name = "div_err", fn = div_err },
    { name = "div", fn = div },
    { name = "g_div_panic", fn = g_div_panic },
    { name = "g_div", fn = g_div },
}


local function do_pcall_fn()
    for _, fn in ipairs(fn_table) do
        pcall_fn(fn, 10, 2)
        pcall_fn(fn, 10, 0)
    end
end

local function do_xpcall_fn()
    for _, fn in ipairs(fn_table) do
        xpcall_fn(fn, 10, 2)
        xpcall_fn(fn, 10, 0)
    end
end


do_pcall_fn()
do_xpcall_fn()
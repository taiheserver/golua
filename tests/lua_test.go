package tests

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/taiheserver/golua/lua"
)

func divPanic(L *lua.State) int {
	a := L.ToInteger(1)
	b := L.ToInteger(2)
	L.PushInteger(int64(a / b))
	return 1
}

const (
	ErrDivByZero = "div by zero"
)

func div(L *lua.State) int {
	a := L.ToInteger(1)
	b := L.ToInteger(2)
	if b == 0 {
		L.PushString(ErrDivByZero)
		return -1
	}
	L.PushInteger(int64(a / b))
	return 1
}

type testingT interface {
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}

func checkTop(L *lua.State, t testingT) {
	if L.GetTop() != 0 {
		t.Fatalf("stack not empty: %d", L.GetTop())
	}
}

func initLuaState() (_ *lua.State, onStop func()) {
	L := lua.NewState()
	L.OpenLibs()
	return L, func() {
		L.DoString("io.flush()") // flush pending output
		L.Close()
	}
}

func TestError(t *testing.T) {
	L, cancel := initLuaState()
	defer cancel()
	L.Register("div", div)
	L.Register("divPanic", divPanic)
	L.SetErrorHandle(func(L *lua.State, reason string) {
		t.Logf("error: %s", reason)
	})

	t.Run("normal", func(t *testing.T) {
		do := func(fn string) {
			err := L.DoString(fmt.Sprintf("return %s(10, 2)", fn))
			if err != nil {
				t.Fatal(err)
			}
			if L.ToInteger(-1) != 5 {
				t.Fatalf("%s(10, 2) != 5", fn)
			}
			L.Pop(1)
			checkTop(L, t)
		}
		do("div")
		do("divPanic")
	})

	t.Run("error", func(t *testing.T) {
		err := L.DoString("return div(10, 0)")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != ErrDivByZero {
			t.Fatalf("unexpected error: %v", err)
		}
		checkTop(L, t)
	})

	t.Run("error_panic", func(t *testing.T) {
		err := L.DoString("return divPanic(10, 0)")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "[GO PANIC] runtime error: integer divide by zero" {
			t.Fatalf("unexpected error: %v", err)
		}
		checkTop(L, t)
	})
}

func runTest(t *testing.T, L *lua.State) {
	if !L.IsTable(-1) {
		t.Fatalf("not a table: %v", L.Typename(-1))
	}
	L.GetField(-1, "do_pcall_fn")
	err := L.Call(0, 0)
	if err != nil {
		t.Fatalf("do_pcall_fn: %v", err)
	}
	L.GetField(-1, "do_xpcall_fn")
	err = L.Call(0, 0)
	if err != nil {
		t.Fatalf("do_xpcall_fn: %v", err)
	}
}

func TestPcall(t *testing.T) {
	L, cancel := initLuaState()
	defer cancel()
	L.Register("div", div)
	L.Register("divPanic", divPanic)

	err := L.DoFile("test.lua")
	if err != nil {
		checkTop(L, t)
		t.Fatal(err)
	}
	runTest(t, L)
	L.Pop(1)
	checkTop(L, t)
}

func TestChunkLoad(t *testing.T) {

	L, cancel := initLuaState()

	run := func() {
		if err := L.Call(0, 1); err != nil {
			t.Fatal(err)
		}
		runTest(t, L)
	}

	L.Register("div", div)
	L.Register("divPanic", divPanic)

	data, err := os.ReadFile("test.lua")
	if err != nil {
		t.Fatal(err)
	}

	var name = []byte{
		'1', '2', '3', 0,
	}

	if ret := L.UnsafeLoad(data, name); ret != 0 {
		msg := L.ToString(-1)
		L.Pop(1)
		checkTop(L, t)
		t.Fatalf("load: %v", msg)
	}
	if L.Dump() != 0 {
		err := L.ToString(-1)
		t.Fatalf("dump: %v", err)
	}
	chunk := L.ToBytes(-1)
	L.Pop(2)
	checkTop(L, t)

	if ret := L.UnsafeLoad(chunk, name); ret != 0 {
		msg := L.ToString(-1)
		L.Pop(1)
		checkTop(L, t)
		t.Fatalf("load: %v", msg)
	}
	run()
	L.Pop(1)
	checkTop(L, t)
	cancel()
}

func BenchmarkLuaLoad(b *testing.B) {

	data, err := os.ReadFile("test.lua")
	if err != nil {
		b.Fatal(err)
	}
	L, cancel := initLuaState()
	_ = cancel

	var name = []byte{
		'1', '2', '3', 0,
	}

	if ret := L.UnsafeLoad(data, name); ret != 0 {
		msg := L.ToString(-1)
		L.Pop(1)
		checkTop(L, b)
		b.Fatalf("load: %v", msg)
	}
	if L.Dump() != 0 {
		err := L.ToString(-1)
		b.Fatalf("dump: %v", err)
	}
	chunk := L.ToBytes(-1)
	L.Pop(2)
	cancel()

	b.Run("FileLoad", func(b *testing.B) {
		L, _ := initLuaState()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ret := L.UnsafeLoad(data, name)
			if ret != 0 {
				msg := L.ToString(-1)
				L.Pop(1)
				b.Fatalf("load: %v", msg)
			}
			L.Pop(1)
		}
	})

	// _ = L.Load(chunk, "123")

	b.Run("ChunkLoad", func(b *testing.B) {
		L, _ := initLuaState()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ret := L.UnsafeLoad(chunk, name)
			if ret != 0 {
				msg := L.ToString(-1)
				L.Pop(1)
				b.Fatalf("load: %v", msg)
			}
			L.Pop(1)
		}
	})
}

type Data struct {
	Name string
	Age  int
}

func TestGoStruct(t *testing.T) {
	L, cancel := initLuaState()
	defer cancel()

	L.SetGCHook(func(v interface{}) {
		t.Logf("GC: %+v type %v", v, reflect.TypeOf(v))
	})

	t.Run("lua_call", func(t *testing.T) {
		L.Register("newData", func(L *lua.State) int {
			if L.GetTop() != 2 {
				L.PushString("invalid arguments")
				return -1
			}
			if !L.IsString(1) {
				L.PushString("invalid name")
				return -1
			}
			if !L.IsNumber(2) {
				L.PushString("invalid age")
				return -1
			}
			data := &Data{
				Name: L.ToString(1),
				Age:  int(L.ToInteger(2)),
			}
			L.PushGoStruct(data)
			return 1
		})

		L.Register("printData", func(L *lua.State) int {
			if !L.IsGoStruct(1) {
				L.PushString("invalid data")
				return -1
			}
			data, ok := L.ToGoStruct(1).(*Data)
			if !ok {
				L.PushString("invalid data")
				return -1
			}
			fmt.Printf("GO: Name: %s, Age: %d\n", data.Name, data.Age)
			return 0
		})

		err := L.DoString(`
			local data = newData("Alice", 20)
			printData(data)
			print("LUA:", data.Name, data.Age)
		`)
		if err != nil {
			checkTop(L, t)
			t.Fatal(err)
		}
		checkTop(L, t)
		L.GC(lua.LUA_GCCOLLECT, 0) // test for gc
	})

	t.Run("index", func(t *testing.T) {
		checkTop(L, t)
		data := &Data{
			Name: "123",
			Age:  456,
		}
		L.PushGoStruct(data)
		L.SetGlobal("data")
		err := L.DoString(`
			print("LUA:", data.Name, data.Age)
			data.Name = "Alice"
		`)
		if err != nil {
			checkTop(L, t)
			t.Fatal(err)
		}
		checkTop(L, t)
		if data.Name != "Alice" {
			t.Fatalf("data.Name != Alice: %s", data.Name)
		}
	})

	t.Run("err_index", func(t *testing.T) {
		checkTop(L, t)
		data := &Data{
			Name: "123",
			Age:  456,
		}
		L.PushGoStruct(data)
		L.SetGlobal("data")
		err := L.DoString(`
			local function error_handle(err) 
				print("error: " , err)
				print(debug.traceback())
				return err
			end
			print("LUA:", data.Name, data.Age)

			xpcall(function()
				data.Name = 123
			end, error_handle)

			xpcall(function()
				data.Age = "Alice"
			end, error_handle)

			xpcall(function()
				data.Name2 = "Alice"
			end, error_handle)

			xpcall(function()
				data.Age2 = 123
			end, error_handle)
		`)
		if err != nil {
			t.Fatal(err)
		}
		checkTop(L, t)
	})
}

func BenchmarkCall(b *testing.B) {

	L, cancel := initLuaState()
	defer cancel()
	L.Register("div", div)
	L.DoString(`
	function div2(a, b)
		return a / b
	end
	`)

	b.Run("call1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			L.GetGlobal("div")
			L.PushNumber(10)
			L.PushNumber(2)
			err := L.Call(2, 1)
			_ = err
			// if err != nil {
			// 	b.Fatal(err)
			// }
			val := L.ToNumber(-1)
			_ = val
			// if val != 5 {
			// 	b.Fatalf("val != 5: %d", val)
			// }
			L.Pop(1)
		}

		checkTop(L, b)
	})

	b.Run("call2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			L.GetGlobal("div2")
			L.PushNumber(10)
			L.PushNumber(2)
			err := L.Call(2, 1)
			_ = err
			// if err != nil {
			// 	b.Fatal(err)
			// }
			val := L.ToNumber(-1)
			_ = val
			// if val != 5 {
			// 	b.Fatalf("val != 5: %d", val)
			// }
			L.Pop(1)
		}

		checkTop(L, b)
	})
}

func TestJitVSGo(t *testing.T) {

	t.Run("jit", func(t *testing.T) {
		L, cancel := initLuaState()
		defer cancel()

		err := L.DoString(`
	-- 检查 JIT 编译器状态
	
	-- 输出 JIT 版本信息
	print("JIT 版本:", jit.version)
	
	-- 检查 JIT 编译器是否启用
	print("JIT 启用状态:", jit.status())
	
	-- 显示 JIT 编译器配置
	print("\nJIT 配置:")
	print(jit.arch)
		`)

		t.Logf("err: %v", err)
		checkTop(L, t)
	})

	t.Run("Bench", func(t *testing.T) {
		L, cancel := initLuaState()
		defer cancel()
		L.Register("div", div)
		err := L.DoString(`
-- 纯 Lua 实现

local dump = require("jit.dump")

dump.start("+T") 

function div_lua(a, b)
    return a / b
end

-- 使用 Go 实现
-- div 已通过 L.Register 注册


-- 输出 JIT 版本信息
print("JIT 版本:", jit.version)

-- 检查 JIT 编译器是否启用
print("JIT 启用状态:", jit.status())

-- 显示 JIT 编译器配置
print("\nJIT 配置:")
print(jit.arch)

-- jit.opt.start("maxmcode=256")  -- 增加 mcode 内存上限到 256MB (默认为 64MB)

-- 在测试前添加这些行
local success, v = pcall(require, "jit.v")
if success then
	-- v.off()  -- 全局禁用 JIT
    v.on()  -- 启用详细输出
else
    print("jit.v 模块无法加载:", v)
end

local count = 1000000
local start = os.clock()
local sum1 = 0
for i = 1, count do
    sum1 = sum1 + div_lua(10, 2)
end
local lua_time = os.clock() - start

start = os.clock()
local sum2 = 0
for i = 1, count do
    sum2 = sum2 + div(10, 2)
end
local go_time = os.clock() - start

print("Pure Lua:", sum1, "Time:", lua_time)
print("Go func:", sum2, "Time:", go_time)
print("Ratio:", lua_time / go_time)
	`)

		t.Logf("err: %v", err)
		checkTop(L, t)
	})
}

func TestGoLib(t *testing.T) {
	L, cancel := initLuaState()
	defer cancel()

	L.RegisterLibrary("gm", map[string]lua.LuaGoFunction{
		"add": func(L *lua.State) int {
			a := L.ToInteger(1)
			b := L.ToInteger(2)
			L.PushInteger(int64(a + b))
			return 1
		},
		"sub": func(L *lua.State) int {
			a := L.ToInteger(1)
			b := L.ToInteger(2)
			L.PushInteger(int64(a - b))
			return 1
		},
	})
	checkTop(L, t)

	err := L.DoString(`

	local gm = require("gm")

	print("add:", gm.add(1, 2))
	print("sub:", gm.sub(1, 2))
	`)
	if err != nil {
		t.Fatal(err)
	}
	checkTop(L, t)
}

func TestFFI(t *testing.T) {
	L, cancel := initLuaState()
	defer cancel()
	checkTop(L, t)

	t.Run("clib", func(t *testing.T) {
		err := L.DoString(`
	local ffi = require("ffi")

	ffi.cdef[[
		int printf(const char *format, ...);
	]]
	ffi.C.printf("Hello, %s!\n", "world")
	`)
		if err != nil {
			t.Fatal(err)
		}
		checkTop(L, t)
	})

	t.Run("golib", func(t *testing.T) {
		err := L.DoString(`
	local ffi = require("ffi")
	ffi.cdef[[
		int Add(int a, int b);
		const char* Hello(const char* name);
		void Free(const char* s);
	]]

	local go = ffi.load("./lib/lib.so")
	
	print("Add:", go.Add(1, 2))
	local s = go.Hello(ffi.cast("const char*", "Alice"))
	print(ffi.string(s))
	go.Free(s)
		`)

		if err != nil {
			t.Fatal(err)
		}

		checkTop(L, t)
	})
}

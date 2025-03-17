package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

//export Add
func Add(a, b C.int) C.int {
	return a + b
}

//export Hello
func Hello(name *C.char) *C.char {
	return C.CString("Hello, " + C.GoString(name) + "!")
}

//export Free
func Free(s *C.char) {
	C.free(unsafe.Pointer(s))
	fmt.Println("call Free")
}

//go:generate go build -buildmode=c-shared -o lib.so lib.go
func main() {}

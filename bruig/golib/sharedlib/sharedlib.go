package main

/*
#include <stdint.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"runtime/cgo"
	"unsafe"

	"github.com/companyzero/bisonrelay/bruig/golib"
)

func errorCString(err error) *C.char {
	if err == nil {
		return C.CString("")
	}
	return C.CString(err.Error())
}

//export GetURL
func GetURL(url *C.char) (*C.char, *C.char) {
	r, e := golib.GetURL(C.GoString(url))
	return C.CString(r), errorCString(e)
}

//export Hello
func Hello() {
	golib.Hello()
}

//export SetTag
func SetTag(newt *C.char) {
	golib.SetTag(C.GoString(newt))
}

//export NextTime
func NextTime() *C.char {
	return C.CString(golib.NextTime())
}

//export WriteStr
func WriteStr(s *C.char) {
	golib.WriteStr(C.GoString(s))
}

//export ReadStr
func ReadStr() *C.char {
	return C.CString(golib.ReadStr())
}

//export AsyncCall
func AsyncCall(typ uint32, id, client uint32, payload unsafe.Pointer, payloadLen C.int) {
	p := C.GoBytes(payload, payloadLen)
	golib.AsyncCall(golib.CmdType(typ), id, client, p)
}

//export NextCallResult
func NextCallResult() (C.uintptr_t, C.ulonglong, C.ulonglong, C.ulonglong) {
	r := golib.NextCmdResult()
	h := cgo.NewHandle(r)
	isErr := C.ulonglong(0)
	if r.Err != nil {
		isErr = 1
		errPayload, err := json.Marshal(r.Err.Error())
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(errPayload))
		r.Payload = errPayload
	}
	return C.uintptr_t(h), C.ulonglong(len(r.Payload)), C.ulonglong(r.Type), isErr
}

//export CopyCallResult
func CopyCallResult(handle C.uintptr_t, p *C.char) C.ulonglong {
	h := cgo.Handle(handle)
	r := h.Value().(*golib.CmdResult)
	rp := r.Payload
	length := len(rp)
	slice := (*[1 << 28]byte)(unsafe.Pointer(p))[:length:length]
	copy(slice, rp)
	id := r.ID
	h.Delete()
	return C.ulonglong(id)
}

func main() {

}

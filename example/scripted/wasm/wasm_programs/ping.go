//go:build
// +build tinygo

// Debug your wasm at: https://webassembly.github.io/wabt/demo/wasm2wat/

package main

import (
	"bytes"
)

//export stage_log_info
func stage_log_info(*byte, int32) int32

func logMsg(buf []byte) {
	stage_log_info(&buf[0], int32(len(buf)))
}

func longBuf(sz int, buf *bytes.Buffer) {
	for i := 0; i < sz; i++ {
		buf.WriteString(" aa ")
		buf.WriteString(" ")
	}
}

func main() {}

//export actor_main
func actor_main() int {
	buf := bytes.Buffer{}
	longBuf(10000, &buf)
	logMsg(buf.Bytes())
	buf2 := bytes.Buffer{}
	longBuf(10000, &buf2)
	logMsg(buf2.Bytes())
	return 0
}

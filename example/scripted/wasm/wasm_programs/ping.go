//go:build
// +build tinygo

// Debug your wasm at: https://webassembly.github.io/wabt/demo/wasm2wat/

package main

import (
	"bytes"
	"strconv"
)

//export stage_log_info
func stage_log_info(*byte, int)

func logMsg(str string) {
	buf := []byte(str)
	stage_log_info(&buf[0], len(buf))
}

func main() {}

//export actor_main
func actor_main() int {
	buf := bytes.Buffer{}
	for i := 0; i < 1000; i++ {
		buf.WriteString(strconv.Itoa(i))
		buf.WriteString(" ")
	}
	logMsg(buf.String())
	return 0
}

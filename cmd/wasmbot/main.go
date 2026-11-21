//go:build wasip1

// Command wasmbot is a local-testing build of the bot as a WebAssembly reactor: the host (tools/wasirun.mjs
// under Node) reads stdin lines and calls handle() for each. It exists only because Windows Smart App Control
// blocks freshly built native .exe files on the dev PC. The submitted image builds cmd/bot instead.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"unsafe"

	"mechbot/mech"
)

var (
	brain  *mech.Brain
	inBuf  []byte
	outBuf []byte
)

//go:wasmexport alloc
func alloc(n int32) int32 {
	if n < 1 {
		n = 1
	}
	if int32(cap(inBuf)) < n {
		inBuf = make([]byte, n)
	}
	inBuf = inBuf[:n]
	return int32(uintptr(unsafe.Pointer(&inBuf[0])))
}

// handle processes the n bytes written at alloc's pointer and returns (ptr<<32 | len) of the reply line,
// 0 for no reply, or -1 when the match has ended.
//
//go:wasmexport handle
func handle(n int32) int64 {
	out, end := process(inBuf[:n])
	if end {
		return -1
	}
	if len(out) == 0 {
		return 0
	}
	outBuf = out
	return int64(uintptr(unsafe.Pointer(&outBuf[0])))<<32 | int64(len(outBuf))
}

func process(raw []byte) ([]byte, bool) {
	var probe struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return nil, false
	}
	switch probe.Type {
	case "hello":
		var h mech.Hello
		if err := json.Unmarshal(raw, &h); err != nil {
			fmt.Fprintln(os.Stderr, "hello:", err)
		}
		dir := h.Map.Dir
		if dir == "" {
			dir = "/arena"
		}
		w, err := mech.LoadWorld(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "map:", err, "- playing on a flat map")
			w = mech.FlatWorld()
		}
		brain = mech.NewBrain(&h, w)
		b, _ := json.Marshal(map[string]string{"type": "ready", "name": "mech-royale-bot"})
		return b, false
	case "obs":
		var o mech.Obs
		if err := json.Unmarshal(raw, &o); err != nil || brain == nil {
			var t struct {
				Tick int `json:"tick"`
			}
			json.Unmarshal(raw, &t)
			b, _ := json.Marshal(mech.Action{Tick: t.Tick})
			return b, false
		}
		b, _ := json.Marshal(brain.Act(&o))
		return b, false
	case "end":
		return nil, true
	}
	return nil, false
}

func main() {}

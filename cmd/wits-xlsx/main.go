//go:build js

// Command wits-xlsx is the phone's .xlsx reader, compiled to WebAssembly on its
// own: it reads a spreadsheet into an importer.Workbook, as JSON, and nothing
// else. The engine (cmd/wits-wasm) is built without the library that reads
// .xlsx files, and the page loads this only when a workbook is imported.
package main

import (
	"bytes"
	"encoding/json"
	"syscall/js"

	"github.com/TheDonDope/wits/pkg/importer"
)

func main() {
	js.Global().Set("witsReadWorkbook", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 {
			return map[string]any{"error": "witsReadWorkbook takes the workbook's bytes"}
		}
		data := make([]byte, args[0].Get("length").Int())
		js.CopyBytesToGo(data, args[0])
		book, err := importer.Load(bytes.NewReader(data))
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		out, err := json.Marshal(book)
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"workbook": string(out)}
	}))
	if ready := js.Global().Get("witsXLSXReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

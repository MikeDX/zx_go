// Command nexts_run runs a ZX Next test-suite .nex headless in the zx_go
// test harness, captures one PNG per visual phase (handshake through the
// NXTS results table) and prints the results table as JSON.
//
// Usage: nexts_run -nex nexts.nex -out DIR [-frames 600]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/conorarmstrong/zx_go/pkg/roms"
	"github.com/conorarmstrong/zx_go/pkg/testharness"
)

const (
	tbl   = 0x8000
	phase = 0x8006
	ack   = 0x8007
)

type result struct {
	Test   int `json:"test"`
	Result int `json:"result"`
	Detail int `json:"detail"`
}

type report struct {
	Magic   string            `json:"magic"`
	Version int               `json:"version"`
	Frames  int               `json:"frames"`
	Phases  map[string]string `json:"phases"`
	Results []result          `json:"results"`
}

func main() {
	nexPath := flag.String("nex", "", ".nex file")
	out := flag.String("out", "out", "output directory")
	maxFrames := flag.Int("frames", 600, "frame budget")
	settle := flag.Int("settle", 2, "frames to render after a phase appears")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	h, err := testharness.New(roms.ModelNext)
	if err != nil {
		fail(err)
	}
	if err := h.LoadNEX(*nexPath); err != nil {
		fail(err)
	}
	mem := h.MemoryBus()
	rep := report{Phases: map[string]string{}}
	f := 0
	for f < *maxFrames {
		h.RunFrames(1)
		f++
		p := mem.Read(phase)
		if p == 0 || mem.Read(ack) == p {
			continue
		}
		h.RunFrames(*settle)
		f += *settle
		name := fmt.Sprintf("phase_%02x.png", p)
		if err := h.SaveScreenshot(filepath.Join(*out, name)); err != nil {
			fail(err)
		}
		rep.Phases[fmt.Sprintf("%02x", p)] = name
		mem.Write(ack, p)
		if p == 0xFF {
			break
		}
	}
	rep.Frames = f
	rep.Magic = string([]byte{mem.Read(tbl), mem.Read(tbl + 1), mem.Read(tbl + 2), mem.Read(tbl + 3)})
	rep.Version = int(mem.Read(tbl + 4))
	n := int(mem.Read(tbl + 5))
	for i := 0; i < n && i < 64; i++ {
		a := uint16(tbl + 0x10 + 2*i)
		rep.Results = append(rep.Results, result{i + 1, int(mem.Read(a)), int(mem.Read(a + 1))})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "nexts_run:", err)
	os.Exit(1)
}

package main

// --nexts-run: run a NXTS test-suite .nex headless on the SAME emulator
// construction the GUI uses (newEmulator -> wireNextSubsystems: tilemap
// layer, ULA palette, layer priorities, NR handlers), capturing one PNG per
// visual phase. The pkg/testharness Next harness wires a reduced compositor
// (no tilemap layer, no ULA palette), so its screenshots are not what the
// GUI shows.

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/conorarmstrong/zx_go/pkg/next/nex"
	"github.com/conorarmstrong/zx_go/pkg/roms"
)

var (
	nextsNex    = flag.String("nexts-run", "", "Run a NXTS test-suite .nex headless with the GUI compositor, write phase PNGs + results.json, exit")
	nextsOut    = flag.String("nexts-out", "nexts-out", "Output directory for --nexts-run")
	nextsFrames = flag.Int("nexts-frames", 600, "Frame budget for --nexts-run")
)

type nextsResult struct {
	Test   int `json:"test"`
	Result int `json:"result"`
	Detail int `json:"detail"`
}

func runNexts() {
	if err := runNextsErr(); err != nil {
		fmt.Fprintln(os.Stderr, "nexts-run:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runNextsErr() error {
	const tbl, phase, ack = 0x8000, 0x8006, 0x8007
	if err := os.MkdirAll(*nextsOut, 0o755); err != nil {
		return err
	}
	emu, err := newEmulator(roms.ModelNext)
	if err != nil {
		return err
	}
	n, err := nex.ParseFile(*nextsNex)
	if err != nil {
		return err
	}
	for bank, data := range n.Banks {
		if page := emu.mem.GetPage(bank); page != nil {
			copy(page, data)
		}
	}
	emu.cpu.SP = n.Header.SP
	emu.cpu.PC = n.Header.PC
	emu.mem.PageMemory(n.Header.EntryBank & 0x07)

	phases := map[string]string{}
	frames := 0
	step := func() {
		runOneFrameHeadless(emu, roms.ModelNext)
		emu.renderFrame()
		frames++
	}
	for frames < *nextsFrames {
		step()
		p := emu.mem.Read(phase)
		if p == 0 || emu.mem.Read(ack) == p {
			continue
		}
		step()
		step()
		name := fmt.Sprintf("phase_%02x.png", p)
		fp, err := os.Create(filepath.Join(*nextsOut, name))
		if err != nil {
			return err
		}
		if err := png.Encode(fp, emu.lastFrame()); err != nil {
			fp.Close()
			return err
		}
		fp.Close()
		phases[fmt.Sprintf("%02x", p)] = name
		emu.mem.Write(ack, p)
		if p == 0xFF {
			break
		}
	}
	rd := emu.mem.Read
	rep := map[string]any{
		"runner":  "zx_go --nexts-run (GUI compositor)",
		"magic":   string([]byte{rd(tbl), rd(tbl + 1), rd(tbl + 2), rd(tbl + 3)}),
		"version": int(rd(tbl + 4)),
		"frames":  frames,
		"phases":  phases,
	}
	var res []nextsResult
	for i := 0; i < int(rd(tbl+5)) && i < 64; i++ {
		a := uint16(tbl + 0x10 + 2*i)
		res = append(res, nextsResult{i + 1, int(rd(a)), int(rd(a + 1))})
	}
	rep["results"] = res
	b, _ := json.MarshalIndent(rep, "", "  ")
	return os.WriteFile(filepath.Join(*nextsOut, "results.json"), append(b, '\n'), 0o644)
}

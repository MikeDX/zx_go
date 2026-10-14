package ula

import (
	"testing"

	"github.com/conorarmstrong/zx_go/pkg/memory"
)

// TestActiveVideoLine verifies the live raster-line counter exposed via
// NextReg $1E/$1F. NextZXOS dot commands (e.g. NextGuide) DISABLE
// interrupts and poll it to wait for the raster, so the crucial property
// is that it advances as the CPU's T-state counter advances within the
// frame — otherwise the wait loop hangs forever (the "frozen Guide" bug).
func TestActiveVideoLine(t *testing.T) {
	var ts uint64
	mem := &memory.Memory{}
	mem.TStates = &ts
	u := &ULA{mem: mem, frameStartTstate: 100}

	// Part-way through line 5.
	ts = 100 + uint64(TStatesPerLine*5+50)
	if got := u.ActiveVideoLine(); got != 5 {
		t.Errorf("ActiveVideoLine = %d, want 5", got)
	}
	// Advances with T-states — the property the DI'd raster-wait needs.
	ts = 100 + uint64(TStatesPerLine*7)
	if got := u.ActiveVideoLine(); got != 7 {
		t.Errorf("ActiveVideoLine = %d, want 7 (must advance)", got)
	}
	// The counter wraps at the FRAME, not at 9 bits. This previously asserted
	// `513 & 0x1FF == 1`, which encoded a bug: 511 is not a raster position,
	// and software polling NextReg $1E/$1F for a scanline compares against a
	// line that has to exist. Line 513 is line 513-311 = 202 of the next
	// frame.
	ts = 100 + uint64(TStatesPerLine*513)
	if got, want := u.ActiveVideoLine(), 513-LinesPerFrame; got != want {
		t.Errorf("ActiveVideoLine = %d, want %d (wraps at the frame)", got, want)
	}
	// No T-state source → 0, not a panic.
	if got := (&ULA{}).ActiveVideoLine(); got != 0 {
		t.Errorf("ActiveVideoLine with nil mem = %d, want 0", got)
	}
}

// TestBeamPosition verifies the per-T-state beam-position model: scanline
// and 8-pixel hpos derived from the T-state counter (the foundation for
// per-T-state Copper / contention / ULA-per-scanline).
func TestBeamPosition(t *testing.T) {
	var ts uint64
	mem := &memory.Memory{}
	mem.TStates = &ts
	u := &ULA{mem: mem, frameStartTstate: 100}

	// Part-way through line 5: T-state offset 40 → hpos 40/4 = 10.
	ts = 100 + uint64(TStatesPerLine*5+40)
	if line, hpos := u.BeamPosition(); line != 5 || hpos != 10 {
		t.Errorf("mid line 5: got line=%d hpos=%d, want 5,10", line, hpos)
	}
	// Top-left of the frame.
	ts = 100
	if line, hpos := u.BeamPosition(); line != 0 || hpos != 0 {
		t.Errorf("frame start: got line=%d hpos=%d, want 0,0", line, hpos)
	}
	// ActiveVideoLine stays consistent with BeamPosition's line.
	ts = 100 + uint64(TStatesPerLine*7+200)
	if u.ActiveVideoLine() != 7 {
		t.Errorf("ActiveVideoLine = %d, want 7", u.ActiveVideoLine())
	}
	// No T-state source → (0,0), not a panic.
	if line, hpos := (&ULA{}).BeamPosition(); line != 0 || hpos != 0 {
		t.Errorf("nil mem: got line=%d hpos=%d, want 0,0", line, hpos)
	}
}

// TestBeamPositionAtTurboSpeed: the raster runs on the video clock. At 7, 14
// or 28 MHz the CPU counter advances SpeedMultiplier T-states per video
// T-state, so the beam position divides it back down. Counting raw CPU
// T-states ran NextReg $1E/$1F eight times too fast at 28 MHz; TX-1696 polls
// it in a tight loop and gave up once port reads were sampled at their real
// point in the I/O cycle.
func TestBeamPositionAtTurboSpeed(t *testing.T) {
	var ts uint64
	mem := &memory.Memory{}
	mem.TStates = &ts
	mem.SpeedMultiplier = func() int { return 8 }
	u := &ULA{mem: mem, frameStartTstate: 100}

	ts = 100 + 8*uint64(TStatesPerLine*5+40)
	if line, hpos := u.BeamPosition(); line != 5 || hpos != 10 {
		t.Errorf("mid line 5 at 28 MHz: got line=%d hpos=%d, want 5,10", line, hpos)
	}
}

// TestActiveVideoLineFollowsTheFrameFraction pins the NextReg $1E/$1F
// formula at every CPU speed:
//
//	line = T-states since frame start * lines per frame / frame length
//
// The frame length grows with the speed, so 100 CPU T-states past line 5
// is still line 5 at 7, 14 and 28 MHz. Dividing the raw CPU counter by
// the 3.5 MHz line length reports a different line as soon as the speed
// is above 1.
func TestActiveVideoLineFollowsTheFrameFraction(t *testing.T) {
	for _, speed := range []int{1, 2, 4, 8} {
		var ts uint64
		mem := &memory.Memory{}
		mem.TStates = &ts
		mem.SpeedMultiplier = func() int { return speed }
		u := &ULA{mem: mem}

		const into = 100 // CPU T-states past the start of line 5
		elapsed := speed*TStatesPerLine*5 + into
		ts = uint64(elapsed)

		lines := LinesPerFrame
		frameLen := TStatesPerLine * lines * speed
		wrapped := elapsed % frameLen
		wantLine := wrapped * lines / frameLen
		wantH := ((wrapped - wantLine*(frameLen/lines)) / speed) / 4

		if line, hpos := u.BeamPosition(); line != wantLine || hpos != wantH {
			t.Errorf("speed %d: line=%d hpos=%d, want %d,%d", speed, line, hpos, wantLine, wantH)
		}
		if got := u.ActiveVideoLine(); got != wantLine {
			t.Errorf("speed %d: ActiveVideoLine = %d, want %d", speed, got, wantLine)
		}
		if speed > 1 && elapsed/TStatesPerLine == wantLine {
			t.Fatalf("speed %d fixture matches a raw 3.5 MHz count, so it cannot catch a missing speed scale", speed)
		}
	}
}

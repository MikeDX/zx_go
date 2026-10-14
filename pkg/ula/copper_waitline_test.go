package ula

import (
	"testing"

	"github.com/conorarmstrong/zx_go/pkg/next/copper"
)

// TestCopperWaitLineFiresEightLinesLater pins the WAIT vertical match.
//
// The render loop still passes the paper row as the copper scanline, so
// line 0 is the first paper line. The WAIT compare is 8 lines later than
// that numbering: WAIT line L first shows on paper row L+8. Rows before
// that stay clear. Line 232 releases at line 240, still below the paper,
// and still runs once.
func TestCopperWaitLineFiresEightLinesLater(t *testing.T) {
	cases := []struct {
		name      string
		line      int
		wantPaper int // paper row the MOVE first shows on; -1 means no paper row
	}{
		{"line 0 shows from paper row 8", 0, 8},
		{"line 16 shows from paper row 24", 16, 24},
		{"line 232 is still below the paper", 232, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, _ := newFloatingBusULA(t)
			var state byte

			c := copper.New()
			c.SetWritePtrLow(0)
			// WAIT for the target line at column 0, one MOVE, then park.
			// Column 0 releases at displayed pixel 0 (hcount threshold 12).
			wait := uint16(0x8000) | uint16(tc.line)
			c.WriteData(byte(wait >> 8))
			c.WriteData(byte(wait))
			c.WriteData(0x40) // MOVE
			c.WriteData(0x01)
			c.WriteData(0xFF) // WAIT 511, the list terminator
			c.WriteData(0xFF)
			c.SetRegWriter(countingRegWriter{n: &state})
			c.SetWritePtrHighAndMode(byte(copper.StartFromZero) << 6)

			u.SetNextCopper(c)
			u.SetNextCompositor(&stateStampCompositor{state: &state})
			u.applyNextCompositor()

			if state != 1 {
				t.Fatalf("MOVE ran %d times, want 1: WAIT line %d must release inside the frame", state, tc.line)
			}
			shows := func(y int) bool {
				row := u.img.Pix[(BorderTop+y)*u.img.Stride+BorderLeft*4:]
				for x := 0; x < ScreenWidth; x++ {
					if row[x*4] != 0 {
						return true
					}
				}
				return false
			}
			for y := 0; y < ScreenHeight; y++ {
				got := shows(y)
				// The MOVE's state stays set, so every paper row from the
				// release onward shows it. Rows above the release must not.
				want := tc.wantPaper >= 0 && y >= tc.wantPaper
				if got != want {
					t.Fatalf("paper row %d shows the MOVE: %v, want %v (WAIT line %d)",
						y, got, want, tc.line)
				}
			}
		})
	}
}

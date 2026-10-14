package ula

import (
	"fmt"
	"testing"
)

// borderTrace records the order the display path composes paper rows and
// tilemap border rows.
type borderTrace struct {
	ev []string
}

func (b *borderTrace) ComposeScanline(y int, _, _ []byte) {
	b.ev = append(b.ev, fmt.Sprintf("paper %d", y))
}

func (b *borderTrace) ComposeScanlineRange(y int, ula, dst []byte, _, _ int) {
	b.ComposeScanline(y, ula, dst)
}

func (b *borderTrace) HasActiveTilemap() bool { return true }

func (b *borderTrace) ComposeBorderRow(y int, _ []byte, _ func(int) bool) {
	b.ev = append(b.ev, fmt.Sprintf("border %d", y))
}

func (b *borderTrace) HasActiveSprites() bool                             { return false }
func (b *borderTrace) ComposeSpriteBorderRow(int, []byte, func(int) bool) {}
func (b *borderTrace) TilemapIs80Col() bool                               { return false }
func (b *borderTrace) ComposeWideTilemapRow(int, []byte)                  {}
func (b *borderTrace) HiResLayer2Active() bool                            { return false }
func (b *borderTrace) Layer2Width() int                                   { return 256 }
func (b *borderTrace) ComposeWideLayer2Row(int, []byte)                   {}

// lineTraceCopper notes the first column of each copper line into the same
// event log the compositor uses, so the test can see which side of a border
// paint the line's copper steps fell on.
type lineTraceCopper struct {
	ev *[]string
}

func (c *lineTraceCopper) Step(line, hcount uint16, _ int) int {
	if hcount == 0 {
		*c.ev = append(*c.ev, fmt.Sprintf("cu %d", line))
	}
	return 1
}

func (c *lineTraceCopper) Wrote() bool { return false }
func (c *lineTraceCopper) Idle() bool  { return false }

func indexOf(ev []string, want string) int {
	for i, e := range ev {
		if e == want {
			return i
		}
	}
	return -1
}

// TestTilemapBorderIsPaintedPerScanline pins that each image row's border
// is painted with the scroll in force on that line, not with one scroll
// sampled after the whole frame's copper has run.
//
// The top border is painted before paper row 0. Each paper row's left and
// right columns are painted after that row's copper steps, at the image
// row (paper + 24). The bottom border is painted as the copper walks the
// lines below the paper, so a MOVE on copper line 192 affects the first
// bottom-border row and not the rows above it.
func TestTilemapBorderIsPaintedPerScanline(t *testing.T) {
	u, _ := newFloatingBusULA(t)
	comp := &borderTrace{}
	u.SetNextCompositor(comp)
	u.SetNextCopper(&lineTraceCopper{ev: &comp.ev})

	u.applyNextCompositor()

	ev := comp.ev
	if indexOf(ev, "border 0") < 0 || indexOf(ev, "border 0") > indexOf(ev, "paper 0") {
		t.Fatalf("top border row 0 must be painted before paper row 0, events start %v", ev[:8])
	}
	// Paper row 16's copper (line 16) runs before its border columns,
	// which live on image row 40.
	cu16 := indexOf(ev, "cu 16")
	border40 := indexOf(ev, "border 40")
	paper17 := indexOf(ev, "paper 17")
	if cu16 < 0 || border40 < 0 || !(cu16 < border40 && border40 < paper17) {
		t.Fatalf("want cu 16, then border 40, then paper 17; got cu16=%d border40=%d paper17=%d",
			cu16, border40, paper17)
	}
	// Copper line 192 is the first bottom-border image row (192+24=216),
	// painted after that line's steps and before the next copper line.
	cu192 := indexOf(ev, "cu 192")
	border216 := indexOf(ev, "border 216")
	cu193 := indexOf(ev, "cu 193")
	if cu192 < 0 || border216 < 0 || cu193 < 0 || !(cu192 < border216 && border216 < cu193) {
		t.Fatalf("want cu 192, then border 216, then cu 193; got %d, %d, %d",
			cu192, border216, cu193)
	}
	cu215 := indexOf(ev, "cu 215")
	border239 := indexOf(ev, "border 239")
	cu216 := indexOf(ev, "cu 216")
	if cu215 < 0 || border239 < 0 || cu216 < 0 || !(cu215 < border239 && border239 < cu216) {
		t.Fatalf("want cu 215, then border 239, then cu 216; got %d, %d, %d",
			cu215, border239, cu216)
	}
}

// idleBorderCopper is a stopped copper: the display path takes the single
// compose per paper row, and the bottom border is painted after that.
type idleBorderCopper struct{}

func (idleBorderCopper) Step(uint16, uint16, int) int { return 0 }
func (idleBorderCopper) Wrote() bool                  { return false }
func (idleBorderCopper) Idle() bool                   { return true }

// TestTilemapBorderWithoutCopperStillUsesImageRows pins the idle-copper
// path. It still paints every image row, top border first and the bottom
// border after the paper, at image coordinates.
func TestTilemapBorderWithoutCopperStillUsesImageRows(t *testing.T) {
	u, _ := newFloatingBusULA(t)
	comp := &borderTrace{}
	u.SetNextCompositor(comp)
	u.SetNextCopper(idleBorderCopper{})

	u.applyNextCompositor()

	ev := comp.ev
	if indexOf(ev, "border 0") > indexOf(ev, "paper 0") {
		t.Fatal("top border painted after the paper")
	}
	paper0 := indexOf(ev, "paper 0")
	border24 := indexOf(ev, "border 24")
	paper1 := indexOf(ev, "paper 1")
	if !(paper0 < border24 && border24 < paper1) {
		t.Fatalf("want paper 0, border 24, paper 1; got %d, %d, %d", paper0, border24, paper1)
	}
	paper191 := indexOf(ev, "paper 191")
	border215 := indexOf(ev, "border 215")
	border216 := indexOf(ev, "border 216")
	border239 := indexOf(ev, "border 239")
	if !(paper191 < border215 && border215 < border216 && border216 < border239) {
		t.Fatalf("bottom border out of order: paper191=%d border215=%d border216=%d border239=%d",
			paper191, border215, border216, border239)
	}
	if indexOf(ev, "cu 0") >= 0 {
		t.Fatal("an idle copper was stepped")
	}
}

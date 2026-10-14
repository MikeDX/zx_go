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
// right columns are painted after that row's copper steps, at frame row
// L+32. The bottom border is painted as the copper walks the lines below
// the paper, so a MOVE on copper line 192 affects the first bottom-border
// row and not the rows above it.
func TestTilemapBorderIsPaintedPerScanline(t *testing.T) {
	u, _ := newFloatingBusULA(t)
	comp := &borderTrace{}
	u.SetNextCompositor(comp)
	u.SetNextCopper(&lineTraceCopper{ev: &comp.ev})

	u.applyNextCompositor()

	ev := comp.ev
	// Image row 0 is frame row 8. It is painted before the paper.
	if indexOf(ev, "border 8") < 0 || indexOf(ev, "border 8") > indexOf(ev, "paper 0") {
		t.Fatalf("top border frame row 8 must be painted before paper row 0, events start %v", ev[:8])
	}
	// Copper line 16 runs before that line's border, which is frame row 48.
	cu16 := indexOf(ev, "cu 16")
	border48 := indexOf(ev, "border 48")
	paper17 := indexOf(ev, "paper 17")
	if cu16 < 0 || border48 < 0 || !(cu16 < border48 && border48 < paper17) {
		t.Fatalf("want cu 16, then border 48, then paper 17; got cu16=%d border48=%d paper17=%d",
			cu16, border48, paper17)
	}
	// Copper line 192 is the first bottom-border frame row (192+32=224),
	// painted after that line's steps and before the next copper line.
	cu192 := indexOf(ev, "cu 192")
	border224 := indexOf(ev, "border 224")
	cu193 := indexOf(ev, "cu 193")
	if cu192 < 0 || border224 < 0 || cu193 < 0 || !(cu192 < border224 && border224 < cu193) {
		t.Fatalf("want cu 192, then border 224, then cu 193; got %d, %d, %d",
			cu192, border224, cu193)
	}
	cu215 := indexOf(ev, "cu 215")
	border247 := indexOf(ev, "border 247")
	cu216 := indexOf(ev, "cu 216")
	if cu215 < 0 || border247 < 0 || cu216 < 0 || !(cu215 < border247 && border247 < cu216) {
		t.Fatalf("want cu 215, then border 247, then cu 216; got %d, %d, %d",
			cu215, border247, cu216)
	}
}

// idleBorderCopper is a stopped copper: the display path takes the single
// compose per paper row, and the bottom border is painted after that.
type idleBorderCopper struct{}

func (idleBorderCopper) Step(uint16, uint16, int) int { return 0 }
func (idleBorderCopper) Wrote() bool                  { return false }
func (idleBorderCopper) Idle() bool                   { return true }

// TestTilemapBorderWithoutCopperStillUsesImageRows pins the idle-copper
// path. It still paints every row, top border first and the bottom border
// after the paper, at frame coordinates (paper line L is frame row L+32).
func TestTilemapBorderWithoutCopperStillUsesImageRows(t *testing.T) {
	u, _ := newFloatingBusULA(t)
	comp := &borderTrace{}
	u.SetNextCompositor(comp)
	u.SetNextCopper(idleBorderCopper{})

	u.applyNextCompositor()

	ev := comp.ev
	if indexOf(ev, "border 8") > indexOf(ev, "paper 0") {
		t.Fatal("top border painted after the paper")
	}
	paper0 := indexOf(ev, "paper 0")
	border32 := indexOf(ev, "border 32")
	paper1 := indexOf(ev, "paper 1")
	if !(paper0 < border32 && border32 < paper1) {
		t.Fatalf("want paper 0, border 32, paper 1; got %d, %d, %d", paper0, border32, paper1)
	}
	paper191 := indexOf(ev, "paper 191")
	border223 := indexOf(ev, "border 223")
	border224 := indexOf(ev, "border 224")
	border247 := indexOf(ev, "border 247")
	if !(paper191 < border223 && border223 < border224 && border224 < border247) {
		t.Fatalf("bottom border out of order: paper191=%d border223=%d border224=%d border247=%d",
			paper191, border223, border224, border247)
	}
	if indexOf(ev, "cu 0") >= 0 {
		t.Fatal("an idle copper was stepped")
	}
}

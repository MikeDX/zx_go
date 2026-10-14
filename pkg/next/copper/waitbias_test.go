package copper

import "testing"

// TestWaitBiasDelaysOnlyTheMatch pins that a non-zero bias moves the WAIT
// equality and nothing else.
//
// Bias 0 is the device: WAIT line L releases on scanline L, at the usual
// horizontal threshold. Bias 8 holds that WAIT through scanline L and
// releases it on L+8, still only once the horizontal threshold is met.
// StartOnVBL still restarts when the scanline argument wraps, because the
// bias is not applied to the line remembered for that compare.
func TestWaitBiasDelaysOnlyTheMatch(t *testing.T) {
	const line = 10
	release := func(bias uint16) *Copper {
		c := New()
		c.SetWritePtrLow(0)
		wait := uint16(0x8000) | line
		c.WriteData(byte(wait >> 8))
		c.WriteData(byte(wait))
		c.WriteData(0x40) // MOVE
		c.WriteData(0x01)
		c.WriteData(0xFF) // WAIT 511 parks the list
		c.WriteData(0xFF)
		c.SetRegWriter(&fakeRegWriter{})
		c.SetWritePtrHighAndMode(byte(StartFromZero) << 6)
		c.SetWaitLineBias(bias)
		return c
	}
	writes := func(c *Copper) int {
		rw := c.regs.(*fakeRegWriter)
		return len(rw.writes)
	}

	plain := release(0)
	plain.Step(line, 12, 8)
	if writes(plain) != 1 {
		t.Fatalf("bias 0: WAIT line %d released into %d MOVEs on its own scanline, want 1", line, writes(plain))
	}

	held := release(8)
	held.Step(line, 12, 8)
	if writes(held) != 0 {
		t.Fatalf("bias 8: WAIT line %d released on scanline %d; it must wait until %d", line, line, line+8)
	}
	// One column short of the horizontal threshold, on the delayed line.
	held.Step(line+8, 11, 4)
	if writes(held) != 0 {
		t.Fatalf("bias 8: WAIT line %d released at hcount 11 on scanline %d; the column gate is unchanged", line, line+8)
	}
	held.Step(line+8, 12, 8)
	if writes(held) != 1 {
		t.Fatalf("bias 8: WAIT line %d released into %d MOVEs on scanline %d, want 1", line, writes(held), line+8)
	}

	// A MOVE with no WAIT still re-runs when the scanline wraps, bias or not.
	c := New()
	c.SetWritePtrLow(0)
	c.WriteData(0x40)
	c.WriteData(0xAA)
	rw := &fakeRegWriter{}
	c.SetRegWriter(rw)
	c.SetWritePtrHighAndMode(byte(StartOnVBL) << 6)
	c.SetWaitLineBias(8)
	c.Step(0, 0, 1) // the enabling clock executes nothing
	c.Step(0, 0, 4)
	if len(rw.writes) != 1 {
		t.Fatalf("frame start: %d MOVEs, want 1", len(rw.writes))
	}
	c.Step(20, 0, 4)
	if len(rw.writes) != 1 {
		t.Fatalf("mid-frame: %d MOVEs, want 1", len(rw.writes))
	}
	c.Step(0, 0, 4)
	if len(rw.writes) != 2 {
		t.Fatalf("after the scanline wrapped to 0: %d MOVEs, want 2 (bias must not move the frame restart)", len(rw.writes))
	}
}

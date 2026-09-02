package gifsicle_test

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"testing"

	gs "github.com/binaryblackhole-studio/gifsicle-go"
)

// testPalette returns a palette with n distinguishable colors.
func testPalette(n int) color.Palette {
	pal := color.Palette{}
	for i := 0; i < n; i++ {
		pal = append(pal, color.RGBA{
			R: uint8(i * 255 / (n - 1)),
			G: uint8((i * 97) % 256),
			B: uint8((i * 57) % 256),
			A: 255,
		})
	}
	return pal
}

// makeAnim encodes an n-frame 64x48 GIF: a static checker background with
// a small square that moves, so frame-to-frame changes stay local and the
// optimizer has something to crop. delays are in centiseconds.
func makeAnim(t *testing.T, n int, delays []int) []byte {
	t.Helper()
	pal := testPalette(8)
	imgs := make([]*image.Paletted, n)
	for i := 0; i < n; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 64, 48), pal)
		for y := 0; y < 48; y++ {
			for x := 0; x < 64; x++ {
				img.SetColorIndex(x, y, uint8((x/8+y/8)%2))
			}
		}
		// 8x8 square sweeping along a diagonal
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				px := (2*i + x) % 56
				py := (i + y) % 40
				img.SetColorIndex(px, py, uint8(4+i%4))
			}
		}
		imgs[i] = img
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     imgs,
		Delay:     delays,
		LoopCount: -1, // no NETSCAPE extension unless a test asks for one
		Config:    image.Config{Width: 64, Height: 48, ColorModel: pal},
	}); err != nil {
		t.Fatalf("encoding test gif: %v", err)
	}
	return buf.Bytes()
}

func mustDecode(t *testing.T, data []byte) *gif.GIF {
	t.Helper()
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding exported gif: %v", err)
	}
	return g
}

func TestOpenAndFrames(t *testing.T) {
	data := makeAnim(t, 4, []int{10, 20, 30, 40})
	g, err := gs.Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := g.Frames(); got != 4 {
		t.Errorf("Frames() = %d, want 4", got)
	}
	for i, want := range []int{10, 20, 30, 40} {
		if got, err := g.Delay(i); err != nil || got != want {
			t.Errorf("Delay(%d) = %d, %v; want %d, nil", i, got, err, want)
		}
	}
	if _, err := g.Delay(4); err == nil {
		t.Error("Delay(4) out of range: want error")
	}
	if _, err := g.Delay(-1); err == nil {
		t.Error("Delay(-1) out of range: want error")
	}
}

func TestOpenInvalid(t *testing.T) {
	if _, err := gs.Open([]byte("this is not a gif")); err == nil {
		t.Error("Open(non-gif): want error")
	}
	if _, err := gs.Open(nil); err == nil {
		t.Error("Open(nil): want error")
	}
}

func TestExportEmpty(t *testing.T) {
	g := gs.New()
	if _, err := g.Export(); err != gs.ErrEmpty {
		t.Errorf("Export of empty gif = %v, want ErrEmpty", err)
	}
	if got := g.Frames(); got != 0 {
		t.Errorf("Frames() = %d, want 0", got)
	}
}

func TestAddFramePreservesDelays(t *testing.T) {
	// A gif whose frames have different delays: 100ms, 20ms, 10ms.
	data := makeAnim(t, 3, []int{10, 2, 1})
	g := gs.New()
	if err := g.AddFrame(data); err != nil {
		t.Fatalf("AddFrame: %v", err)
	}
	if got := g.Frames(); got != 3 {
		t.Fatalf("Frames() = %d, want 3", got)
	}
	for i, want := range []int{10, 2, 1} {
		if got, _ := g.Delay(i); got != want {
			t.Errorf("Delay(%d) = %d, want %d", i, got, want)
		}
	}
	// Delays survive export.
	out, err := g.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	dec := mustDecode(t, out)
	if len(dec.Delay) != 3 || dec.Delay[0] != 10 || dec.Delay[1] != 2 || dec.Delay[2] != 1 {
		t.Errorf("exported delays = %v, want [10 2 1]", dec.Delay)
	}
}

func TestSetDelay(t *testing.T) {
	g, _ := gs.Open(makeAnim(t, 2, []int{10, 10}))
	if err := g.SetDelay(1, 40); err != nil {
		t.Fatalf("SetDelay: %v", err)
	}
	if got, _ := g.Delay(1); got != 40 {
		t.Errorf("Delay(1) = %d, want 40", got)
	}
	if err := g.SetDelay(0, -1); err == nil {
		t.Error("SetDelay(-1): want error")
	}
	if err := g.SetDelay(0, 65536); err == nil {
		t.Error("SetDelay(65536): want error")
	}
	if err := g.SetDelay(2, 1); err == nil {
		t.Error("SetDelay(2) out of range: want error")
	}
}

func TestLoopCount(t *testing.T) {
	data := makeAnim(t, 2, []int{10, 10})
	g, _ := gs.Open(data)
	if got := g.LoopCount(); got != -1 {
		t.Errorf("LoopCount() = %d, want -1 (no NETSCAPE extension)", got)
	}

	g.SetLoopCount(5)
	if got := g.LoopCount(); got != 5 {
		t.Errorf("LoopCount() = %d, want 5", got)
	}
	out, err := g.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if got := mustDecode(t, out).LoopCount; got != 5 {
		t.Errorf("exported loop count = %d, want 5", got)
	}

	// Export with Loop() forces loop-forever even if the input had none.
	out, err = g.Export(gs.Loop())
	if err != nil {
		t.Fatalf("Export(Loop): %v", err)
	}
	if got := mustDecode(t, out).LoopCount; got != 0 {
		t.Errorf("exported loop count = %d, want 0 (forever)", got)
	}
}

func TestGetFrameAndFrames(t *testing.T) {
	g, _ := gs.Open(makeAnim(t, 5, []int{1, 2, 3, 4, 5}))

	mid, err := g.GetFrame(2)
	if err != nil {
		t.Fatalf("GetFrame(2): %v", err)
	}
	if mid.Frames() != 1 {
		t.Fatalf("GetFrame: Frames() = %d, want 1", mid.Frames())
	}
	if got, _ := mid.Delay(0); got != 3 {
		t.Errorf("GetFrame(2) delay = %d, want 3", got)
	}

	part, err := g.GetFrames(1, 3)
	if err != nil {
		t.Fatalf("GetFrames(1,3): %v", err)
	}
	if part.Frames() != 3 {
		t.Fatalf("GetFrames: Frames() = %d, want 3", part.Frames())
	}
	for i, want := range []int{2, 3, 4} {
		if got, _ := part.Delay(i); got != want {
			t.Errorf("GetFrames(1,3) delay(%d) = %d, want %d", i, got, want)
		}
	}

	// The returned Gif is an independent copy.
	part.SetDelay(0, 99)
	if got, _ := g.Delay(1); got != 2 {
		t.Errorf("modifying a copy changed the original: Delay(1) = %d, want 2", got)
	}

	for _, tc := range []struct{ i, j int }{{-1, 0}, {0, 5}, {3, 1}, {5, 5}} {
		if _, err := g.GetFrames(tc.i, tc.j); err == nil {
			t.Errorf("GetFrames(%d,%d): want error", tc.i, tc.j)
		}
	}
	if _, err := g.GetFrame(-1); err == nil {
		t.Error("GetFrame(-1): want error")
	}
	if _, err := g.GetFrame(5); err == nil {
		t.Error("GetFrame(5): want error")
	}
}

func TestAddGif(t *testing.T) {
	a, _ := gs.Open(makeAnim(t, 2, []int{10, 10}))
	b, _ := gs.Open(makeAnim(t, 3, []int{5, 6, 7}))
	if err := a.AddGif(b); err != nil {
		t.Fatalf("AddGif: %v", err)
	}
	if got := a.Frames(); got != 5 {
		t.Fatalf("Frames() = %d, want 5", got)
	}
	for i, want := range []int{10, 10, 5, 6, 7} {
		if got, _ := a.Delay(i); got != want {
			t.Errorf("Delay(%d) = %d, want %d", i, got, want)
		}
	}

	// Exporting the combined gif and adding it back must not lose frames.
	out, err := a.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	c, _ := gs.Open(out)
	if c.Frames() != 5 {
		t.Errorf("Frames() after re-open = %d, want 5", c.Frames())
	}
}

func TestExportDoesNotModifyReceiver(t *testing.T) {
	g, _ := gs.Open(makeAnim(t, 3, []int{10, 20, 30}))
	before, err := g.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if _, err := g.Export(gs.Optimize(gs.O3), gs.LossyAt(50), gs.Interlaced()); err != nil {
		t.Fatalf("Export(optimize): %v", err)
	}
	after, err := g.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("Export modified the receiver: exports before/after differ")
	}
	if g.Frames() != 3 {
		t.Errorf("Frames() = %d, want 3", g.Frames())
	}
}

func TestOptimizeReducesSize(t *testing.T) {
	// Many similar frames: optimization should shrink the file.
	data := makeAnim(t, 12, make([]int, 12))
	g, _ := gs.Open(data)
	plain, err := g.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	for _, lvl := range []gs.OptimizationLevel{gs.O1, gs.O2, gs.O3} {
		opt, err := g.Export(gs.Optimize(lvl))
		if err != nil {
			t.Fatalf("Export(Optimize(%d)): %v", lvl, err)
		}
		if len(opt) >= len(plain) {
			t.Errorf("Optimize(%d) size = %d, want < %d", lvl, len(opt), len(plain))
		}
		mustDecode(t, opt) // still a valid gif
	}
}

func TestExportFromOpenIsIdempotent(t *testing.T) {
	data, err := os.ReadFile("testdata/logo.gif")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	g, err := gs.Open(data)
	if err != nil {
		t.Fatalf("Open(logo.gif): %v", err)
	}
	if got := g.Frames(); got != 12 {
		t.Errorf("Frames() = %d, want 12", got)
	}
	out, err := g.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	again, _ := gs.Open(out)
	// Re-exporting an already-merged gif must be stable.
	out2, err := again.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Error("re-exporting an exported gif produced different bytes")
	}
	mustDecode(t, out)
}

func TestCarefulExportIsValid(t *testing.T) {
	data := makeAnim(t, 4, []int{10, 10, 10, 10})
	g, _ := gs.Open(data)
	out, err := g.Export(gs.BeCareful(), gs.Optimize(gs.O2))
	if err != nil {
		t.Fatalf("Export(careful): %v", err)
	}
	mustDecode(t, out)
}

func TestConserveMemoryExportIsValid(t *testing.T) {
	data := makeAnim(t, 4, []int{10, 10, 10, 10})
	g, _ := gs.Open(data)
	out, err := g.Export(gs.ConserveMemory())
	if err != nil {
		t.Fatalf("Export(conserve): %v", err)
	}
	dec := mustDecode(t, out)
	if len(dec.Image) != 4 {
		t.Errorf("frames = %d, want 4", len(dec.Image))
	}
}
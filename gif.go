package gifsicle

/*
#cgo CFLAGS: -I${SRCDIR} -I${SRCDIR}/build/gifsicle/src -I${SRCDIR}/build/gifsicle/include -DHAVE_CONFIG_H=1
#cgo LDFLAGS: -L${SRCDIR}/build -Wl,-rpath,${SRCDIR}/build -lgifsicle-go -lm

#if !__has_include(<gifsicle.h>)
#error "the upstream gifsicle sources are not prepared; run: make prepare lib (see README)"
#endif

#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

// Errors returned by the package.
var (
	// ErrEmpty is returned when exporting a Gif that has no frames.
	ErrEmpty = errors.New("gifsicle: gif has no frames")
	// ErrClosed is returned when a Gif was already released.
	ErrClosed = errors.New("gifsicle: gif is closed")
)

type gifImpl struct {
	stream *C.Gif_Stream
}

func newGifImpl(stream *C.Gif_Stream) *gifImpl {
	g := &gifImpl{stream: stream}
	runtime.SetFinalizer(g, func(g *gifImpl) {
		if g.stream != nil {
			C.bridge_delete_stream(g.stream)
			g.stream = nil
		}
	})
	return g
}

// New returns an empty Gif, ready for AddFrame.
func New() Gif {
	return newGifImpl(C.bridge_new_stream())
}

// readStream parses an encoded GIF into a raw C stream. The caller owns
// the result and must release it with C.bridge_delete_stream.
func readStream(data []byte) (*C.Gif_Stream, error) {
	if len(data) == 0 {
		return nil, errors.New("gifsicle: empty gif data")
	}
	if len(data) > 1<<32-1 {
		return nil, errors.New("gifsicle: gif too large")
	}
	var errs C.bridge_read_errors
	stream := C.bridge_read(
		(*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(data))),
		C.uint32_t(len(data)), &errs)
	if stream == nil {
		msg := C.GoString(&errs.first_error[0])
		if msg == "" {
			msg = "not a GIF file"
		}
		return nil, fmt.Errorf("gifsicle: %s", msg)
	}
	if errs.errors > 0 {
		C.bridge_delete_stream(stream)
		return nil, fmt.Errorf("gifsicle: %d error(s) while reading: %s",
			int(errs.errors), C.GoString(&errs.first_error[0]))
	}
	return stream, nil
}

// Open parses an encoded GIF from data.
func Open(data []byte) (Gif, error) {
	stream, err := readStream(data)
	if err != nil {
		return nil, err
	}
	return newGifImpl(stream), nil
}

func (g *gifImpl) check() error {
	if g == nil || g.stream == nil {
		return ErrClosed
	}
	return nil
}

func (g *gifImpl) AddFrame(frame []byte) error {
	if err := g.check(); err != nil {
		return err
	}
	src, err := readStream(frame)
	if err != nil {
		return err
	}
	defer C.bridge_delete_stream(src)
	if C.bridge_add_frames(g.stream, src, 0,
		C.int(int(C.bridge_nimages(src))-1)) < 0 {
		return errors.New("gifsicle: failed to add frame")
	}
	return nil
}

func (g *gifImpl) GetFrame(i int) (Gif, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	if i < 0 || i >= g.Frames() {
		return nil, fmt.Errorf("gifsicle: frame %d out of range", i)
	}
	return g.GetFrames(i, i)
}

func (g *gifImpl) GetFrames(i, j int) (Gif, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	if i < 0 || j < i || j >= g.Frames() {
		return nil, fmt.Errorf("gifsicle: frame range [%d,%d] out of range", i, j)
	}
	stream := C.bridge_copy_frames(g.stream, C.int(i), C.int(j))
	if stream == nil {
		return nil, errors.New("gifsicle: failed to copy frames")
	}
	return newGifImpl(stream), nil
}

func (g *gifImpl) AddGif(gif Gif) error {
	if err := g.check(); err != nil {
		return err
	}
	other, ok := gif.(*gifImpl)
	if !ok {
		return fmt.Errorf("gifsicle: unsupported Gif implementation %T", gif)
	}
	if err := other.check(); err != nil {
		return err
	}
	if C.bridge_add_frames(g.stream, other.stream, 0,
		C.int(int(C.bridge_nimages(other.stream))-1)) < 0 {
		return errors.New("gifsicle: failed to add gif")
	}
	return nil
}

func (g *gifImpl) Frames() int {
	if g == nil || g.stream == nil {
		return 0
	}
	return int(C.bridge_nimages(g.stream))
}

func (g *gifImpl) Delay(i int) (int, error) {
	if err := g.check(); err != nil {
		return 0, err
	}
	if i < 0 || i >= g.Frames() {
		return 0, fmt.Errorf("gifsicle: frame %d out of range", i)
	}
	return int(C.bridge_frame_delay(g.stream, C.int(i))), nil
}

func (g *gifImpl) SetDelay(i int, delay int) error {
	if err := g.check(); err != nil {
		return err
	}
	if i < 0 || i >= g.Frames() {
		return fmt.Errorf("gifsicle: frame %d out of range", i)
	}
	if delay < 0 || delay > 65535 {
		return fmt.Errorf("gifsicle: delay %d out of range", delay)
	}
	C.bridge_set_frame_delay(g.stream, C.int(i), C.int(delay))
	return nil
}

func (g *gifImpl) LoopCount() int {
	if g == nil || g.stream == nil {
		return -1
	}
	return int(C.bridge_loopcount(g.stream))
}

func (g *gifImpl) SetLoopCount(n int) {
	if g == nil || g.stream == nil {
		return
	}
	C.bridge_set_loopcount(g.stream, C.long(n))
}

// ExportOptions mirrors gifsicle's write pipeline:
// merge (done incrementally by AddFrame/AddGif) -> interlace ->
// optimize -> compress-immediately -> write.
func (g *gifImpl) Export(opts ...ExportOption) ([]byte, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	ep := ExportParams{loopCount: -1}
	for _, o := range opts {
		o(&ep)
	}

	// Export works on an independent stream so the receiver stays
	// unmodified and repeated exports are deterministic. Frames are
	// merged through gifsicle's merge machinery (same as the CLI), which
	// also unifies local palettes into one global colormap.
	stream := C.bridge_new_stream()
	defer C.bridge_delete_stream(stream)

	if int(C.bridge_nimages(g.stream)) == 0 {
		return nil, ErrEmpty
	}
	if C.bridge_add_frames(stream, g.stream, 0,
		C.int(int(C.bridge_nimages(g.stream))-1)) < 0 {
		return nil, errors.New("gifsicle: failed to merge gif for export")
	}

	var flags C.int
	if ep.careful {
		flags = C.GIF_WRITE_CAREFUL_MIN_CODE_SIZE | C.GIF_WRITE_EAGER_CLEAR
	}
	C.bridge_set_write_info(flags, C.int(ep.lossyLevel))

	if ep.interlaced {
		C.bridge_set_interlace(stream)
	}
	if ep.optimizeLvl != 0 {
		C.bridge_optimize(stream, C.int(ep.optimizeLvl), cInt(hugeStream(stream)))
	}
	if ep.conserveMemory && ep.optimizeLvl == 0 {
		C.bridge_compress_frames(stream)
	}
	if ep.loop {
		C.bridge_set_loopcount(stream, C.long(ep.loopCount))
	}

	var out *C.uint8_t
	var outLen C.uint32_t
	if C.bridge_write(stream, flags, C.int(ep.lossyLevel), &out, &outLen) == 0 {
		return nil, errors.New("gifsicle: failed to write gif")
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoBytes(unsafe.Pointer(out), C.int(outLen)), nil
}

// hugeStream replicates the CLI's huge_stream heuristic
// (support.c merge_frame_interval: more than ~200MB of raw frame data).
func hugeStream(stream *C.Gif_Stream) bool {
	var total uint64
	n := int(C.bridge_nimages(stream))
	for i := 0; i < n; i++ {
		w := int(C.bridge_frame_width(stream, C.int(i)))
		h := int(C.bridge_frame_height(stream, C.int(i)))
		total += uint64(w*h)/1024 + 1
	}
	return total > 200*1024
}

func cInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
// Package gifsicle is a Go binding for gifsicle's GIF library. It can
// read, assemble, optimize and write GIFs with the same behavior as the
// gifsicle command line tool, without spawning an external process.
//
// The module path is github.com/binaryblackhole-studio/gifsicle-go; the package
// drops the -go suffix, following the usual Go convention (git2go, go-sqlite3).
package gifsicle

type OptimizationLevel int

const (
	O1 OptimizationLevel = iota + 1
	O2
	O3
)

type Gif interface {
	// AddFrame appends every frame of an encoded GIF held in frame to
	// the end of this gif; if this is an empty gif, then they will be
	// its first frames. Per-frame properties (delay, disposal, position,
	// transparency) are preserved, so a frame may keep its own 20ms
	// delay while others stay at 100ms. The frame colors are remapped
	// into this gif's palette the same way gifsicle merges streams.
	AddFrame(frame []byte) error
	// GetFrame returns frame at position i as a new Gif, can fail if
	// frame does not exist. The returned Gif is an independent copy.
	GetFrame(i int) (Gif, error)
	// GetFrames returns a new Gif with the frames from this Gif from i
	// to j, inclusive. The returned Gif is an independent copy.
	GetFrames(i, j int) (Gif, error)
	// AddGif is a short cut to iterate over all frames from `gif` and
	// add them with the parameters from that frame.
	AddGif(gif Gif) error
	// Frames returns how many frames this gif has
	Frames() int
	// Delay returns the delay of frame i in centiseconds (1/100 s).
	Delay(i int) (int, error)
	// SetDelay sets the delay of frame i in centiseconds.
	SetDelay(i int, delay int) error
	// LoopCount returns the GIF's NETSCAPE loop count: -1 means the GIF
	// does not loop, 0 means it loops forever, n > 0 means n loops.
	LoopCount() int
	// SetLoopCount sets the GIF's NETSCAPE loop count (see LoopCount).
	SetLoopCount(n int)
	// Export returns a []byte with all the frames combined as a gif
	// file that can be written to disk or used as is. Options mirror
	// gifsicle's flags; the receiver is left unmodified.
	Export(...ExportOption) ([]byte, error)
}

type ExportParams struct {
	careful        bool
	conserveMemory bool
	interlaced     bool
	loop           bool
	loopCount      int
	lossy          bool
	lossyLevel     int
	optimizeLvl    OptimizationLevel
}

type ExportOption func(*ExportParams)

// BeCareful mirrors gifsicle's --careful: write valid GIFs even when
// frames use more than 255 colors or have unusual color tables.
func BeCareful() ExportOption {
	return func(ep *ExportParams) {
		ep.careful = true
	}
}

// ConserveMemory mirrors gifsicle's --conserve-memory: compress frames
// as they are processed to reduce peak memory use. It is ignored when
// combined with Optimize, mirroring gifsicle.
func ConserveMemory() ExportOption {
	return func(ep *ExportParams) {
		ep.conserveMemory = true
	}
}

// Interlaced mirrors gifsicle's --interlace: every frame is stored
// interlaced.
func Interlaced() ExportOption {
	return func(ep *ExportParams) {
		ep.interlaced = true
	}
}

// Loop mirrors gifsicle's --loopcount: the exported GIF loops forever.
func Loop() ExportOption {
	return func(ep *ExportParams) {
		ep.loop = true
		ep.loopCount = 0
	}
}

// LoopCount makes the exported GIF loop n times (0 means forever). It
// mirrors gifsicle's --loopcount=N.
func LoopCount(n int) ExportOption {
	return func(ep *ExportParams) {
		ep.loop = true
		ep.loopCount = n
	}
}

// Lossy mirrors gifsicle's --lossy with its default level (20).
func Lossy() ExportOption {
	return func(ep *ExportParams) {
		ep.lossy = true
		ep.lossyLevel = 20
	}
}

// LossyAt mirrors gifsicle's --lossy=N, trading compression ratio for
// visual fidelity at level N (2..200 is the useful range).
func LossyAt(n int) ExportOption {
	return func(ep *ExportParams) {
		ep.lossy = true
		ep.lossyLevel = n
	}
}

// Optimize mirrors gifsicle's --optimize, at O1 (smallest changed
// rectangle per frame), O2 (also transparency) or O3 (also larger
// transparent runs).
func Optimize(o OptimizationLevel) ExportOption {
	return func(ep *ExportParams) {
		ep.optimizeLvl = o
	}
}
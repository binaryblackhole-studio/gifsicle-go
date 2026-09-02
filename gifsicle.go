package gifsicle

type OptimizationLevel int

const (
	O1 OptimizationLevel = iota + 1
	O2
	O3
)

type ExportParams struct {
	careful        bool
	conserveMemory bool
	interlaced     bool
	loop           bool
	lossy          bool
	optimizeLvl    OptimizationLevel
}

type ExportOption func(*ExportParams)

func BeCareful() ExportOption {
	return func(ep *ExportParams) {
		ep.careful = true
	}
}

func ConserveMemory() ExportOption {
	return func(ep *ExportParams) {
		ep.conserveMemory = true
	}
}

func Interlaced() ExportOption {
	return func(ep *ExportParams) {
		ep.interlaced = true
	}
}

func Loop() ExportOption {
	return func(ep *ExportParams) {
		ep.loop = true
	}
}

func Lossy() ExportOption {
	return func(ep *ExportParams) {
		ep.lossy = true
	}
}

func Optimize(o OptimizationLevel) ExportOption {
	return func(ep *ExportParams) {
		ep.optimizeLvl = o
	}
}

type Gif interface {
	// AddFrame adds a new frame to the end of the gif, if this is an empty gif, then it will be the first frame
	AddFrame(delay int, loop bool, frame []byte) error
	// GetFrame returns frame at position i as a new Gif, can fail if frame does not exist
	GetFrame(i int) (Gif, error)
	// GetFrames returns a new Gif with the frames from this Gif from i to j, inclusive.
	GetFrames(i, j int) (Gif, error)
	// AddGif is a short cut to iterate over all frames from `gif` and use AddFrame with the parameters from that frame
	AddGif(gif Gif) error
	// Frames returns how many frames this frame has
	Frames() int
	// Export returns a []byte with all the frames combined as a gif file that can be written to disk or used as is
	Export(...ExportOption) ([]byte, error)
}

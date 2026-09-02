# gifsicle-go

**This is not a fork of gifsicle.** It is a Go binding for the
[Gifsicle](https://www.lcdf.org/gifsicle/) GIF library and optimizer: it lets
you assemble, manipulate, and optimize animated GIFs from inside Go with the
same behavior as the `gifsicle` command line tool, without spawning an
external process. This version is mostly focused on GIF optimization; future
releases might include more manipulation options.

The gifsicle C sources are **not** vendored here: the repository contains
only this binding's own glue code, and a `Makefile` fetches upstream
gifsicle at a pinned release tag and compiles it (unmodified) into a small
shared library that cgo links against; upstream code is never copied into
this repository.

## Install

    go get github.com/binaryblackhole-studio/gifsicle-go

Because the upstream sources must be fetched and compiled once, prepare the
shared library before the first `go build` (Go and git, plus a C compiler,
are required):

    make prepare lib    # clone upstream at the pinned release tag and build build/libgifsicle-go.so

or, without cloning again, point the build at an existing local checkout:

    GIFSICLE_SRC=/path/to/gifsicle make prepare lib

Afterwards `go build`, `go test`, and `go install` work as usual (the built
library is found via its embedded rpath). `make clean` removes `build/`.

The pinned upstream release (`GIFSICLE_VERSION`, tagged `vX.Y` upstream)
lives in the `Makefile`; bumping it and re-running `make clean prepare lib`
is how the binding tracks new gifsicle releases — releases only, never
master.

## Usage

### Build and optimize a GIF from frames

`AddFrame` accepts encoded GIF bytes; every frame of the input is appended
and its colors are remapped into the destination palette, exactly like
gifsicle merges streams. Per-frame properties (delay, disposal, position,
transparency) are preserved, so a frame can keep its own 20 ms delay while
others stay at 100 ms.

```go
ng := gifsicle.New()
for _, frame := range encodedFrames {
    if err := ng.AddFrame(frame); err != nil {
        return err
    }
}
out, err := ng.Export(gifsicle.Optimize(gifsicle.O2))
if err != nil {
    return err
}
os.WriteFile("out.gif", out, 0644)
```

### Open an existing GIF

```go
g, err := gifsicle.Open(data)
if err != nil { ... }

n := g.Frames()             // number of frames
d, _ := g.Delay(3)          // frame 3 delay in centiseconds
g.SetDelay(3, 20)           // make frame 3 show for 200 ms
g.SetLoopCount(0)           // loop forever (n>0 = n loops, -1 = don't loop)

first, _ := g.GetFrame(0)   // independent 1-frame Gif
rest, _ := g.GetFrames(2, g.Frames()-1)
_ = g.AddGif(rest)          // append all frames of another Gif
```

### Export options

`Export` mirrors the corresponding gifsicle flags and leaves the receiver
unmodified, so it can be called repeatedly with different options:

| Option | gifsicle flag | Meaning |
| --- | --- | --- |
| `Optimize(O1/O2/O3)` | `-O1`, `-O2`, `-O3` | crop frames to their changed rectangle (O1), also use transparency (O2), also transparent runs (O3) |
| `Lossy()` / `LossyAt(n)` | `--lossy[=N]` | trade compression ratio for visual fidelity |
| `BeCareful()` | `--careful` | write valid GIFs even for frames with unusual color tables |
| `Interlaced()` | `-i` | store every frame interlaced |
| `Loop()` / `LoopCount(n)` | `-l`, `-l=N` | force the NETSCAPE loop count on export |
| `ConserveMemory()` | `-j` | compress frames as they are processed (ignored when combined with `Optimize`, like gifsicle) |

### Errors

`Export` of a Gif with no frames returns `ErrEmpty`; range checks on
`Delay`/`SetDelay`/`GetFrame(s)` and invalid GIF data passed to `Open` or
`AddFrame` return descriptive errors.

## Verification

The binding aims for byte-identical output with gifsicle 1.96. It is
continuously checked against the real CLI over gifsicle's own test corpus
(single- and multi-stream merges, with `-O1/-O2/-O3`, `-l`, `-i`,
`--careful`, `-j`, `--lossy` and their combinations — more than 1450
combinations). `go test ./...` covers the Go API itself.

## License

The binding links against gifsicle, which is distributed under the GNU
General Public License, version 2 (see [LICENSE](LICENSE)). Using it inside
your program means your program must comply with the GPL — this holds for
dynamic linking just as for vendoring.

## Disclaimer

This repository was written using LLM models while still being supervised by
a human.
# example

A small CLI that optimizes a GIF with gifsicle-go and writes the result to
stdout — the same thing as `gifsicle -O3 in.gif > out.gif`, as a library
call instead of a subprocess.

    go run ./example input.gif > output.gif          # O3 (default)
    go run ./example -O2 input.gif > output.gif
    go run ./example -O0 input.gif > output.gif      # merge only, no optimize
    go run ./example -lossy input.gif > output.gif
    cat input.gif | go run ./example - > output.gif  # stdin

`input.gif` is a bundled sample animation (20 frames, 210x210).

The docker image built by `Dockerfile.example` (in the repository root)
runs this program; see that file's header comment for the exact
`docker build` / `docker run` invocations.
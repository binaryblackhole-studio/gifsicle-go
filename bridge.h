/* bridge.h - C API exposed to the CGo bindings for gifsicle's library.
   The upstream gifsicle sources expect a handful of hooks normally
   provided by the gifsicle CLI (gifsicle.c); bridge.c provides them and
   adds the read-from-memory / write-to-memory / merge / optimize entry
   points the Go layer needs. bridge.c is compiled into the binary by
   cgo; the rest of the library comes from build/libgifsicle-go.so (built
   by the Makefile from unmodified upstream sources). */

#ifndef GIFSICLE_BRIDGE_H
#define GIFSICLE_BRIDGE_H

#include <stdint.h>
#include "gifsicle.h"

#ifdef __cplusplus
extern "C" {
#endif

/* Collected read diagnostics. is_error > 0 counts as an error, == 0 as a
   warning (same convention as Gif_ReadErrorHandler). */
typedef struct bridge_read_errors {
    int errors;
    int warnings;
    char first_error[256];
} bridge_read_errors;

/* Create an empty GIF stream (no frames, no global colormap). */
Gif_Stream* bridge_new_stream(void);

/* Read a GIF from memory. Returns NULL if the data is not a GIF. Image
   data is kept in compressed form (GIF_READ_COMPRESSED); call
   bridge_uncompress_frame before pixel access if needed. */
Gif_Stream* bridge_read(const uint8_t* data, uint32_t length,
                        bridge_read_errors* errs);

/* Write a GIF to memory (open_memstream on POSIX, tmpfile on Windows).
   On success *out is malloc'd and the caller must free it. Returns 1 on
   success, 0 on failure. */
int bridge_write(Gif_Stream* gfs, int compress_flags, int loss,
                 uint8_t** out, uint32_t* outlen);

/* Append src->images[first..last] (inclusive) to dst, remapping colors
   into dst->global the same way gifsicle's merge machinery does. Returns
   the index of the first newly added image, or -1 on failure. */
int bridge_add_frames(Gif_Stream* dst, Gif_Stream* src, int first, int last);

/* Build a new stream containing deep copies of src->images[first..last]
   plus the source stream's global colormap / screen size / loop count.
   Returns NULL on failure. */
Gif_Stream* bridge_copy_frames(Gif_Stream* src, int first, int last);

/* Run gifsicle's optimizer over gfs in place (optimize_fragments).
   level is 1..3 (gifsicle -O1/-O2/-O3); huge mirrors the CLI's
   huge_stream heuristic. */
void bridge_optimize(Gif_Stream* gfs, int level, int huge);

/* Compress every frame now and release its uncompressed data, like the
   CLI's --conserve-memory. Uses the global gif_write_info settings. */
void bridge_compress_frames(Gif_Stream* gfs);

/* Force every frame to be stored uncompressed and re-mark it interlaced.
   Any cached compressed data is released so the writer recompresses. */
void bridge_set_interlace(Gif_Stream* gfs);

/* Simple accessors and mutations. */
int bridge_nimages(Gif_Stream* gfs);
int bridge_frame_delay(Gif_Stream* gfs, int i);
void bridge_set_frame_delay(Gif_Stream* gfs, int i, int delay);
int bridge_frame_width(Gif_Stream* gfs, int i);
int bridge_frame_height(Gif_Stream* gfs, int i);
long bridge_loopcount(Gif_Stream* gfs);
void bridge_set_loopcount(Gif_Stream* gfs, long loopcount);
int bridge_screen_width(Gif_Stream* gfs);
int bridge_screen_height(Gif_Stream* gfs);

/* Settings shared by the optimizer (optimize_fragments reads the global
   gif_write_info, exactly like the CLI) and bridge_compress_frames. */
void bridge_set_write_info(int compress_flags, int loss);

/* The optimizer reads compressed frames fine, but expose explicit
   control for completeness. */
void bridge_uncompress_frame(Gif_Stream* gfs, int i);

void bridge_delete_stream(Gif_Stream* gfs);

#ifdef __cplusplus
}
#endif

#endif
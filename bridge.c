/* bridge.c - CGo bridge for the upstream gifsicle library sources,
   which are fetched at a pinned release tag and compiled into
   build/libgifsicle-go.so by the Makefile (never vendored here).
   Copyright (C) 2026 the gifsicle-go authors.

   gifsicle is free software. It is distributed under the GNU Public
   License, version 2; you can copy, distribute, or alter it at will, as
   long as this notice is kept intact and this source code is made
   available. There is no warranty, express or implied. */

#include <config.h>
#include "bridge.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>
#include <assert.h>
#include <math.h>

/* ---- Hooks the upstream gifsicle sources expect from the CLI ----
   (Normally defined in gifsicle.c.) */

const char* program_name = "gifsicle-go";

Gif_CompressInfo gif_write_info = { 0, 0, { 0, 0, 0, 0, 0, 0, 0 } };
int thread_count = 1;
int warn_local_colormaps = 1;

/* Library mode: gifsicle's diagnostics are normally printed by the CLI.
   Keep the last message in a thread-local buffer instead of writing to
   stderr, unless GIFSICLE_VERBOSE is set in the environment. */
static __thread char last_warning[512];

static int
verbose_warnings(void)
{
    static int checked = -1;
    if (checked == 0)
        checked = getenv("GIFSICLE_VERBOSE") ? 1 : -1;
    return checked > 0;
}

static void
log_warning(const char* format, va_list ap)
{
    if (verbose_warnings()) {
        fprintf(stderr, "gifsicle warning: ");
        vfprintf(stderr, format, ap);
        fprintf(stderr, "\n");
    } else if (last_warning[0] == '\0')
        vsnprintf(last_warning, sizeof(last_warning), format, ap);
}

void
fatal_error(const char* format, ...)
{
    va_list ap;
    va_start(ap, format);
    fprintf(stderr, "gifsicle: fatal: ");
    vfprintf(stderr, format, ap);
    va_end(ap);
    abort();
}

void
warning(int need_file, const char* format, ...)
{
    (void) need_file;
    va_list ap;
    va_start(ap, format);
    log_warning(format, ap);
    va_end(ap);
}

void
lwarning(const char* landmark, const char* format, ...)
{
    va_list ap;
    va_start(ap, format);
    log_warning(format, ap);
    (void) landmark;
    va_end(ap);
}

/* gifsicle's error() also lives in support.c. The library's only call
   sites are the color-transformation subprocess path in xform.c, which
   the binding does not expose; log it like a warning instead of
   exiting the process. */
void
error(int need_file, const char* format, ...)
{
    (void) need_file;
    va_list ap;
    va_start(ap, format);
    log_warning(format, ap);
    va_end(ap);
}

Gif_Colormap*
read_colormap_file(const char* landmark, FILE* f)
{
    /* --use-colormap is not exposed by the Go binding. */
    (void) landmark, (void) f;
    return NULL;
}

/* ---- error collection ------------------------------------------------- */

/* Gif_ReadErrorHandler carries no user pointer, so route diagnostics
   through a thread-local slot (each goroutine runs its CGo call on one
   thread for the duration of the call). */
static __thread bridge_read_errors* current_errors;

static void
bridge_error_handler(Gif_Stream* gfs, Gif_Image* gfi, int is_error,
                     const char* error_text)
{
    bridge_read_errors* errs = current_errors;
    (void) gfs, (void) gfi;
    if (!errs)
        return;
    if (is_error < 0) {
        /* final flush call: reader stores the error count here */
        if (gfs)
            errs->errors += (int) gfs->errors;
        return;
    }
    if (is_error > 0) {
        ++errs->errors;
        if (!errs->first_error[0] && error_text)
            snprintf(errs->first_error, sizeof(errs->first_error), "%s",
                     error_text);
    } else
        ++errs->warnings;
}

Gif_Stream*
bridge_new_stream(void)
{
    return Gif_NewStream();
}

Gif_Stream*
bridge_read(const uint8_t* data, uint32_t length, bridge_read_errors* errs)
{
    Gif_Record record;
    Gif_Stream* gfs;

    memset(errs, 0, sizeof(*errs));
    record.data = data;
    record.length = length;

    current_errors = errs;
    gfs = Gif_FullReadRecord(&record, GIF_READ_COMPRESSED, 0,
                             bridge_error_handler);
    current_errors = NULL;
    return gfs;
}

/* ---- write ------------------------------------------------------------ */

int
bridge_write(Gif_Stream* gfs, int compress_flags, int loss,
             uint8_t** out, uint32_t* outlen)
{
    Gif_CompressInfo gcinfo;
    FILE* f;
    int ok;
#ifndef _WIN32
    char* buf = NULL;
    size_t sz = 0;
    f = open_memstream(&buf, &sz);
    if (!f)
        return 0;
#else
    /* Windows fallback: write to a temp file, then slurp it back. */
    char* buf = NULL;
    size_t sz = 0;
    f = tmpfile();
    if (!f)
        return 0;
#endif

    Gif_InitCompressInfo(&gcinfo);
    gcinfo.flags = compress_flags;
    gcinfo.loss = loss;

    ok = Gif_FullWriteFile(gfs, &gcinfo, f);

    if (ok) {
#ifndef _WIN32
        fclose(f);
        *out = (uint8_t*) buf;
        *outlen = (uint32_t) sz;
#else
        long pos = ftell(f);
        rewind(f);
        buf = (char*) malloc(pos > 0 ? (size_t) pos : 1);
        if (buf)
            sz = fread(buf, 1, (size_t) pos, f);
        fclose(f);
        *out = (uint8_t*) buf;
        *outlen = (uint32_t) sz;
#endif
    } else {
        fclose(f);
#ifndef _WIN32
        free(buf);
#else
        if (buf)
            free(buf);
#endif
    }
    return ok;
}

/* ---- merging ---------------------------------------------------------- */

static void
add_stream_level_info(Gif_Stream* dst, Gif_Stream* src, int whole_source)
{
    /* merge_stream() semantics: adopt the loop count until one is set,
       and carry over trailing comments when the whole source is merged. */
    if (dst->loopcount < 0)
        dst->loopcount = src->loopcount;
    if (whole_source && src->end_comment) {
        if (!dst->end_comment)
            dst->end_comment = Gif_NewComment();
        merge_comments(dst->end_comment, src->end_comment);
    }
}

/* ---- background resolution --------------------------------------------- */

/* gifsicle's set_background() (support.c) decides the output stream's
   background index after merging, over ALL frames at once with the
   final logical screen size. The background a frame's source stream
   would leave visible (fr->transparent, captured by merge_frame_done)
   is therefore attached to every merged image as user_data here and
   interpreted by resolve_background_all() only once the whole merge is
   done -- so incremental AddFrame calls agree with a single merge run. */

static void
set_frame_bgcap(Gif_Image* gfi, Gif_Color cap)
{
    Gif_Color* p = (Gif_Color*) malloc(sizeof(Gif_Color));
    if (!p)
        return;
    *p = cap;
    gfi->user_data = p;
    gfi->free_user_data = Gif_Free;
}

static Gif_Color
get_frame_bgcap(Gif_Image* gfi)
{
    if (gfi && gfi->user_data)
        return *(Gif_Color*) gfi->user_data;
    Gif_Color none;
    memset(&none, 0, sizeof(none));
    return none;
}

/* The source stream's own background contribution (merge_frame_done:
   transparent if its first image is transparent, else the stream's
   background color). */
static Gif_Color
stream_bgcap(Gif_Stream* src)
{
    Gif_Color cap;
    memset(&cap, 0, sizeof(cap));
    if (src->nimages > 0 && src->images[0]->transparent >= 0)
        cap.haspixel = 2;
    else if (src->global && src->background < src->global->ncol) {
        cap = src->global->col[src->background];
        cap.haspixel = 1;
    }
    return cap;
}

static void
resolve_background_all(Gif_Stream* dst)
{
    Gif_Color background;
    memset(&background, 0, sizeof(background));
    int conflict = 0, want_transparent = 0;

    int i;
    for (i = 0; i < dst->nimages; i++) {
        Gif_Image* gfi = dst->images[i];
        if (gfi->disposal == GIF_DISPOSAL_BACKGROUND
            || (i == 0 && (gfi->left != 0 || gfi->top != 0
                           || gfi->width != dst->screen_width
                           || gfi->height != dst->screen_height))) {
            Gif_Color cap = get_frame_bgcap(gfi);
            int original_bg_transparent = (cap.haspixel == 2);
            if ((original_bg_transparent && background.haspixel)
                || (!original_bg_transparent && want_transparent))
                conflict = 2;
            else if (original_bg_transparent)
                want_transparent = 1;
            else if (cap.haspixel) {
                if (background.haspixel
                    && !GIF_COLOREQ(&background, &cap))
                    conflict = 1;
                else {
                    background = cap;
                    background.haspixel = 1;
                }
            }
        }
    }

    if (conflict
        || (want_transparent && dst->nimages > 0
            && dst->images[0]->transparent < 0))
        warning(1, "input images have conflicting background colors\n"
                   "  (This means some animation frames may appear"
                   " incorrect.)");

    /* If no important background color, bag. */
    if (!background.haspixel) {
        dst->background = 0;
        return;
    }
    dst->background = 0;
    if (dst->global) {
        Gif_Color c = background;
        c.haspixel = 1;
        int idx = Gif_FindColor(dst->global, &c);
        if (idx >= 0)
            dst->background = idx;
        else
            lwarning(dst->landmark, "background color not in colormap");
    }
}

int
bridge_add_frames(Gif_Stream* dst, Gif_Stream* src, int first, int last)
{
    Gt_Frame fr;
    int i;
    int whole_source;

    if (!dst || !src || src->nimages == 0)
        return -1;
    if (first < 0)
        first = 0;
    if (last < 0 || last >= src->nimages)
        last = src->nimages - 1;
    if (first > last)
        return -1;

    whole_source = (first == 0 && last == src->nimages - 1);

    /* A fresh destination stream needs an empty global colormap, exactly
       like merge_frame_interval() does. */
    if (!dst->global) {
        Gif_Colormap* global = Gif_NewFullColormap(256, 256);
        if (!global)
            return -1;
        global->ncol = 0;
        dst->global = global;
    }

    Gif_CalculateScreenSize(src, 0);
    add_stream_level_info(dst, src, whole_source);

    /* merge_stream() unmarks the source colormaps so cached mappings from
       previous merges cannot leak into this destination. */
    if (src->global)
        unmark_colors_2(src->global);
    for (i = 0; i < src->nimages; i++)
        if (src->images[i]->local)
            unmark_colors_2(src->images[i]->local);

    memset(&fr, 0, sizeof(fr));

    {
        int first_added = dst->nimages;
        int last_added;
        /* capture before merging: merge_image() may touch
           src->images[0]->transparent temporarily */
        Gif_Color src_bg = stream_bgcap(src);

        /* The CLI marks the used colors of every frame before merging any
           of them, so the destination colormap is complete when
           merge_image() picks transparent slots. Keep that order. */
        for (i = first; i <= last; i++) {
            Gif_Image* srci = src->images[i];
            Gif_UncompressImage(src, srci);
            mark_used_colors(src, srci, NULL, 0);
        }
        for (i = first; i <= last; i++) {
            merge_image(dst, src, src->images[i], &fr, 0);
            /* carry the frame's source-background contribution; frames
               merged from another merge output already carry theirs */
            set_frame_bgcap(dst->images[dst->nimages - 1],
                            get_frame_bgcap(src->images[i]).haspixel
                            ? get_frame_bgcap(src->images[i]) : src_bg);
        }
        last_added = dst->nimages - 1;

        /* logical screen: max of the source streams' screen sizes */
        if (src->screen_width > dst->screen_width)
            dst->screen_width = src->screen_width;
        if (src->screen_height > dst->screen_height)
            dst->screen_height = src->screen_height;

        /* gifsicle's merge tail: resolve the background color, then make
           sure a wholly-transparent stream still gets some colormap */
        resolve_background_all(dst);
        if (dst->global && dst->global->ncol == 0) {
            for (i = 0; i < dst->nimages; i++)
                if (!dst->images[i]->local) {
                    GIF_SETCOLOR(&dst->global->col[0], 0, 0, 0);
                    GIF_SETCOLOR(&dst->global->col[1], 255, 255, 255);
                    dst->global->ncol = 2;
                    break;
                }
        }
    }
    return first;
}

Gif_Stream*
bridge_copy_frames(Gif_Stream* src, int first, int last)
{
    Gif_Stream* dst;
    int i;

    if (!src || src->nimages == 0)
        return NULL;
    if (first < 0)
        first = 0;
    if (last < 0 || last >= src->nimages)
        last = src->nimages - 1;
    if (first > last)
        return NULL;

    dst = Gif_CopyStreamSkeleton(src);
    if (!dst)
        return NULL;
    for (i = first; i <= last; i++) {
        Gif_Image* img = Gif_CopyImage(src->images[i]);
        if (!img || !Gif_AddImage(dst, img)) {
            Gif_DeleteStream(dst);
            return NULL;
        }
    }
    return dst;
}

/* ---- optimize / conserve / interlace ----------------------------------- */

void
bridge_optimize(Gif_Stream* gfs, int level, int huge)
{
    if (!gfs || gfs->nimages == 0)
        return;
    optimize_fragments(gfs, level & GT_OPT_MASK, huge);
}

void
bridge_compress_frames(Gif_Stream* gfs)
{
    int i;
    if (!gfs)
        return;
    for (i = 0; i < gfs->nimages; i++) {
        Gif_Image* gfi = gfs->images[i];
        if (gfi->img) {
            Gif_FullCompressImage(gfs, gfi, &gif_write_info);
            Gif_ReleaseUncompressedImage(gfi);
        }
    }
}

void
bridge_set_interlace(Gif_Stream* gfs)
{
    int i;
    if (!gfs)
        return;
    for (i = 0; i < gfs->nimages; i++) {
        Gif_Image* gfi = gfs->images[i];
        Gif_UncompressImage(gfs, gfi);
        gfi->interlace = 1;
        /* make sure the writer recompresses instead of reusing the old
           (non-interlaced) compressed data */
        Gif_ReleaseCompressedImage(gfi);
    }
}

/* ---- accessors --------------------------------------------------------- */

int
bridge_nimages(Gif_Stream* gfs)
{
    return gfs ? gfs->nimages : 0;
}

int
bridge_frame_delay(Gif_Stream* gfs, int i)
{
    if (!gfs || i < 0 || i >= gfs->nimages)
        return -1;
    return gfs->images[i]->delay;
}

void
bridge_set_frame_delay(Gif_Stream* gfs, int i, int delay)
{
    if (!gfs || i < 0 || i >= gfs->nimages)
        return;
    gfs->images[i]->delay = (uint16_t) delay;
}

int
bridge_frame_width(Gif_Stream* gfs, int i)
{
    if (!gfs || i < 0 || i >= gfs->nimages)
        return 0;
    return gfs->images[i]->width;
}

int
bridge_frame_height(Gif_Stream* gfs, int i)
{
    if (!gfs || i < 0 || i >= gfs->nimages)
        return 0;
    return gfs->images[i]->height;
}

long
bridge_loopcount(Gif_Stream* gfs)
{
    return gfs ? gfs->loopcount : -1;
}

void
bridge_set_loopcount(Gif_Stream* gfs, long loopcount)
{
    if (gfs)
        gfs->loopcount = loopcount;
}

int
bridge_screen_width(Gif_Stream* gfs)
{
    return gfs ? gfs->screen_width : 0;
}

int
bridge_screen_height(Gif_Stream* gfs)
{
    return gfs ? gfs->screen_height : 0;
}

void
bridge_set_write_info(int compress_flags, int loss)
{
    gif_write_info.flags = compress_flags;
    gif_write_info.loss = loss;
}

void
bridge_uncompress_frame(Gif_Stream* gfs, int i)
{
    if (!gfs || i < 0 || i >= gfs->nimages)
        return;
    Gif_UncompressImage(gfs, gfs->images[i]);
}

void
bridge_delete_stream(Gif_Stream* gfs)
{
    Gif_DeleteStream(gfs);
}
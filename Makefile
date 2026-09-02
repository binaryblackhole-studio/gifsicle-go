# Builds the shared library this binding links against.
#
# Upstream gifsicle does not ship a library target, so we fetch its
# sources at a pinned release tag and compile the library parts
# (unmodified) into a small shared library. Nothing from upstream is
# vendored into this repository.
#
#   make prepare   fetch upstream sources into build/gifsicle
#   make lib       compile build/libgifsicle-go.so
#   make all       lib + go build
#   make test      all + go test
#
# GIFSICLE_SRC=/path/to/gifsicle make prepare
#   uses an existing local checkout instead of cloning. The checkout is
#   expected to be at the pinned release (or a compatible one).
#
# To track a new upstream release: bump GIFSICLE_VERSION (upstream tags
# releases as vX.Y) and re-run "make clean prepare lib".

GIFSICLE_URL     := https://github.com/kohler/gifsicle.git
GIFSICLE_VERSION := 1.96

UPSTREAM := build/gifsicle
LIB      := build/libgifsicle-go.so

# Library sources from upstream, compiled unmodified. Excluded on
# purpose: the CLI mains (gifsicle.c gifdiff.c gifview.c gifx.c),
# the CLI's argument parser (clp.c) and glue (support.c), the
# debugging allocator (fmalloc.c, upstream-quirk duplicate symbols),
# and the X11 viewer bits. Our bridge.c provides the CLI hooks the
# library expects and the merge pipeline entry points.
UPSTREAM_SRCS := \
	giffunc.c \
	gifread.c \
	gifwrite.c \
	gifunopt.c \
	merge.c \
	optimize.c \
	quantize.c \
	kcolor.c \
	xform.c

CC ?= cc
CFLAGS ?= -O2
SOFLAGS := -fPIC -shared
INCLUDES := -I. -I$(UPSTREAM)/src -I$(UPSTREAM)/include -DHAVE_CONFIG_H=1

lib: $(LIB)

all: $(LIB)
	go build ./...

test: all
	go test ./...

prepare: $(UPSTREAM)/.prepared

$(UPSTREAM)/.prepared:
	mkdir -p build
ifdef GIFSICLE_SRC
	rm -rf $(UPSTREAM)
	cp -r "$(GIFSICLE_SRC)" $(UPSTREAM)
	rm -rf $(UPSTREAM)/.git
else
	test -d $(UPSTREAM) || git clone --quiet --depth 1 --branch v$(GIFSICLE_VERSION) $(GIFSICLE_URL) $(UPSTREAM)
endif
	@touch $@

$(LIB): prepare $(wildcard bridge.c bridge.h config.h) $(UPSTREAM)/.prepared
	$(CC) $(CFLAGS) $(SOFLAGS) $(INCLUDES) -o $@ \
	    $(patsubst %,$(UPSTREAM)/src/%,$(UPSTREAM_SRCS)) -lm

clean:
	rm -rf build

.PHONY: all test prepare clean
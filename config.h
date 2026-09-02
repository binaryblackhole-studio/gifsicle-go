/* Minimal config.h for building gifsicle's library + optimizer sources
   outside the autoconf build (mirrors what configure.ac would produce
   on a modern POSIX system). */
#ifndef GIFSICLE_LIB_CONFIG_H
#define GIFSICLE_LIB_CONFIG_H

#include <stdint.h>
#include <inttypes.h>

#define RANDOM          random
#define HAVE_POW        1
#define HAVE_CBRTF      1
#define HAVE_INT64_T    1
#define HAVE_INT64_TYPES 1
#define HAVE_INTTYPES_H 1
#define HAVE_UINTPTR_T  1
#define HAVE_SYS_TYPES_H 1
#define HAVE_U_INT_TYPES 1

#endif

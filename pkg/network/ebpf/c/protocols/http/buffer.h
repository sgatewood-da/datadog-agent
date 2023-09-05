#ifndef __HTTP_BUFFER_H
#define __HTTP_BUFFER_H

#include "ktypes.h"
#if defined(COMPILE_PREBUILT) || defined(COMPILE_RUNTIME)
#include <linux/err.h>
#endif

#include "bpf_builtins.h"
#include "bpf_telemetry.h"

#include "protocols/http/types.h"
#include "protocols/read_into_buffer.h"

// This function reads a constant number of bytes into the fragment buffer of the http
// transaction object, and returns the number of bytes of the valid data. The number of
// bytes are used in userspace to zero out the garbage we may have read into the buffer.
//
// This function is used for the uprobe-based HTTPS monitoring (eg. OpenSSL, GnuTLS etc)
static __always_inline void read_into_buffer(char *buffer, char *data, size_t data_size) {
    bpf_memset(buffer, 0, HTTP_BUFFER_SIZE);
    const u32 final_size = data_size < HTTP_BUFFER_SIZE ? data_size : HTTP_BUFFER_SIZE;

    // we read HTTP_BUFFER_SIZE-1 bytes to ensure that the string is always null terminated
    bpf_probe_read_user_with_telemetry(buffer, final_size - 1, data);
}

static __always_inline void read_into_buffer_classification(char *buffer, char *data, size_t data_size) {
    bpf_memset(buffer, 0, CLASSIFICATION_MAX_BUFFER);

    // we read CLASSIFICATION_MAX_BUFFER-1 bytes to ensure that the string is always null terminated
    if (bpf_probe_read_user_with_telemetry(buffer, CLASSIFICATION_MAX_BUFFER - 1, data) < 0) {
// note: arm64 bpf_probe_read_user() could page fault if the CLASSIFICATION_MAX_BUFFER overlap a page
#pragma unroll(CLASSIFICATION_MAX_BUFFER - 1)
        for (int i = 0; i < CLASSIFICATION_MAX_BUFFER - 1; i++) {
            bpf_probe_read_user(&buffer[i], 1, &data[i]);
            if (buffer[i] == 0) {
                return;
            }
        }
    }
}

READ_INTO_BUFFER(skb, HTTP_BUFFER_SIZE, BLK_SIZE)

#endif

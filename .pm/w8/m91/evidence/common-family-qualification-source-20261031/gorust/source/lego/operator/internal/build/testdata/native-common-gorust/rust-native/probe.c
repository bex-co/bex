#include <stdint.h>
#include <zlib.h>

uint32_t bex_probe_checksum(const unsigned char *text, unsigned int size) {
    return (uint32_t)crc32(0, text, size);
}
int64_t bex_probe_arithmetic(int64_t left, int64_t right) {
    return left * right - 17;
}
const char *bex_probe_header(void) { return ZLIB_VERSION; }
const char *bex_probe_library(void) { return zlibVersion(); }

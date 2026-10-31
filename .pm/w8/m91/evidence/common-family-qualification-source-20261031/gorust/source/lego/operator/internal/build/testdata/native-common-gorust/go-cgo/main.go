/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

/*
#cgo LDFLAGS: -lz
#include <zlib.h>
static unsigned long probe_checksum(const unsigned char *data, unsigned int size) {
    return adler32(adler32(0L, Z_NULL, 0), data, size);
}
static long probe_arithmetic(long value) { return value * value + 17; }
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func main() {
	text := []byte("Bex 雪 λ🙂")
	checksum := C.probe_checksum((*C.uchar)(unsafe.Pointer(&text[0])), C.uint(len(text)))
	fmt.Printf("go-cgo|%08x|%d|%s|%s\n", uint64(checksum), int64(C.probe_arithmetic(73)), C.ZLIB_VERSION, C.GoString(C.zlibVersion()))
}

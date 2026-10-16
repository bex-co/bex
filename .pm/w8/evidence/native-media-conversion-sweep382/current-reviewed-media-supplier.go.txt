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

package build

// nativeFFmpegImage is the reviewed FFmpeg every native build carries, as
// Render's native runtimes all include ffmpeg at build and runtime (w8/m82).
// The image's /ffmpeg and /ffprobe are fully static (libmp3lame, libopus,
// libvpx, x264 and the rest compiled in), so they need no library closure and
// run unchanged on every reviewed toolchain base; the pin is a multi-arch
// index (linux/amd64 + linux/arm64), inventoried in toolchain-freshness.json.
const nativeFFmpegImage = "mwader/static-ffmpeg:9.0.2-1@sha256:f5d100184884eb3e82b0eb14bb5ebf98da99826ac824989720297fb9ef21c822"

// nativeMediaTools is the generated step that puts ffmpeg and ffprobe on the
// image PATH before the source and the authored build, so the build, the
// pre-deploy step and the started service all find them; a static site's
// build does too, as on Render. The binaries are copied to / because a
// --link copy that creates no parent directory is byte-reproducible: every
// native image, on every base and build, then carries the same ~110 MB layer,
// which the registry stores once and a node pulls once (a copy into
// /usr/local/bin recreates those directories with the build's timestamps and
// so is a new blob each time). The links are a few bytes on top. It follows
// the base's other generated steps, so a pin change rebuilds nothing before
// it.
const nativeMediaTools = "COPY --link --from=" + nativeFFmpegImage + " /ffmpeg /ffprobe /\n" +
	"RUN ln -sf /ffmpeg /usr/local/bin/ffmpeg && ln -sf /ffprobe /usr/local/bin/ffprobe\n"

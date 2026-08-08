/**
 * Streaming, byte-bounded request-body readers for the SSR mutation handlers.
 *
 * `request.formData()` / `request.json()` buffer the WHOLE body into memory with
 * no ceiling, and a `Content-Length` check does not help: a chunked or HTTP/2
 * body omits that header, so a length-only guard sees `0` and buffers everything
 * anyway. A signed-in user could exhaust the single dashboard replica that way.
 *
 * These readers pull the body stream chunk by chunk and abort the moment the cap
 * is exceeded — before the whole body is resident — trusting the bytes actually
 * read, never the declared length. Callers keep any `Content-Length` check only
 * as an early fast-reject and route the real read through here.
 */

/** Thrown when a body exceeds its cap mid-stream. Callers map it to a 413. */
export class BodyTooLargeError extends Error {
  constructor(public readonly limit: number) {
    super(`request body exceeds ${limit} bytes`);
    this.name = "BodyTooLargeError";
  }
}

/**
 * Reads `request`'s body, throwing `BodyTooLargeError` as soon as more than
 * `limit` bytes have been seen. The stream is cancelled on overflow so the
 * remainder is never pulled into memory.
 */
export async function readBoundedBody(
  request: Request,
  limit: number,
): Promise<Uint8Array<ArrayBuffer>> {
  const stream = request.body;
  if (!stream) {
    // No stream to meter (a body already consumed, or a non-stream runtime):
    // fall back to arrayBuffer but still enforce the cap.
    const buffered = new Uint8Array(await request.arrayBuffer());
    if (buffered.byteLength > limit) throw new BodyTooLargeError(limit);
    return buffered;
  }

  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (!value) continue;
    total += value.byteLength;
    if (total > limit) {
      await reader.cancel().catch(() => undefined);
      throw new BodyTooLargeError(limit);
    }
    chunks.push(value);
  }

  const out = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    out.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return out;
}

/** Bounded body → decoded UTF-8 string. */
export async function readBoundedText(
  request: Request,
  limit: number,
): Promise<string> {
  return new TextDecoder().decode(await readBoundedBody(request, limit));
}

/** Bounded body → parsed JSON. Throws `BodyTooLargeError` (413) on overflow or a
 * `SyntaxError` (the caller's malformed-body path) on invalid JSON. */
export async function readBoundedJson<T>(
  request: Request,
  limit: number,
): Promise<T> {
  return JSON.parse(await readBoundedText(request, limit)) as T;
}

/**
 * Bounded body → parsed `FormData`. The bounded bytes are handed to a fresh
 * `Request` carrying the original `Content-Type` so the platform parser handles
 * `application/x-www-form-urlencoded` and `multipart/form-data` (boundary
 * preserved) exactly as `request.formData()` would have.
 */
export async function readBoundedFormData(
  request: Request,
  limit: number,
): Promise<FormData> {
  const bytes = await readBoundedBody(request, limit);
  const contentType =
    request.headers.get("content-type") ?? "application/x-www-form-urlencoded";
  // Wrap the bounded bytes in a Blob (an unambiguous BodyInit) carrying the
  // original Content-Type so the platform parser reads urlencoded/multipart
  // (boundary preserved) exactly as request.formData() would have.
  const bounded = new Request(request.url, {
    method: "POST",
    body: new Blob([bytes], { type: contentType }),
  });
  return bounded.formData();
}

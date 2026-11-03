import { config } from "@/config/config";

/**
 * An attachment as the API describes it: the upload route's 201 body and the
 * snapshot replayed on `data-user-prompt.attachments` (ADR095 D2/D3).
 */
export interface AttachmentView {
  id: string;
  name: string;
  mediaType: string;
  size: number;
}

/**
 * Why an upload failed. `server` carries the API's own words verbatim; the
 * other kinds are translated where they are shown.
 */
export type UploadFailure =
  | { kind: "network" }
  | { kind: "http"; status: number }
  | { kind: "server"; message: string };

/** A rejected upload, carrying its typed failure. */
export class AttachmentUploadError extends Error {
  constructor(readonly failure: UploadFailure) {
    super(
      failure.kind === "server"
        ? failure.message
        : failure.kind === "http"
          ? `upload failed (HTTP ${failure.status})`
          : "network error",
    );
    this.name = "AttachmentUploadError";
  }
}

/** The four raster types the API serves inline (ADR095 D7/D9). SVG is not one. */
const INLINE_IMAGE_TYPES = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
]);

export function isInlineImage(mediaType: string): boolean {
  return INLINE_IMAGE_TYPES.has(mediaType.toLowerCase());
}

/** Reads the API's `{message}` error envelope; falls back to the status. */
export function uploadFailureFrom(status: number, body: string): UploadFailure {
  if (status === 0) return { kind: "network" };
  try {
    const parsed = JSON.parse(body) as { message?: unknown };
    if (typeof parsed.message === "string" && parsed.message.trim()) {
      return { kind: "server", message: parsed.message };
    }
  } catch {
    // Not JSON: an edge or proxy answered. Fall through to the status.
  }
  return { kind: "http", status };
}

export interface UploadOptions {
  ownerId: string | null;
  onProgress?: (fraction: number) => void;
  signal?: AbortSignal;
}

/**
 * Streams one file to `POST /v1/agent-attachments` with the dashboard's cookie
 * session. XHR rather than fetch because only XHR reports upload progress.
 */
export function uploadAttachment(
  file: File,
  { ownerId, onProgress, signal }: UploadOptions,
): Promise<AttachmentView> {
  return new Promise((resolve, reject) => {
    const params = new URLSearchParams();
    if (ownerId) params.set("ownerId", ownerId);
    params.set("name", file.name || "attachment");
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${config.apiBaseUrl}/v1/agent-attachments?${params}`);
    xhr.withCredentials = true;
    // The server sniffs the real type; this only satisfies the raw-body route.
    xhr.setRequestHeader(
      "Content-Type",
      file.type || "application/octet-stream",
    );
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress?.(event.loaded / event.total);
      }
    };
    xhr.onload = () => {
      if (xhr.status !== 201) {
        reject(
          new AttachmentUploadError(
            uploadFailureFrom(xhr.status, xhr.responseText),
          ),
        );
        return;
      }
      const view = attachmentFrom(safeJSON(xhr.responseText));
      if (view) resolve(view);
      else reject(new AttachmentUploadError({ kind: "http", status: 201 }));
    };
    xhr.onerror = () => reject(new AttachmentUploadError({ kind: "network" }));
    xhr.onabort = () =>
      reject(new DOMException("upload canceled", "AbortError"));
    if (signal) {
      if (signal.aborted) {
        xhr.abort();
        return;
      }
      signal.addEventListener("abort", () => xhr.abort(), { once: true });
    }
    xhr.send(file);
  });
}

function safeJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/**
 * Deletes an upload that was never sent. Best effort: an unbound upload the
 * user dropped would also expire on its own after 24 hours.
 */
export async function deleteUnboundAttachment(id: string): Promise<void> {
  await fetch(
    `${config.apiBaseUrl}/v1/agent-attachments/${encodeURIComponent(id)}`,
    { method: "DELETE", credentials: "include" },
  );
}

/** One attachment from untrusted JSON, or null when it has no id. */
function attachmentFrom(item: unknown): AttachmentView | null {
  if (!item || typeof item !== "object") return null;
  const { id, name, mediaType, size } = item as Record<string, unknown>;
  if (typeof id !== "string" || !id) return null;
  return {
    id,
    name: typeof name === "string" && name ? name : id,
    mediaType: typeof mediaType === "string" ? mediaType : "",
    size: typeof size === "number" && Number.isFinite(size) ? size : 0,
  };
}

/** Parses the replayed `data-user-prompt.attachments` snapshot defensively. */
export function turnAttachmentsFrom(value: unknown): AttachmentView[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => attachmentFrom(item) ?? []);
}

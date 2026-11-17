/**
 * Triggers a browser download of in-memory text as a named file via a
 * short-lived object URL. The one shared implementation for every "save this
 * generated/revealed content" affordance (env export, Blueprint manifest,
 * database server CA) so the Blob/anchor/revoke dance can't drift per feature.
 */
export function downloadTextFile(
  filename: string,
  contents: string,
  type = "text/plain;charset=utf-8",
): void {
  const url = URL.createObjectURL(new Blob([contents], { type }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

/**
 * Hands a ready-made download URL (a presigned object-store link) to the
 * browser as a named file. The link is a bearer capability, so the anchor sends
 * no referrer and opens no window that could reach this page.
 */
export function openDownloadURL(url: string, filename?: string | null): void {
  const anchor = document.createElement("a");
  anchor.href = url;
  if (filename) anchor.download = filename;
  anchor.rel = "noopener noreferrer";
  anchor.click();
}

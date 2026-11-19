import { redirect, type ParsedLocation } from "@tanstack/react-router";

/** The Compute page's Manual Scaling section anchor (w6/m483). */
export const MANUAL_SCALING_ANCHOR = "manual-scaling";

/**
 * Sends a URL from before Render's Compute page (2026-08-26) to it: the
 * service's Plan or Scaling tab becomes `/services/{id}/compute` with its
 * query kept (w6/m483). `anchor` names the section the old tab was, unless
 * the URL already carries a hash of its own.
 */
export function redirectToComputePage(
  serviceId: string,
  location: Pick<ParsedLocation, "hash">,
  anchor?: string,
): never {
  throw redirect({
    to: "/services/$serviceId/compute",
    params: { serviceId },
    search: (prev) => prev,
    hash: location.hash || anchor,
    replace: true,
  });
}

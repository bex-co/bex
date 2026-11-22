import { redirect, type ParsedLocation } from "@tanstack/react-router";
import type { ServiceSettingsSectionId } from "@/features/services/lib/settings-sections";

/** The Compute page's Manual Scaling section anchor (w6/m483). */
export const MANUAL_SCALING_ANCHOR = "manual-scaling";

/** The Settings section that holds a cron job's compute plan (w3/m554). */
export const COMPUTE_SECTION_ANCHOR =
  "compute" as const satisfies ServiceSettingsSectionId;

/**
 * Where a service's compute plan is changed, as Render links it: a cron job's
 * Settings' Compute section (`settings#compute`), every other type's Compute
 * page (w3/m554, docs/render-artifacts/compute-page.md).
 */
export function computePlanLink(cron: boolean) {
  return cron
    ? ({
        to: "/services/$serviceId/settings",
        hash: COMPUTE_SECTION_ANCHOR,
      } as const)
    : ({ to: "/services/$serviceId/compute", hash: undefined } as const);
}

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

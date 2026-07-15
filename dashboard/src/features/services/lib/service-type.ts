import { Box, Clock, Cog, FileCode, Globe, Lock } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { en } from "@/i18n";
import type { ServiceView, ServiceTypeKey } from "@/features/services/types";

// Render's serviceType wire values → the UI's short type key. Kept next to the
// label map so a new type is added in one place (mirrors labels.ts for status).
const TYPE_KEY: Record<string, ServiceTypeKey> = {
  web_service: "web",
  private_service: "private",
  background_worker: "worker",
  cron_job: "cron",
  static_site: "static",
};

/** Resolve a wire serviceType onto its type key; unknown/legacy types map to "unknown". */
export function deriveServiceType(type: string): ServiceTypeKey {
  return TYPE_KEY[type] ?? "unknown";
}

/** Maps a resolved type key to its i18n badge label (mirrors STATUS_LABEL). */
export const SERVICE_TYPE_LABEL: Record<ServiceTypeKey, keyof typeof en> = {
  web: "services.typeWeb",
  private: "services.typePrivate",
  worker: "services.typeWorker",
  cron: "services.typeCron",
  static: "services.typeStatic",
  unknown: "services.typeUnknown",
};

/**
 * The glyph shown next to a type label — Render's service-header eyebrow pairs
 * its uppercase type name with the same icon its sidebar uses for the type.
 */
export const SERVICE_TYPE_ICON: Record<ServiceTypeKey, LucideIcon> = {
  web: Globe,
  private: Lock,
  worker: Cog,
  cron: Clock,
  static: FileCode,
  unknown: Box,
};

/** True for a cron_job — the type whose detail shows a schedule + run history. */
export function isCron(s: ServiceView): boolean {
  return s.type === "cron_job";
}

/** True for a static_site — the type whose settings show publishPath + edge rules. */
export function isStaticSite(s: ServiceView): boolean {
  return s.type === "static_site";
}

/** True for a background_worker — no public URL, no health-check path. */
export function isWorker(s: ServiceView): boolean {
  return s.type === "background_worker";
}

/** True when the service owns a long-running pod with a SIGTERM grace window. */
export function supportsMaxShutdownDelay(s: ServiceView): boolean {
  return (
    s.type === "web_service" ||
    s.type === "private_service" ||
    s.type === "background_worker"
  );
}

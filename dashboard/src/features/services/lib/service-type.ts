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

// Render's destructive sudo confirmations name the type in lowercase words —
// "sudo delete web service <name>", "sudo suspend static site <name>" (live
// captures: docs/render-artifacts/protected-environments.md). These are part
// of the exact phrase the user must type, so they are never translated.
const SUDO_TYPE_WORDS: Record<ServiceTypeKey, string> = {
  web: "web service",
  private: "private service",
  worker: "background worker",
  cron: "cron job",
  static: "static site",
  unknown: "service",
};

/** Lowercase type words used inside a service's sudo confirmation phrase. */
export function sudoServiceTypeWords(s: ServiceView): string {
  return SUDO_TYPE_WORDS[deriveServiceType(s.type)];
}

/** Render's exact sudo confirmation phrase for a destructive service verb. */
export function serviceSudoPhrase(verb: "delete", s: ServiceView): string {
  return `sudo ${verb} ${sudoServiceTypeWords(s)} ${s.name}`;
}

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

/**
 * True for a web_service — the only type maintenance mode applies to
 * (Render parity, w1/m37). Empty spec.type reads back as web_service from
 * bex-api, so a legacy App with no explicit type still matches.
 */
export function isWebService(s: ServiceView): boolean {
  return isWebServiceType(s.type);
}

/** Raw-type form for callers that have not built a ServiceView. */
export function isWebServiceType(type: string): boolean {
  return type === "web_service";
}

/**
 * True for a private_service — reachable only on the private network, so its
 * header shows a copyable Service Address (internalAddress) instead of a
 * public-URL link (ADR041 D4, w9/m58).
 */
export function isPrivateService(s: ServiceView): boolean {
  return s.type === "private_service";
}

/**
 * True when this type binds an HTTP port of its own, so port/URL vocabulary
 * applies to it. This deliberately includes private services; public routing
 * and auto-sleep have narrower predicates.
 *
 * Takes the raw wire value, like `deriveServiceType`, so callers holding an
 * untyped `ServiceView.type` need no cast.
 */
export function servesHttp(type: string): boolean {
  return type === "web_service" || type === "private_service";
}

/**
 * True when this type is served at a public host — so custom domains and the
 * platform `.onbex.co` subdomain are things it can actually have. Mirrors the
 * CRD contract's `AppSpec.PubliclyRoutable` (`lego/types/v1alpha1`), which is
 * what decides whether the operator builds an Ingress at all. Note this is
 * narrower than `servesHttp`: a private_service serves HTTP, but only inside
 * the platform network.
 *
 * Takes the raw wire value, like `servesHttp` and `deriveServiceType`.
 */
export function publiclyRoutable(type: string): boolean {
  return (
    type !== "private_service" &&
    type !== "background_worker" &&
    type !== "cron_job"
  );
}

/** True when the service owns a long-running pod with a SIGTERM grace window. */
export function supportsMaxShutdownDelay(s: ServiceView): boolean {
  return servesHttp(s.type) || isWorker(s);
}

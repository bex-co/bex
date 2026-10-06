import {
  isCronType,
  isDockerBuild,
  isStaticSiteType,
  isWebServiceType,
  publiclyRoutable,
  servesHttp,
} from "@/features/services/lib/service-type";

/** A section of a service's Settings page, named by its anchor id. */
export type ServiceSettingsSectionId =
  | "general"
  | "deploy"
  | "source"
  | "build"
  | "static-site"
  | "domains"
  | "networking"
  | "outbound-ips"
  | "registry-credential"
  | "notifications"
  | "port"
  | "health-checks"
  | "maintenance"
  | "deploy-hook"
  | "suspend"
  | "danger-zone";

/** A section the page's navigation links: all but Outbound IPs, a reference card. */
export type NavigableSettingsSectionId = Exclude<
  ServiceSettingsSectionId,
  "outbound-ips"
>;

/** What the section list reads from a service. */
export interface SettingsSectionsService {
  type: string;
  repo?: string | null;
  runtime?: string | null;
  builder?: string | null;
}

/**
 * The sections of a service's Settings page, in page order. The page renders
 * exactly these, its navigation links them, and its pending skeleton draws one
 * region for each, so the three cannot drift apart (w5/m124). An undefined
 * service is one not loaded (still loading, failed or not found): the page then
 * shows General, Notifications and Health Checks, whose rows stay disabled until
 * it arrives (w6/027).
 */
export function serviceSettingsSections(
  service: SettingsSectionsService | undefined,
): ServiceSettingsSectionId[] {
  if (!service) return ["general", "notifications", "health-checks"];
  const { type, repo } = service;
  const cron = isCronType(type);
  const staticSite = isStaticSiteType(type);
  const sections: ServiceSettingsSectionId[] = ["general"];
  if (cron) {
    // Its Deploy section holds the schedule and command. A git-sourced cron job
    // still builds, so it keeps Build (w2/m9, w5/010).
    sections.push("deploy");
    if (repo) sections.push("build");
  } else {
    if (!staticSite) sections.push("source");
    // An Existing Image service has no build: its Deploy card stands alone.
    if (!repo && !staticSite) sections.push("deploy");
    if (repo) sections.push("build");
    if (staticSite) sections.push("static-site");
    // Custom domains, the platform subdomain and the inbound IP allowlist exist
    // only for a type served at a public host (w6/m46, w7/m32): bex-api refuses
    // them on any other.
    if (publiclyRoutable(type)) sections.push("domains", "networking");
    sections.push("outbound-ips");
  }
  if (!staticSite && (!repo || isDockerBuild(service))) {
    sections.push("registry-credential");
  }
  sections.push("notifications");
  // web_service and private_service bind the port bex routes to and probe.
  if (servesHttp(type)) sections.push("port", "health-checks");
  // Maintenance mode is web_service only, as bex-api's requireWebService (w1/m37).
  if (isWebServiceType(type)) sections.push("maintenance");
  // Every other type holds its Deploy Hook in its Deploy card; a cron job's
  // Deploy section is its schedule, so its hook stands alone.
  if (cron) sections.push("deploy-hook");
  // Suspend (or Resume) and delete, at the bottom as on Render.
  sections.push("suspend", "danger-zone");
  return sections;
}

/** The sections the page's navigation links. */
export function navigableSettingsSections(
  sections: ServiceSettingsSectionId[],
): NavigableSettingsSectionId[] {
  return sections.filter(
    (section): section is NavigableSettingsSectionId =>
      section !== "outbound-ips",
  );
}

import { canonicalEnvironmentLinkId } from "@/features/environments/lib/link-id";

/** The five bex service types (Render's `web_service`/`private_service`/
 *  `background_worker`/`cron_job`/`static_site`). Single source of truth for the
 *  create page, its create paths, and the create-type list the New menus and
 *  the breadcrumb type switcher share (create-type-items.ts). */
export const SERVICE_TYPES = [
  "web_service",
  "private_service",
  "background_worker",
  "cron_job",
  "static_site",
] as const;

export type ServiceType = (typeof SERVICE_TYPES)[number];

/**
 * Each service type's create URL, Render's own (`/web/new`, `/pserv/new`,
 * `/worker/new`, `/cron/new`, `/static/new` — its New menu and the type
 * switcher, live capture 2026-11-21). The page's type is its path.
 */
export const SERVICE_TYPE_CREATE_PATHS = {
  web_service: "/web/new",
  private_service: "/pserv/new",
  background_worker: "/worker/new",
  cron_job: "/cron/new",
  static_site: "/static/new",
} as const satisfies Record<ServiceType, string>;

export type ServiceCreatePath = (typeof SERVICE_TYPE_CREATE_PATHS)[ServiceType];

/** The service type a create URL names, or null off those URLs. */
export function serviceTypeFromCreatePath(
  pathname: string,
): ServiceType | null {
  const entry = Object.entries(SERVICE_TYPE_CREATE_PATHS).find(
    ([, path]) => path === pathname,
  );
  return entry ? (entry[0] as ServiceType) : null;
}

/** The type `/services/new` (and a create path bex does not know) falls back
 *  to. Exported so the form state and the skeleton resolve the same type. */
export const DEFAULT_SERVICE_TYPE: ServiceType = "web_service";

function isServiceType(v: unknown): v is ServiceType {
  return (
    typeof v === "string" && (SERVICE_TYPES as readonly string[]).includes(v)
  );
}

/** A service type's create-page copy: heading, subtitle and submit label. */
export interface ServiceTypeCreateCopy {
  titleKey: string;
  descriptionKey: string;
  submitKey: string;
}

/**
 * Create-wizard page title + subtitle, per service type. The wizard's heading
 * has to name the thing being created — "New Service / Deploy a web service…"
 * over a selected Background Worker is the page contradicting its own type
 * picker. Kept beside the other per-type tables above so a sixth service type
 * cannot be added without an obvious empty cell here.
 *
 */
export const SERVICE_TYPE_CREATE_COPY: Record<
  ServiceType,
  ServiceTypeCreateCopy
> = {
  web_service: {
    titleKey: "services.createWebTitle",
    descriptionKey: "services.createWebDescription",
    submitKey: "services.createSubmitWeb",
  },
  private_service: {
    titleKey: "services.createPrivateTitle",
    descriptionKey: "services.createPrivateDescription",
    submitKey: "services.createSubmitPrivate",
  },
  background_worker: {
    titleKey: "services.createWorkerTitle",
    descriptionKey: "services.createWorkerDescription",
    submitKey: "services.createSubmitWorker",
  },
  cron_job: {
    titleKey: "services.createCronTitle",
    descriptionKey: "services.createCronDescription",
    submitKey: "services.createSubmitCron",
  },
  static_site: {
    titleKey: "services.createStaticTitle",
    descriptionKey: "services.createStaticDescription",
    submitKey: "services.createSubmitStatic",
  },
};

/** The wizard's heading + subtitle keys. An absent OR unknown `?type=` resolves
 *  to the same default the form itself starts on, so the two cannot disagree.
 *
 *  The parameter is a plain string on purpose (w4/102): defaulting only on
 *  nullish once indexed the table to `undefined` for `/services/new?type=worker`
 *  (bex's old create URL, now a redirect to the per-type paths)
 *  and threw on `.titleKey`, because the route's `head` resolver then read the
 *  raw search. It reads validated search now (w5/078); the function stays total
 *  so no caller depends on that. */
export function serviceTypeCreateCopy(
  type: string | undefined,
): ServiceTypeCreateCopy {
  return SERVICE_TYPE_CREATE_COPY[
    isServiceType(type) ? type : DEFAULT_SERVICE_TYPE
  ];
}

export interface NewServiceSearch {
  /** Preselects the wizard's service type (Render's per-type create deep link,
   *  on bex's old `/services/new`, which redirects to its path). Unknown
   *  values dropped. */
  type?: ServiceType;
  projectId?: string;
  environmentId?: string;
}

/** Accept only a known service `type` plus nonempty string ids from contextual
 *  service-create links. A rejected key is set to undefined, not omitted: the
 *  router merges the validated search over the raw one, so an omitted key
 *  keeps its raw value for every `useSearch()` reader (w5/078). */
export function parseNewServiceSearch(
  search: Record<string, unknown>,
): NewServiceSearch {
  return {
    type: isServiceType(search.type) ? search.type : undefined,
    projectId:
      typeof search.projectId === "string" && search.projectId
        ? search.projectId
        : undefined,
    environmentId:
      typeof search.environmentId === "string" && search.environmentId
        ? canonicalEnvironmentLinkId(search.environmentId)
        : undefined,
  };
}

import { canonicalEnvironmentLinkId } from "@/features/environments/lib/link-id";

/** The five bex service types (Render's `web_service`/`private_service`/
 *  `background_worker`/`cron_job`/`static_site`). Single source of truth for the
 *  create wizard, the `?type=` deep-link, and the per-type "New" menu items. */
export const SERVICE_TYPES = [
  "web_service",
  "private_service",
  "background_worker",
  "cron_job",
  "static_site",
] as const;

export type ServiceType = (typeof SERVICE_TYPES)[number];

/** What the wizard creates when `?type=` is absent. Exported so the form
 *  state and the route's document title resolve the same type. */
export const DEFAULT_SERVICE_TYPE: ServiceType = "web_service";

function isServiceType(v: unknown): v is ServiceType {
  return (
    typeof v === "string" && (SERVICE_TYPES as readonly string[]).includes(v)
  );
}

/** Per-type service-create menu entries. Render's New menu lists service types
 *  individually; each entry deep-links the create wizard to its type via
 *  `?type=`. Label keys are the same ones the wizard's type picker uses. */
export const SERVICE_TYPE_CREATE_ITEMS: {
  type: ServiceType;
  labelKey: string;
}[] = [
  { type: "web_service", labelKey: "services.typeWeb" },
  { type: "private_service", labelKey: "services.typePrivate" },
  { type: "background_worker", labelKey: "services.typeWorker" },
  { type: "cron_job", labelKey: "services.typeCron" },
  { type: "static_site", labelKey: "services.typeStatic" },
];

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
  { titleKey: string; descriptionKey: string }
> = {
  web_service: {
    titleKey: "services.createWebTitle",
    descriptionKey: "services.createWebDescription",
  },
  private_service: {
    titleKey: "services.createPrivateTitle",
    descriptionKey: "services.createPrivateDescription",
  },
  background_worker: {
    titleKey: "services.createWorkerTitle",
    descriptionKey: "services.createWorkerDescription",
  },
  cron_job: {
    titleKey: "services.createCronTitle",
    descriptionKey: "services.createCronDescription",
  },
  static_site: {
    titleKey: "services.createStaticTitle",
    descriptionKey: "services.createStaticDescription",
  },
};

/** The wizard's heading + subtitle keys. An absent OR unknown `?type=` resolves
 *  to the same default the form itself starts on, so the two cannot disagree.
 *
 *  The parameter is a plain string on purpose (w4/102): defaulting only on
 *  nullish once indexed the table to `undefined` for `/services/new?type=worker`
 *  and threw on `.titleKey`, because the route's `head` resolver then read the
 *  raw search. It reads validated search now (w5/078); the function stays total
 *  so no caller depends on that. */
export function serviceTypeCreateCopy(type: string | undefined): {
  titleKey: string;
  descriptionKey: string;
} {
  return SERVICE_TYPE_CREATE_COPY[
    isServiceType(type) ? type : DEFAULT_SERVICE_TYPE
  ];
}

export interface NewServiceSearch {
  /** Preselects the wizard's service type (Render's per-type create deep link,
   *  e.g. `/cron/new` → `/services/new?type=cron_job`). Unknown values dropped. */
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

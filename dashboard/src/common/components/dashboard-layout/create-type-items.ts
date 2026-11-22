import {
  Database,
  Globe2,
  KeyRound,
  ListEnd,
  Lock,
  PanelsTopLeft,
  Timer,
  type LucideIcon,
} from "lucide-react";
import {
  SERVICE_TYPE_CREATE_COPY,
  SERVICE_TYPE_CREATE_PATHS,
  type ServiceCreatePath,
  type ServiceType,
} from "@/features/services/lib/create-context";

export interface CreateTypeItem {
  id: ServiceType | "postgres" | "keyvalue";
  /** Its create page, at Render's path. */
  to: ServiceCreatePath | "/new/database" | "/new/redis";
  /** A service type: the five share one form, so a switch between them keeps
   *  the draft. */
  service: boolean;
  /** The type's name, as the New menu and the type switcher list it. */
  labelKey: string;
  /** Its create page's heading ("New Web Service"). */
  titleKey: string;
  icon: LucideIcon;
}

function serviceItem(
  id: ServiceType,
  labelKey: string,
  icon: LucideIcon,
): CreateTypeItem {
  return {
    id,
    to: SERVICE_TYPE_CREATE_PATHS[id],
    service: true,
    labelKey,
    titleKey: SERVICE_TYPE_CREATE_COPY[id].titleKey,
    icon,
  };
}

/**
 * Every resource bex creates on a create page, in the order Render's
 * breadcrumb type switcher ("New Web Service ▾ / Choose a service type") and
 * flat New menu list them (live capture 2026-11-21): the service types, then
 * the datastores. One list, so the switcher and the New menu cannot drift.
 */
export const CREATE_TYPE_ITEMS: CreateTypeItem[] = [
  serviceItem("static_site", "services.typeStatic", PanelsTopLeft),
  serviceItem("web_service", "services.typeWeb", Globe2),
  serviceItem("private_service", "services.typePrivate", Lock),
  serviceItem("background_worker", "services.typeWorker", ListEnd),
  serviceItem("cron_job", "services.typeCron", Timer),
  {
    id: "postgres",
    to: "/new/database",
    service: false,
    labelKey: "common.createTypePostgres",
    titleKey: "databases.createTitle",
    icon: Database,
  },
  {
    id: "keyvalue",
    to: "/new/redis",
    service: false,
    labelKey: "keyvalue.resourceType",
    titleKey: "keyvalue.createTitle",
    icon: KeyRound,
  },
];

/** The create type a page shows, or null off the create pages. */
export function currentCreateType(pathname: string): CreateTypeItem | null {
  return CREATE_TYPE_ITEMS.find((item) => item.to === pathname) ?? null;
}

import { createContext, useContext } from "react";

/**
 * The two URL bases a service-detail page can live under. A `static_site` is
 * canonical under `/static/<id>` (Render parity, w5/m57 — Render serves static
 * sites at `dashboard.render.com/static/<id>`); every other service type is
 * canonical under `/services/<id>`. The detail UI (layout, sidebar, tab pages)
 * is shared between the two route trees — a service is bounced to its canonical
 * base by the shared loader (service-detail-loader.ts), and intra-detail links
 * stay under the current base via `useServiceBase()`.
 */
export type ServiceBase = "/services" | "/static";

/** The canonical base for a wire serviceType (`static_site`, `web_service`, …). */
export function serviceBaseForType(
  type: string | null | undefined,
): ServiceBase {
  return type === "static_site" ? "/static" : "/services";
}

/**
 * The base the current service-detail route renders under, supplied by
 * ServiceDetailLayout (each route tree provides its own). Kept as context
 * rather than sniffed from the router so intra-detail links resolve without a
 * router hook — a component rendered standalone (unit tests) safely reads the
 * `/services` default instead of crashing.
 */
const ServiceBaseContext = createContext<ServiceBase>("/services");
export const ServiceBaseProvider = ServiceBaseContext.Provider;

export function useServiceBase(): ServiceBase {
  return useContext(ServiceBaseContext);
}

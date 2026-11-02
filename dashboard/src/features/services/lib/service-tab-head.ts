import {
  routeResourceTitle,
  titleHead,
  translatedText,
  type RouteResource,
} from "@/common/lib/document-head";
import { deriveServiceType, SERVICE_TYPE_LABEL } from "./service-type";

interface TitledService {
  displayName?: string | null;
  name: string;
  type?: string | null;
}

interface HeadMatch {
  routeId: string;
  loaderData?: unknown;
  status?: string;
}

/**
 * Document title for one service tab: the tab's own name first, then the
 * service and its type, e.g. "Logs ・ api ・ Web Service ・ bex Dashboard"
 * (w4/m249). The service layout route sets one title for all of its tabs, so
 * a tab switch gave assistive technology and history no signal. The label is
 * the sidebar's own key, so the title and the active nav item cannot drift.
 */
export function serviceTabHead(
  labelKey: string,
  layoutRouteId: "/services/$serviceId" | "/static/$serviceId",
) {
  return ({ matches }: { matches: readonly HeadMatch[] }) => {
    const layout = matches.find((match) => match.routeId === layoutRouteId);
    return titleHead(
      routeResourceTitle(
        layout?.loaderData as RouteResource<TitledService> | undefined,
        (service) => [
          translatedText(labelKey),
          service.displayName?.trim() || service.name,
          translatedText(
            SERVICE_TYPE_LABEL[deriveServiceType(service.type ?? "")],
          ),
        ],
      ),
      matches,
    );
  };
}

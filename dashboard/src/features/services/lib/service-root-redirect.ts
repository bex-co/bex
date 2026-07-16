import { redirect } from "@tanstack/react-router";

/** Service-list/project links land on deploy history; Events remains a sibling tab. */
export function redirectServiceRoot(serviceId: string): never {
  throw redirect({
    to: "/services/$serviceId/deploys",
    params: { serviceId },
    replace: true,
  });
}

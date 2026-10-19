import { createContext, useContext } from "react";

/**
 * Tells the service-detail layout that its own service is being deleted
 * (w4/340). A successful delete evicts the Service; the layout, still mounted
 * while the navigation home loads, refetches `server(id)`, reads not-found, and
 * its dead-id redirect would toast "doesn't exist or was deleted" beside the
 * success toast. ServiceDetailLayout supplies the setter, so the flag lives and
 * dies with that layout; outside it the default is a no-op.
 */
const ServiceDeletingContext = createContext<(deleting: boolean) => void>(
  () => {},
);
export const ServiceDeletingProvider = ServiceDeletingContext.Provider;

export function useMarkServiceDeleting(): (deleting: boolean) => void {
  return useContext(ServiceDeletingContext);
}

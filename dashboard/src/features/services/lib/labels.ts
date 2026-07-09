import type { en } from "@/i18n";
import type { ServiceStatusKey } from "@/features/services/types";

/**
 * Maps a resolved status key to its i18n badge label. Shared by the services
 * list (`routes/index.tsx`) and the service-detail header/overview so the same
 * phase renders the same word everywhere.
 */
export const STATUS_LABEL: Record<ServiceStatusKey, keyof typeof en> = {
  running: "services.statusRunning",
  suspended: "services.statusSuspended",
  sleeping: "services.statusSleeping",
  pending: "services.statusPending",
  building: "services.statusBuilding",
  deploying: "services.statusDeploying",
  failed: "services.statusFailed",
  unknown: "services.statusUnknown",
};

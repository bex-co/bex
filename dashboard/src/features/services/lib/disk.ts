import type { en } from "@/i18n";

/**
 * The persistent-disk rules the Disk tab and the create form share
 * (docs/ADR082-persistent-disks.md), mirroring bex-api's validateDisk
 * (lego/backend/internal/apps/disks.go) so both mark a disk the server would
 * refuse before it is sent.
 */
type Reason = keyof typeof en;

/**
 * The name a dashboard-added disk gets. Render's dashboard asks for no name,
 * on its create form or its Disk page, and sends `disk` from both.
 */
export const DASHBOARD_DISK_NAME = "disk";

/**
 * Render's quick-select sizes, 1 GB and up as on Render (.pm/w8/215.md).
 *
 * Hetzner's smallest Cloud Volume is 10 GB, so a 1 or 5 GB disk sits on a
 * 10 GB volume; bex absorbs the difference and reports and bills the size
 * chosen here (ADR082 D6/D8).
 */
export const DISK_SIZE_CHIPS = [1, 5, 10, 50, 100] as const;
export const DISK_DEFAULT_SIZE_GB = 10;
export const DISK_MIN_SIZE_GB = 1;
/**
 * Render's cap on both disk forms, 1 TB: a larger size is a support request,
 * and bex-api refuses it too (w4/732).
 */
export const DISK_MAX_SIZE_GB = 1000;

/** The service types validateDisk accepts: a long-running instance to mount on. */
const DISK_ELIGIBLE_TYPES: readonly string[] = [
  "web_service",
  "private_service",
  "background_worker",
];

export function diskEligibleType(serviceType: string): boolean {
  return DISK_ELIGIBLE_TYPES.includes(serviceType);
}

/**
 * Whether a service of this type on this plan may have a disk: a paid plan,
 * so not Free and not before the catalog has picked one.
 */
export function diskAllowed(serviceType: string, plan: string): boolean {
  return diskEligibleType(serviceType) && plan !== "" && plan !== "free";
}

/** Paths the platform populates; a disk may mount a subdirectory of each. */
const RESERVED_MOUNT_PATHS = [
  "/etc",
  "/etc/secrets",
  "/home",
  "/home/render",
  "/opt",
  "/opt/render",
  "/opt/render/project",
  "/opt/render/project/src",
  "/bex-health",
];

/** The mount-path rules the CRD enforces, for a trimmed path. */
export function diskMountPathError(path: string): Reason | null {
  if (path === "") return "services.diskMountPathRequired";
  if (!path.startsWith("/")) return "services.diskMountPathAbsolute";
  if (path.endsWith("/")) return "services.diskMountPathNotRoot";
  if (RESERVED_MOUNT_PATHS.includes(path)) {
    return "services.diskMountPathReserved";
  }
  return null;
}

/**
 * Why a size cannot be sent: past the 1 TB cap, Render's words sending the
 * user to support; otherwise not a whole number of GB in bex-api's range.
 */
export function diskSizeError(sizeGB: number): Reason | null {
  if (sizeGB > DISK_MAX_SIZE_GB) return "services.diskSizeAboveMax";
  return Number.isInteger(sizeGB) && sizeGB >= DISK_MIN_SIZE_GB
    ? null
    : "services.diskSizeRange";
}

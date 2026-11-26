/** The sandbox drawer's tabs, Render's Events, Snapshots and Details. */
export const SANDBOX_DRAWER_TABS = ["events", "snapshots", "details"] as const;
export type SandboxDrawerTab = (typeof SANDBOX_DRAWER_TABS)[number];

export function isSandboxDrawerTab(value: unknown): value is SandboxDrawerTab {
  return (
    typeof value === "string" &&
    (SANDBOX_DRAWER_TABS as readonly string[]).includes(value)
  );
}

/** The image drawer's tabs, Render's Versions, Details and Sandboxes. */
export const IMAGE_DRAWER_TABS = ["versions", "details", "sandboxes"] as const;
export type ImageDrawerTab = (typeof IMAGE_DRAWER_TABS)[number];

export function isImageDrawerTab(value: unknown): value is ImageDrawerTab {
  return (
    typeof value === "string" &&
    (IMAGE_DRAWER_TABS as readonly string[]).includes(value)
  );
}

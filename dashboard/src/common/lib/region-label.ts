import type { en } from "@/i18n";
import { ownValue } from "@/common/lib/own-value";

/**
 * Render's region aliases (`resourcemeta.RenderRegions`,
 * lego/backend/internal/resourcemeta/metadata.go) mapped to their display
 * names. The API keeps the slug, as Render's does; the dashboard names the
 * place, as Render's region docs do ("Frankfurt, Germany"), in the dashboard
 * language (w4/693).
 */
const REGION_LABEL: Record<string, keyof typeof en> = {
  frankfurt: "common.regionFrankfurt",
  ohio: "common.regionOhio",
  oregon: "common.regionOregon",
  singapore: "common.regionSingapore",
  virginia: "common.regionVirginia",
};

/** A region's display name; a slug this dashboard does not know renders verbatim. */
export function regionLabel(
  region: string,
  t: (key: keyof typeof en) => string,
): string {
  const key = ownValue(REGION_LABEL, region);
  return key ? t(key) : region;
}

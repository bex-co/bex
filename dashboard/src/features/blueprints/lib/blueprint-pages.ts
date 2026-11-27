import type { BlueprintPagePath } from "@/features/blueprints/components/blueprint-nav";
import { splatParts } from "@/common/lib/render-alias";

/**
 * Where a sub-path of `/blueprints/<id>/` with no page of its own lands
 * (w3/m677, docs/render-artifacts/blueprint-pages.md § Routes): Render's
 * `sync` and `sync/<syncId>` on Syncs, until w3/m678 gives a sync its page;
 * anything else on the bare URL, Resources, as Render's `*` route shows
 * Resources. Null is the bare URL.
 */
export function blueprintSubpathLanding(
  splat: string | undefined,
): BlueprintPagePath | null {
  const [first] = splatParts(splat);
  return first === "sync" ? "/syncs" : null;
}

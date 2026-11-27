import { useRouterState } from "@tanstack/react-router";
import {
  BLUEPRINT_NAV_GROUPS,
  isBlueprintNavItemActive,
} from "@/features/blueprints/components/blueprint-nav";
import { useBlueprint } from "@/features/blueprints/hooks/use-blueprint";
import { canonicalBlueprintLinkId } from "@/features/blueprints/lib/link-id";
import { ResourceSidebar, type ResourceSidebarBack } from "./resource-sidebar";
import { SidebarNavGroups } from "./sidebar-nav-groups";

// Render's back link from a Blueprint goes to the Blueprints list.
const BACK_TO_BLUEPRINTS: ResourceSidebarBack = {
  to: "/blueprints",
  labelKey: "common.navBlueprints",
};

/**
 * Render parity (w3/m677): a Blueprint's pages swap the workspace nav for a
 * resource sidebar, as a database's and a Key Value instance's do: the
 * Blueprints back link, the name, then Resources, Syncs and Settings (the
 * list lives in features/blueprints/components/blueprint-nav.tsx).
 *
 * Reads the Blueprint through `useBlueprint` with the id the layout reads
 * (the API's exs- spelling of a `blp-` bookmark), cache-first and without a
 * second poll, so Apollo serves it with no extra request and the sidebar
 * agrees with the layout on whether the Blueprint exists. The entries never
 * depend on the Blueprint, so the loading sidebar already draws them all.
 */
export function BlueprintSidebar({ blueprintId }: { blueprintId: string }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const { blueprint, loading } = useBlueprint(
    canonicalBlueprintLinkId(blueprintId),
    { poll: false },
  );
  // An id the caller can't see gets no nav — the layout redirects it home,
  // and a sidebar of dead links would contradict it.
  const showNav = loading || blueprint !== null;

  return (
    <ResourceSidebar
      name={blueprint?.name ?? blueprintId}
      back={BACK_TO_BLUEPRINTS}
    >
      {showNav ? (
        <SidebarNavGroups
          groups={[...BLUEPRINT_NAV_GROUPS]}
          linkParams={{ blueprintId }}
          isItemActive={(to) =>
            isBlueprintNavItemActive(pathname, blueprintId, to)
          }
        />
      ) : null}
    </ResourceSidebar>
  );
}

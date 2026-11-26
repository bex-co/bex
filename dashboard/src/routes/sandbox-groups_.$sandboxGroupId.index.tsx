import { createFileRoute } from "@tanstack/react-router";
import { SandboxesPageSkeleton } from "@/features/sandbox-group-pages/components/group-page-skeletons";
import { SandboxesPage } from "@/features/sandbox-group-pages/components/sandboxes-page";
import {
  isSandboxDrawerTab,
  type SandboxDrawerTab,
} from "@/features/sandbox-group-pages/lib/drawer-tabs";
import { sandboxGroupPageHead } from "@/features/sandbox-group-pages/lib/page-head";

export interface SandboxesSearch {
  /** The sandbox whose drawer is open. */
  sandbox?: string;
  /** The drawer's tab; Events when absent or unknown. */
  tab?: SandboxDrawerTab;
}

const SANDBOX_ID = /^sbx-[0-9a-v]{20}$/;

/**
 * A sandbox group's index is its Sandboxes list, as Render's
 * `/sandbox-group/<id>` is (w1/m658); `?sandbox=<sbx-…>&tab=<tab>` opens a
 * sandbox's drawer on a tab, as Render's `sandboxes/<id>/<tab>` does.
 */
export const Route = createFileRoute("/sandbox-groups_/$sandboxGroupId/")({
  head: sandboxGroupPageHead("sandboxGroupPages.navSandboxes"),
  component: RouteComponent,
  pendingComponent: SandboxesPageSkeleton,
  validateSearch: (search: Record<string, unknown>): SandboxesSearch => ({
    sandbox:
      typeof search.sandbox === "string" && SANDBOX_ID.test(search.sandbox)
        ? search.sandbox
        : undefined,
    tab: isSandboxDrawerTab(search.tab) ? search.tab : undefined,
  }),
});

function RouteComponent() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <SandboxesPage
      openSandboxId={search.sandbox}
      openTab={search.tab ?? "events"}
      onOpen={(sandbox, tab) =>
        void navigate({ search: { sandbox, tab: tab === "events" ? undefined : tab } })
      }
      onClose={() => void navigate({ search: { sandbox: undefined, tab: undefined } })}
    />
  );
}

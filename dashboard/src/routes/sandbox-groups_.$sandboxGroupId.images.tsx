import { createFileRoute } from "@tanstack/react-router";
import { ImagesPageSkeleton } from "@/features/sandbox-group-pages/components/group-page-skeletons";
import { ImagesPage } from "@/features/sandbox-group-pages/components/images-page";
import {
  isImageDrawerTab,
  type ImageDrawerTab,
} from "@/features/sandbox-group-pages/lib/drawer-tabs";
import { sandboxGroupPageHead } from "@/features/sandbox-group-pages/lib/page-head";

export interface ImagesSearch {
  /** The image name whose drawer is open. */
  image?: string;
  /** The drawer's tab; Versions when absent or unknown. */
  tab?: ImageDrawerTab;
}

/**
 * A sandbox group's Custom images page, Render's `builds/*` (w1/m658);
 * `?image=<name>&tab=<tab>` opens an image's drawer.
 */
export const Route = createFileRoute("/sandbox-groups_/$sandboxGroupId/images")({
  head: sandboxGroupPageHead("sandboxGroupPages.navImages"),
  component: RouteComponent,
  pendingComponent: ImagesPageSkeleton,
  validateSearch: (search: Record<string, unknown>): ImagesSearch => ({
    image:
      typeof search.image === "string" && search.image.trim() !== ""
        ? search.image
        : undefined,
    tab: isImageDrawerTab(search.tab) ? search.tab : undefined,
  }),
});

function RouteComponent() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <ImagesPage
      openImage={search.image}
      openTab={search.tab ?? "versions"}
      onOpen={(image, tab) =>
        void navigate({ search: { image, tab: tab === "versions" ? undefined : tab } })
      }
      onClose={() => void navigate({ search: { image: undefined, tab: undefined } })}
    />
  );
}

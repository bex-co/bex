import { createFileRoute } from "@tanstack/react-router";
import { KeyValueInfoSkeleton } from "@/common/components/route-skeletons";
import { KeyValueInfoPage } from "@/features/keyvalue/pages/key-value-info-page";
import { keyValuePageHead } from "@/features/keyvalue/lib/key-value-page-head";

/** Render's `/r/<id>/info`, the Info page its sidebar links (w3/m649). */
export const Route = createFileRoute("/keyvalue/$keyValueId/info")({
  head: keyValuePageHead("keyvalue.navInfo"),
  component: KeyValueInfoPage,
  pendingComponent: KeyValueInfoSkeleton,
});

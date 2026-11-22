import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { KeyValueCreatePageSkeleton } from "@/common/components/route-skeletons";
import { NewKeyValuePage } from "@/features/keyvalue/pages/new-key-value-page";

/** Render's Key Value create URL (`/new/redis` — the legacy Redis segment,
 *  live captures 2026-07-16 and 2026-11-21): the page lives here, and `/r/new`
 *  and bex's old `/keyvalue/new` land on it. */
export const Route = createFileRoute("/new/redis")({
  staticData: { chrome: true },
  component: NewKeyValuePage,
  pendingComponent: KeyValueCreatePageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("keyvalue.createTitle", match),
});

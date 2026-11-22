import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { SERVICE_TYPE_CREATE_COPY } from "@/features/services/lib/create-context";

/** Render's background worker create URL; the page is the `_serviceCreate`
 *  layout's. */
export const Route = createFileRoute("/_serviceCreate/worker/new")({
  staticData: { chrome: true },
  head: ({ match }) =>
    translatedTitleHead(
      SERVICE_TYPE_CREATE_COPY.background_worker.titleKey,
      match,
    ),
});

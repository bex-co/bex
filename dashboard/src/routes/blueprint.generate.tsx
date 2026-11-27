import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  formatDashboardTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";
import { BlueprintGeneratePageSkeleton } from "@/common/components/route-skeletons";
import { GenerateBlueprintPage } from "@/features/blueprints/pages/generate-blueprint-page";
import {
  generateBlueprintTitle,
  readBlueprintExport,
} from "@/features/blueprints/lib/blueprint-export";

/**
 * Render's Create Blueprint page (`/blueprint/generate`, w3/m681): the
 * resources a Generate Blueprint entry point hands over in the navigation's
 * state, exported into Render's three steps. With none it goes to `/`.
 */
export const Route = createFileRoute("/blueprint/generate")({
  staticData: { chrome: true },
  component: GenerateBlueprintPage,
  pendingComponent: BlueprintGeneratePageSkeleton,
  beforeLoad: requireAuth(),
  // The document title names the environment the page came from, which only
  // the navigation's state knows.
  loader: ({ location }) => ({ handed: readBlueprintExport(location.state) }),
  head: ({ match, loaderData }) =>
    titleHead(
      formatDashboardTitle(
        generateBlueprintTitle(translatedText, loaderData?.handed),
      ),
      match,
    ),
});

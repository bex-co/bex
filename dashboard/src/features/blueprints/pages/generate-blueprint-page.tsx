import { useEffect, useMemo } from "react";
import { Link, Navigate, useMatch, useRouter } from "@tanstack/react-router";
import { useQuery } from "@apollo/client/react";
import { Check, Copy, Download, ExternalLink } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { BlueprintGeneratePageSkeleton } from "@/common/components/route-skeletons";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";
import { conjunctionList } from "@/common/lib/format/language";
import { downloadTextFile } from "@/common/lib/download-file";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { GenerateBlueprintDocument } from "@/graphql/definitions";
import {
  GenerateBlueprintSteps,
  GeneratingBox,
  Snippet,
} from "@/features/blueprints/components/generate-blueprint-steps";
import { useHandedBlueprintExport } from "@/features/blueprints/hooks/use-handed-blueprint-export";
import {
  blueprintExportVariables,
  generateBlueprintTitle,
  orderedExportResources,
  type BlueprintExport,
} from "@/features/blueprints/lib/blueprint-export";

/**
 * Render's Create Blueprint page at `/blueprint/generate` (w3/m681,
 * docs/render-artifacts/blueprint-pages.md § Create Blueprint): the resources
 * a Generate Blueprint entry handed over, exported through `generateBlueprint`
 * into Render's three steps. With nothing handed over (a pasted link) it goes
 * to `/`, as Render's does.
 */
export function GenerateBlueprintPage() {
  const handed = useHandedBlueprintExport();
  if (handed === undefined) return <BlueprintGeneratePageSkeleton />;
  if (handed === null) return <Navigate to="/" replace />;
  return <GenerateBlueprintReady handed={handed} />;
}

/**
 * The document title is the route loader's, and a reload runs that loader on
 * the server, which the navigation's state does not reach. When the title it
 * gave differs from the page's, the loader runs again in the browser, so the
 * tab names the environment the page came from, as the heading does.
 */
function useTitleFromHandedExport(title: string) {
  const { t } = useTranslations();
  const router = useRouter();
  const match = useMatch({ from: "/blueprint/generate", shouldThrow: false });
  const stale =
    match !== undefined &&
    match.status === "success" &&
    generateBlueprintTitle(t, match.loaderData?.handed) !== title;
  useEffect(() => {
    if (!stale) return;
    void router.invalidate({
      filter: (candidate) => candidate.routeId === "/blueprint/generate",
    });
  }, [stale, router]);
}

function GenerateBlueprintReady({ handed }: { handed: BlueprintExport }) {
  const { t, i18n } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const resources = useMemo(
    () => orderedExportResources(handed.resources),
    [handed.resources],
  );
  const { data, error } = useQuery(GenerateBlueprintDocument, {
    variables: {
      ownerId: currentWorkspaceId,
      ...blueprintExportVariables(resources),
    },
    fetchPolicy: "no-cache",
    skip: !currentWorkspaceId,
  });
  const manifest = data?.generateBlueprint?.manifest ?? null;
  const generating = manifest === null && error === undefined;
  const { copied, copy } = useCopyToClipboard({
    errorText: t("blueprints.generateCopyFailed"),
  });
  const title = generateBlueprintTitle(t, handed);
  useTitleFromHandedExport(title);
  const names = resources.map((resource) => resource.name).filter(Boolean);

  const exportBody = generating ? (
    <GeneratingBox />
  ) : (
    <>
      <p>
        {t("blueprints.generateIncludes", { count: names.length })}{" "}
        {conjunctionList(i18n.language)
          .formatToParts(names)
          .map((part, index) =>
            part.type === "element" ? (
              <span key={index} className="font-semibold">
                {part.value}
              </span>
            ) : (
              part.value
            ),
          )}
      </p>
      {manifest !== null ? (
        <Snippet text={manifest} className="max-h-96 overflow-y-auto" />
      ) : (
        <Alert variant="destructive">
          <AlertDescription>
            {mutationErrorMessage(error, t("blueprints.generateError"))}
          </AlertDescription>
        </Alert>
      )}
    </>
  );

  return (
    <DashboardLayout>
      <PageFrame width="standard" header={<PageHeader title={title} />}>
        <GenerateBlueprintSteps
          exportBody={exportBody}
          exportActions={
            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                disabled={manifest === null}
                onClick={() => {
                  if (manifest !== null) {
                    downloadTextFile(
                      "render.yaml",
                      manifest,
                      "application/yaml",
                    );
                  }
                }}
              >
                <Download />
                {t("blueprints.generateDownload")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={manifest === null}
                onClick={() => {
                  if (manifest !== null) void copy(manifest);
                }}
              >
                {copied ? <Check /> : <Copy />}
                {t(
                  copied
                    ? "blueprints.generateCopiedLabel"
                    : "blueprints.generateCopy",
                )}
              </Button>
            </div>
          }
          connectAction={
            generating ? (
              <Button size="sm" disabled>
                {t("blueprints.generateConnect")}
                <ExternalLink />
              </Button>
            ) : (
              <Button size="sm" asChild>
                <Link to="/blueprint/new" target="_blank" rel="noopener">
                  {t("blueprints.generateConnect")}
                  <ExternalLink />
                </Link>
              </Button>
            )
          }
        />
      </PageFrame>
    </DashboardLayout>
  );
}

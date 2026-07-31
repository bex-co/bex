import { useState } from "react";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { BlueprintStatusBadge } from "@/features/blueprints/components/blueprint-status-badge";
import { ValidatePanel } from "@/features/blueprints/components/validate-panel";
import { useBlueprint } from "@/features/blueprints/hooks/use-blueprint";
import { useSyncBlueprint } from "@/features/blueprints/hooks/use-sync-blueprint";
import { formatRelativeAge } from "@/features/services/lib/format";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { protectedServiceName } from "@/features/services/lib/protected-confirmation";
import { BlueprintDocument } from "@/features/blueprints/api/operations";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/blueprints/$blueprintId")({
  component: BlueprintDetailPage,
  beforeLoad: requireAuth(),
  loader: ({ context, params }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: BlueprintDocument,
          variables: {
            id: params.blueprintId,
            ownerId: context.workspaceId,
          },
          fetchPolicy: "network-only",
          errorPolicy: "all",
        }),
      (data) => (data?.blueprint?.name?.trim() ? data.blueprint : null),
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (blueprint) => [
        blueprint.name,
        translatedText("blueprints.resourceType"),
      ]),
      match,
    ),
});

export function BlueprintDetailPage() {
  const { blueprintId } = Route.useParams();
  const { t } = useTranslations();
  const router = useRouter();
  const { blueprint, loading, error, refetch } = useBlueprint(blueprintId);
  const { sync, busy } = useSyncBlueprint();
  const [confirming, setConfirming] = useState(false);
  const [protectedConfirmation, setProtectedConfirmation] = useState<
    string | null
  >(null);

  // A dead id redirects home (w9/m55); a failed query stays put on the inline
  // error state so an outage never masquerades as a deleted blueprint. A
  // roll-window loader failure re-runs once (w1/m52) so the title recovers.
  useNotFoundRedirect(!loading && !blueprint && !error);
  useLoaderErrorRetry(Route.useLoaderData(), blueprintId);
  const showError = !loading && !blueprint && !!error;

  async function handleSync(confirmation?: string) {
    setConfirming(false);
    const result = await sync(blueprintId, confirmation);
    if (result.status === "confirmation_required") {
      setProtectedConfirmation(result.confirmation);
      return;
    }
    if (result.status === "success") {
      setProtectedConfirmation(null);
      void router.invalidate();
    }
  }

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
        <div className="flex items-center gap-2">
          <h1 className="truncate text-xl font-semibold">
            {blueprint?.name ?? blueprintId}
          </h1>
          {blueprint ? (
            <BlueprintStatusBadge status={blueprint.status} />
          ) : null}
        </div>
        {blueprint ? (
          <Button size="sm" onClick={() => setConfirming(true)} disabled={busy}>
            {t("blueprints.syncButton")}
          </Button>
        ) : null}
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          {showError ? (
            <ResourceLoadError onRetry={refetch} />
          ) : blueprint ? (
            <>
              <Card>
                <CardHeader>
                  <CardTitle className="text-base">{blueprint.name}</CardTitle>
                </CardHeader>
                <CardContent>
                  <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
                    <div>
                      <dt className="text-muted-foreground">
                        {t("blueprints.metaRepo")}
                      </dt>
                      <dd className="truncate font-medium">{blueprint.repo}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">
                        {t("blueprints.metaBranch")}
                      </dt>
                      <dd className="font-medium">{blueprint.branch}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">
                        {t("blueprints.metaCreated")}
                      </dt>
                      <dd className="font-medium">
                        {blueprint.createdAt
                          ? formatRelativeAge(blueprint.createdAt)
                          : "—"}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">
                        {t("blueprints.metaUpdated")}
                      </dt>
                      <dd className="font-medium">
                        {blueprint.updatedAt
                          ? formatRelativeAge(blueprint.updatedAt)
                          : "—"}
                      </dd>
                    </div>
                  </dl>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">
                    {t("blueprints.manifestTitle")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs leading-relaxed">
                    <code>{blueprint.manifest || "—"}</code>
                  </pre>
                </CardContent>
              </Card>

              <ValidatePanel manifest={blueprint.manifest} />
            </>
          ) : (
            <>
              <CardSkeleton rows={3} />
              <CardSkeleton rows={6} />
              <CardSkeleton rows={2} />
            </>
          )}
        </div>
      </div>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("blueprints.syncConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("blueprints.syncConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("blueprints.syncCancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleSync()}>
              {t("blueprints.syncConfirmAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <ProtectedConfirmationDialog
        key={protectedConfirmation ? `open:${protectedConfirmation}` : "closed"}
        open={protectedConfirmation !== null}
        resourceName={
          protectedConfirmation
            ? protectedServiceName(protectedConfirmation)
            : (blueprint?.name ?? blueprintId)
        }
        requiredConfirmation={protectedConfirmation ?? ""}
        actionLabel={t("blueprints.syncConfirmAction")}
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirmation(null)}
        onConfirm={async (confirmation) => {
          await handleSync(confirmation);
        }}
      />
    </DashboardLayout>
  );
}

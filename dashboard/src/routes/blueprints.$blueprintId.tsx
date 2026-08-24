import { useState } from "react";
import { Check, Loader2, Pencil, X } from "lucide-react";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import {
  resourceFailed,
  resourceNotFound,
  useNotFoundRedirect,
} from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Switch } from "@/common/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
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
import { useBlueprintPreview } from "@/features/blueprints/hooks/use-blueprint-preview";
import { BlueprintPlanSummary } from "@/features/blueprints/components/blueprint-plan-summary";
import { useSyncBlueprint } from "@/features/blueprints/hooks/use-sync-blueprint";
import { useUpdateBlueprint } from "@/features/blueprints/hooks/use-update-blueprint";
import { useDisconnectBlueprint } from "@/features/blueprints/hooks/use-disconnect-blueprint";
import { useBlueprintSyncs } from "@/features/blueprints/hooks/use-blueprint-syncs";
import { formatRelativeAge } from "@/features/services/lib/format";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { protectedServiceName } from "@/features/services/lib/protected-confirmation";
import { BlueprintDocument } from "@/graphql/definitions";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/blueprints/$blueprintId")({
  staticData: { chrome: true },
  component: BlueprintDetailPage,
  pendingComponent: BlueprintDetailPage,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: BlueprintDocument,
          variables: {
            id: params.blueprintId,
            ownerId: context.workspaceId,
          },
          fetchPolicy: titleLoaderFetchPolicy(cause),
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

/** Client-side mirror of the backend's approvedBlueprintPath rule (m19):
 * clean repo-relative .yaml/.yml, no escapes. The backend re-validates. */
function validBlueprintPath(path: string): boolean {
  const p = path.trim();
  if (!p || p.startsWith("/") || p.includes("\\")) return false;
  if (p.split("/").some((seg) => seg === "" || seg === "." || seg === "..")) {
    return false;
  }
  return /\.(yaml|yml)$/.test(p);
}

/** Inline value editor for the metadata card (w8/m21 t003): pencil toggles
 * an input; save PATCHes only the changed field. */
function InlineEdit({
  value,
  label,
  mono,
  validate,
  invalidMessage,
  onSave,
  disabled,
}: {
  value: string;
  label: string;
  mono?: boolean;
  validate?: (next: string) => boolean;
  invalidMessage?: string;
  onSave: (next: string) => Promise<void>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const invalid = editing && validate ? !validate(draft) : false;
  if (!editing) {
    return (
      <span className="flex items-center gap-1.5">
        <span className={mono ? "font-mono font-medium" : "font-medium"}>
          {value}
        </span>
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t("blueprints.editField", { field: label })}
          disabled={disabled}
          onClick={() => {
            setDraft(value);
            setEditing(true);
          }}
        >
          <Pencil className="size-3.5" />
        </Button>
      </span>
    );
  }
  return (
    <span className="flex flex-col gap-1">
      <span className="flex items-center gap-1.5">
        <Input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          className={mono ? "h-8 font-mono" : "h-8"}
          aria-label={label}
          autoFocus
        />
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t("blueprints.saveField", { field: label })}
          disabled={disabled || invalid || draft.trim() === ""}
          onClick={() => {
            const next = draft.trim();
            setEditing(false);
            if (next !== value) void onSave(next);
          }}
        >
          <Check className="size-3.5" />
        </Button>
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t("blueprints.cancelEdit")}
          onClick={() => setEditing(false)}
        >
          <X className="size-3.5" />
        </Button>
      </span>
      {invalid && invalidMessage ? (
        <span className="text-xs text-destructive">{invalidMessage}</span>
      ) : null}
    </span>
  );
}

export function BlueprintDetailPage() {
  const { blueprintId } = Route.useParams();
  const { t } = useTranslations();
  const router = useRouter();
  const { blueprint, loading, error, refetch } = useBlueprint(blueprintId);
  const { sync, busy: syncBusy } = useSyncBlueprint();
  const { update, busy: updateBusy } = useUpdateBlueprint();
  const { disconnect, busy: disconnectBusy } = useDisconnectBlueprint();
  const { syncs } = useBlueprintSyncs(blueprintId);

  const [confirming, setConfirming] = useState(false);
  // Pre-sync plan (w8/m21 t002): fetched only while the dialog is open — an
  // empty repo skips the query. Render shows a computed diff before sync;
  // applying blindly is the pre-m21 behavior this replaces.
  const {
    preview: syncPreview,
    loading: syncPreviewLoading,
    error: syncPreviewError,
  } = useBlueprintPreview(
    confirming && blueprint ? blueprint.repo : "",
    confirming && blueprint ? blueprint.branch : "",
    confirming ? (blueprint?.path ?? "") : "",
  );
  const [disconnecting, setDisconnecting] = useState(false);
  const [protectedConfirmation, setProtectedConfirmation] = useState<
    string | null
  >(null);

  const busy = syncBusy || updateBusy || disconnectBusy;

  useNotFoundRedirect(resourceNotFound(blueprint, loading, error));
  useLoaderErrorRetry(Route.useLoaderData(), blueprintId);
  const showError = resourceFailed(blueprint, loading, error);

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

  async function handleAutoSyncToggle(value: boolean) {
    await update(blueprintId, { autoSync: value });
    void router.invalidate();
  }

  async function handleDisconnect() {
    setDisconnecting(false);
    const ok = await disconnect(blueprintId);
    if (ok) {
      void router.navigate({ to: "/blueprints", replace: true });
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
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              onClick={() => setConfirming(true)}
              disabled={busy}
            >
              {t("blueprints.syncButton")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setDisconnecting(true)}
              disabled={busy}
            >
              {t("blueprints.disconnectButton")}
            </Button>
          </div>
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
                  <CardTitle className="text-base">
                    <InlineEdit
                      value={blueprint.name}
                      label={t("blueprints.createNameLabel")}
                      onSave={async (next) => {
                        await update(blueprintId, { name: next });
                        void router.invalidate();
                      }}
                      disabled={busy}
                    />
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-3">
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
                        {t("blueprints.metaPath")}
                      </dt>
                      <dd>
                        <InlineEdit
                          value={blueprint.path || "render.yaml"}
                          label={t("blueprints.metaPath")}
                          mono
                          validate={validBlueprintPath}
                          invalidMessage={t("blueprints.pathInvalid")}
                          onSave={async (next) => {
                            await update(blueprintId, { path: next });
                            void router.invalidate();
                          }}
                          disabled={busy}
                        />
                      </dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">
                        {t("blueprints.metaAutoSync")}
                      </dt>
                      <dd className="flex items-center gap-2 font-medium">
                        <Switch
                          checked={blueprint.autoSync}
                          onCheckedChange={(v) => void handleAutoSyncToggle(v)}
                          disabled={busy}
                          aria-label={t("blueprints.metaAutoSync")}
                        />
                        <span>
                          {blueprint.autoSync
                            ? t("blueprints.autoSyncOn")
                            : t("blueprints.autoSyncOff")}
                        </span>
                      </dd>
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
                        {blueprint.lastSync
                          ? formatRelativeAge(blueprint.lastSync)
                          : "—"}
                      </dd>
                    </div>
                  </dl>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">
                    {t("blueprints.resourcesTitle")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {blueprint.resources && blueprint.resources.length > 0 ? (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("blueprints.colName")}</TableHead>
                          <TableHead>Type</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {blueprint.resources.map((r) => (
                          <TableRow key={r.id}>
                            <TableCell className="font-medium">
                              {r.name}
                            </TableCell>
                            <TableCell className="text-muted-foreground capitalize">
                              {r.type.replace(/_/g, " ")}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      {t("blueprints.resourcesEmpty")}
                    </p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-base">
                    {t("blueprints.syncHistoryTitle")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {syncs.length > 0 ? (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("blueprints.syncColCommit")}</TableHead>
                          <TableHead>{t("blueprints.syncColState")}</TableHead>
                          <TableHead>
                            {t("blueprints.syncColStarted")}
                          </TableHead>
                          <TableHead>
                            {t("blueprints.syncColCompleted")}
                          </TableHead>
                          <TableHead>{t("blueprints.syncColError")}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {syncs.map((run) => (
                          <TableRow key={run.id}>
                            <TableCell className="font-mono text-xs">
                              {run.commitId ? run.commitId.slice(0, 8) : "—"}
                            </TableCell>
                            <TableCell>
                              <BlueprintStatusBadge status={run.state} />
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {run.startedAt
                                ? formatRelativeAge(run.startedAt)
                                : "—"}
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {run.completedAt
                                ? formatRelativeAge(run.completedAt)
                                : "—"}
                            </TableCell>
                            <TableCell
                              className="max-w-xs truncate text-muted-foreground"
                              title={run.errorMessage ?? undefined}
                            >
                              {run.errorMessage ?? "—"}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  ) : (
                    <p className="text-sm text-muted-foreground">
                      {t("blueprints.syncHistoryEmpty")}
                    </p>
                  )}
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
              <CardSkeleton rows={4} />
              <CardSkeleton rows={3} />
              <CardSkeleton rows={3} />
              <CardSkeleton rows={6} />
              <CardSkeleton rows={2} />
            </>
          )}
        </div>
      </div>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("blueprints.syncConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("blueprints.syncConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {syncPreviewLoading ? (
            <div className="flex items-center gap-2 py-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              {t("blueprints.syncPreviewLoading")}
            </div>
          ) : syncPreview?.found && syncPreview.validation?.valid === true ? (
            <div className="space-y-3">
              <BlueprintPlanSummary
                plan={syncPreview.validation.plan}
                pricing={syncPreview.validation.estimatedPricing}
              />
            </div>
          ) : syncPreview ? (
            <div className="space-y-2 rounded-md border border-destructive/50 p-3 text-sm">
              <p className="font-medium text-destructive">
                {t("blueprints.syncPreviewInvalid")}
              </p>
              <ul className="list-disc space-y-1 pl-4 text-muted-foreground">
                {(syncPreview.validation?.errors ?? [syncPreview.error])
                  .filter((e): e is string => !!e)
                  .map((e, i) => (
                    <li key={i}>{e}</li>
                  ))}
              </ul>
            </div>
          ) : syncPreviewError ? (
            <p className="text-sm text-amber-600 dark:text-amber-500">
              {t("blueprints.syncPreviewUnavailable")}
            </p>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel>{t("blueprints.syncCancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => void handleSync()}
              disabled={syncPreviewLoading}
            >
              {t("blueprints.syncConfirmAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={disconnecting} onOpenChange={setDisconnecting}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("blueprints.disconnectTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("blueprints.disconnectBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("blueprints.disconnectCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => void handleDisconnect()}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {t("blueprints.disconnectAction")}
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
        busy={syncBusy}
        onOpenChange={(open) => !open && setProtectedConfirmation(null)}
        onConfirm={async (confirmation) => {
          await handleSync(confirmation);
        }}
      />
    </DashboardLayout>
  );
}

import { useState } from "react";
import {
  AlertTriangle,
  History,
  KeyRound,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Switch } from "@/common/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  useSandboxGroupCredentialMutations,
  useSandboxGroupCredentials,
  type SandboxGroupCredential,
} from "@/features/sandbox-groups/hooks/use-sandbox-group-credentials";
import { sudoDeleteCredentialPhrase } from "@/features/sandbox-groups/lib/credential-validation";
import { CredentialFormDialog } from "@/features/sandbox-groups/components/credential-form-dialog";
import { CredentialVersionsDialog } from "@/features/sandbox-groups/components/credential-versions-dialog";
import { SandboxGroupCredentialsPanelSkeleton } from "@/features/sandbox-groups/components/sandbox-group-skeletons";

export interface SandboxGroupCredentialsPanelProps {
  sandboxGroupId: string;
  ownerId?: string | null;
}

export function SandboxGroupCredentialsPanel({
  sandboxGroupId,
  ownerId,
}: SandboxGroupCredentialsPanelProps) {
  const { t } = useTranslations();
  const { canManage, canCreate, loading: capsLoading } = useCapabilities();
  const { credentials, loading, error, unavailable, refetch } =
    useSandboxGroupCredentials(sandboxGroupId, ownerId);

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<SandboxGroupCredential | null>(null);
  const [versionsTarget, setVersionsTarget] =
    useState<SandboxGroupCredential | null>(null);
  const [deleteTarget, setDeleteTarget] =
    useState<SandboxGroupCredential | null>(null);

  const {
    createCredential,
    updateCredential,
    deleteCredential,
    restoreCredential,
    busyId,
    submitting,
  } = useSandboxGroupCredentialMutations(sandboxGroupId, ownerId, () => {
    void refetch();
  });

  if (loading || capsLoading) {
    return <SandboxGroupCredentialsPanelSkeleton />;
  }

  const canEditRouting = canCreate || canManage;

  const openCreate = () => {
    setEditing(null);
    setFormOpen(true);
  };

  const openEdit = (cred: SandboxGroupCredential) => {
    setEditing(cred);
    setFormOpen(true);
  };

  const handleToggleEnabled = (cred: SandboxGroupCredential, next: boolean) => {
    void updateCredential(cred.id, { enabled: next }, { silentToast: true });
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    const deleted = await deleteCredential(deleteTarget.id);
    if (deleted) {
      setDeleteTarget(null);
    }
  };

  const injectionSummary = (cred: SandboxGroupCredential): string => {
    if (cred.injection.type === "basic") {
      return t("sandboxGroups.credInjectionBasic");
    }
    return cred.injection.name
      ? `${cred.injection.type}: ${cred.injection.name}`
      : cred.injection.type;
  };

  const activeVersionsCred = versionsTarget
    ? (credentials.find((c) => c.id === versionsTarget.id) ?? versionsTarget)
    : null;

  return (
    <>
      <Card className="gap-4">
        <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle>{t("sandboxGroups.credSectionTitle")}</CardTitle>
            <CardDescription>
              {t("sandboxGroups.credSectionDescription")}
            </CardDescription>
          </div>
          {canManage ? (
            <Button
              size="sm"
              className="shrink-0"
              onClick={openCreate}
              disabled={unavailable}
            >
              <Plus aria-hidden="true" />
              {t("sandboxGroups.credAddButton")}
            </Button>
          ) : null}
        </CardHeader>

        <CardContent className="space-y-4">
          {error && credentials.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-10 text-center">
              <AlertTriangle
                className="text-muted-foreground size-6"
                aria-hidden="true"
              />
              <p className="text-muted-foreground text-sm">
                {unavailable
                  ? t("sandboxGroups.credErrorUnavailable")
                  : t("sandboxGroups.credLoadError")}
              </p>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("common.tryAgain")}
              </Button>
            </div>
          ) : credentials.length === 0 ? (
            <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed py-10 text-center">
              <KeyRound
                className="text-muted-foreground size-7"
                aria-hidden="true"
              />
              <p className="text-sm font-medium">
                {t("sandboxGroups.credEmptyTitle")}
              </p>
              <p className="text-muted-foreground max-w-md text-xs">
                {canManage
                  ? t("sandboxGroups.credEmptyDescriptionAdmin")
                  : t("sandboxGroups.credEmptyDescriptionReadOnly")}
              </p>
              {canManage ? (
                <Button
                  size="sm"
                  variant="outline"
                  className="mt-2"
                  onClick={openCreate}
                >
                  <Plus aria-hidden="true" />
                  {t("sandboxGroups.credAddButton")}
                </Button>
              ) : null}
            </div>
          ) : (
            <>
              {/* Desktop table */}
              <div className="hidden overflow-hidden rounded-lg border md:block">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[18%]">
                        {t("sandboxGroups.credColId")}
                      </TableHead>
                      <TableHead className="w-[22%]">
                        {t("sandboxGroups.credColDomain")}
                      </TableHead>
                      <TableHead className="w-[20%]">
                        {t("sandboxGroups.credColInjection")}
                      </TableHead>
                      <TableHead className="w-[16%]">
                        {t("sandboxGroups.credColRouting")}
                      </TableHead>
                      <TableHead className="w-[10%]">
                        {t("sandboxGroups.credColEnabled")}
                      </TableHead>
                      <TableHead className="w-[14%] text-right">
                        {t("sandboxGroups.credColActions")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {credentials.map((cred) => {
                      const rowBusy = busyId === cred.id;
                      return (
                        <TableRow key={cred.id}>
                          <TableCell className="py-3.5 align-top">
                            <div className="space-y-0.5">
                              <div className="font-mono text-sm font-medium">
                                {cred.id}
                              </div>
                              <div className="text-muted-foreground text-xs">
                                v{cred.version.id}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell className="py-3.5 align-top font-mono text-xs">
                            {cred.domain}
                          </TableCell>
                          <TableCell className="py-3.5 align-top text-xs">
                            <span className="font-mono">
                              {injectionSummary(cred)}
                            </span>
                            {cred.injection.template ? (
                              <div className="text-muted-foreground mt-0.5 font-mono text-[11px]">
                                {cred.injection.template}
                              </div>
                            ) : null}
                          </TableCell>
                          <TableCell className="py-3.5 align-top">
                            <RoutingBadges
                              methods={cred.methods}
                              paths={cred.paths}
                            />
                          </TableCell>
                          <TableCell className="py-3.5 align-top">
                            {canEditRouting ? (
                              <Switch
                                checked={cred.enabled}
                                disabled={rowBusy}
                                aria-label={t(
                                  "sandboxGroups.credToggleEnabledLabel",
                                  { id: cred.id },
                                )}
                                onCheckedChange={(next) =>
                                  handleToggleEnabled(cred, next)
                                }
                              />
                            ) : (
                              <Badge
                                variant={
                                  cred.enabled ? "secondary" : "outline"
                                }
                              >
                                {cred.enabled
                                  ? t("sandboxGroups.credStatusEnabled")
                                  : t("sandboxGroups.credStatusDisabled")}
                              </Badge>
                            )}
                          </TableCell>
                          <TableCell className="py-3.5 text-right align-top">
                            <div className="flex justify-end gap-1">
                              <Button
                                size="icon-sm"
                                variant="ghost"
                                aria-label={t(
                                  "sandboxGroups.credVersionsActionLabel",
                                  { id: cred.id },
                                )}
                                onClick={() => setVersionsTarget(cred)}
                              >
                                <History
                                  className="size-4"
                                  aria-hidden="true"
                                />
                              </Button>
                              {canEditRouting ? (
                                <Button
                                  size="icon-sm"
                                  variant="ghost"
                                  aria-label={t(
                                    "sandboxGroups.credEditActionLabel",
                                    { id: cred.id },
                                  )}
                                  onClick={() => openEdit(cred)}
                                >
                                  <Pencil
                                    className="size-4"
                                    aria-hidden="true"
                                  />
                                </Button>
                              ) : null}
                              {canManage ? (
                                <Button
                                  size="icon-sm"
                                  variant="ghost"
                                  className="text-destructive hover:text-destructive"
                                  disabled={rowBusy}
                                  aria-label={t(
                                    "sandboxGroups.credDeleteActionLabel",
                                    { id: cred.id },
                                  )}
                                  onClick={() => setDeleteTarget(cred)}
                                >
                                  <Trash2
                                    className="size-4"
                                    aria-hidden="true"
                                  />
                                </Button>
                              ) : null}
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>

              {/* Narrow-mobile stacked cards (< md, 390px) */}
              <div className="space-y-3 md:hidden">
                {credentials.map((cred) => {
                  const rowBusy = busyId === cred.id;
                  return (
                    <div
                      key={cred.id}
                      className="space-y-3 rounded-lg border p-4"
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0 space-y-0.5">
                          <div className="truncate font-mono text-sm font-medium">
                            {cred.id}
                          </div>
                          <div className="text-muted-foreground truncate font-mono text-xs">
                            {cred.domain}
                          </div>
                        </div>
                        {canEditRouting ? (
                          <Switch
                            checked={cred.enabled}
                            disabled={rowBusy}
                            aria-label={t(
                              "sandboxGroups.credToggleEnabledLabel",
                              { id: cred.id },
                            )}
                            onCheckedChange={(next) =>
                              handleToggleEnabled(cred, next)
                            }
                          />
                        ) : (
                          <Badge
                            variant={cred.enabled ? "secondary" : "outline"}
                          >
                            {cred.enabled
                              ? t("sandboxGroups.credStatusEnabled")
                              : t("sandboxGroups.credStatusDisabled")}
                          </Badge>
                        )}
                      </div>

                      <div className="space-y-1.5 text-xs">
                        <div className="font-mono">
                          {injectionSummary(cred)}
                        </div>
                        <RoutingBadges
                          methods={cred.methods}
                          paths={cred.paths}
                        />
                      </div>

                      <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-2.5">
                        <span className="text-muted-foreground text-xs">
                          v{cred.version.id}
                        </span>
                        <div className="flex flex-wrap gap-1.5">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => setVersionsTarget(cred)}
                          >
                            <History className="size-3.5" aria-hidden="true" />
                            {t("sandboxGroups.credVersionsButton")}
                          </Button>
                          {canEditRouting ? (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => openEdit(cred)}
                            >
                              <Pencil className="size-3.5" aria-hidden="true" />
                              {t("sandboxGroups.credEditButton")}
                            </Button>
                          ) : null}
                          {canManage ? (
                            <Button
                              size="sm"
                              variant="outline"
                              className="text-destructive hover:text-destructive"
                              disabled={rowBusy}
                              onClick={() => setDeleteTarget(cred)}
                            >
                              <Trash2 className="size-3.5" aria-hidden="true" />
                              {t("sandboxGroups.credDeleteButton")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      <CredentialFormDialog
        open={formOpen}
        onOpenChange={(next) => {
          setFormOpen(next);
          if (!next) setEditing(null);
        }}
        editing={editing}
        canManage={canManage}
        submitting={submitting}
        onCreate={createCredential}
        onUpdate={updateCredential}
      />

      <CredentialVersionsDialog
        sandboxGroupId={sandboxGroupId}
        ownerId={ownerId}
        credential={activeVersionsCred}
        open={versionsTarget != null}
        onOpenChange={(next) => {
          if (!next) setVersionsTarget(null);
        }}
        canManage={canManage}
        restoring={busyId != null}
        onRestore={restoreCredential}
      />

      {deleteTarget ? (
        <ConfirmDialog
          open={true}
          onOpenChange={(next) => {
            if (!next) setDeleteTarget(null);
          }}
          title={t("sandboxGroups.credDeleteConfirmTitle", {
            id: deleteTarget.id,
          })}
          description={t("sandboxGroups.credDeleteConfirmDescription", {
            id: deleteTarget.id,
          })}
          confirmLabel={t("sandboxGroups.credDeleteButton")}
          phrase={sudoDeleteCredentialPhrase(deleteTarget.id)}
          pending={busyId === deleteTarget.id}
          closeOnConfirm={false}
          onConfirm={() => void handleConfirmDelete()}
        />
      ) : null}
    </>
  );
}

function RoutingBadges({
  methods,
  paths,
}: {
  methods: readonly string[];
  paths: readonly string[];
}) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-wrap gap-1">
      {methods.length === 0 ? (
        <Badge variant="outline" className="text-[11px]">
          {t("sandboxGroups.credAllMethods")}
        </Badge>
      ) : (
        methods.map((m) => (
          <Badge key={m} variant="secondary" className="font-mono text-[11px]">
            {m}
          </Badge>
        ))
      )}
      {paths.length === 0 ? (
        <Badge variant="outline" className="text-[11px]">
          {t("sandboxGroups.credAllPaths")}
        </Badge>
      ) : (
        paths.map((p) => (
          <Badge key={p} variant="outline" className="font-mono text-[11px]">
            {p}
          </Badge>
        ))
      )}
    </div>
  );
}

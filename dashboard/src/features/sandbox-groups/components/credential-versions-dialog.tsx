import { useState } from "react";
import { AlertTriangle, History, RotateCcw } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { LocalDateTime } from "@/common/components/local-time";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useSandboxGroupCredentialVersions,
  type SandboxGroupCredential,
} from "@/features/sandbox-groups/hooks/use-sandbox-group-credentials";
import { sudoRestoreCredentialPhrase } from "@/features/sandbox-groups/lib/credential-validation";
import { CredentialVersionsSkeleton } from "@/features/sandbox-groups/components/sandbox-group-skeletons";

export interface CredentialVersionsDialogProps {
  sandboxGroupId: string;
  ownerId?: string | null;
  credential: SandboxGroupCredential | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  canManage: boolean;
  restoring: boolean;
  onRestore: (
    credentialId: string,
    versionId: number,
  ) => Promise<SandboxGroupCredential | null>;
}

export function CredentialVersionsDialog({
  sandboxGroupId,
  ownerId,
  credential,
  open,
  onOpenChange,
  canManage,
  restoring,
  onRestore,
}: CredentialVersionsDialogProps) {
  const { t } = useTranslations();
  const [restoreTarget, setRestoreTarget] = useState<number | null>(null);

  const { versions, loading, error, refetch } =
    useSandboxGroupCredentialVersions(
      sandboxGroupId,
      open && credential ? credential.id : null,
      ownerId,
    );

  const currentVersionId = credential?.version.id ?? versions[0]?.id ?? null;

  const handleConfirmRestore = async () => {
    if (!credential || restoreTarget == null) return;
    const restored = await onRestore(credential.id, restoreTarget);
    if (restored) {
      setRestoreTarget(null);
      void refetch();
    }
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <History className="size-4" aria-hidden="true" />
              {t("sandboxGroups.credVersionsTitle", {
                id: credential?.id ?? "",
              })}
            </DialogTitle>
            <DialogDescription>
              {t("sandboxGroups.credVersionsDescription")}
            </DialogDescription>
          </DialogHeader>

          {loading ? (
            <CredentialVersionsSkeleton />
          ) : error && versions.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-8 text-center">
              <AlertTriangle
                className="text-muted-foreground size-6"
                aria-hidden="true"
              />
              <p className="text-muted-foreground text-sm">
                {t("sandboxGroups.credVersionsError")}
              </p>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("common.tryAgain")}
              </Button>
            </div>
          ) : versions.length === 0 ? (
            <p className="text-muted-foreground py-6 text-center text-sm">
              {t("sandboxGroups.credVersionsEmpty")}
            </p>
          ) : (
            <ul className="space-y-3">
              {versions.map((v) => {
                const isCurrent = v.id === currentVersionId;
                return (
                  <li
                    key={v.id}
                    className="flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="min-w-0 space-y-1">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <Badge variant="secondary">v{v.id}</Badge>
                        {isCurrent ? (
                          <Badge variant="default">
                            {t("sandboxGroups.credVersionCurrent")}
                          </Badge>
                        ) : null}
                        {v.restoredFromVersionId != null ? (
                          <Badge variant="outline">
                            {t("sandboxGroups.credVersionRestoredFrom", {
                              version: v.restoredFromVersionId,
                            })}
                          </Badge>
                        ) : null}
                      </div>
                      <p className="text-muted-foreground font-mono text-xs">
                        {t("sandboxGroups.credFingerprintLabel")}:{" "}
                        {v.fingerprint.slice(0, 16)}
                      </p>
                      <p className="text-muted-foreground text-xs">
                        {v.updatedAt ? (
                          <LocalDateTime value={v.updatedAt} />
                        ) : null}
                        {v.updatedBy
                          ? ` · ${t("sandboxGroups.credUpdatedBy", { actor: v.updatedBy })}`
                          : null}
                      </p>
                    </div>
                    {canManage && !isCurrent ? (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={restoring}
                        onClick={() => setRestoreTarget(v.id)}
                      >
                        <RotateCcw className="size-3.5" aria-hidden="true" />
                        {t("sandboxGroups.credRestoreAction")}
                      </Button>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}
        </DialogContent>
      </Dialog>

      {credential && restoreTarget != null ? (
        <ConfirmDialog
          open={true}
          onOpenChange={(next) => {
            if (!next) setRestoreTarget(null);
          }}
          title={t("sandboxGroups.credRestoreConfirmTitle", {
            id: credential.id,
            version: restoreTarget,
          })}
          description={t("sandboxGroups.credRestoreConfirmDescription", {
            id: credential.id,
            version: restoreTarget,
          })}
          confirmLabel={t("sandboxGroups.credRestoreAction")}
          phrase={sudoRestoreCredentialPhrase(credential.id)}
          pending={restoring}
          closeOnConfirm={false}
          onConfirm={() => void handleConfirmRestore()}
        />
      ) : null}
    </>
  );
}

import { useState } from "react";
import { Loader2, DatabaseBackup, Download } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatRelativeAge } from "@/features/services/lib/format";
import {
  useRecovery,
  type BackupItem,
} from "@/features/databases/hooks/use-recovery";

/**
 * The database detail's Recovery section (Render's Recovery tab): the PITR
 * window + backup list, an on-demand export trigger, and a restore-to-new-
 * instance flow. When the plan has no backups the panel renders a disabled
 * state rather than hiding — recovery is unavailable, not broken.
 */
export function RecoveryPanel({ id }: { id: string }) {
  const { t } = useTranslations();
  const {
    info,
    exports,
    loading,
    exporting,
    recovering,
    createExport,
    recover,
  } = useRecovery(id);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [targetTime, setTargetTime] = useState("");

  async function handleRestore() {
    // datetime-local yields "YYYY-MM-DDTHH:mm"; send RFC3339 (append seconds+Z).
    const iso = targetTime ? `${targetTime}:00Z` : undefined;
    const ok = await recover({ name: newName.trim(), targetTime: iso });
    if (ok) {
      setRestoreOpen(false);
      setNewName("");
      setTargetTime("");
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.recoveryTitle")}</CardTitle>
        <CardDescription>{t("databases.recoveryDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {loading && !info.enabled ? (
          <p className="text-sm text-muted-foreground">
            {t("databases.loading")}
          </p>
        ) : !info.enabled ? (
          <p className="text-sm text-muted-foreground">
            {t("databases.recoveryDisabled")}
          </p>
        ) : (
          <>
            <dl className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
              <Field
                label={t("databases.recoveryEarliest")}
                value={
                  info.earliestRecoveryTime
                    ? formatRelativeAge(info.earliestRecoveryTime)
                    : t("databases.recoveryNoBackupYet")
                }
              />
              <Field
                label={t("databases.recoveryLatest")}
                value={
                  info.latestRecoveryTime
                    ? formatRelativeAge(info.latestRecoveryTime)
                    : "—"
                }
              />
            </dl>

            <div className="flex flex-wrap gap-2">
              <Button onClick={() => setRestoreOpen(true)}>
                <DatabaseBackup />
                {t("databases.recoveryRestore")}
              </Button>
              <Button
                variant="outline"
                onClick={() => void createExport()}
                disabled={exporting}
              >
                {exporting ? (
                  <Loader2 className="animate-spin" />
                ) : (
                  <Download />
                )}
                {t("databases.recoveryCreateExport")}
              </Button>
            </div>

            <BackupList
              title={t("databases.recoveryBackups")}
              items={info.backups}
              emptyText={t("databases.recoveryNoBackups")}
            />
            <BackupList
              title={t("databases.recoveryExports")}
              items={exports}
              emptyText={t("databases.recoveryNoExports")}
            />
          </>
        )}
      </CardContent>

      <Dialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("databases.recoveryRestoreTitle")}</DialogTitle>
            <DialogDescription>
              {t("databases.recoveryRestoreBody")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="restore-name">
                {t("databases.recoveryRestoreName")}
              </Label>
              <Input
                id="restore-name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="my-db-restored"
                autoComplete="off"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="restore-time">
                {t("databases.recoveryRestoreTime")}
              </Label>
              <Input
                id="restore-time"
                type="datetime-local"
                value={targetTime}
                onChange={(e) => setTargetTime(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                {t("databases.recoveryRestoreTimeHint")}
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRestoreOpen(false)}>
              {t("databases.deleteCancel")}
            </Button>
            <Button
              onClick={() => void handleRestore()}
              disabled={!newName.trim() || recovering}
            >
              {recovering ? <Loader2 className="animate-spin" /> : null}
              {t("databases.recoveryRestoreConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-4 border-b pb-2">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="truncate text-sm font-medium">{value}</dd>
    </div>
  );
}

function BackupList({
  title,
  items,
  emptyText,
}: {
  title: string;
  items: BackupItem[];
  emptyText: string;
}) {
  return (
    <div className="space-y-2">
      <h4 className="text-sm font-medium">{title}</h4>
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{emptyText}</p>
      ) : (
        <ul className="space-y-1">
          {items.map((b) => (
            <li
              key={b.id}
              className="flex items-center justify-between gap-3 rounded-md border px-3 py-2"
            >
              <code className="truncate font-mono text-xs">{b.id}</code>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">
                  {formatRelativeAge(b.createdAt)}
                </span>
                <Badge variant="outline">{b.status}</Badge>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

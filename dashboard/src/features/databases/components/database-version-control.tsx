import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useUpdateDatabaseVersion } from "@/features/databases/hooks/use-update-database-version";
import { postgresUpgradeTargets } from "@/features/databases/lib/versions";
import type { DatabaseDetailView } from "@/features/databases/types";

export function DatabaseVersionControl({
  database,
  onChanged,
}: {
  database: DatabaseDetailView;
  onChanged: () => void;
}) {
  const { t } = useTranslations();
  const targets = useMemo(
    () => postgresUpgradeTargets(database.version),
    [database.version],
  );
  const [open, setOpen] = useState(false);
  const [target, setTarget] = useState(targets[0] ?? "");
  const { updateVersion, busy, error, clearError } = useUpdateDatabaseVersion();

  const selectedTarget = targets.includes(target) ? target : (targets[0] ?? "");

  const upgrading = database.status.toLowerCase() === "upgrading";

  async function submit() {
    if (!selectedTarget) return;
    const ok = await updateVersion(database.id, selectedTarget);
    if (!ok) return;
    onChanged();
    setOpen(false);
  }

  return (
    <>
      <div className="flex items-center justify-end gap-2">
        <span className="truncate">
          {database.version
            ? t("databases.versionLabel", { version: database.version })
            : "—"}
        </span>
        {upgrading ? (
          <span className="text-xs text-muted-foreground">
            {t("databases.versionUpgrading")}
          </span>
        ) : targets.length > 0 ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => {
              clearError();
              setTarget(targets[0] ?? "");
              setOpen(true);
            }}
          >
            {t("databases.versionUpgradeAction")}
          </Button>
        ) : null}
      </div>

      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!busy) setOpen(next);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("databases.versionUpgradeTitle")}</DialogTitle>
            <DialogDescription>
              {t("databases.versionUpgradeDescription", {
                version: database.version ?? "—",
              })}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <label
              className="text-sm font-medium"
              htmlFor="postgres-upgrade-version"
            >
              {t("databases.versionUpgradeTarget")}
            </label>
            <Select
              value={selectedTarget}
              onValueChange={(value) => {
                clearError();
                setTarget(value);
              }}
            >
              <SelectTrigger id="postgres-upgrade-version" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {targets.map((version) => (
                  <SelectItem key={version} value={version}>
                    {t("databases.versionLabel", { version })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-sm text-muted-foreground">
              {t("databases.versionUpgradeDowntime")}
            </p>
          </div>

          {error ? (
            <Alert variant="destructive">
              <AlertTitle>{t("databases.versionUpgradeErrorTitle")}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => setOpen(false)}
            >
              {t("databases.versionUpgradeCancel")}
            </Button>
            <Button
              type="button"
              disabled={busy || !selectedTarget}
              onClick={() => void submit()}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("databases.versionUpgradeConfirm", {
                version: selectedTarget,
              })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

import { useState } from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
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
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useSetEnvironmentACL } from "@/features/environments/hooks/use-set-environment-acl";

export interface EnvironmentSettingsDialogProps {
  environment: EnvironmentView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Render-shaped Environment permissions, isolation, and inbound-IP settings. */
export function EnvironmentSettingsDialog({
  environment,
  open,
  onOpenChange,
}: EnvironmentSettingsDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <EnvironmentSettingsForm
          key={[
            environment.protectedStatus,
            environment.networkIsolationEnabled,
            ...environment.ipAllowList,
          ].join(":")}
          environment={environment}
          onClose={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function EnvironmentSettingsForm({
  environment,
  onClose,
}: {
  environment: EnvironmentView;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const { saveACL, saving } = useSetEnvironmentACL();
  const [isProtected, setIsProtected] = useState(
    environment.protectedStatus === "protected",
  );
  const [networkIsolationEnabled, setNetworkIsolationEnabled] = useState(
    environment.networkIsolationEnabled,
  );
  const [ipAllowList, setIPAllowList] = useState(environment.ipAllowList);
  const [entry, setEntry] = useState("");

  const dirty =
    isProtected !== (environment.protectedStatus === "protected") ||
    networkIsolationEnabled !== environment.networkIsolationEnabled ||
    ipAllowList.length !== environment.ipAllowList.length ||
    ipAllowList.some((cidr, index) => cidr !== environment.ipAllowList[index]);

  function addCIDR() {
    const cidr = entry.trim();
    if (cidr && !ipAllowList.includes(cidr)) {
      setIPAllowList([...ipAllowList, cidr]);
    }
    setEntry("");
  }

  async function save() {
    const ok = await saveACL(environment.id, environment.name, {
      protectedStatus: isProtected ? "protected" : "unprotected",
      networkIsolationEnabled,
      ipAllowList,
    });
    if (ok) onClose();
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {t("environments.settingsTitle", { name: environment.name })}
        </DialogTitle>
        <DialogDescription>
          {t("environments.settingsDescription")}
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-5">
        <section className="flex items-center justify-between gap-4 rounded-md border p-3">
          <div className="space-y-0.5">
            <Label htmlFor={`environment-protected-${environment.id}`}>
              {t("environments.protectedLabel")}
            </Label>
            <p className="text-sm text-muted-foreground">
              {t("environments.protectedHint")}
            </p>
          </div>
          <Switch
            id={`environment-protected-${environment.id}`}
            checked={isProtected}
            onCheckedChange={setIsProtected}
            disabled={saving}
          />
        </section>

        <section className="flex items-center justify-between gap-4 rounded-md border p-3">
          <div className="space-y-0.5">
            <Label htmlFor={`environment-isolation-${environment.id}`}>
              {t("environments.isolationLabel")}
            </Label>
            <p className="text-sm text-muted-foreground">
              {t("environments.isolationHint")}
            </p>
          </div>
          <Switch
            id={`environment-isolation-${environment.id}`}
            checked={networkIsolationEnabled}
            onCheckedChange={setNetworkIsolationEnabled}
            disabled={saving}
          />
        </section>

        <section className="space-y-2">
          <Label>{t("environments.ipAllowListLabel")}</Label>
          <p className="text-sm text-muted-foreground">
            {t("environments.ipAllowListHint")}
          </p>
          <div className="flex flex-wrap gap-2">
            {ipAllowList.length === 0 ? (
              <span className="text-sm text-muted-foreground">
                {t("environments.ipAllowListOpen")}
              </span>
            ) : (
              ipAllowList.map((cidr) => (
                <span
                  key={cidr}
                  className="inline-flex items-center gap-1 rounded-md border bg-muted px-2 py-1 text-xs"
                >
                  <code className="font-mono">{cidr}</code>
                  <button
                    type="button"
                    aria-label={t("environments.ipAllowListRemove", { cidr })}
                    onClick={() =>
                      setIPAllowList(
                        ipAllowList.filter((item) => item !== cidr),
                      )
                    }
                    className="text-muted-foreground hover:text-foreground"
                    disabled={saving}
                  >
                    <Trash2 className="size-3" />
                  </button>
                </span>
              ))
            )}
          </div>
          <div className="flex gap-2">
            <Input
              value={entry}
              onChange={(event) => setEntry(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  addCIDR();
                }
              }}
              placeholder="203.0.113.0/24"
              className="max-w-xs"
              disabled={saving}
            />
            <Button
              variant="outline"
              size="sm"
              onClick={addCIDR}
              disabled={saving || !entry.trim()}
            >
              <Plus />
              {t("environments.ipAllowListAdd")}
            </Button>
          </div>
        </section>
      </div>

      <DialogFooter>
        <Button variant="outline" onClick={onClose} disabled={saving}>
          {t("environments.cancel")}
        </Button>
        <Button onClick={() => void save()} disabled={saving || !dirty}>
          {saving ? <Loader2 className="animate-spin" /> : null}
          {t("environments.settingsSave")}
        </Button>
      </DialogFooter>
    </>
  );
}

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
          key={JSON.stringify([
            environment.protectedStatus,
            environment.networkIsolationEnabled,
            environment.ipAllowListEntries,
          ])}
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
  const [ipAllowListEntries, setIPAllowListEntries] = useState(
    environment.ipAllowListEntries,
  );
  const [cidrBlock, setCIDRBlock] = useState("");
  const [description, setDescription] = useState("");

  const originalEntries = JSON.stringify(environment.ipAllowListEntries);
  const draftEntries = JSON.stringify(ipAllowListEntries);

  const dirty =
    isProtected !== (environment.protectedStatus === "protected") ||
    networkIsolationEnabled !== environment.networkIsolationEnabled ||
    draftEntries !== originalEntries;

  function addCIDR() {
    const cidr = cidrBlock.trim();
    if (cidr && !ipAllowListEntries.some((entry) => entry.cidrBlock === cidr)) {
      setIPAllowListEntries([
        ...ipAllowListEntries,
        { cidrBlock: cidr, description: description.trim() },
      ]);
    }
    setCIDRBlock("");
    setDescription("");
  }

  function updateEntry(
    index: number,
    field: "cidrBlock" | "description",
    value: string,
  ) {
    setIPAllowListEntries((current) =>
      current.map((entry, entryIndex) =>
        entryIndex === index ? { ...entry, [field]: value } : entry,
      ),
    );
  }

  function removeEntry(index: number) {
    setIPAllowListEntries((current) =>
      current.filter((_, entryIndex) => entryIndex !== index),
    );
  }

  async function save() {
    const ok = await saveACL(environment.id, environment.name, {
      protectedStatus: isProtected ? "protected" : "unprotected",
      networkIsolationEnabled,
      ipAllowListEntries: ipAllowListEntries.map((entry) => ({
        cidrBlock: entry.cidrBlock.trim(),
        description: entry.description.trim(),
      })),
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
          <div className="space-y-2">
            {ipAllowListEntries.length === 0 ? (
              <span className="text-sm text-muted-foreground">
                {t("environments.ipAllowListOpen")}
              </span>
            ) : (
              ipAllowListEntries.map((entry, index) => (
                <div
                  key={index}
                  className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]"
                >
                  <Input
                    value={entry.cidrBlock}
                    onChange={(event) =>
                      updateEntry(index, "cidrBlock", event.target.value)
                    }
                    aria-label={t("environments.ipAllowListRuleCIDR", {
                      number: index + 1,
                    })}
                    placeholder="203.0.113.0/24"
                    className="font-mono"
                    disabled={saving}
                  />
                  <Input
                    value={entry.description}
                    onChange={(event) =>
                      updateEntry(index, "description", event.target.value)
                    }
                    aria-label={t("environments.ipAllowListRuleDescription", {
                      number: index + 1,
                    })}
                    placeholder={t(
                      "environments.ipAllowListDescriptionPlaceholder",
                    )}
                    disabled={saving}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={t("environments.ipAllowListRemove", {
                      cidr: entry.cidrBlock,
                    })}
                    onClick={() => removeEntry(index)}
                    disabled={saving}
                  >
                    <Trash2 />
                  </Button>
                </div>
              ))
            )}
          </div>
          <div className="flex flex-wrap gap-2">
            <Input
              value={cidrBlock}
              onChange={(event) => setCIDRBlock(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  addCIDR();
                }
              }}
              aria-label={t("environments.ipAllowListNewCIDR")}
              placeholder="203.0.113.0/24"
              className="max-w-xs"
              disabled={saving}
            />
            <Input
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  addCIDR();
                }
              }}
              aria-label={t("environments.ipAllowListNewDescription")}
              placeholder={t("environments.ipAllowListDescriptionPlaceholder")}
              className="max-w-xs"
              disabled={saving}
            />
            <Button
              variant="outline"
              size="sm"
              onClick={addCIDR}
              disabled={saving || !cidrBlock.trim()}
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

import { useMemo, useState } from "react";
import { Loader2, Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useCreateDatabase } from "@/features/databases/hooks/use-create-database";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";

// PostgreSQL major versions bex offers. Keep in sync with the Database CRD's
// authoritative enum (lego/types/v1alpha1/database_types.go, spec.version
// `+kubebuilder:validation:Enum="13";…;"18"`) — the apiserver validates against
// it. "default" is a sentinel (Radix Select forbids an empty-string value),
// mapped back to "" so the operator/CNPG picks its own default image rather than
// pinning a tag that may not be pulled.
const VERSION_DEFAULT = "default";
const VERSIONS = ["18", "17", "16", "15", "14", "13"] as const;

export interface CreateDatabaseDialogProps {
  /** Called with the new database id once creation is accepted. */
  onCreated: (id: string) => void;
}

/**
 * Render's "New Postgres" create form, bex's subset: name, plan (from the shared
 * tiers catalog — never a hardcoded ladder), version, disk size, and the public
 * (external endpoint) toggle. The deferred axes (region, HA, backups) are
 * omitted, not faked. Self-contained: owns its trigger button and resets on
 * close.
 */
export function CreateDatabaseDialog({ onCreated }: CreateDatabaseDialogProps) {
  const { t } = useTranslations();
  const { instanceTypes } = useDatabaseInstanceTypes();
  const { create, busy } = useCreateDatabase();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  // Only an explicit user pick is stored; the effective plan defaults to the
  // catalog's first (default) tier, so no effect is needed to seed it once the
  // async catalog loads (React: "you might not need an effect").
  const [planOverride, setPlanOverride] = useState<string | null>(null);
  const [version, setVersion] = useState<string>(VERSION_DEFAULT);
  const [diskSizeGB, setDiskSizeGB] = useState<string>("");
  const [isPublic, setIsPublic] = useState(false);

  const plan = planOverride ?? instanceTypes[0]?.id ?? "";
  const selectedPlan = useMemo(
    () => instanceTypes.find((it) => it.id === plan),
    [instanceTypes, plan],
  );

  const nameValid = isValidDnsLabel(name);
  const showNameError = name.length > 0 && !nameValid;
  const canSubmit = nameValid && plan !== "" && !busy;

  function reset() {
    setName("");
    setPlanOverride(null);
    setVersion(VERSION_DEFAULT);
    setDiskSizeGB("");
    setIsPublic(false);
  }

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (!next) reset();
  }

  async function handleSubmit() {
    if (!canSubmit) return;
    const id = await create({
      name,
      plan,
      version: version === VERSION_DEFAULT ? "" : version,
      diskSizeGB: Number(diskSizeGB) || 0,
      public: isPublic,
    });
    if (id) {
      handleOpenChange(false);
      onCreated(id);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button>
          <Plus />
          {t("databases.createButton")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("databases.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("databases.createDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="db-name">{t("databases.fieldName")}</Label>
            <Input
              id="db-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("databases.fieldNamePlaceholder")}
              autoComplete="off"
              aria-invalid={showNameError}
            />
            {showNameError ? (
              <p className="text-sm text-destructive">
                {t("databases.fieldNameError")}
              </p>
            ) : null}
          </div>

          <div className="space-y-2">
            <Label htmlFor="db-plan">{t("databases.fieldPlan")}</Label>
            <Select value={plan} onValueChange={setPlanOverride}>
              <SelectTrigger id="db-plan" className="w-full">
                <SelectValue
                  placeholder={t("databases.fieldPlanPlaceholder")}
                />
              </SelectTrigger>
              <SelectContent>
                {instanceTypes.map((it) => (
                  <SelectItem key={it.id} value={it.id}>
                    {it.name} — {formatInstanceMemory(it.memory)} RAM,{" "}
                    {formatInstanceCPU(it.cpu)}, {it.storageGB} GB
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="db-version">{t("databases.fieldVersion")}</Label>
              <Select value={version} onValueChange={setVersion}>
                <SelectTrigger id="db-version" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={VERSION_DEFAULT}>
                    {t("databases.fieldVersionDefault")}
                  </SelectItem>
                  {VERSIONS.map((v) => (
                    <SelectItem key={v} value={v}>
                      PostgreSQL {v}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="db-disk">{t("databases.fieldDisk")}</Label>
              <Input
                id="db-disk"
                type="number"
                min={1}
                value={diskSizeGB}
                onChange={(e) => setDiskSizeGB(e.target.value)}
                placeholder={
                  selectedPlan ? String(selectedPlan.storageGB) : undefined
                }
              />
            </div>
          </div>

          <div className="flex items-center justify-between rounded-md border p-3">
            <div className="space-y-0.5 pr-4">
              <Label htmlFor="db-public">{t("databases.fieldPublic")}</Label>
              <p className="text-sm text-muted-foreground">
                {t("databases.fieldPublicHint")}
              </p>
            </div>
            <Switch
              id="db-public"
              checked={isPublic}
              onCheckedChange={setIsPublic}
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={busy}
          >
            {t("databases.createCancel")}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("databases.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

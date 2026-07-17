import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { ArrowUpRight, Loader2 } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { isValidDnsLabel } from "@/common/lib/utils/dns-label";
import type { en } from "@/i18n";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { useCreateKeyValue } from "@/features/keyvalue/hooks/use-create-key-value";
import { KeyValuePlanPicker } from "@/features/keyvalue/components/key-value-plan-picker";
import { ProjectEnvironmentSelector } from "@/features/environments/components/project-environment-selector";

// Valkey major versions bex offers, matching the KeyValue CRD's authoritative
// enum (lego/types/v1alpha1/keyvalue_types.go, spec.version
// `+kubebuilder:validation:Enum="7";"8"`). "default" is a sentinel (Radix
// Select forbids an empty-string value), mapped back to "" so the operator
// picks its own default rather than pinning a version that may not be pulled.
const VERSION_DEFAULT = "default";
const VERSIONS = ["8", "7"] as const;

// Maxmemory (eviction) policies + persistence modes, matching the KeyValue CRD's
// enums (lego/types/v1alpha1/keyvalue_types.go) and Render's Key Value create
// form (docs/render-artifacts/key-value.md). Policy identifiers are technical
// tokens (shown as-is, like the metrics quantiles); persistence modes get i18n
// labels. `allkeys-lru` leads — Render's cache-oriented default. The Free plan
// has no persistence, so both are locked (persistence forced Off) when selected.
const MAXMEMORY_POLICIES = [
  "allkeys-lru",
  "allkeys-lfu",
  "volatile-lru",
  "volatile-lfu",
  "allkeys-random",
  "volatile-random",
  "volatile-ttl",
  "noeviction",
] as const;
const PERSISTENCE_MODES = ["journal-snapshot", "snapshot", "off"] as const;
const PERSISTENCE_LABEL_KEYS: Record<
  (typeof PERSISTENCE_MODES)[number],
  keyof typeof en
> = {
  "journal-snapshot": "keyvalue.persistenceJournalSnapshot",
  snapshot: "keyvalue.persistenceSnapshot",
  off: "keyvalue.persistenceOff",
};
const FREE_PLAN = "free";

export const Route = createFileRoute("/keyvalue/new")({
  component: NewKeyValuePage,
  beforeLoad: requireAuth(),
  head: () => ({
    meta: [{ title: "New Key Value · bex dashboard" }],
  }),
});

/**
 * Render's "New Key Value" form (`/new/redis`, docs/render-artifacts/key-value.md):
 * a full page, not a dialog — bex's subset covers name, plan (tier cards from
 * the shared tiers catalog), version, the public (external endpoint) toggle, and
 * the Maxmemory Policy / Persistence Mode settings (w5/011, now backed by the
 * KeyValue CR). Project/Environment assignment uses the shared create selector;
 * region remains omitted because it is not caller-selectable in bex's contract.
 */
export function NewKeyValuePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { instanceTypes } = useKeyValueInstanceTypes();
  const { create, busy, capLimit } = useCreateKeyValue();

  const [name, setName] = useState("");
  const [planOverride, setPlanOverride] = useState<string | null>(null);
  const [version, setVersion] = useState<string>(VERSION_DEFAULT);
  const [isPublic, setIsPublic] = useState(false);
  const [maxmemoryPolicy, setMaxmemoryPolicy] = useState<string>(
    MAXMEMORY_POLICIES[0],
  );
  const [persistenceMode, setPersistenceMode] = useState<string>(
    PERSISTENCE_MODES[0],
  );
  const [projectId, setProjectId] = useState<string | null>(null);
  const [environmentId, setEnvironmentId] = useState<string | null>(null);

  const plan = planOverride ?? instanceTypes[0]?.id ?? "";
  // The Free plan has no persistent disk, so persistence is forced Off and both
  // settings lock — Render's exact behavior (docs/render-artifacts/key-value.md).
  const isFree = plan === FREE_PLAN;
  const effectivePersistence = isFree ? "off" : persistenceMode;

  const nameValid = isValidDnsLabel(name);
  const showNameError = name.length > 0 && !nameValid;
  const canSubmit = nameValid && plan !== "" && !busy;

  async function handleSubmit() {
    if (!canSubmit) return;
    const id = await create({
      name,
      plan,
      version: version === VERSION_DEFAULT ? "" : version,
      public: isPublic,
      maxmemoryPolicy,
      persistenceMode: effectivePersistence,
      environmentId: environmentId ?? undefined,
    });
    if (id) {
      void navigate({
        to: "/keyvalue/$keyValueId",
        params: { keyValueId: id },
      });
    }
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>
                <h1 className="text-xl font-semibold">
                  {t("keyvalue.createTitle")}
                </h1>
              </CardTitle>
              <CardDescription>
                {t("keyvalue.createDescription")}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="kv-name">{t("keyvalue.fieldName")}</Label>
                <Input
                  id="kv-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("keyvalue.fieldNamePlaceholder")}
                  autoComplete="off"
                  aria-invalid={showNameError}
                />
                {showNameError ? (
                  <p className="text-sm text-destructive">
                    {t("keyvalue.fieldNameError")}
                  </p>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label>{t("keyvalue.fieldPlan")}</Label>
                <KeyValuePlanPicker
                  instanceTypes={instanceTypes}
                  value={plan}
                  onChange={setPlanOverride}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="kv-version">{t("keyvalue.fieldVersion")}</Label>
                <Select value={version} onValueChange={setVersion}>
                  <SelectTrigger id="kv-version" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={VERSION_DEFAULT}>
                      {t("keyvalue.fieldVersionDefault")}
                    </SelectItem>
                    {VERSIONS.map((v) => (
                      <SelectItem key={v} value={v}>
                        Valkey {v}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="kv-maxmemory">
                  {t("keyvalue.fieldMaxmemoryPolicy")}
                </Label>
                <Select
                  value={maxmemoryPolicy}
                  onValueChange={setMaxmemoryPolicy}
                  disabled={isFree}
                >
                  <SelectTrigger id="kv-maxmemory" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {MAXMEMORY_POLICIES.map((p) => (
                      <SelectItem key={p} value={p}>
                        {p}
                        {p === "allkeys-lru"
                          ? ` ${t("keyvalue.fieldMaxmemoryRecommended")}`
                          : ""}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-sm text-muted-foreground">
                  {t("keyvalue.fieldMaxmemoryPolicyHint")}
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="kv-persistence">
                  {t("keyvalue.fieldPersistenceMode")}
                </Label>
                <Select
                  value={effectivePersistence}
                  onValueChange={setPersistenceMode}
                  disabled={isFree}
                >
                  <SelectTrigger id="kv-persistence" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PERSISTENCE_MODES.map((m) => (
                      <SelectItem key={m} value={m}>
                        {t(PERSISTENCE_LABEL_KEYS[m])}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-sm text-muted-foreground">
                  {isFree
                    ? t("keyvalue.fieldPersistenceFreeHint")
                    : t("keyvalue.fieldPersistenceModeHint")}
                </p>
              </div>

              <ProjectEnvironmentSelector
                projectId={projectId}
                environmentId={environmentId}
                onProjectChange={setProjectId}
                onEnvironmentChange={setEnvironmentId}
              />

              <div className="flex items-center justify-between rounded-md border p-3">
                <div className="space-y-0.5 pr-4">
                  <Label htmlFor="kv-public">{t("keyvalue.fieldPublic")}</Label>
                  <p className="text-sm text-muted-foreground">
                    {t("keyvalue.fieldPublicHint")}
                  </p>
                </div>
                <Switch
                  id="kv-public"
                  checked={isPublic}
                  onCheckedChange={setIsPublic}
                />
              </div>

              {capLimit ? (
                <Alert variant="destructive">
                  <AlertTitle>{t("keyvalue.capLimitTitle")}</AlertTitle>
                  <AlertDescription className="flex flex-col gap-2">
                    <span>{capLimit}</span>
                    <Button
                      variant="outline"
                      size="sm"
                      className="self-start"
                      onClick={() =>
                        void navigate({
                          to: "/workspace/settings",
                          search: { plan: "change" },
                        })
                      }
                    >
                      <ArrowUpRight className="size-3.5" />
                      {t("keyvalue.capLimitUpgrade")}
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : null}

              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={() => void navigate({ to: "/" })}
                  disabled={busy}
                >
                  {t("keyvalue.createCancel")}
                </Button>
                <Button
                  onClick={() => void handleSubmit()}
                  disabled={!canSubmit}
                >
                  {busy ? <Loader2 className="animate-spin" /> : null}
                  {t("keyvalue.createSubmit")}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  );
}

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
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { useCreateKeyValue } from "@/features/keyvalue/hooks/use-create-key-value";
import { KeyValuePlanPicker } from "@/features/keyvalue/components/key-value-plan-picker";

// Valkey major versions bex offers, matching the KeyValue CRD's authoritative
// enum (lego/types/v1alpha1/keyvalue_types.go, spec.version
// `+kubebuilder:validation:Enum="7";"8"`). "default" is a sentinel (Radix
// Select forbids an empty-string value), mapped back to "" so the operator
// picks its own default rather than pinning a version that may not be pulled.
const VERSION_DEFAULT = "default";
const VERSIONS = ["8", "7"] as const;

export const Route = createFileRoute("/keyvalue/new")({
  component: NewKeyValuePage,
  beforeLoad: requireAuth("/keyvalue/new"),
  head: () => ({
    meta: [{ title: "New Key Value · bex dashboard" }],
  }),
});

/**
 * Render's "New Key Value" form (`/new/redis`, docs/render-artifacts/key-value.md):
 * a full page, not a dialog — bex's subset covers name, plan (tier cards from
 * the shared tiers catalog), version, and the public (external endpoint)
 * toggle. Render's captured maxmemoryPolicy/persistenceMode/project/region axes
 * are omitted, not faked (bex's createKeyValue doesn't accept them).
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

  const plan = planOverride ?? instanceTypes[0]?.id ?? "";

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
    });
    if (id) {
      void navigate({ to: "/keyvalue/$keyValueId", params: { keyValueId: id } });
    }
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-2xl space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("keyvalue.createTitle")}</CardTitle>
              <CardDescription>{t("keyvalue.createDescription")}</CardDescription>
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
                  onClick={() => void navigate({ to: "/keyvalue" })}
                  disabled={busy}
                >
                  {t("keyvalue.createCancel")}
                </Button>
                <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
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

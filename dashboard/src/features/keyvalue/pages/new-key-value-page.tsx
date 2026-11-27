import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import { SelectItem } from "@/common/components/ui/select";
import { Switch } from "@/common/components/ui/switch";
import {
  CREATE_SWITCH_CLASS,
  CreateActionBar,
  CreateCapLimitAlert,
  CreateField,
  CreateForm,
  CreateSelectField,
  CreateTextField,
  PlanSummary,
} from "@/common/components/create-form";
import { isValidLabel } from "@/common/lib/utils/resource-label";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { useCreateKeyValue } from "@/features/keyvalue/hooks/use-create-key-value";
import {
  MAXMEMORY_POLICIES,
  PERSISTENCE_LABEL_KEYS,
  PERSISTENCE_MODES,
  RECOMMENDED_MAXMEMORY_POLICY,
  recommendedSuffix,
} from "@/features/keyvalue/lib/labels";
import {
  KEY_VALUE_PLAN_SLOTS,
  PlanTierSlots,
} from "@/common/components/datastore-plan-preview";
import { PlanCatalogUnavailable } from "@/common/components/plan-catalog-unavailable";
import {
  CREATE_PLAN_ROWS,
  ComputePlanList,
} from "@/common/components/compute-plan-list";
import { ProjectEnvironmentSelector } from "@/features/environments/components/project-environment-selector";

// The persistence modes and maxmemory (eviction) policies live in the
// feature's shared lib so the create page and the detail-page editors agree.
// Every bex plan — the Free tier included (tiers.yaml gives it a 1 GB disk) —
// can persist, so neither dropdown is plan-locked (w6/m127); the default is
// journal-snapshot, matching the API create path.

/**
 * Render's "New Key Value" form (`/new/redis`, docs/render-artifacts/key-value.md):
 * a full page, not a dialog — bex's subset covers name, plan (tier cards from
 * the shared tiers catalog), the public (external endpoint) toggle, and the
 * Maxmemory Policy / Persistence Mode settings (w5/011, now backed by the
 * KeyValue CR). Project/Environment assignment uses the shared create selector;
 * region remains omitted because it is not caller-selectable in bex's contract.
 * There is no version field, as Render's form has none: a new instance always
 * runs Valkey 8 (w4/743).
 */
export function NewKeyValuePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const {
    instanceTypes,
    loading: catalogLoading,
    retry: retryCatalog,
    retrying: catalogRetrying,
  } = useKeyValueInstanceTypes();
  const { create, busy, capLimit, capLimitUpgrade } = useCreateKeyValue();

  const [name, setName] = useState("");
  const [planOverride, setPlanOverride] = useState<string | null>(null);
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
  const selectedPlan = instanceTypes.find((it) => it.id === plan);

  // A free-form label (w5/m153), sent trimmed.
  const nameValid = isValidLabel(name);
  const showNameError = name.length > 0 && !nameValid;
  const canSubmit = nameValid && plan !== "" && !busy;

  async function handleSubmit() {
    if (!canSubmit) return;
    const id = await create({
      name: name.trim(),
      plan,
      public: isPublic,
      maxmemoryPolicy,
      persistenceMode,
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
      <PageFrame
        width="standard"
        header={
          <PageHeader
            title={t("keyvalue.createTitle")}
            description={t("keyvalue.createDescription")}
          />
        }
        footer={
          <CreateActionBar
            submitLabel={t("keyvalue.createSubmit")}
            onSubmit={() => void handleSubmit()}
            canSubmit={canSubmit}
            busy={busy}
            cancelLabel={t("keyvalue.createCancel")}
            onCancel={() => void navigate({ to: "/" })}
            summary={
              <PlanSummary plan={selectedPlan} loading={catalogLoading} />
            }
            blocked={
              !nameValid
                ? { reason: t("common.createBlockedName"), fieldId: "kv-name" }
                : selectedPlan == null && catalogLoading
                  ? { reason: t("common.createBlockedLoadingPlans") }
                  : null
            }
          />
        }
      >
        <CreateForm>
          <CreateTextField
            id="kv-name"
            label={t("keyvalue.fieldName")}
            description={t("keyvalue.fieldNameDescription")}
            value={name}
            onChange={setName}
            placeholder={t("keyvalue.fieldNamePlaceholder")}
            error={showNameError ? t("keyvalue.fieldNameError") : undefined}
          />

          <ProjectEnvironmentSelector
            projectId={projectId}
            environmentId={environmentId}
            onProjectChange={setProjectId}
            onEnvironmentChange={setEnvironmentId}
          />

          <CreateSelectField
            id="kv-maxmemory"
            label={t("keyvalue.fieldMaxmemoryPolicy")}
            description={t("keyvalue.fieldMaxmemoryPolicyHint")}
            value={maxmemoryPolicy}
            onValueChange={setMaxmemoryPolicy}
          >
            {MAXMEMORY_POLICIES.map((p) => (
              <SelectItem key={p} value={p}>
                {p}
                {p === RECOMMENDED_MAXMEMORY_POLICY
                  ? recommendedSuffix(t("keyvalue.fieldMaxmemoryRecommended"))
                  : ""}
              </SelectItem>
            ))}
          </CreateSelectField>

          <CreateSelectField
            id="kv-persistence"
            label={t("keyvalue.fieldPersistenceMode")}
            description={t("keyvalue.fieldPersistenceModeHint")}
            value={persistenceMode}
            onValueChange={setPersistenceMode}
          >
            {PERSISTENCE_MODES.map((m) => (
              <SelectItem key={m} value={m}>
                {t(PERSISTENCE_LABEL_KEYS[m])}
              </SelectItem>
            ))}
          </CreateSelectField>

          <CreateField
            label={t("keyvalue.fieldPlan")}
            description={t("keyvalue.fieldPlanDescription")}
          >
            {/* The ready list's inert rows, one per catalog plan with its
                connection-limit column, while the catalog first loads, so
                nothing below jumps (w4/330, w6/m459); a settled failed or
                empty catalog says so and retries in place (w4/m218). */}
            {catalogLoading && instanceTypes.length === 0 ? (
              <PlanTierSlots
                count={KEY_VALUE_PLAN_SLOTS}
                connections
                collapsedCount={CREATE_PLAN_ROWS}
              />
            ) : instanceTypes.length === 0 ? (
              <PlanCatalogUnavailable
                onRetry={() => void retryCatalog()}
                retrying={catalogRetrying}
              />
            ) : (
              <ComputePlanList
                instanceTypes={instanceTypes}
                value={plan}
                ariaLabel={t("keyvalue.fieldPlan")}
                onChange={setPlanOverride}
                collapsedCount={CREATE_PLAN_ROWS}
              />
            )}
          </CreateField>

          <CreateField
            label={t("keyvalue.fieldPublic")}
            htmlFor="kv-public"
            description={t("keyvalue.fieldPublicHint")}
          >
            <Switch
              className={CREATE_SWITCH_CLASS}
              id="kv-public"
              checked={isPublic}
              onCheckedChange={setIsPublic}
            />
          </CreateField>

          {capLimit ? (
            <CreateCapLimitAlert
              title={t("keyvalue.capLimitTitle")}
              message={capLimit}
              upgradeLabel={t("keyvalue.capLimitUpgrade")}
              showUpgrade={capLimitUpgrade}
            />
          ) : null}
        </CreateForm>
      </PageFrame>
    </DashboardLayout>
  );
}

import { useId, useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Info } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { useTranslations } from "@/common/hooks/use-translations";
import { SelectItem } from "@/common/components/ui/select";
import { Input } from "@/common/components/ui/input";
import { Switch } from "@/common/components/ui/switch";
import { cn } from "@/common/lib/utils/utils";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  CREATE_INPUT_CLASS,
  CREATE_SWITCH_CLASS,
  CreateActionBar,
  CreateCapLimitAlert,
  CreateField,
  CreateForm,
  CreateSelectField,
  CreateTextField,
  PlanSummary,
} from "@/common/components/create-form";
import {
  CREATE_PLAN_ROWS,
  ComputePlanList,
} from "@/common/components/compute-plan-list";
import {
  POSTGRES_PLAN_SLOTS,
  PlanTierSlots,
} from "@/common/components/datastore-plan-preview";
import { PlanCatalogUnavailable } from "@/common/components/plan-catalog-unavailable";
import { isValidLabel } from "@/common/lib/utils/resource-label";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useCreateDatabase } from "@/features/databases/hooks/use-create-database";
import { POSTGRES_VERSIONS } from "@/features/databases/lib/versions";
import {
  isReservedPostgresDatabaseName,
  isReservedPostgresRole,
  isValidPostgresIdentifier,
} from "@/features/databases/lib/identifiers";
import { planStorageFixed } from "@/features/databases/lib/storage";
import { ProjectEnvironmentSelector } from "@/features/environments/components/project-environment-selector";

// PostgreSQL major versions bex offers. Keep in sync with the Database CRD's
// authoritative enum (lego/types/v1alpha1/database_types.go, spec.version
// `+kubebuilder:validation:Enum="13";…;"18"`) — the apiserver validates against
// it. "default" is a sentinel (Radix Select forbids an empty-string value),
// mapped back to "" so the operator/CNPG picks its own default image rather than
// pinning a tag that may not be pulled.
const VERSION_DEFAULT = "default";

/**
 * Render's "New Postgres" form, bex's subset: name, project, the optional
 * physical database and user, version, plan (from the shared tiers catalog —
 * never a hardcoded ladder), storage, high availability, offered on the plans
 * the catalog marks as supporting it (w2/m171), and the public (external
 * endpoint) toggle. The deferred axes (region, Datadog, storage autoscaling at
 * create) are omitted, not faked.
 */
export function NewDatabasePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const {
    instanceTypes,
    loading: catalogLoading,
    retry: retryCatalog,
    retrying: catalogRetrying,
  } = useDatabaseInstanceTypes();
  const { create, busy, capLimit, capLimitUpgrade } = useCreateDatabase();

  const [name, setName] = useState("");
  const [databaseName, setDatabaseName] = useState("");
  const [databaseUser, setDatabaseUser] = useState("");
  // Only an explicit user pick is stored; the effective plan defaults to the
  // catalog's first (default) tier, so no effect is needed to seed it once the
  // async catalog loads (React: "you might not need an effect").
  const [planOverride, setPlanOverride] = useState<string | null>(null);
  const [version, setVersion] = useState<string>(VERSION_DEFAULT);
  const [diskSizeGB, setDiskSizeGB] = useState<string>("");
  // A new database accepts connections from any address until restricted, as
  // on Render (w4/553); switching this off creates it private.
  const [isPublic, setIsPublic] = useState(true);
  const [highAvailability, setHighAvailability] = useState(false);
  const [projectId, setProjectId] = useState<string | null>(null);
  const [environmentId, setEnvironmentId] = useState<string | null>(null);
  const diskDescriptionId = useId();
  const haDescriptionId = useId();
  const haNoteId = useId();
  const publicDescriptionId = useId();

  const plan = planOverride ?? instanceTypes[0]?.id ?? "";
  const selectedPlan = useMemo(
    () => instanceTypes.find((it) => it.id === plan),
    [instanceTypes, plan],
  );
  // HA is offered only where the catalog says the plan supports it (bex-api's
  // rule, Render's "at least 1 CPU"). The switch keeps the user's choice, but
  // a plan without HA shows it off and disabled and never sends it.
  const haSupported = selectedPlan?.supportsHighAvailability === true;
  const enableHighAvailability = highAvailability && haSupported;

  // A free-form label (w5/m153), sent trimmed.
  const nameValid = isValidLabel(name);
  const showNameError = name.length > 0 && !nameValid;
  const databaseNameReserved = isReservedPostgresDatabaseName(databaseName);
  const databaseUserReserved = isReservedPostgresRole(databaseUser);
  const databaseNameValid =
    databaseName === "" ||
    (isValidPostgresIdentifier(databaseName) && !databaseNameReserved);
  const databaseUserValid =
    databaseUser === "" ||
    (isValidPostgresIdentifier(databaseUser) && !databaseUserReserved);
  const diskFloor = selectedPlan?.storageGB ?? 1;
  const diskCeiling = selectedPlan?.maxStorageGB;
  // A plan whose floor is its ceiling (Free: 1 GB) has a fixed disk: say so in
  // words, never validate a range of one, and let the server apply it (w4/376).
  const diskFixed = selectedPlan != null && planStorageFixed(selectedPlan);
  const diskEntered = !diskFixed && diskSizeGB.trim() !== "";
  const diskNum = Number(diskSizeGB);
  const diskValid =
    !diskEntered ||
    (Number.isInteger(diskNum) &&
      diskNum >= diskFloor &&
      diskCeiling != null &&
      diskNum <= diskCeiling);
  const showDiskError = diskEntered && !diskValid && diskCeiling != null;
  const canSubmit =
    nameValid &&
    databaseNameValid &&
    databaseUserValid &&
    diskValid &&
    selectedPlan != null &&
    diskCeiling != null &&
    !busy;

  async function handleSubmit() {
    if (!canSubmit) return;
    const id = await create({
      name: name.trim(),
      databaseName,
      databaseUser,
      plan,
      version: version === VERSION_DEFAULT ? "" : version,
      diskSizeGB: diskFixed ? 0 : Number(diskSizeGB) || 0,
      public: isPublic,
      highAvailability: enableHighAvailability,
      environmentId: environmentId ?? undefined,
    });
    if (id) {
      void navigate({
        to: "/databases/$databaseId",
        params: { databaseId: id },
      });
    }
  }

  return (
    <DashboardLayout>
      <PageFrame
        width="standard"
        header={
          <PageHeader
            title={t("databases.createTitle")}
            description={t("databases.createDescription")}
          />
        }
        footer={
          <CreateActionBar
            submitLabel={t("databases.createSubmit")}
            onSubmit={() => void handleSubmit()}
            canSubmit={canSubmit}
            busy={busy}
            cancelLabel={t("databases.createCancel")}
            onCancel={() => void navigate({ to: "/" })}
            summary={
              <PlanSummary plan={selectedPlan} loading={catalogLoading} />
            }
            blocked={
              !nameValid
                ? { reason: t("common.createBlockedName"), fieldId: "db-name" }
                : !databaseNameValid || !databaseUserValid || !diskValid
                  ? { reason: t("common.createBlockedFields") }
                  : selectedPlan == null && catalogLoading
                    ? { reason: t("common.createBlockedLoadingPlans") }
                    : null
            }
          />
        }
      >
        <CreateForm>
          <CreateTextField
            id="db-name"
            label={t("databases.fieldName")}
            description={t("databases.fieldNameDescription")}
            value={name}
            onChange={setName}
            placeholder={t("databases.fieldNamePlaceholder")}
            error={showNameError ? t("databases.fieldNameError") : undefined}
          />

          <ProjectEnvironmentSelector
            projectId={projectId}
            environmentId={environmentId}
            onProjectChange={setProjectId}
            onEnvironmentChange={setEnvironmentId}
          />

          <CreateTextField
            id="db-database-name"
            label={t("databases.fieldDatabaseName")}
            optional
            description={t("databases.fieldDatabaseNameDescription")}
            value={databaseName}
            onChange={setDatabaseName}
            placeholder={t("databases.fieldDatabaseNamePlaceholder")}
            mono
            error={
              databaseNameValid
                ? undefined
                : databaseNameReserved
                  ? t("databases.fieldPhysicalNameReserved", {
                      name: databaseName,
                    })
                  : t("databases.fieldPhysicalNameError")
            }
          />

          <CreateTextField
            id="db-database-user"
            label={t("databases.fieldDatabaseUser")}
            optional
            description={t("databases.fieldDatabaseUserDescription")}
            value={databaseUser}
            onChange={setDatabaseUser}
            placeholder={t("databases.fieldDatabaseUserPlaceholder")}
            mono
            error={
              databaseUserValid
                ? undefined
                : databaseUserReserved
                  ? t("databases.fieldPhysicalNameReserved", {
                      name: databaseUser,
                    })
                  : t("databases.fieldPhysicalNameError")
            }
          />

          <CreateSelectField
            id="db-version"
            label={t("databases.fieldVersion")}
            description={t("databases.fieldVersionDescription")}
            value={version}
            onValueChange={setVersion}
          >
            <SelectItem value={VERSION_DEFAULT}>
              {t("databases.fieldVersionDefault")}
            </SelectItem>
            {POSTGRES_VERSIONS.map((v) => (
              <SelectItem key={v} value={v}>
                PostgreSQL {v}
              </SelectItem>
            ))}
          </CreateSelectField>

          <CreateField
            label={t("databases.fieldPlan")}
            description={t("databases.fieldPlanDescription")}
          >
            {/* The ready list's inert rows while the catalog first loads, so
                nothing below jumps; a settled failed or empty catalog says so
                and re-reads in place rather than leaving an empty picker
                (w4/m218). */}
            {catalogLoading && instanceTypes.length === 0 ? (
              <PlanTierSlots
                count={POSTGRES_PLAN_SLOTS}
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
                ariaLabel={t("databases.fieldPlan")}
                onChange={setPlanOverride}
                collapsedCount={CREATE_PLAN_ROWS}
              />
            )}
          </CreateField>

          <CreateField
            label={t("databases.fieldDisk")}
            htmlFor={diskFixed ? undefined : "db-disk"}
            description={t("databases.fieldDiskDescription")}
            descriptionId={diskDescriptionId}
          >
            {selectedPlan == null && catalogLoading ? (
              // Until the catalog says whether the plan's disk is fixed, the
              // field's own height, so nothing below moves when it does.
              <Skeleton
                aria-hidden
                className={cn(CREATE_INPUT_CLASS, "w-40 rounded-md")}
              />
            ) : diskFixed ? (
              // A fixed disk is a fact of the plan, not a field: no input
              // that looks editable but ignores typing (w4/376) — a note.
              <p className="flex min-h-11 items-start gap-2 py-2.5 text-sm sm:min-h-9 sm:py-2">
                <Info
                  aria-hidden
                  className="text-muted-foreground mt-0.5 size-4 shrink-0"
                />
                {t("databases.fieldDiskFixed", { size: diskFloor })}
              </p>
            ) : (
              <>
                <div className="flex items-center gap-2">
                  <Input
                    id="db-disk"
                    type="number"
                    inputMode="numeric"
                    min={diskFloor}
                    max={diskCeiling ?? undefined}
                    disabled={diskCeiling == null}
                    value={diskSizeGB}
                    onChange={(e) => setDiskSizeGB(e.target.value)}
                    placeholder={
                      selectedPlan ? String(selectedPlan.storageGB) : undefined
                    }
                    aria-invalid={showDiskError}
                    aria-describedby={
                      showDiskError
                        ? `${diskDescriptionId} db-disk-error`
                        : diskDescriptionId
                    }
                    className={`w-32 tabular-nums ${CREATE_INPUT_CLASS}`}
                  />
                  <span className="text-sm text-muted-foreground">
                    {t("databases.fieldDiskUnit")}
                  </span>
                </div>
                {showDiskError ? (
                  <p
                    id="db-disk-error"
                    role="alert"
                    className="text-sm text-destructive"
                  >
                    {t("databases.fieldDiskError", {
                      min: diskFloor,
                      max: diskCeiling,
                    })}
                  </p>
                ) : null}
              </>
            )}
          </CreateField>

          <CreateField
            label={t("databases.fieldHighAvailability")}
            htmlFor="db-ha"
            description={t("databases.fieldHighAvailabilityHint")}
            descriptionId={haDescriptionId}
          >
            <Switch
              className={CREATE_SWITCH_CLASS}
              id="db-ha"
              checked={enableHighAvailability}
              disabled={!haSupported}
              aria-describedby={
                !haSupported && selectedPlan != null
                  ? `${haDescriptionId} ${haNoteId}`
                  : haDescriptionId
              }
              onCheckedChange={setHighAvailability}
            />
            {selectedPlan == null && catalogLoading ? (
              // The plan decides whether a reason follows: hold its line.
              <span aria-hidden className="flex h-5 items-center">
                <Skeleton className="h-3.5 w-3/4" />
              </span>
            ) : null}
            {!haSupported && selectedPlan != null ? (
              <p id={haNoteId} className="text-sm text-muted-foreground">
                {t("databases.fieldHighAvailabilityPlanUnsupported", {
                  name: selectedPlan.name,
                })}
              </p>
            ) : null}
          </CreateField>

          <CreateField
            label={t("databases.fieldPublic")}
            htmlFor="db-public"
            description={t(
              isPublic
                ? "databases.fieldPublicHint"
                : "databases.fieldPublicHintOff",
            )}
            descriptionId={publicDescriptionId}
          >
            <Switch
              className={CREATE_SWITCH_CLASS}
              id="db-public"
              checked={isPublic}
              aria-describedby={publicDescriptionId}
              onCheckedChange={setIsPublic}
            />
          </CreateField>

          {capLimit ? (
            <CreateCapLimitAlert
              title={t("databases.capLimitTitle")}
              message={capLimit}
              upgradeLabel={t("databases.capLimitUpgrade")}
              showUpgrade={capLimitUpgrade}
            />
          ) : null}
        </CreateForm>
      </PageFrame>
    </DashboardLayout>
  );
}

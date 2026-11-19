import { useId, useMemo, useState } from "react";
import { ReadUnavailable } from "@/common/components/read-unavailable";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Switch } from "@/common/components/ui/switch";
import { useCatalogRetry } from "@/common/hooks/use-catalog-retry";
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import {
  ALL_SELECTION,
  CLEARED_SELECTION,
  effectiveLevels,
  highestGrantableLevel,
  readOnlySelection,
  withArea,
  withLevel,
} from "@/features/api-keys/lib/permissions";
import type {
  MemberRole,
  PermissionAreaView,
  PermissionLevel,
  PermissionLevelView,
  PermissionSelection,
  PermissionView,
} from "@/features/api-keys/types";

export interface ApiKeyPermissionsPickerProps {
  /** bex-api's catalog; undefined while it loads or when it failed. */
  areas: PermissionAreaView[] | undefined;
  /** The catalog failed to load: `errorMessage` and a Try again stand in for the picker. */
  error: Error | undefined;
  errorMessage: string;
  /** Re-reads the catalog (`useApiKeyPermissionCatalog`'s refetch). */
  refetch: () => Promise<unknown>;
  value: PermissionSelection;
  onChange: (next: PermissionSelection) => void;
  /** True while the create or save is in flight. */
  disabled?: boolean;
}

/**
 * Render's scoped API key permissions (w9/m496, render.com/docs/api-keys read
 * 2026-11-18), shared by the create dialog and a key's permissions editor:
 * "Select the permissions this API key can use.", the All and Read only
 * presets on the left and Clear all on the right, then one section per area
 * with a toggle. Each permission is a choice of None or one of the levels the
 * catalog offers, and the chosen level's actions are listed under it. A level
 * the viewer's role cannot grant is disabled, with the role it needs.
 *
 * Everything shown comes from bex-api's catalog; its names are Render's
 * English, as Render shows them, so they are not translated.
 */
export function ApiKeyPermissionsPicker({
  areas,
  error,
  errorMessage,
  refetch,
  value,
  onChange,
  disabled = false,
}: ApiKeyPermissionsPickerProps) {
  const { t } = useTranslations();
  const introId = useId();
  const { retry, retrying } = useCatalogRetry(refetch);
  // Areas the user switched on: their permissions stay listed even after
  // each is set back to None, so a row never vanishes under the pointer.
  const [opened, setOpened] = useState<ReadonlySet<string>>(() => new Set());

  const levels = useMemo(
    () => (areas ? effectiveLevels(value, areas) : {}),
    [value, areas],
  );

  const nothingSelected = !value.all && Object.keys(value.levels).length === 0;

  return (
    <div
      role={areas ? "group" : undefined}
      aria-labelledby={areas ? introId : undefined}
      className="space-y-3"
    >
      <p id={introId} className="text-sm font-medium">
        {t("apiKeys.permIntro")}
      </p>
      {!areas ? (
        error ? (
          <ReadUnavailable
            message={errorMessage}
            onRetry={() => void retry()}
            retrying={retrying}
          />
        ) : (
          <PickerLoading />
        )
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              onClick={() => onChange(ALL_SELECTION)}
            >
              {t("apiKeys.permPresetAll")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled}
              onClick={() => onChange(readOnlySelection(areas))}
            >
              {t("apiKeys.permPresetReadOnly")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="ml-auto"
              disabled={disabled}
              onClick={() => {
                setOpened(new Set());
                onChange(CLEARED_SELECTION);
              }}
            >
              {t("apiKeys.permPresetClear")}
            </Button>
          </div>
          {value.all ? (
            <p className="text-muted-foreground text-sm">
              {t("apiKeys.permAllHelp")}
            </p>
          ) : nothingSelected ? (
            <p className="text-muted-foreground text-sm">
              {t("apiKeys.permNoneHelp")}
            </p>
          ) : null}
          <div className="divide-y rounded-md border">
            {areas.map((area) => (
              <PermissionAreaSection
                key={area.id}
                area={area}
                levels={levels}
                opened={opened.has(area.id)}
                disabled={disabled}
                onToggle={(on) => {
                  setOpened((prev) => {
                    const next = new Set(prev);
                    if (on) next.add(area.id);
                    else next.delete(area.id);
                    return next;
                  });
                  onChange(withArea(value, areas, area, on));
                }}
                onLevel={(permissionId, level) =>
                  onChange(withLevel(value, areas, permissionId, level))
                }
              />
            ))}
          </div>
        </>
      )}
    </div>
  );
}

/** The picker's frame while the catalog loads: the presets and four areas. */
function PickerLoading() {
  const { t } = useTranslations();
  return (
    <div
      role="status"
      aria-label={t("apiKeys.permCatalogLoading")}
      className="space-y-3"
    >
      <div className="flex items-center gap-2" aria-hidden="true">
        <Skeleton className="h-8 w-12" />
        <Skeleton className="h-8 w-24" />
        <Skeleton className="ml-auto h-8 w-20" />
      </div>
      <div className="divide-y rounded-md border" aria-hidden="true">
        {[0, 1, 2, 3].map((i) => (
          <div
            key={i}
            className="flex items-center justify-between gap-3 p-3 sm:p-4"
          >
            <Skeleton className="h-5 w-28" />
            <Skeleton className="h-5 w-9 rounded-full" />
          </div>
        ))}
      </div>
    </div>
  );
}

function PermissionAreaSection({
  area,
  levels,
  opened,
  disabled,
  onToggle,
  onLevel,
}: {
  area: PermissionAreaView;
  levels: Record<string, PermissionLevel>;
  opened: boolean;
  disabled: boolean;
  onToggle: (on: boolean) => void;
  onLevel: (permissionId: string, level: PermissionLevel | null) => void;
}) {
  const { t } = useTranslations();
  const headingId = useId();
  const granted = area.permissions.some((p) => levels[p.id]);
  const grantable = area.permissions.some(
    (p) => highestGrantableLevel(p) !== null,
  );
  // An area the viewer can grant nothing in stays listed, so each of its
  // levels shows the role it needs.
  const listed = granted || opened || !grantable;

  return (
    <div
      role="group"
      aria-labelledby={headingId}
      data-area={area.id}
      className="space-y-4 p-3 sm:p-4"
    >
      <div className="flex items-center justify-between gap-3">
        <h3 id={headingId} className="text-sm font-semibold">
          {area.name}
        </h3>
        <Switch
          checked={granted}
          disabled={disabled || !grantable}
          onCheckedChange={onToggle}
          aria-label={t("apiKeys.permAreaToggle", { area: area.name })}
        />
      </div>
      {listed
        ? area.permissions.map((permission) => (
            <PermissionRow
              key={permission.id}
              permission={permission}
              level={levels[permission.id] ?? null}
              disabled={disabled}
              onLevel={(level) => onLevel(permission.id, level)}
            />
          ))
        : null}
    </div>
  );
}

const ROLE_REASON: Record<MemberRole, string> = {
  viewer: "apiKeys.permRequiresRole.viewer",
  contributor: "apiKeys.permRequiresRole.contributor",
  developer: "apiKeys.permRequiresRole.developer",
  admin: "apiKeys.permRequiresRole.admin",
};

/** Why the viewer cannot grant `level`: the lowest role that can. */
function requiredRoleReason(t: Translate, level: PermissionLevelView): string {
  const key = level.requiredRole
    ? ROLE_REASON[level.requiredRole]
    : "apiKeys.permRequiresHigherRole";
  return t(key, { level: level.name });
}

/**
 * One permission: its name, a segmented choice of None and its levels (native
 * radios, so arrow keys move between them and skip a disabled level), the
 * chosen level's actions, and the reason beside each level the viewer cannot
 * grant.
 */
function PermissionRow({
  permission,
  level,
  disabled,
  onLevel,
}: {
  permission: PermissionView;
  level: PermissionLevel | null;
  disabled: boolean;
  onLevel: (level: PermissionLevel | null) => void;
}) {
  const { t } = useTranslations();
  const id = useId();
  const nameId = `${id}-name`;
  const helpId = `${id}-help`;
  const reasonId = (l: PermissionLevelView) => `${id}-reason-${l.level}`;
  const chosen = permission.levels.find((l) => l.level === level);
  const blocked = permission.levels.filter((l) => !l.grantable);

  const options = [
    {
      value: "" as const,
      label: t("apiKeys.permLevelNone"),
      level: undefined,
    },
    ...permission.levels.map((l) => ({
      value: l.level,
      label: l.name,
      level: l,
    })),
  ];

  return (
    <div
      data-permission={permission.id}
      className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start sm:gap-x-4"
    >
      <p id={nameId} className="text-sm [overflow-wrap:anywhere]">
        {permission.name}
      </p>
      <div
        role="radiogroup"
        aria-labelledby={nameId}
        aria-describedby={chosen ? helpId : undefined}
        className="bg-muted/40 flex w-fit flex-wrap gap-0.5 rounded-md border p-0.5"
      >
        {options.map((option) => {
          const blockedLevel = option.level && !option.level.grantable;
          return (
            <label
              key={option.value || "none"}
              className={cn(
                "text-muted-foreground hover:text-foreground cursor-pointer rounded-[5px] px-2 py-1 text-xs sm:px-2.5 font-medium whitespace-nowrap transition-colors",
                "has-checked:bg-primary has-checked:text-primary-foreground",
                "has-focus-visible:ring-ring has-focus-visible:ring-2",
                "has-disabled:hover:text-muted-foreground has-disabled:cursor-not-allowed has-disabled:opacity-50",
              )}
            >
              <input
                type="radio"
                className="sr-only"
                name={id}
                value={option.value}
                checked={(level ?? "") === option.value}
                disabled={disabled || !!blockedLevel}
                aria-describedby={
                  blockedLevel ? reasonId(option.level!) : undefined
                }
                onChange={() => onLevel(option.value || null)}
              />
              {option.label}
            </label>
          );
        })}
      </div>
      {chosen || blocked.length > 0 ? (
        <div className="text-muted-foreground space-y-1 text-xs sm:col-span-2">
          {chosen ? (
            <div id={helpId}>
              <p>{t("apiKeys.permLevelGrants", { level: chosen.name })}</p>
              <ul className="mt-0.5 list-disc space-y-0.5 pl-4">
                {chosen.actions.map((action) => (
                  <li key={action.id}>{action.name}</li>
                ))}
              </ul>
            </div>
          ) : null}
          {blocked.map((l) => (
            <p key={l.level} id={reasonId(l)}>
              {requiredRoleReason(t, l)}
            </p>
          ))}
        </div>
      ) : null}
    </div>
  );
}

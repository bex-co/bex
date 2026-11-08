import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ProjectView } from "@/features/projects/hooks/use-projects";
import {
  WORKSPACE_SCOPE,
  environmentScopeName,
} from "@/features/env-groups/lib/scope";

/**
 * The Environment scope picker shared by every env-group surface that chooses
 * one: Move, Clone, and — since w4/m111 — Create. One component so the option
 * set and its labels cannot drift between the dialog that mints a group and
 * the dialogs that relocate it.
 */
export function ScopeSelect({
  id,
  value,
  environments,
  projects,
  loading = false,
  restricts,
  restrictedReason,
  onValueChange,
}: {
  id: string;
  value: string;
  environments: EnvironmentView[];
  /** Names each option's project; same-named environments are common. */
  projects: ProjectView[];
  loading?: boolean;
  /** An environment the caller may not place the group in — a protected one
   *  for a non-Admin (w3/m281). Its option is disabled and restrictedReason
   *  describes the trigger. */
  restricts?: (environment: EnvironmentView) => boolean;
  restrictedReason?: string;
  onValueChange: (value: string) => void;
}) {
  const { t } = useTranslations();
  const restricted = (environment: EnvironmentView) =>
    restricts?.(environment) ?? false;
  const anyRestricted = environments.some(restricted);
  const reasonId = `${id}-restricted-reason`;
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{t("envGroups.environmentLabel")}</Label>
      <Select value={value} onValueChange={onValueChange} disabled={loading}>
        {/* "Project / Environment" names are unbounded user input: wrap the
            full label (even unbroken segments) inside the containing width
            instead of the primitive's one-line, fit-content trigger and menu
            (w4/270). */}
        <SelectTrigger
          id={id}
          aria-describedby={anyRestricted ? reasonId : undefined}
          className="min-h-9 max-w-full text-left whitespace-normal wrap-anywhere data-[size=default]:h-auto *:data-[slot=select-value]:line-clamp-none"
        >
          <SelectValue placeholder={t("envGroups.environmentPlaceholder")} />
        </SelectTrigger>
        <SelectContent className="max-w-(--radix-select-content-available-width)">
          <SelectItem value={WORKSPACE_SCOPE}>
            {t("envGroups.workspaceScope")}
          </SelectItem>
          {environments.map((environment) => (
            <SelectItem
              key={environment.id}
              value={environment.id}
              className="wrap-anywhere"
              disabled={restricted(environment)}
            >
              {restricted(environment)
                ? t("environments.protectedOptionAdminOnly", {
                    environment: environmentScopeName(environment, projects),
                  })
                : environmentScopeName(environment, projects)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {anyRestricted && restrictedReason ? (
        <p id={reasonId} className="text-sm text-muted-foreground">
          {restrictedReason}
        </p>
      ) : null}
    </div>
  );
}

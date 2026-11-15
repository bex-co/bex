import { ReadUnavailable } from "@/common/components/read-unavailable";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { ScopeSelect } from "@/features/env-groups/components/scope-select";
import {
  scopeEnvironmentId,
  scopeValue,
} from "@/features/env-groups/lib/scope";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import type { ProjectView } from "@/features/projects/hooks/use-projects";

/** What the field reads of the workspace's environment index. */
export interface EnvironmentIndexView {
  /** An index has been read for this workspace (it stays set over a re-read). */
  ready: boolean;
  loading: boolean;
  error?: Error;
  environments: EnvironmentView[];
  /** Names each option's project; same-named environments are common. */
  projects: ProjectView[];
  retry: () => void;
}

export interface SandboxGroupEnvironmentFieldProps {
  index: EnvironmentIndexView;
  /** The chosen environment's id; null is No environment. */
  value: string | null;
  onValueChange: (environmentId: string | null) => void;
  disabled?: boolean;
  /** A refusal of the chosen environment, shown under the select. */
  refusal?: string | null;
}

/**
 * The optional project environment of a new sandbox group, as Render's
 * dashboard offers it (w22/m412): the env group dialog's own scope select,
 * spanning the form, on No environment until one is chosen. It exists only for
 * a workspace that has environments, so a workspace without any sees the Name
 * field alone, and while the index is read it is previewed by its skeleton.
 * A failed read says so and offers to read again; the group can still be
 * created without an environment.
 */
export function SandboxGroupEnvironmentField({
  index,
  value,
  onValueChange,
  disabled = false,
  refusal = null,
}: SandboxGroupEnvironmentFieldProps) {
  const { t } = useTranslations();
  const placement = useProtectedPlacement();

  if (!index.ready) {
    return index.error ? (
      <ReadUnavailable
        message={t("sandboxGroups.environmentsError")}
        onRetry={index.retry}
        retrying={false}
      />
    ) : (
      <SandboxGroupEnvironmentFieldSkeleton />
    );
  }
  if (index.environments.length === 0) return null;

  return (
    <>
      <ScopeSelect
        id="sandbox-group-environment"
        value={scopeValue(value)}
        environments={index.environments}
        projects={index.projects}
        loading={disabled}
        restricts={placement.restricts}
        restrictedReason={placement.reason}
        noneLabel={t("environments.assignmentEnvironmentNone")}
        fullWidth
        onValueChange={(next) => onValueChange(scopeEnvironmentId(next))}
      />
      {refusal ? (
        <p role="alert" className="text-destructive text-sm">
          {refusal}
        </p>
      ) : null}
    </>
  );
}

/**
 * The field's loading state: the ready select's own outline (its wrapper, the
 * label's line and a full-width control of the trigger's height), so the swap
 * moves nothing. The trigger is 38px: one 20px text-sm line, 16px of padding
 * and its 2px border (ScopeSelect lets it grow to wrap a long name). Keep this
 * in step with ScopeSelect's layout (sandbox-group-environment-field.test.tsx
 * pins the mapping; measured in a browser at 1200px and 390px, w22/m412).
 */
export function SandboxGroupEnvironmentFieldSkeleton() {
  const { t } = useTranslations();
  return (
    <div
      className="space-y-2"
      role="status"
      aria-label={t("sandboxGroups.environmentLoading")}
    >
      <Skeleton className="h-3.5 w-24" />
      <Skeleton className="h-9.5 w-full" />
    </div>
  );
}

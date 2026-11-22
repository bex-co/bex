import { useEffect, useId } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  CREATE_SELECT_CLASS,
  PROJECT_SELECTS_GRID_CLASS,
  CreateField,
} from "@/common/components/create-form";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCatalogRetry } from "@/common/hooks/use-catalog-retry";
import { ReadUnavailable } from "@/common/components/read-unavailable";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import { useProjects } from "@/features/projects/hooks/use-projects";

export interface ProjectEnvironmentSelectorProps {
  projectId: string | null;
  environmentId: string | null;
  onProjectChange: (projectId: string | null) => void;
  onEnvironmentChange: (environmentId: string | null) => void;
}

/**
 * Shared create-form selector, Render's "Project · Optional" row: the
 * Project and Environment selects side by side. Project scopes the
 * Environment list; only the Environment id is submitted because assignment
 * auto-joins its parent Project. A note appears only when the choice needs
 * one: a Project with no Environment, or a Project picked without one.
 */
export function ProjectEnvironmentSelector({
  projectId,
  environmentId,
  onProjectChange,
  onEnvironmentChange,
}: ProjectEnvironmentSelectorProps) {
  const { t } = useTranslations();
  const protectedReasonId = useId();
  const descriptionId = useId();
  const {
    projects,
    loading: projectsLoading,
    error: projectsError,
    refetch: refetchProjects,
  } = useProjects();
  const {
    environments,
    loading: environmentsLoading,
    error: environmentsError,
    refetch: refetchEnvironments,
  } = useEnvironments(projectId);
  // A non-Admin cannot create in a protected environment (w3/m281): its
  // option is disabled and the reason is the trigger's description, so a
  // keyboard or screen-reader user hears it without opening the list.
  const placement = useProtectedPlacement();
  const blockedEnvironments = environments.some((environment) =>
    placement.restricts(environment),
  );
  // Only a list that settled without an error proves a selection missing. A
  // failed or pending read keeps the customer's placement (w4/m221): clearing
  // it would silently create the resource outside its Project.
  const projectsSettled = !projectsLoading && !projectsError;
  const environmentsSettled = !environmentsLoading && !environmentsError;
  const { retry, retrying } = useCatalogRetry(() =>
    Promise.all([
      projectsError ? refetchProjects() : null,
      environmentsError ? refetchEnvironments() : null,
    ]),
  );
  const projectListed = projects.some((p) => p.id === projectId);
  const selectedEnvironment = environments.find(
    (environment) =>
      environment.id === environmentId && environment.projectId === projectId,
  );
  const environmentListed = selectedEnvironment !== undefined;
  // A preselected protected environment a confirmed non-Admin cannot create
  // in is cleared on load (w3/m292): the reason below names why, instead of
  // a refusal on submit.
  const selectionRestricted = placement.restricts(selectedEnvironment);

  // URL preselection is only a convenience: clear stale or foreign ids before
  // the form can submit them, then leave the ordinary selectors editable.
  useEffect(() => {
    if (environmentId && !projectId) {
      onEnvironmentChange(null);
      return;
    }
    if (projectsSettled && projectId && !projectListed) {
      onProjectChange(null);
      onEnvironmentChange(null);
      return;
    }
    if (
      environmentsSettled &&
      environmentId &&
      (!environmentListed || selectionRestricted)
    ) {
      onEnvironmentChange(null);
    }
  }, [
    environmentId,
    environmentListed,
    selectionRestricted,
    environmentsSettled,
    onEnvironmentChange,
    onProjectChange,
    projectId,
    projectListed,
    projectsSettled,
  ]);

  // A kept selection whose list has not (yet) named it still shows as chosen,
  // never as No project/No environment.
  const pendingLabel = (error: unknown) =>
    error
      ? t("environments.assignmentSelectedUnavailable")
      : t("environments.assignmentSelectedLoading");
  const unlistedProject = !projectsSettled && projectId && !projectListed;
  const unlistedEnvironment =
    !environmentsSettled && environmentId && !environmentListed;

  // The note a Project chosen without an Environment needs: there is none to
  // choose yet, or leaving it empty creates the resource outside the Project.
  const environmentNote =
    projectId == null || environmentId != null || !environmentsSettled
      ? null
      : environments.length === 0
        ? t("environments.assignmentNoEnvironments")
        : t("environments.assignmentPickEnvironment");

  return (
    <CreateField
      label={t("environments.assignmentTitle")}
      optional
      description={t("environments.assignmentDescription")}
      descriptionId={descriptionId}
    >
      <div className={PROJECT_SELECTS_GRID_CLASS}>
        <Select
          value={projectId ?? "unassigned"}
          onValueChange={(value) => {
            onProjectChange(value === "unassigned" ? null : value);
            onEnvironmentChange(null);
          }}
        >
          <SelectTrigger
            aria-label={t("environments.assignmentProject")}
            aria-describedby={descriptionId}
            className={CREATE_SELECT_CLASS}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="unassigned">
              {t("environments.assignmentProjectNone")}
            </SelectItem>
            {projects.map((project) => (
              <SelectItem key={project.id} value={project.id}>
                {project.name}
              </SelectItem>
            ))}
            {unlistedProject ? (
              <SelectItem value={projectId}>
                {pendingLabel(projectsError)}
              </SelectItem>
            ) : null}
          </SelectContent>
        </Select>
        <span
          aria-hidden
          className="text-muted-foreground hidden px-1 @2xl:block"
        >
          /
        </span>
        <Select
          disabled={projectId == null}
          value={environmentId ?? "unassigned"}
          onValueChange={(value) =>
            onEnvironmentChange(value === "unassigned" ? null : value)
          }
        >
          <SelectTrigger
            aria-label={t("environments.assignmentEnvironment")}
            aria-describedby={
              blockedEnvironments ? protectedReasonId : undefined
            }
            className={CREATE_SELECT_CLASS}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="unassigned">
              {projectId == null
                ? t("environments.assignmentEnvironmentPickProject")
                : t("environments.assignmentEnvironmentNone")}
            </SelectItem>
            {environments.map((environment) =>
              placement.restricts(environment) ? (
                <SelectItem
                  key={environment.id}
                  value={environment.id}
                  disabled
                >
                  {t("environments.protectedOptionAdminOnly", {
                    environment: environment.name,
                  })}
                </SelectItem>
              ) : (
                <SelectItem key={environment.id} value={environment.id}>
                  {environment.name}
                </SelectItem>
              ),
            )}
            {unlistedEnvironment ? (
              <SelectItem value={environmentId}>
                {pendingLabel(environmentsError)}
              </SelectItem>
            ) : null}
          </SelectContent>
        </Select>
      </div>
      {projectsError || environmentsError ? (
        <ReadUnavailable
          message={t("environments.assignmentReadError")}
          onRetry={() => void retry()}
          retrying={retrying}
        />
      ) : null}
      {blockedEnvironments ? (
        <p id={protectedReasonId} className="text-sm text-muted-foreground">
          {placement.reason}
        </p>
      ) : null}
      {environmentNote ? (
        <p className="text-sm text-muted-foreground">{environmentNote}</p>
      ) : null}
    </CreateField>
  );
}

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useProjects } from "@/features/projects/hooks/use-projects";

export interface ProjectEnvironmentSelectorProps {
  projectId: string | null;
  environmentId: string | null;
  onProjectChange: (projectId: string | null) => void;
  onEnvironmentChange: (environmentId: string | null) => void;
}

/**
 * Shared create-form selector. Project scopes the Environment list; only the
 * Environment id is submitted because assignment auto-joins its parent Project.
 */
export function ProjectEnvironmentSelector({
  projectId,
  environmentId,
  onProjectChange,
  onEnvironmentChange,
}: ProjectEnvironmentSelectorProps) {
  const { t } = useTranslations();
  const { projects } = useProjects();
  const { environments } = useEnvironments(projectId);

  return (
    <div className="space-y-2">
      <Label>{t("environments.assignmentTitle")}</Label>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <Select
          value={projectId ?? "unassigned"}
          onValueChange={(value) => {
            onProjectChange(value === "unassigned" ? null : value);
            onEnvironmentChange(null);
          }}
        >
          <SelectTrigger aria-label={t("environments.assignmentProject")}>
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
          </SelectContent>
        </Select>
        <Select
          disabled={projectId == null}
          value={environmentId ?? "unassigned"}
          onValueChange={(value) =>
            onEnvironmentChange(value === "unassigned" ? null : value)
          }
        >
          <SelectTrigger aria-label={t("environments.assignmentEnvironment")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="unassigned">
              {t("environments.assignmentEnvironmentNone")}
            </SelectItem>
            {environments.map((environment) => (
              <SelectItem key={environment.id} value={environment.id}>
                {environment.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <p className="text-sm text-muted-foreground">
        {t("environments.assignmentHint")}
      </p>
    </div>
  );
}

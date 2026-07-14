import { useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { Loader2, Pencil } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button.tsx";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog.tsx";
import { Input } from "@/common/components/ui/input.tsx";
import { useServices } from "@/features/services/hooks/use-services";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useGroupedResources } from "@/features/projects/hooks/use-grouped-resources";
import { useRenameProject } from "@/features/projects/hooks/use-rename-project";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";

export const Route = createFileRoute("/project/$projectId/")({
  component: ProjectPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.projectId} · bex dashboard` }],
  }),
});

/**
 * A single project's own page (Render parity: dashboard.render.com/project/{id})
 * — the project's merged services+databases+key-value table, reached from an
 * Overview project card or the contextual project sidebar's "Overview" item.
 * Owns rename only (an inline pencil next to the name, Render parity) — delete
 * lives solely on the Settings sub-page (`project.$projectId.settings.tsx`),
 * matching Render's own split of affordances.
 */
export function ProjectPage() {
  const { projectId } = Route.useParams();
  const { t } = useTranslations();

  const { services, refetch: refetchServices } = useServices();
  const {
    databases,
    refetch: refetchDatabases,
  } = useDatabases();
  const { keyValues, refetch: refetchKeyValues } = useKeyValues();
  const { projects, loading, refetch: refetchProjects } = useProjects();
  const { pending, run } = useServiceLifecycle({ refetch: refetchServices });

  const { groups } = useGroupedResources({ projects, services, databases, keyValues });
  const project = projects.find((p) => p.id === projectId);
  const group = groups.find((g) => g.id === projectId);

  const { rename, busy: renaming } = useRenameProject();
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState("");

  function refetchAll() {
    void refetchServices();
    void refetchDatabases();
    void refetchKeyValues();
    void refetchProjects();
  }

  function openRename() {
    setRenameValue(project?.name ?? "");
    setRenameOpen(true);
  }

  async function handleRename() {
    const trimmed = renameValue.trim();
    if (!project || !trimmed || trimmed === project.name) {
      setRenameOpen(false);
      return;
    }
    const ok = await rename(projectId, trimmed);
    if (ok) {
      setRenameOpen(false);
      refetchAll();
    }
  }

  const showNotFound = !loading && !project;

  return (
    <>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="w-full space-y-6">
          {showNotFound ? (
            <p className="text-sm text-muted-foreground">
              {t("projects.notFound")}
            </p>
          ) : (
            <>
              <div>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("projects.eyebrow")}
                </p>
                <div className="flex items-center gap-2">
                  <h1 className="text-2xl font-semibold">
                    {project?.name ?? projectId}
                  </h1>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("projects.editNameButton")}
                    onClick={openRename}
                  >
                    <Pencil />
                  </Button>
                </div>
              </div>

              <EnvironmentsPanel
                projectId={projectId}
                services={services}
                servicePending={pending}
                onRunServiceAction={run}
              />

              <section className="space-y-4">
                <h2 className="text-lg font-semibold">
                  {t("projects.allResourcesHeading")}
                </h2>
                {group && group.rows.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    {t("projects.emptyBody")}
                  </p>
                ) : (
                  <ResourceTable
                    rows={group?.rows ?? []}
                    loading={loading}
                    servicePending={pending}
                    onRunServiceAction={run}
                    onDatabaseDeleted={refetchAll}
                    onKeyValueDeleted={refetchAll}
                  />
                )}
              </section>
            </>
          )}
        </div>
      </div>

      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("projects.renameTitle")}</DialogTitle>
          </DialogHeader>
          <Input
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            autoComplete="off"
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleRename();
            }}
          />
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRenameOpen(false)}
              disabled={renaming}
            >
              {t("projects.createCancel")}
            </Button>
            <Button
              onClick={() => void handleRename()}
              disabled={renaming || renameValue.trim().length === 0}
            >
              {renaming ? <Loader2 className="animate-spin" /> : null}
              {t("projects.renameSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

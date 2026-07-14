import { useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Loader2, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog.tsx";
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
import { useDeleteProject } from "@/features/projects/hooks/use-delete-project";
import { ResourceTable } from "@/features/projects/components/resource-table";

export const Route = createFileRoute("/project/$projectId")({
  component: ProjectPage,
  beforeLoad: requireAuth("/"),
  head: ({ params }) => ({
    meta: [{ title: `${params.projectId} · bex dashboard` }],
  }),
});

/**
 * A single project's page (Render parity: dashboard.render.com/project/{id})
 * — the project's own merged services+databases+key-value table, reached from
 * an Overview project card. Owns rename/delete (the same "•••" menu the
 * flattened list used to carry, w1/m31).
 */
export function ProjectPage() {
  const { projectId } = Route.useParams();
  const { t } = useTranslations();
  const navigate = useNavigate();

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
  const { remove, deleting } = useDeleteProject();
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deletingThis = deleting === projectId;

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

  async function handleDelete() {
    if (!project) return;
    const ok = await remove(projectId, project.name);
    if (ok) {
      setDeleteOpen(false);
      void navigate({ to: "/" });
    }
  }

  const showNotFound = !loading && !project;

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="w-full space-y-6">
          {showNotFound ? (
            <p className="text-sm text-muted-foreground">
              {t("projects.notFound")}
            </p>
          ) : (
            <>
              <div className="flex items-start justify-between gap-2">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    {t("projects.eyebrow")}
                  </p>
                  <h1 className="text-2xl font-semibold">
                    {project?.name ?? projectId}
                  </h1>
                </div>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t("projects.projectActionsMenu")}
                    >
                      <MoreHorizontal />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onSelect={openRename}>
                      <Pencil />
                      {t("projects.actionRename")}
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      variant="destructive"
                      onSelect={() => setDeleteOpen(true)}
                    >
                      <Trash2 />
                      {t("projects.actionDelete")}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

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

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("projects.deleteConfirmTitle", { name: project?.name ?? "" })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("projects.deleteConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingThis}>
              {t("projects.createCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                void handleDelete();
              }}
              disabled={deletingThis}
            >
              {deletingThis ? <Loader2 className="animate-spin" /> : null}
              {t("projects.actionDelete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </DashboardLayout>
  );
}

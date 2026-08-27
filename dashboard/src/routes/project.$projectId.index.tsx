import { useState } from "react";
import {
  createFileRoute,
  getRouteApi,
  useRouter,
} from "@tanstack/react-router";
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
import { useGroupedResources } from "@/features/projects/hooks/use-grouped-resources";
import { useRenameProject } from "@/features/projects/hooks/use-rename-project";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import {
  parseProjectResourceSearch,
  type ProjectResourceFilterState,
} from "@/features/projects/lib/resource-filter";
import { ProjectOverviewPageSkeleton } from "@/common/components/route-skeletons";
import { ResourceLoadError } from "@/common/components/resource-load-error";

export const Route = createFileRoute("/project/$projectId/")({
  component: ProjectPage,
  pendingComponent: ProjectOverviewPageSkeleton,
  validateSearch: parseProjectResourceSearch,
});

const projectRoute = getRouteApi("/project/$projectId");

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
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const router = useRouter();
  const { t } = useTranslations();
  const resourceFilter: ProjectResourceFilterState = {
    environmentId: search.env ?? null,
    query: search.q ?? "",
    kind: search.kind ?? "all",
  };

  function setResourceFilter(next: ProjectResourceFilterState) {
    void navigate({
      search: {
        env: next.environmentId ?? undefined,
        q: next.query || undefined,
        kind: next.kind === "all" ? undefined : next.kind,
      },
      replace: true,
    });
  }

  const { services, refetch: refetchServices } = useServices();
  const { databases, refetch: refetchDatabases } = useDatabases();
  const { keyValues, refetch: refetchKeyValues } = useKeyValues();
  const { pending, run } = useServiceLifecycle({ refetch: refetchServices });
  const projectResult = projectRoute.useLoaderData();
  const project =
    projectResult.state === "ready" ? projectResult.resource : undefined;

  const { groups } = useGroupedResources({
    projects: project ? [project] : [],
    services,
    databases,
    keyValues,
  });
  const group = groups.find((g) => g.id === projectId);

  const { rename, busy: renaming } = useRenameProject();
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState("");

  function refetchAll() {
    void refetchServices();
    void refetchDatabases();
    void refetchKeyValues();
    void router.invalidate();
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

  const showNotFound = projectResult.state === "not-found";
  const showError = projectResult.state === "error";
  const showAuthError = projectResult.state === "unauthenticated";

  return (
    <>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="w-full space-y-6">
          {showAuthError ? (
            <ResourceLoadError variant="unauthenticated" />
          ) : showError ? (
            <p className="text-sm text-destructive">
              {t("projects.errorTitle")}
            </p>
          ) : showNotFound ? null : ( // the layout's redirect home is in flight (w9/m55)
            <>
              <div>
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("projects.eyebrow")}
                </p>
                <div className="flex items-center gap-2">
                  <h1 className="truncate text-xl font-semibold">
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
                databases={databases}
                keyValues={keyValues}
                projectRows={group?.rows ?? []}
                servicePending={pending}
                onRunServiceAction={run}
                onDatabaseDeleted={refetchAll}
                onKeyValueDeleted={refetchAll}
                resourceFilter={resourceFilter}
                onResourceFilterChange={setResourceFilter}
              />
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

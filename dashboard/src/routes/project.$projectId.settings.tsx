import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { useEffect, useMemo, useState } from "react";
import {
  doomedResourceNames,
  type DoomedResourceIds,
} from "@/features/projects/lib/doomed-resources";
import { useServices } from "@/features/services/hooks/use-services";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import {
  createFileRoute,
  getRouteApi,
  useNavigate,
} from "@tanstack/react-router";
import { Check, Loader2, Pencil, X } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card.tsx";
import { Button } from "@/common/components/ui/button.tsx";
import { Input } from "@/common/components/ui/input.tsx";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { LabelError } from "@/common/components/label-error";
import {
  isInvalidLabel,
  isValidLabel,
} from "@/common/lib/utils/resource-label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRenameProject } from "@/features/projects/hooks/use-rename-project";
import { useDeleteProject } from "@/features/projects/hooks/use-delete-project";
import {
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";
import { ProjectSettingsPageSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/project/$projectId/settings")({
  component: ProjectSettingsPage,
  pendingComponent: ProjectSettingsPageSkeleton,
  // The project page's name links here with `?edit=name`, as Render's does
  // (w1/m682): the Name card opens editing, and the parameter is cleared.
  validateSearch: (search: Record<string, unknown>): { edit?: "name" } => ({
    edit: search.edit === "name" ? "name" : undefined,
  }),
  loader: async ({ parentMatchPromise }) =>
    (await parentMatchPromise).loaderData,
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (project) => [
        `${project.name} / ${translatedText("common.navSettings")}`,
      ]),
      match,
    ),
});

const projectRoute = getRouteApi("/project/$projectId");

/**
 * Render parity: a project's Settings sub-page, reached from the contextual
 * project sidebar's "Settings" item — a "Project Name" card (inline
 * pencil-edit, same idiom as `HealthCheckPathRow`) and the "Delete Project"
 * danger zone (Render's own split: Delete lives only here, not on Overview).
 */
export function ProjectSettingsPage() {
  const { projectId } = Route.useParams();
  const { edit } = Route.useSearch();
  const navigate = useNavigate();
  const { t } = useTranslations();
  const projectResult = projectRoute.useLoaderData();
  const project =
    projectResult.state === "ready" ? projectResult.resource : undefined;

  const { rename, busy: renaming } = useRenameProject();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  // The removed project-page rename dialog's rule (w21/036): a name the label
  // rule refuses reads inline and is never sent.
  const canSave = isValidLabel(draft) && draft.trim() !== project?.name;
  const projectName = project?.name;

  // `?edit=name` opens the editor once the name is known, then leaves the
  // address (w1/m682).
  const [openedByLink, setOpenedByLink] = useState(false);
  if (edit === "name" && projectName !== undefined && !openedByLink) {
    setOpenedByLink(true);
    setDraft(projectName);
    setEditing(true);
  }
  if (edit !== "name" && openedByLink) setOpenedByLink(false);
  useEffect(() => {
    if (edit !== "name" || !openedByLink) return;
    void navigate({
      to: "/project/$projectId/settings",
      params: { projectId },
      search: {},
      replace: true,
      resetScroll: false,
    });
  }, [edit, openedByLink, projectId, navigate]);

  const { remove, deleting } = useDeleteProject();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deletingThis = deleting === projectId;
  // Services, databases and key value stores are deleted with the project
  // (w4/m264, w6/m290).
  const doomedResourceCount =
    (project?.serviceIds.length ?? 0) +
    (project?.databaseIds.length ?? 0) +
    (project?.keyValueIds.length ?? 0);

  function startEdit() {
    setDraft(project?.name ?? "");
    setEditing(true);
  }

  async function handleSave() {
    const trimmed = draft.trim();
    if (!project || !trimmed || trimmed === project.name) {
      setEditing(false);
      return;
    }
    // The hook republishes the route loader after re-reading the saved name
    // (w4/m186); invalidating here too would race that ordering.
    if (await rename(projectId, trimmed)) setEditing(false);
  }

  async function handleDelete() {
    if (!project) return;
    const ok = await remove(projectId, project.name);
    if (ok) {
      setDeleteOpen(false);
      void navigate({ to: "/", replace: true });
    }
  }

  if (!project) {
    // not-found renders nothing: the layout's redirect home is in flight
    // (w9/m55). Only a query error stays put, on its inline message.
    if (projectResult.state !== "error") return null;
    return (
      <PageFrame width="form">
        <p className="text-sm text-muted-foreground">
          {t("projects.errorTitle")}
        </p>
      </PageFrame>
    );
  }

  return (
    <PageFrame
      width="form"
      header={<PageHeader title={t("projects.settingsTitle")} />}
    >
      <Card>
        <CardHeader>
          {/* Both mutually exclusive inputs below point at this heading, so
                the control is named by its purpose in either state rather than
                by whatever value it currently holds (w4/134). */}
          <CardTitle id="project-name-card-title">
            {t("projects.nameCardTitle")}
          </CardTitle>
          <CardDescription>{t("projects.nameCardDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {editing ? (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Input
                  aria-labelledby="project-name-card-title"
                  aria-invalid={isInvalidLabel(draft)}
                  aria-describedby={
                    isInvalidLabel(draft) ? "project-name-error" : undefined
                  }
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  autoFocus
                  autoComplete="off"
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && canSave) void handleSave();
                    if (e.key === "Escape") setEditing(false);
                  }}
                />
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("projects.renameSubmit")}
                  disabled={renaming || !canSave}
                  onClick={() => void handleSave()}
                >
                  {renaming ? (
                    <Loader2 className="animate-spin" />
                  ) : (
                    <Check className="text-emerald-600" />
                  )}
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  aria-label={t("projects.createCancel")}
                  disabled={renaming}
                  onClick={() => setEditing(false)}
                >
                  <X />
                </Button>
              </div>
              <LabelError id="project-name-error" value={draft} />
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <Input
                aria-labelledby="project-name-card-title"
                value={project?.name ?? ""}
                disabled
                readOnly
                className="flex-1"
              />
              <Button variant="outline" size="sm" onClick={startEdit}>
                <Pencil />
                {t("projects.editNameButton")}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-destructive">
            {t("projects.deleteCardTitle")}
          </CardTitle>
          <CardDescription>
            {t("projects.deleteCardDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
            {t("projects.deleteCardButton")}
          </Button>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("projects.deleteConfirmTitle", { name: project?.name ?? "" })}
        description={
          project && doomedResourceCount > 0 ? (
            <ProjectDoomedResources project={project} />
          ) : (
            t("projects.deleteConfirmBody")
          )
        }
        phrase={
          doomedResourceCount > 0
            ? `sudo delete project ${project?.name ?? ""}`
            : undefined
        }
        cancelLabel={t("projects.createCancel")}
        confirmLabel={
          <>
            {deletingThis ? <Loader2 className="animate-spin" /> : null}
            {t("projects.deleteCardButton")}
          </>
        }
        pending={deletingThis}
        onConfirm={() => void handleDelete()}
      />
    </PageFrame>
  );
}

/**
 * The project delete's description, naming every service, database and key
 * value store it removes, as the environment delete does (w4/542). It mounts
 * only with the open dialog, so the page reads the workspace's lists only when
 * someone is about to delete; until they answer, a resource reads as its id.
 */
function ProjectDoomedResources({ project }: { project: DoomedResourceIds }) {
  const { t } = useTranslations();
  const { services } = useServices({ poll: false });
  const { databases } = useDatabases();
  const { keyValues } = useKeyValues();
  const names = useMemo(
    () => doomedResourceNames(project, [services, databases, keyValues]),
    [project, services, databases, keyValues],
  );
  return t("projects.deleteConfirmBodyResources", {
    resources: names.join(", "),
  });
}

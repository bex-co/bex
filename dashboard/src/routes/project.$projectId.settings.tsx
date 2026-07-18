import { useState } from "react";
import {
  createFileRoute,
  getRouteApi,
  useNavigate,
  useRouter,
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
import { useTranslations } from "@/common/hooks/use-translations";
import { useRenameProject } from "@/features/projects/hooks/use-rename-project";
import { useDeleteProject } from "@/features/projects/hooks/use-delete-project";
import {
  routeResourceTitle,
  titleHead,
  translatedText,
} from "@/common/lib/document-head";

export const Route = createFileRoute("/project/$projectId/settings")({
  component: ProjectSettingsPage,
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
  const navigate = useNavigate();
  const router = useRouter();
  const { t } = useTranslations();
  const projectResult = projectRoute.useLoaderData();
  const project =
    projectResult.state === "ready" ? projectResult.resource : undefined;

  const { rename, busy: renaming } = useRenameProject();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const canSave = draft.trim().length > 0 && draft.trim() !== project?.name;

  const { remove, deleting } = useDeleteProject();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deletingThis = deleting === projectId;

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
    const ok = await rename(projectId, trimmed);
    if (ok) {
      setEditing(false);
      void router.invalidate();
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

  if (!project) {
    // not-found renders nothing: the layout's redirect home is in flight
    // (w9/m55). Only a query error stays put, on its inline message.
    if (projectResult.state !== "error") return null;
    return (
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <p className="text-sm text-muted-foreground">
          {t("projects.errorTitle")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-auto p-4 sm:p-6">
      <div className="w-full max-w-2xl space-y-6">
        <h1 className="text-xl font-semibold">{t("projects.settingsTitle")}</h1>

        <Card>
          <CardHeader>
            <CardTitle>{t("projects.nameCardTitle")}</CardTitle>
            <CardDescription>
              {t("projects.nameCardDescription")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {editing ? (
              <div className="flex items-center gap-2">
                <Input
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
            ) : (
              <div className="flex items-center gap-2">
                <Input
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
      </div>

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
              {t("projects.deleteCardButton")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

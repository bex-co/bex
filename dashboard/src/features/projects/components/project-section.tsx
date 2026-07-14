import { useState } from "react";
import { Loader2, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
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
import { useTranslations } from "@/common/hooks/use-translations";
import { useRenameProject } from "@/features/projects/hooks/use-rename-project";
import { useDeleteProject } from "@/features/projects/hooks/use-delete-project";
import { ResourceTable, type ResourceTableProps } from "@/features/projects/components/resource-table";

export interface ProjectSectionProps
  extends Pick<
    ResourceTableProps,
    | "loading"
    | "servicePending"
    | "onRunServiceAction"
    | "onDatabaseDeleted"
    | "onKeyValueDeleted"
  > {
  id: string;
  name: string;
  rows: ResourceTableProps["rows"];
  /** Called after a rename/delete so the page refetches the projects list. */
  onChanged: () => void;
}

/** One project's section on the unified Projects page: name header, a "•••"
 * menu (rename / delete), and its merged services+databases+key-value table. */
export function ProjectSection({
  id,
  name,
  rows,
  onChanged,
  ...tableProps
}: ProjectSectionProps) {
  const { t } = useTranslations();
  const { rename, busy: renaming } = useRenameProject();
  const { remove, deleting } = useDeleteProject();
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState(name);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deletingThis = deleting === id;

  function openRename() {
    setRenameValue(name);
    setRenameOpen(true);
  }

  async function handleRename() {
    const trimmed = renameValue.trim();
    if (!trimmed || trimmed === name) {
      setRenameOpen(false);
      return;
    }
    const ok = await rename(id, trimmed);
    if (ok) {
      setRenameOpen(false);
      onChanged();
    }
  }

  async function handleDelete() {
    const ok = await remove(id, name);
    if (ok) {
      setDeleteOpen(false);
      onChanged();
    }
  }

  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {t("projects.groupLabel")}: {name}
        </p>
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

      <ResourceTable rows={rows} {...tableProps} />

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
              {t("projects.deleteConfirmTitle", { name })}
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
    </div>
  );
}

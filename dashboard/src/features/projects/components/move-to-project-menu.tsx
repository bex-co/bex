import { FolderKanban, FolderMinus } from "lucide-react";
import {
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useMoveToProject,
  type ProjectResourceKind,
} from "@/features/projects/hooks/use-move-to-project";

export interface MoveToProjectMenuProps {
  kind: ProjectResourceKind;
  resourceId: string;
  resourceName: string;
  disabled?: boolean;
}

/**
 * A "Move to project ▸" submenu shared by the services/databases/key-value row
 * actions menus (w1/m31 extension). Lists every project in the workspace plus,
 * when the resource is currently assigned to one, a "Remove from project"
 * entry — self-contained around useMoveToProject so each row-actions component
 * only needs to render this one item. Renders nothing when the workspace has
 * no projects yet.
 */
export function MoveToProjectMenu({
  kind,
  resourceId,
  resourceName,
  disabled,
}: MoveToProjectMenuProps) {
  const { t } = useTranslations();
  const { projects, currentProjectId, moveTo, removeFromProject, busyId } =
    useMoveToProject(kind);
  const current = currentProjectId(resourceId);
  const busy = busyId === resourceId;

  if (projects.length === 0) return null;

  // Owns its own leading separator (rather than each row-actions caller adding
  // one) so a workspace with no projects yet renders no dangling separator.
  return (
    <>
      <DropdownMenuSeparator />
      <DropdownMenuSub>
        <DropdownMenuSubTrigger disabled={disabled || busy}>
          <FolderKanban />
          {t("projects.moveToProject")}
        </DropdownMenuSubTrigger>
        <DropdownMenuPortal>
          <DropdownMenuSubContent>
            {projects.map((p) => (
              <DropdownMenuItem
                key={p.id}
                disabled={p.id === current || busy}
                onSelect={() => void moveTo(resourceId, resourceName, p.id)}
              >
                {p.name}
              </DropdownMenuItem>
            ))}
            {current ? (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  disabled={busy}
                  onSelect={() =>
                    void removeFromProject(resourceId, resourceName)
                  }
                >
                  <FolderMinus />
                  {t("projects.removeFromProject")}
                </DropdownMenuItem>
              </>
            ) : null}
          </DropdownMenuSubContent>
        </DropdownMenuPortal>
      </DropdownMenuSub>
    </>
  );
}

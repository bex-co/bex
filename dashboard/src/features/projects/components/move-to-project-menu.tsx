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
import type { ProjectResourceFamily } from "@/features/projects/lib/resource-filter";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { placementIndex } from "@/features/environments/lib/placement";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";

const FAMILY: Record<ProjectResourceKind, ProjectResourceFamily> = {
  service: "services",
  database: "databases",
  keyvalue: "keyvalues",
};

export interface MoveToProjectMenuProps {
  kind: ProjectResourceKind;
  resourceId: string;
  resourceName: string;
  disabled?: boolean;
  /**
   * Intercepts a move before it writes (w4/m238): the host decides whether
   * the move may proceed immediately or must be confirmed first, and calls
   * `proceed` to run the original single write. Hosted by the row-actions
   * component because selecting a menu item unmounts this menu — a dialog
   * owned here would die with it. Datastore hosts omit it: datastores never
   * link groups, so their moves are never intercepted.
   */
  onMoveRequested?: (proceed: () => void) => void;
}

/**
 * A "Move to project ▸" submenu shared by the services/databases/key-value row
 * actions menus (w1/m31 extension). Lists every project in the workspace plus,
 * when the resource is currently assigned to one, a "Remove from project"
 * entry — self-contained around useMoveToProject so each row-actions component
 * only needs to render this one item. Renders nothing when the workspace has
 * no projects yet.
 */
export function MoveToProjectMenu(props: MoveToProjectMenuProps) {
  const moves = useMoveToProject(props.kind);
  if (moves.projects.length === 0) return null;
  return <MoveToProjectItems {...props} moves={moves} />;
}

/**
 * The submenu itself, mounted only when there is a project to move to, so the
 * environment read behind its protected-placement gate runs only then.
 */
function MoveToProjectItems({
  kind,
  resourceId,
  resourceName,
  disabled,
  onMoveRequested,
  moves,
}: MoveToProjectMenuProps & { moves: ReturnType<typeof useMoveToProject> }) {
  const { t } = useTranslations();
  const { projects, currentProjectId, moveTo, removeFromProject, busyId } =
    moves;
  const current = currentProjectId(resourceId);
  const busy = busyId === resourceId;
  const attempt = (action: () => void) =>
    onMoveRequested ? onMoveRequested(action) : action();
  // A project move takes the resource out of its environment, which a
  // non-Admin may not do to a protected one (w3/m281).
  const { environments } = useEnvGroupScopeIndex();
  const placement = useProtectedPlacement();
  const leaves = placementIndex(environments)(FAMILY[kind], resourceId);
  if (placement.restricts(leaves)) {
    return (
      <>
        <DropdownMenuSeparator />
        <PermissionMenuItem permissionReason={placement.reason}>
          <FolderKanban />
          {t("projects.moveToProject")}
        </PermissionMenuItem>
      </>
    );
  }

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
                onSelect={() =>
                  attempt(() => void moveTo(resourceId, resourceName, p.id))
                }
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
                    attempt(
                      () => void removeFromProject(resourceId, resourceName),
                    )
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

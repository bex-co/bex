import { useState } from "react";
import {
  ChevronDown,
  Copy,
  Loader2,
  MoveRight,
  Pencil,
  Trash2,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceEnvironmentIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { isValidEnvGroupName } from "@/features/env-groups/lib/validation";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { EnvGroupView } from "@/features/env-groups/types";

const WORKSPACE_SCOPE = "__workspace__";

export function EnvGroupActions({
  group,
  environments,
  renameGroup,
  moveGroup,
  cloneGroup,
  deleteGroup,
  busy,
  onDeleted,
  onCloned,
}: {
  group: EnvGroupView;
  environments: EnvironmentView[];
  renameGroup: (id: string, name: string) => Promise<boolean>;
  moveGroup: (id: string, environmentId: string | null) => Promise<boolean>;
  cloneGroup: (
    id: string,
    name: string,
    ownerId: string,
    environmentId: string | null,
  ) => Promise<string | null>;
  deleteGroup: (id: string) => Promise<boolean>;
  busy: boolean;
  onDeleted: () => void;
  onCloned: (id: string) => void;
}) {
  const { t } = useTranslations();
  const { workspaces, currentWorkspaceId, setCurrentWorkspaceId } =
    useWorkspace();
  const [dialog, setDialog] = useState<
    "rename" | "move" | "clone" | "delete" | null
  >(null);
  const [name, setName] = useState(group.name);
  const [moveTarget, setMoveTarget] = useState(
    group.environmentId ?? WORKSPACE_SCOPE,
  );
  const [cloneOwnerId, setCloneOwnerId] = useState(currentWorkspaceId ?? "");
  const [cloneEnvironmentId, setCloneEnvironmentId] = useState(WORKSPACE_SCOPE);
  const [confirmation, setConfirmation] = useState("");
  const remoteCloneScope = useWorkspaceEnvironmentIndex(
    cloneOwnerId && cloneOwnerId !== currentWorkspaceId ? cloneOwnerId : null,
  );
  const cloneEnvironments =
    cloneOwnerId === currentWorkspaceId
      ? environments
      : remoteCloneScope.environments;
  const deletePhrase = `sudo delete env group ${group.name}`;

  function open(next: NonNullable<typeof dialog>) {
    setName(next === "clone" ? `${group.name}-copy` : group.name);
    setMoveTarget(group.environmentId ?? WORKSPACE_SCOPE);
    setCloneOwnerId(currentWorkspaceId ?? group.ownerId ?? "");
    setCloneEnvironmentId(WORKSPACE_SCOPE);
    setConfirmation("");
    setDialog(next);
  }

  async function handleRename() {
    if (!isValidEnvGroupName(name) || busy) return;
    if (await renameGroup(group.id, name.trim())) setDialog(null);
  }

  async function handleMove() {
    if (busy || moveTarget === (group.environmentId ?? WORKSPACE_SCOPE)) return;
    if (
      await moveGroup(
        group.id,
        moveTarget === WORKSPACE_SCOPE ? null : moveTarget,
      )
    ) {
      setDialog(null);
    }
  }

  async function handleClone() {
    if (!isValidEnvGroupName(name) || !cloneOwnerId || busy) return;
    const id = await cloneGroup(
      group.id,
      name.trim(),
      cloneOwnerId,
      cloneEnvironmentId === WORKSPACE_SCOPE ? null : cloneEnvironmentId,
    );
    if (id) {
      setDialog(null);
      if (cloneOwnerId !== currentWorkspaceId) {
        setCurrentWorkspaceId(cloneOwnerId);
      }
      onCloned(id);
    }
  }

  async function handleDelete() {
    if (confirmation !== deletePhrase || busy) return;
    if (await deleteGroup(group.id)) {
      setDialog(null);
      onDeleted();
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm">
            {t("envGroups.manageButton")} <ChevronDown />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => open("rename")}>
            <Pencil /> {t("envGroups.renameButton")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => open("move")}>
            <MoveRight /> {t("envGroups.moveButton")}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => open("clone")}>
            <Copy /> {t("envGroups.cloneButton")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => open("delete")}
          >
            <Trash2 /> {t("envGroups.deleteButton")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog
        open={dialog === "rename"}
        onOpenChange={(isOpen) => !isOpen && setDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("envGroups.renameTitle")}</DialogTitle>
            <DialogDescription>
              {t("envGroups.renameDescription")}
            </DialogDescription>
          </DialogHeader>
          <NameField id="env-group-rename" name={name} setName={setName} />
          <DialogFooter>
            <CancelButton busy={busy} onClick={() => setDialog(null)} />
            <Button
              onClick={() => void handleRename()}
              disabled={!isValidEnvGroupName(name) || busy}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("envGroups.renameSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={dialog === "move"}
        onOpenChange={(isOpen) => !isOpen && setDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("envGroups.moveTitle")}</DialogTitle>
            <DialogDescription>
              {t("envGroups.moveDescription")}
            </DialogDescription>
          </DialogHeader>
          <ScopeSelect
            id="env-group-move-target"
            value={moveTarget}
            environments={environments}
            onValueChange={setMoveTarget}
          />
          <DialogFooter>
            <CancelButton busy={busy} onClick={() => setDialog(null)} />
            <Button
              onClick={() => void handleMove()}
              disabled={
                busy || moveTarget === (group.environmentId ?? WORKSPACE_SCOPE)
              }
            >
              {busy ? <Loader2 className="animate-spin" /> : <MoveRight />}
              {t("envGroups.moveSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={dialog === "clone"}
        onOpenChange={(isOpen) => !isOpen && setDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("envGroups.cloneTitle")}</DialogTitle>
            <DialogDescription>
              {t("envGroups.cloneDescription")}
            </DialogDescription>
          </DialogHeader>
          <NameField id="env-group-clone-name" name={name} setName={setName} />
          <div className="space-y-2">
            <Label htmlFor="env-group-clone-workspace">
              {t("envGroups.workspaceLabel")}
            </Label>
            <Select
              value={cloneOwnerId}
              onValueChange={(ownerId) => {
                setCloneOwnerId(ownerId);
                setCloneEnvironmentId(WORKSPACE_SCOPE);
              }}
            >
              <SelectTrigger id="env-group-clone-workspace">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {workspaces.map((workspace) => (
                  <SelectItem key={workspace.id} value={workspace.id}>
                    {workspace.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <ScopeSelect
            id="env-group-clone-environment"
            value={cloneEnvironmentId}
            environments={cloneEnvironments}
            loading={remoteCloneScope.loading}
            onValueChange={setCloneEnvironmentId}
          />
          <DialogFooter>
            <CancelButton busy={busy} onClick={() => setDialog(null)} />
            <Button
              onClick={() => void handleClone()}
              disabled={
                !isValidEnvGroupName(name) ||
                !cloneOwnerId ||
                busy ||
                remoteCloneScope.loading
              }
            >
              {busy ? <Loader2 className="animate-spin" /> : <Copy />}
              {t("envGroups.cloneSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={dialog === "delete"}
        onOpenChange={(isOpen) => !isOpen && setDialog(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("envGroups.deleteTitle", { name: group.name })}
            </DialogTitle>
            <DialogDescription>
              {t("envGroups.deleteDescription")}
            </DialogDescription>
          </DialogHeader>
          <SudoCommandField
            id="env-group-delete-confirm"
            promptKey="envGroups.deletePrompt"
            phrase={deletePhrase}
            value={confirmation}
            onValueChange={setConfirmation}
          />
          <DialogFooter>
            <CancelButton busy={busy} onClick={() => setDialog(null)} />
            <Button
              variant="destructive"
              onClick={() => void handleDelete()}
              disabled={confirmation !== deletePhrase || busy}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("envGroups.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function NameField({
  id,
  name,
  setName,
}: {
  id: string;
  name: string;
  setName: (name: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{t("envGroups.nameLabel")}</Label>
      <Input
        id={id}
        value={name}
        onChange={(event) => setName(event.target.value)}
        aria-invalid={!isValidEnvGroupName(name)}
        autoComplete="off"
      />
    </div>
  );
}

function ScopeSelect({
  id,
  value,
  environments,
  loading = false,
  onValueChange,
}: {
  id: string;
  value: string;
  environments: EnvironmentView[];
  loading?: boolean;
  onValueChange: (value: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{t("envGroups.environmentLabel")}</Label>
      <Select value={value} onValueChange={onValueChange} disabled={loading}>
        <SelectTrigger id={id}>
          <SelectValue placeholder={t("envGroups.environmentPlaceholder")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={WORKSPACE_SCOPE}>
            {t("envGroups.workspaceScope")}
          </SelectItem>
          {environments.map((environment) => (
            <SelectItem key={environment.id} value={environment.id}>
              {environment.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

function CancelButton({
  busy,
  onClick,
}: {
  busy: boolean;
  onClick: () => void;
}) {
  const { t } = useTranslations();
  return (
    <Button variant="outline" onClick={onClick} disabled={busy}>
      {t("envGroups.cancel")}
    </Button>
  );
}

import { useRef, useState } from "react";
import { Download, Eye, Pencil, Plus, Trash2, Upload } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Switch } from "@/common/components/ui/switch";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { downloadTextFile } from "@/common/lib/download-file";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  useAgentSkills,
  type AgentSkillSummary,
} from "@/features/agent-environment/hooks/use-agent-skills";
import { AgentSkillsCardSkeleton } from "@/features/agent-environment/components/agent-skills-card-skeleton";
import {
  AgentSkillDialog,
  type AgentSkillDialogTarget,
} from "@/features/agent-environment/components/agent-skill-dialog";
import {
  isUserOnly,
  skillErrorKey,
  skillFileName,
} from "@/features/agent-environment/lib/skills";
import type { AgentSkillInput } from "@/graphql/definitions";

/**
 * Settings → Agent skills (ADR099 D6): the workspace's SKILL.md documents every
 * agent session receives — Devin's Knowledge and Playbooks as Devin now ships
 * them. Members see the list and can open a skill; only admins create, edit,
 * enable, import or delete.
 */
export function AgentSkillsCard({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const { canManage } = useCapabilities();
  const {
    skills,
    loading,
    error,
    refetch,
    read,
    create,
    update,
    remove,
    saving,
  } = useAgentSkills(ownerId);
  const [dialog, setDialog] = useState<AgentSkillDialogTarget | null>(null);
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  if (loading) return <AgentSkillsCardSkeleton />;

  async function open(skill: AgentSkillSummary) {
    setErrorKey(null);
    try {
      const detail = await read(skill.id);
      setDialog({ kind: canManage ? "edit" : "view", skill: detail });
    } catch {
      setErrorKey("agentEnvironment.skills.readFailed");
    }
  }

  async function exportSkill(skill: AgentSkillSummary) {
    setErrorKey(null);
    try {
      const detail = await read(skill.id);
      downloadTextFile(
        skillFileName(detail.name),
        detail.content ?? "",
        "text/markdown;charset=utf-8",
      );
    } catch {
      setErrorKey("agentEnvironment.skills.readFailed");
    }
  }

  async function importFile(file: File) {
    setErrorKey(null);
    try {
      setDialog({ kind: "create", content: await file.text() });
    } catch {
      setErrorKey("agentEnvironment.skills.importFailed");
    }
  }

  async function toggle(skill: AgentSkillSummary, enabled: boolean) {
    setErrorKey(null);
    try {
      await update(skill.id, { enabled, ifVersion: skill.version });
    } catch (err) {
      setErrorKey(skillErrorKey(err));
    }
  }

  async function save(input: AgentSkillInput) {
    if (!dialog) return;
    if (dialog.kind === "create") await create(input);
    else await update(dialog.skill.id, input);
    toast.success(
      t("agentEnvironment.skills.saved", {
        name: input.name ?? (dialog.kind === "create" ? "" : dialog.skill.name),
      }),
    );
    setDialog(null);
  }

  async function confirmDelete(skill: AgentSkillSummary) {
    setErrorKey(null);
    try {
      await remove(skill.id);
      toast.success(t("agentEnvironment.skills.deleted", { name: skill.name }));
    } catch (err) {
      setErrorKey(skillErrorKey(err));
    } finally {
      setDeleting(null);
    }
  }

  return (
    <Card data-testid="agent-skills-card">
      <CardHeader>
        <CardTitle>{t("agentEnvironment.skills.title")}</CardTitle>
        <CardDescription>
          {t("agentEnvironment.skills.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && skills.length === 0 ? (
          <Alert variant="destructive">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              <span>{t("agentEnvironment.skills.loadFailed")}</span>
              <Button
                size="sm"
                variant="outline"
                onClick={() => void refetch()}
              >
                {t("agentEnvironment.retry")}
              </Button>
            </AlertDescription>
          </Alert>
        ) : skills.length === 0 ? (
          <p className="text-muted-foreground flex h-14 items-center text-sm">
            {t("agentEnvironment.skills.empty")}
          </p>
        ) : (
          <ul className="divide-y rounded-md border" data-testid="agent-skills">
            {skills.map((skill) => (
              <li
                key={skill.id}
                className="flex h-14 items-center gap-3 pr-1 pl-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-mono text-sm">
                      {skill.name}
                    </span>
                    {isUserOnly(skill.triggers) ? (
                      <Badge variant="outline" className="shrink-0">
                        {t("agentEnvironment.skills.onlyWhenAsked")}
                      </Badge>
                    ) : null}
                  </div>
                  <p className="text-muted-foreground truncate text-xs">
                    {skill.description}
                  </p>
                </div>
                <Switch
                  checked={skill.enabled}
                  onCheckedChange={(enabled) => void toggle(skill, enabled)}
                  disabled={!canManage || saving}
                  aria-label={t("agentEnvironment.skills.enabledLabel", {
                    name: skill.name,
                  })}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  aria-label={t(
                    canManage
                      ? "agentEnvironment.skills.editLabel"
                      : "agentEnvironment.skills.viewLabel",
                    { name: skill.name },
                  )}
                  onClick={() => void open(skill)}
                >
                  {canManage ? (
                    <Pencil className="size-4" />
                  ) : (
                    <Eye className="size-4" />
                  )}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  aria-label={t("agentEnvironment.skills.exportLabel", {
                    name: skill.name,
                  })}
                  onClick={() => void exportSkill(skill)}
                >
                  <Download className="size-4" />
                </Button>
                {canManage ? (
                  <ConfirmDialog
                    open={deleting === skill.id}
                    onOpenChange={(o) => setDeleting(o ? skill.id : null)}
                    trigger={
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t("agentEnvironment.skills.deleteLabel", {
                          name: skill.name,
                        })}
                        disabled={saving}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    }
                    title={t("agentEnvironment.skills.deleteTitle", {
                      name: skill.name,
                    })}
                    description={t("agentEnvironment.skills.deleteBody")}
                    confirmLabel={t("agentEnvironment.skills.deleteConfirm")}
                    destructive
                    closeOnConfirm={false}
                    pending={saving && deleting === skill.id}
                    onConfirm={() => void confirmDelete(skill)}
                  />
                ) : null}
              </li>
            ))}
          </ul>
        )}
        {/* Rendered for everyone, disabled for non-admins, so the loaded card
            keeps its skeleton's geometry. */}
        <div className="space-y-2 border-t pt-4">
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={!canManage}
              onClick={() => {
                setErrorKey(null);
                setDialog({ kind: "create" });
              }}
            >
              <Plus />
              {t("agentEnvironment.skills.new")}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!canManage}
              onClick={() => fileInput.current?.click()}
            >
              <Upload />
              {t("agentEnvironment.skills.import")}
            </Button>
            <input
              ref={fileInput}
              type="file"
              accept=".md,text/markdown,text/plain"
              className="hidden"
              data-testid="agent-skill-import"
              onChange={(e) => {
                const file = e.target.files?.[0];
                e.target.value = "";
                if (file) void importFile(file);
              }}
            />
          </div>
          <p className="text-muted-foreground text-xs">
            {canManage
              ? t("agentEnvironment.skills.help")
              : t("agentEnvironment.skills.readOnly")}
          </p>
          {errorKey ? (
            <p className="text-destructive text-sm" role="alert">
              {t(errorKey)}
            </p>
          ) : null}
        </div>
      </CardContent>
      {dialog ? (
        <AgentSkillDialog
          key={dialog.kind === "create" ? "create" : dialog.skill.id}
          target={dialog}
          onOpenChange={(o) => {
            if (!o) setDialog(null);
          }}
          onSave={save}
          saving={saving}
        />
      ) : null}
    </Card>
  );
}

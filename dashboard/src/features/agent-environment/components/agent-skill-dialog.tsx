import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Switch } from "@/common/components/ui/switch";
import { Textarea } from "@/common/components/ui/textarea";
import { useTranslations } from "@/common/hooks/use-translations";
import type { AgentSkillInput } from "@/graphql/definitions";
import type { AgentSkillDetail } from "@/features/agent-environment/hooks/use-agent-skills";
import {
  isUserOnly,
  skillBodyProblem,
  skillDescriptionProblem,
  skillErrorKey,
  skillNameProblem,
  skillTemplate,
  triggersFor,
} from "@/features/agent-environment/lib/skills";

/** What the dialog opens on: a new skill (optionally an imported file), or an existing one. */
export type AgentSkillDialogTarget =
  | { kind: "create"; content?: string }
  | { kind: "edit" | "view"; skill: AgentSkillDetail };

interface AgentSkillDialogProps {
  target: AgentSkillDialogTarget;
  onOpenChange: (open: boolean) => void;
  onSave: (input: AgentSkillInput) => Promise<void>;
  saving: boolean;
}

/**
 * Create, edit or view one workspace skill (ADR099 D6). Fields by default; the
 * whole SKILL.md (frontmatter plus body) when importing or by choice, so a skill
 * moves between a repository and the workspace unchanged. An edit carries the
 * version it opened, and a concurrent change is refused (AGENT_SKILL_CONFLICT).
 */
export function AgentSkillDialog({
  target,
  onOpenChange,
  onSave,
  saving,
}: AgentSkillDialogProps) {
  const { t } = useTranslations();
  const existing = target.kind === "create" ? null : target.skill;
  const readOnly = target.kind === "view";
  const [asFile, setAsFile] = useState(
    target.kind === "create" && target.content != null,
  );
  const [content, setContent] = useState(
    target.kind === "create"
      ? (target.content ?? skillTemplate())
      : (existing?.content ?? ""),
  );
  const [name, setName] = useState(existing?.name ?? "");
  const [description, setDescription] = useState(existing?.description ?? "");
  const [userOnly, setUserOnly] = useState(
    existing ? isUserOnly(existing.triggers) : false,
  );
  const [body, setBody] = useState(existing?.body ?? "");
  const [errorKey, setErrorKey] = useState<string | null>(null);

  const trimmedName = name.trim();
  const problem = asFile
    ? content.trim() === ""
      ? "agentEnvironment.skills.invalid"
      : null
    : (skillNameProblem(trimmedName) ??
      skillDescriptionProblem(description) ??
      skillBodyProblem(body));
  const touched = asFile || trimmedName !== "" || description !== "";
  const shownError = (touched ? problem : null) ?? errorKey;
  const canSave = !readOnly && !saving && problem == null;

  async function save() {
    setErrorKey(null);
    const input: AgentSkillInput = asFile
      ? { content }
      : {
          name: trimmedName,
          description: description.trim(),
          triggers: triggersFor(userOnly),
          body,
        };
    if (existing) input.ifVersion = existing.version;
    try {
      await onSave(input);
    } catch (err) {
      setErrorKey(skillErrorKey(err));
    }
  }

  const title =
    target.kind === "create"
      ? t("agentEnvironment.skills.createTitle")
      : t(
          readOnly
            ? "agentEnvironment.skills.viewTitle"
            : "agentEnvironment.skills.editTitle",
          { name: existing?.name ?? "" },
        );

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl" data-testid="agent-skill-dialog">
        <DialogHeader>
          <DialogTitle className="font-mono">{title}</DialogTitle>
          <DialogDescription>
            {t("agentEnvironment.skills.dialogDescription")}
          </DialogDescription>
        </DialogHeader>
        <form
          id="agent-skill-form"
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (canSave) void save();
          }}
        >
          {asFile ? (
            <div className="space-y-1.5">
              <Label htmlFor="agent-skill-content">
                {t("agentEnvironment.skills.contentLabel")}
              </Label>
              <Textarea
                id="agent-skill-content"
                value={content}
                onChange={(e) => {
                  setContent(e.target.value);
                  setErrorKey(null);
                }}
                readOnly={readOnly}
                spellCheck={false}
                className="h-80 resize-y font-mono text-xs"
              />
            </div>
          ) : (
            <>
              <div className="grid gap-4 sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)]">
                <div className="space-y-1.5">
                  <Label htmlFor="agent-skill-name">
                    {t("agentEnvironment.skills.nameLabel")}
                  </Label>
                  <Input
                    id="agent-skill-name"
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value);
                      setErrorKey(null);
                    }}
                    readOnly={readOnly}
                    placeholder={t("agentEnvironment.skills.namePlaceholder")}
                    spellCheck={false}
                    autoCapitalize="off"
                    autoComplete="off"
                    className="font-mono"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="agent-skill-description">
                    {t("agentEnvironment.skills.descriptionLabel")}
                  </Label>
                  <Input
                    id="agent-skill-description"
                    value={description}
                    onChange={(e) => {
                      setDescription(e.target.value);
                      setErrorKey(null);
                    }}
                    readOnly={readOnly}
                    placeholder={t(
                      "agentEnvironment.skills.descriptionPlaceholder",
                    )}
                  />
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Switch
                  id="agent-skill-user-only"
                  checked={userOnly}
                  onCheckedChange={setUserOnly}
                  disabled={readOnly}
                />
                <div className="space-y-0.5">
                  <Label htmlFor="agent-skill-user-only">
                    {t("agentEnvironment.skills.onlyWhenAskedLabel")}
                  </Label>
                  <p className="text-muted-foreground text-xs">
                    {t("agentEnvironment.skills.onlyWhenAskedHint")}
                  </p>
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="agent-skill-body">
                  {t("agentEnvironment.skills.bodyLabel")}
                </Label>
                <Textarea
                  id="agent-skill-body"
                  value={body}
                  onChange={(e) => {
                    setBody(e.target.value);
                    setErrorKey(null);
                  }}
                  readOnly={readOnly}
                  placeholder={t("agentEnvironment.skills.bodyPlaceholder")}
                  className="h-56 resize-y font-mono text-xs"
                />
              </div>
            </>
          )}
          {shownError ? (
            <p className="text-destructive text-sm" role="alert">
              {t(shownError)}
            </p>
          ) : null}
        </form>
        <DialogFooter className="gap-2 sm:justify-between">
          {readOnly ? (
            <span />
          ) : (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setAsFile((v) => !v);
                setErrorKey(null);
              }}
            >
              {asFile
                ? t("agentEnvironment.skills.editAsFields")
                : t("agentEnvironment.skills.editAsFile")}
            </Button>
          )}
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {readOnly
                ? t("agentEnvironment.skills.close")
                : t("agentEnvironment.skills.cancel")}
            </Button>
            {readOnly ? null : (
              <Button type="submit" form="agent-skill-form" disabled={!canSave}>
                {saving ? <Loader2 className="animate-spin" /> : null}
                {t("agentEnvironment.skills.save")}
              </Button>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

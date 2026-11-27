import { useState } from "react";
import { Check, Pencil, X } from "lucide-react";
import { useRouter } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Switch } from "@/common/components/ui/switch";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { RelativeAge } from "@/common/components/relative-time";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { ValidatePanel } from "@/features/blueprints/components/validate-panel";
import { useUpdateBlueprint } from "@/features/blueprints/hooks/use-update-blueprint";
import { useDisconnectBlueprint } from "@/features/blueprints/hooks/use-disconnect-blueprint";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";

/** Client-side mirror of the backend's approvedBlueprintPath rule (m19):
 * clean repo-relative .yaml/.yml, no escapes. The backend re-validates. */
function validBlueprintPath(path: string): boolean {
  const p = path.trim();
  if (!p || p.startsWith("/") || p.includes("\\")) return false;
  if (p.split("/").some((seg) => seg === "" || seg === "." || seg === "..")) {
    return false;
  }
  return /\.(yaml|yml)$/.test(p);
}

/** Inline value editor for the metadata card (w8/m21 t003): pencil toggles
 * an input; save PATCHes only the changed field. */
function InlineEdit({
  value,
  label,
  mono,
  validate,
  invalidMessage,
  onSave,
  disabled,
}: {
  value: string;
  label: string;
  mono?: boolean;
  validate?: (next: string) => boolean;
  invalidMessage?: string;
  onSave: (next: string) => Promise<void>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const invalid = editing && validate ? !validate(draft) : false;
  if (!editing) {
    return (
      <span className="flex items-center gap-1.5">
        <span className={mono ? "font-mono font-medium" : "font-medium"}>
          {value}
        </span>
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t("blueprints.editField", { field: label })}
          disabled={disabled}
          onClick={() => {
            setDraft(value);
            setEditing(true);
          }}
        >
          <Pencil className="size-3.5" />
        </Button>
      </span>
    );
  }
  return (
    <span className="flex flex-col gap-1">
      <span className="flex items-center gap-1.5">
        <Input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          className={mono ? "h-8 font-mono" : "h-8"}
          aria-label={label}
          autoFocus
        />
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t("blueprints.saveField", { field: label })}
          disabled={disabled || invalid || draft.trim() === ""}
          onClick={() => {
            const next = draft.trim();
            setEditing(false);
            if (next !== value) void onSave(next);
          }}
        >
          <Check className="size-3.5" />
        </Button>
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t("blueprints.cancelEdit")}
          onClick={() => setEditing(false)}
        >
          <X className="size-3.5" />
        </Button>
      </span>
      {invalid && invalidMessage ? (
        <span className="text-xs text-destructive">{invalidMessage}</span>
      ) : null}
    </span>
  );
}

/**
 * A Blueprint's Settings page, Render's `/blueprint/<id>/settings` (w3/m677):
 * the metadata card (name, repository, branch, manifest path, auto-sync and
 * bex's created and updated dates), then bex's manifest and Validate, then
 * Disconnect last, where Render puts Disconnect Blueprint. w3/m679 makes the
 * card Render's Settings card and Disconnect Render's modal.
 */
export function BlueprintSettingsPage() {
  const { t } = useTranslations();
  const router = useRouter();
  const { blueprint } = useBlueprintDetail();
  const blueprintId = blueprint.id;
  const { update, busy: updateBusy } = useUpdateBlueprint();
  const { disconnect, busy: disconnectBusy } = useDisconnectBlueprint();
  // bex-api disconnects a Blueprint under can_create, as Render gates
  // Disconnect Blueprint on updateBlueprint.
  const gate = useCapabilityGate("canCreate");
  const [disconnecting, setDisconnecting] = useState(false);
  const busy = updateBusy || disconnectBusy;

  async function save(patch: Parameters<typeof update>[1]) {
    await update(blueprintId, patch);
    void router.invalidate();
  }

  async function handleDisconnect() {
    setDisconnecting(false);
    const ok = await disconnect(blueprintId);
    if (ok) {
      void router.navigate({ to: "/blueprints", replace: true });
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            <InlineEdit
              value={blueprint.name}
              label={t("blueprints.createNameLabel")}
              onSave={(next) => save({ name: next })}
              disabled={busy}
            />
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm sm:grid-cols-3">
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaRepo")}
              </dt>
              <dd className="truncate font-medium">{blueprint.repo}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaBranch")}
              </dt>
              <dd className="font-medium">{blueprint.branch}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaPath")}
              </dt>
              <dd>
                <InlineEdit
                  value={blueprint.path || "render.yaml"}
                  label={t("blueprints.metaPath")}
                  mono
                  validate={validBlueprintPath}
                  invalidMessage={t("blueprints.pathInvalid")}
                  onSave={(next) => save({ path: next })}
                  disabled={busy}
                />
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaAutoSync")}
              </dt>
              <dd className="flex items-center gap-2 font-medium">
                <Switch
                  checked={blueprint.autoSync}
                  onCheckedChange={(v) => void save({ autoSync: v })}
                  disabled={busy}
                  aria-label={t("blueprints.metaAutoSync")}
                />
                <span>
                  {blueprint.autoSync
                    ? t("blueprints.autoSyncOn")
                    : t("blueprints.autoSyncOff")}
                </span>
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaCreated")}
              </dt>
              <dd className="font-medium">
                <RelativeAge value={blueprint.createdAt} />
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">
                {t("blueprints.metaUpdated")}
              </dt>
              <dd className="font-medium">
                <RelativeAge value={blueprint.lastSync} />
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            {t("blueprints.manifestTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs leading-relaxed">
            <code>{blueprint.manifest || "—"}</code>
          </pre>
        </CardContent>
      </Card>

      <ValidatePanel manifest={blueprint.manifest} blueprintId={blueprintId} />

      <div>
        <PermissionTooltip reason={gate.denied ? gate.reason : undefined}>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setDisconnecting(true)}
            disabled={gate.denied || busy}
          >
            {t("blueprints.disconnectButton")}
          </Button>
        </PermissionTooltip>
      </div>

      <ConfirmDialog
        open={disconnecting}
        onOpenChange={setDisconnecting}
        title={t("blueprints.disconnectTitle")}
        description={t("blueprints.disconnectBody")}
        cancelLabel={t("blueprints.disconnectCancel")}
        confirmLabel={t("blueprints.disconnectAction")}
        onConfirm={() => void handleDisconnect()}
      />
    </>
  );
}

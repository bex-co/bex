import { useState } from "react";
import { Loader2, Plus } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { LabelError } from "@/common/components/label-error";
import {
  isInvalidLabel,
  isValidLabel,
} from "@/common/lib/utils/resource-label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCreateRegistryCredential } from "@/features/registry-credentials/hooks/use-create-registry-credential";
import { CredentialFieldError } from "@/features/registry-credentials/components/credential-field-error";
import {
  MessageWithNodes,
  RegistryHelp,
} from "@/features/registry-credentials/components/registry-help";
import {
  authTokenError,
  usernameError,
} from "@/features/registry-credentials/lib/credential-fields";
import {
  REGISTRY_HOST_EXAMPLES,
  REGISTRY_LABELS,
  RENDER_REGISTRIES,
  hostForRegistryError,
  isRenderRegistry,
  type RenderRegistry,
} from "@/features/registry-credentials/lib/registries";

/** bex's page on private registries, where Render links its own. */
const PRIVATE_REGISTRY_DOCS =
  "https://bex.co/docs/platform/docker-deploys#use-a-private-registry";

export interface CreateRegistryCredentialDialogProps {
  /** Called with the new credential's id once a create succeeds (before the dialog is dismissed). */
  onCreated: (id: string) => void;
  /**
   * Controlled open state, for a caller that opens the dialog itself (a
   * Credential dropdown's Add credential). Without it the dialog renders its
   * own Add credential button.
   */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

/**
 * Render's Add Credential dialog (w4/m670; w2/m14/t006 before it): Name,
 * Registry, Username and Personal Access Token, in Render's order and words
 * (docs/render-artifacts/registry-credential-dialog.md). It sends Render's
 * `registry`; Google Artifact and ECR also ask for the registry host, bex's
 * `host` extension, because the pull secret is keyed by host and theirs name
 * a project or account. The secret is something the caller typed, so there is
 * nothing to reveal back: a single-step form.
 */
export function CreateRegistryCredentialDialog({
  onCreated,
  open: controlledOpen,
  onOpenChange,
}: CreateRegistryCredentialDialogProps) {
  const { t } = useTranslations();
  const { create, busy } = useCreateRegistryCredential();

  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const [name, setName] = useState("");
  const [registry, setRegistry] = useState<RenderRegistry | "">("");
  const [host, setHost] = useState("");
  const [username, setUsername] = useState("");
  const [authToken, setAuthToken] = useState("");

  const hostExample = registry ? REGISTRY_HOST_EXAMPLES[registry] : undefined;
  const needsHost = hostExample !== undefined;
  // The fields the server refuses are marked in place and hold Add (w21/053).
  const hostReason =
    registry && needsHost ? hostForRegistryError(host, registry) : null;
  const usernameReason = usernameError(username);
  const tokenReason = authTokenError(authToken);
  const canSubmit =
    isValidLabel(name) &&
    registry !== "" &&
    (!needsHost || (host.trim() !== "" && !hostReason)) &&
    username.trim() &&
    authToken.trim() &&
    !usernameReason &&
    !tokenReason;

  function reset() {
    setName("");
    setRegistry("");
    setHost("");
    setUsername("");
    setAuthToken("");
  }

  function setOpen(next: boolean) {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
    if (!next) reset();
  }

  async function handleSubmit() {
    if (!canSubmit || busy) return;
    const id = await create({
      registry,
      host: needsHost ? host.trim() : undefined,
      username: username.trim(),
      authToken: authToken.trim(),
      name: name.trim(),
    });
    if (id) {
      onCreated(id);
      setOpen(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {controlled ? null : (
        <DialogTrigger asChild>
          <Button variant="outline" size="sm">
            <Plus /> {t("registryCredentials.create")}
          </Button>
        </DialogTrigger>
      )}
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("registryCredentials.createTitle")}</DialogTitle>
          <DialogDescription>
            <MessageWithNodes
              id="registryCredentials.createDescription"
              nodes={{
                link: (
                  <a
                    href={PRIVATE_REGISTRY_DOCS}
                    target="_blank"
                    rel="noreferrer"
                    className="text-primary hover:underline"
                  >
                    {t("registryCredentials.createDescriptionLink")}
                  </a>
                ),
              }}
            />
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="rc-name">
              {t("registryCredentials.fieldName")}
            </Label>
            <Input
              id="rc-name"
              aria-invalid={isInvalidLabel(name)}
              aria-describedby={
                isInvalidLabel(name) ? "rc-name-error" : undefined
              }
              maxLength={100}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("registryCredentials.fieldNamePlaceholder")}
              autoComplete="off"
              autoFocus
            />
            <LabelError id="rc-name-error" value={name} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rc-registry">
              {t("registryCredentials.fieldRegistry")}
            </Label>
            <Select
              value={registry}
              onValueChange={(next) => {
                if (isRenderRegistry(next)) setRegistry(next);
              }}
            >
              <SelectTrigger id="rc-registry" className="w-full">
                <SelectValue
                  placeholder={t(
                    "registryCredentials.fieldRegistryPlaceholder",
                  )}
                />
              </SelectTrigger>
              <SelectContent>
                {RENDER_REGISTRIES.map((value) => (
                  <SelectItem key={value} value={value}>
                    {REGISTRY_LABELS[value]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {registry ? <RegistryHelp registry={registry} name={name} /> : null}
          </div>
          {registry && needsHost ? (
            <div className="space-y-2">
              <Label htmlFor="rc-host">
                {t("registryCredentials.fieldHost")}
              </Label>
              <Input
                id="rc-host"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder={hostExample}
                autoComplete="off"
                aria-invalid={hostReason !== null}
                aria-describedby={hostReason ? "rc-host-error" : "rc-host-hint"}
                className="font-mono text-sm"
              />
              {hostReason ? (
                <p
                  id="rc-host-error"
                  role="alert"
                  className="text-destructive text-xs"
                >
                  {t(hostReason, {
                    registry: REGISTRY_LABELS[registry],
                    example: hostExample,
                  })}
                </p>
              ) : (
                <p id="rc-host-hint" className="text-muted-foreground text-xs">
                  {t("registryCredentials.fieldHostHint")}
                </p>
              )}
            </div>
          ) : null}
          <div className="space-y-2">
            <Label htmlFor="rc-username">
              {t("registryCredentials.fieldUsername")}
            </Label>
            <Input
              id="rc-username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="off"
              aria-invalid={usernameReason !== null}
              aria-describedby={
                usernameReason ? "rc-username-error" : undefined
              }
              className="font-mono text-sm"
            />
            <CredentialFieldError
              id="rc-username-error"
              reason={usernameReason}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="rc-auth-token">
              {t(
                registry === "DOCKER"
                  ? "registryCredentials.fieldAuthTokenOrPassword"
                  : "registryCredentials.fieldAuthToken",
              )}
            </Label>
            <Input
              id="rc-auth-token"
              type="password"
              value={authToken}
              onChange={(e) => setAuthToken(e.target.value)}
              autoComplete="new-password"
              aria-invalid={tokenReason !== null}
              aria-describedby={tokenReason ? "rc-auth-token-error" : undefined}
              className="font-mono text-sm"
              onKeyDown={(e) => {
                if (e.key === "Enter" && canSubmit) void handleSubmit();
              }}
            />
            <CredentialFieldError
              id="rc-auth-token-error"
              reason={tokenReason}
            />
          </div>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={busy}>
              {t("registryCredentials.createCancel")}
            </Button>
          </DialogClose>
          <Button
            onClick={() => void handleSubmit()}
            disabled={!canSubmit || busy}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("registryCredentials.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

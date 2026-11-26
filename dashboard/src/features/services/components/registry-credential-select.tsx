import { useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { Label } from "@/common/components/ui/label";
import { CREATE_SELECT_CLASS } from "@/common/components/create-form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { CreateRegistryCredentialDialog } from "@/features/registry-credentials/components/create-registry-credential-dialog";
import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";

const NONE_VALUE = "__none__";
const ADD_VALUE = "__add__";

export interface RegistryCredentialSelectProps {
  id: string;
  value: string;
  onValueChange: (value: string) => void;
  description: string;
  disabled?: boolean;
  /** Leave the label to the caller: a create row names the select itself. */
  hideLabel?: boolean;
}

/**
 * Shared private-registry picker. The UI sentinels never cross the component
 * boundary: callers receive an explicit empty string for "None", matching the
 * backend's clear/no-auth semantics.
 *
 * As Render's Credential dropdown does, it starts with Add credential (w4/m670;
 * docs/render-artifacts/registry-credential-dialog.md): the Add Credential
 * dialog opens over the page, and the new credential becomes the selection,
 * so a half-filled form is never left behind. Only members who may create a
 * credential (bex-api's `can_manage`) see the entry.
 */
export function RegistryCredentialSelect({
  id,
  value,
  onValueChange,
  description,
  disabled = false,
  hideLabel = false,
}: RegistryCredentialSelectProps) {
  const { t } = useTranslations();
  const { credentials, loading, error, refetch } = useRegistryCredentials();
  const { canManage } = useCapabilities();
  const [adding, setAdding] = useState(false);
  // Set while Add credential closes the list, so the list does not hand focus
  // back to its trigger behind the dialog that just took it.
  const openingDialog = useRef(false);
  const descriptionId = `${id}-description`;

  return (
    <div className="space-y-2">
      {hideLabel ? null : (
        <Label htmlFor={id}>
          {t("services.createRegistryCredentialLabel")}
        </Label>
      )}
      <Select
        value={value || NONE_VALUE}
        onValueChange={(next) => {
          // Add credential is an action, not a value: the selection holds
          // until the dialog creates one.
          if (next === ADD_VALUE) {
            openingDialog.current = true;
            setAdding(true);
          } else onValueChange(next === NONE_VALUE ? "" : next);
        }}
        disabled={disabled || loading || error != null}
      >
        <SelectTrigger
          id={id}
          aria-describedby={descriptionId}
          // Full width, 44 px on a phone, wherever the picker appears.
          className={CREATE_SELECT_CLASS}
        >
          <SelectValue
            placeholder={t("services.createRegistryCredentialPlaceholder")}
          />
        </SelectTrigger>
        <SelectContent
          onCloseAutoFocus={(event) => {
            if (!openingDialog.current) return;
            openingDialog.current = false;
            event.preventDefault();
          }}
        >
          {canManage ? (
            <>
              <SelectItem value={ADD_VALUE}>
                <Plus /> {t("services.registryCredentialAdd")}
              </SelectItem>
              <SelectSeparator />
            </>
          ) : null}
          <SelectItem value={NONE_VALUE}>
            {t("services.createRegistryCredentialNone")}
          </SelectItem>
          {credentials.map((credential) => (
            <SelectItem key={credential.id} value={credential.id}>
              {credential.name} ({credential.host})
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p
        id={descriptionId}
        // Two lines held: the copy while the list loads and the copy once it
        // has (often one line) swap without moving the rows below.
        className="text-muted-foreground min-h-10 text-sm"
      >
        {error
          ? t("services.registryCredentialListError")
          : credentials.length === 0 && !loading
            ? t("services.registryCredentialEmpty")
            : description}{" "}
        <Link
          to="/workspace/settings"
          hash="registry-credentials"
          className="text-primary relative hover:underline after:absolute after:inset-x-0 after:-inset-y-3.5 after:content-['']"
        >
          {t("services.registryCredentialManage")}
        </Link>
      </p>
      {/* Mounted only while open, so a page that never adds one never needs
          the create mutation. */}
      {adding ? (
        <CreateRegistryCredentialDialog
          open
          onOpenChange={setAdding}
          onCreated={(created) => {
            void refetch();
            onValueChange(created);
          }}
        />
      ) : null}
    </div>
  );
}

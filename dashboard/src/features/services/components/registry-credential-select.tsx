import { Link } from "@tanstack/react-router";
import { Label } from "@/common/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRegistryCredentials } from "@/features/registry-credentials/hooks/use-registry-credentials";

const NONE_VALUE = "__none__";

export interface RegistryCredentialSelectProps {
  id: string;
  value: string;
  onValueChange: (value: string) => void;
  description: string;
  disabled?: boolean;
}

/**
 * Shared private-registry picker. The UI sentinel never crosses the component
 * boundary: callers receive an explicit empty string for "None", matching the
 * backend's clear/no-auth semantics.
 */
export function RegistryCredentialSelect({
  id,
  value,
  onValueChange,
  description,
  disabled = false,
}: RegistryCredentialSelectProps) {
  const { t } = useTranslations();
  const { credentials, loading, error } = useRegistryCredentials();
  const descriptionId = `${id}-description`;

  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{t("services.createRegistryCredentialLabel")}</Label>
      <Select
        value={value || NONE_VALUE}
        onValueChange={(next) => onValueChange(next === NONE_VALUE ? "" : next)}
        disabled={disabled || loading || error != null}
      >
        <SelectTrigger id={id} aria-describedby={descriptionId}>
          <SelectValue
            placeholder={t("services.createRegistryCredentialPlaceholder")}
          />
        </SelectTrigger>
        <SelectContent>
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
      <p id={descriptionId} className="text-xs text-muted-foreground">
        {error
          ? t("services.registryCredentialListError")
          : credentials.length === 0 && !loading
            ? t("services.registryCredentialEmpty")
            : description}{" "}
        <Link
          to="/settings"
          hash="registry-credentials"
          className="text-primary hover:underline"
        >
          {t("services.registryCredentialManage")}
        </Link>
      </p>
    </div>
  );
}

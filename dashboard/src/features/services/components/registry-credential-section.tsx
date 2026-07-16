import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { RegistryCredentialSelect } from "@/features/services/components/registry-credential-select";
import { useRegistryCredential } from "@/features/services/hooks/use-registry-credential";

export interface RegistryCredentialSectionProps {
  serviceId: string;
  registryCredentialId: string | null;
  onChanged?: () => void;
}

/** Settings card for an existing-image or repository-Docker service. */
export function RegistryCredentialSection({
  serviceId,
  registryCredentialId,
  onChanged,
}: RegistryCredentialSectionProps) {
  const { t } = useTranslations();
  const current = registryCredentialId ?? "";
  const [draft, setDraft] = useState(current);
  const { setRegistryCredential, busy } = useRegistryCredential();
  const changed = draft !== current;

  async function save() {
    const ok = await setRegistryCredential(serviceId, draft);
    if (ok) onChanged?.();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.registryCredentialSettingsTitle")}</CardTitle>
        <CardDescription>
          {t("services.registryCredentialSettingsDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <RegistryCredentialSelect
          id="service-registry-credential"
          value={draft}
          onValueChange={setDraft}
          description={t("services.createRegistryCredentialDescription")}
          disabled={busy}
        />
        <Button disabled={!changed || busy} onClick={() => void save()}>
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("services.registryCredentialSave")}
        </Button>
      </CardContent>
    </Card>
  );
}

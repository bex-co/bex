import { Plus } from "lucide-react";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The credentials list's frame while it loads (w3/m305): the footprint of the
 * empty state most workspaces settle to, its copy reserved invisibly so it
 * wraps as the ready state does. A workspace with credentials draws a table
 * instead, a data-dependent difference.
 */
export function RegistryCredentialsLoadingFrame() {
  const { t } = useTranslations();
  return (
    <div
      aria-hidden="true"
      data-testid="registry-credentials-loading-frame"
      className="flex flex-col items-center justify-center py-12 text-center"
    >
      <Skeleton className="mb-3 size-8" />
      <p className="relative mb-1 font-medium">
        <ReservedText>{t("registryCredentials.emptyTitle")}</ReservedText>
      </p>
      <p className="relative text-sm">
        <ReservedText>{t("registryCredentials.emptyBody")}</ReservedText>
      </p>
    </div>
  );
}

/** RegistryCredentialsPanel's frame while it loads: its static title and
 *  description, the Add button's box (which narrows the description as the
 *  ready one does), then the list's frame. */
export function RegistryCredentialsPanelSkeleton() {
  const { t } = useTranslations();
  return (
    <Card aria-hidden="true" data-testid="registry-credentials-skeleton">
      <CardHeader>
        <CardTitle>{t("registryCredentials.title")}</CardTitle>
        <CardDescription>
          {t("registryCredentials.description")}
        </CardDescription>
        <CardAction>
          <ReservedButton variant="outline" size="sm">
            <Plus /> {t("registryCredentials.create")}
          </ReservedButton>
        </CardAction>
      </CardHeader>
      <CardContent>
        <RegistryCredentialsLoadingFrame />
      </CardContent>
    </Card>
  );
}

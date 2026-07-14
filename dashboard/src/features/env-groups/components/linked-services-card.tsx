import { useState } from "react";
import { AlertTriangle, Link2Off, Loader2, Server } from "lucide-react";
import { Link } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvGroupView } from "@/features/env-groups/types";
import type { ServiceView } from "@/features/services/types";

export function LinkedServicesCard({
  group,
  services,
  linkGroup,
  unlinkGroup,
  busy,
  error,
}: {
  group: EnvGroupView;
  services: ServiceView[];
  linkGroup: (id: string, serviceId: string) => Promise<boolean>;
  unlinkGroup: (id: string, serviceId: string) => Promise<boolean>;
  busy: boolean;
  error?: Error;
}) {
  const { t } = useTranslations();
  const [selected, setSelected] = useState("");
  const byId = new Map(services.map((service) => [service.id, service]));
  const available = services.filter(
    (service) => !group.serviceLinks.includes(service.id),
  );

  async function handleLink() {
    if (!selected) return;
    if (await linkGroup(group.id, selected)) setSelected("");
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("envGroups.servicesTitle")}</CardTitle>
        <CardDescription>{t("envGroups.servicesDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm"
          >
            <AlertTriangle className="mt-0.5 size-4 shrink-0 text-destructive" />
            <span>{t("envGroups.servicesLoadError")}</span>
          </div>
        ) : available.length > 0 ? (
          <div className="flex flex-wrap items-center gap-2">
            <Select
              value={selected}
              onValueChange={setSelected}
              disabled={busy}
            >
              <SelectTrigger className="min-w-56">
                <SelectValue placeholder={t("envGroups.selectService")} />
              </SelectTrigger>
              <SelectContent>
                {available.map((service) => (
                  <SelectItem key={service.id} value={service.id}>
                    {service.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              size="sm"
              disabled={!selected || busy}
              onClick={() => void handleLink()}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("envGroups.linkButton")}
            </Button>
          </div>
        ) : null}

        {group.serviceLinks.length === 0 ? (
          <div className="flex items-center gap-3 rounded-md border border-dashed p-4 text-sm text-muted-foreground">
            <Server className="size-5" />
            {t("envGroups.noLinkedServices")}
          </div>
        ) : (
          <ul className="divide-y rounded-md border">
            {group.serviceLinks.map((serviceId) => {
              const service = byId.get(serviceId);
              return (
                <li
                  key={serviceId}
                  className="flex items-center justify-between gap-3 px-4 py-3"
                >
                  <div className="min-w-0">
                    <Link
                      to="/services/$serviceId"
                      params={{ serviceId }}
                      className="font-medium hover:underline"
                    >
                      {service?.name ?? serviceId}
                    </Link>
                    {service?.name && service.name !== serviceId ? (
                      <p className="truncate text-xs text-muted-foreground">
                        {serviceId}
                      </p>
                    ) : null}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => void unlinkGroup(group.id, serviceId)}
                  >
                    <Link2Off />
                    {t("envGroups.unlinkButton")}
                  </Button>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

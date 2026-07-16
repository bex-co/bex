import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  AlertTriangle,
  FileLock2,
  KeyRound,
  Layers3,
  Server,
} from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Badge } from "@/common/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";
import { EnvGroupMetadata } from "@/features/env-groups/components/env-group-metadata";
import {
  classifyEnvGroupError,
  useEnvGroups,
} from "@/features/env-groups/hooks/use-env-groups";
import { useServices } from "@/features/services/hooks/use-services";

export const Route = createFileRoute("/env-groups")({
  component: EnvGroupsPage,
  beforeLoad: requireAuth("/env-groups"),
  head: () => ({
    meta: [{ title: "Environment Groups · bex dashboard" }],
  }),
});

export function EnvGroupsPage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { groups, loading, error, refetch } = useEnvGroups();
  const { services, loading: servicesLoading } = useServices();
  const errorKind = classifyEnvGroupError(error);
  const initialLoading = loading && groups.length === 0;

  return (
    <DashboardLayout>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-4 sm:px-6">
        <div>
          <h1 className="text-xl font-semibold">{t("envGroups.pageTitle")}</h1>
          <p className="text-sm text-muted-foreground">
            {t("envGroups.pageDescription")}
          </p>
        </div>
        <NewEnvGroupDialog
          refetch={refetch}
          services={services}
          servicesLoading={servicesLoading}
          onCreated={(groupId) =>
            void navigate({
              to: "/env-groups/$groupId",
              params: { groupId },
            })
          }
        />
      </div>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-5xl space-y-6">
          {errorKind && groups.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <AlertTriangle className="size-8 text-destructive" />
              <p className="font-medium">{t(`envGroups.${errorKind}Title`)}</p>
              <p className="text-sm text-muted-foreground">
                {t(`envGroups.${errorKind}Body`)}
              </p>
            </div>
          ) : initialLoading ? (
            <div className="grid gap-4 md:grid-cols-2">
              <Skeleton className="h-44" />
              <Skeleton className="h-44" />
            </div>
          ) : groups.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-12 text-center">
              <Layers3 className="size-8 text-muted-foreground" />
              <p className="font-medium">{t("envGroups.emptyTitle")}</p>
              <p className="max-w-md text-sm text-muted-foreground">
                {t("envGroups.emptyBody")}
              </p>
            </div>
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {groups.map((group) => (
                <Link
                  key={group.id}
                  to="/env-groups/$groupId"
                  params={{ groupId: group.id }}
                  className="block"
                >
                  <Card className="h-full transition-colors hover:border-foreground/30">
                    <CardHeader>
                      <CardTitle className="break-all text-base">
                        {group.name}
                      </CardTitle>
                      <CardDescription className="font-mono text-xs">
                        {group.id}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="flex flex-wrap gap-2">
                        <Badge variant="secondary">
                          <KeyRound />
                          {t("envGroups.varCount", {
                            count: group.envVarKeys.length,
                          })}
                        </Badge>
                        <Badge variant="secondary">
                          <FileLock2 />
                          {t("envGroups.fileCount", {
                            count: group.secretFileNames.length,
                          })}
                        </Badge>
                        <Badge variant="secondary">
                          <Server />
                          {t("envGroups.serviceCount", {
                            count: group.serviceLinks.length,
                          })}
                        </Badge>
                      </div>
                      <EnvGroupMetadata group={group} />
                    </CardContent>
                  </Card>
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
    </DashboardLayout>
  );
}

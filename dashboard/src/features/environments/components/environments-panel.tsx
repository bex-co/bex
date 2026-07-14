import { useState } from "react";
import { Layers, Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { EnvironmentCard } from "@/features/environments/components/environment-card";
import { NewEnvironmentDialog } from "@/features/environments/components/new-environment-dialog";
import type { LifecycleAction, ServiceView } from "@/features/services/types";
import type { PendingLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface EnvironmentsPanelProps {
  projectId: string;
  /** The workspace's services — resolves each environment's rows and the assign candidates. */
  services: ServiceView[];
  /** The workspace's databases — same role as `services` (w6/m20 extension). */
  databases: DatabaseView[];
  /** The workspace's key-value instances — same role as `services` (w6/m20 extension). */
  keyValues: KeyValueView[];
  servicePending: PendingLifecycle | null;
  onRunServiceAction: (action: LifecycleAction, service: ServiceView) => void;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
}

/**
 * The Environments section on a Project's page: named subsets of the
 * project's resources (staging/production), each a card of assigned
 * services, databases, and key-value instances with create/rename/delete +
 * resource-assignment affordances (docs/ADR032-environments.md, w1/m32;
 * dashboard UX w5/m25; w6/m20 extension for databases/key-value). The
 * environment mutations refetch the `Environments` (and, on assignment,
 * `Projects`) queries by name, so this panel just consumes `useEnvironments`
 * — no refetch plumbing.
 */
export function EnvironmentsPanel({
  projectId,
  services,
  databases,
  keyValues,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: EnvironmentsPanelProps) {
  const { t } = useTranslations();
  const { environments, loading, error } = useEnvironments(projectId);
  const [newOpen, setNewOpen] = useState(false);

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Layers className="size-4 text-muted-foreground" />
          <h2 className="text-lg font-semibold">{t("environments.heading")}</h2>
        </div>
        <Button variant="outline" size="sm" onClick={() => setNewOpen(true)}>
          <Plus />
          {t("environments.newButton")}
        </Button>
      </div>

      {loading ? (
        <Skeleton className="h-24 w-full rounded-xl" />
      ) : error ? (
        <p className="text-sm text-muted-foreground">
          {t("environments.errorBody")}
        </p>
      ) : environments.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            {t("environments.emptyBody")}
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {environments.map((environment) => (
            <EnvironmentCard
              key={environment.id}
              environment={environment}
              services={services}
              databases={databases}
              keyValues={keyValues}
              servicePending={servicePending}
              onRunServiceAction={onRunServiceAction}
              onDatabaseDeleted={onDatabaseDeleted}
              onKeyValueDeleted={onKeyValueDeleted}
            />
          ))}
        </div>
      )}

      <NewEnvironmentDialog
        projectId={projectId}
        open={newOpen}
        onOpenChange={setNewOpen}
      />
    </section>
  );
}

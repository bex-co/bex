import { useState } from "react";
import {
  Plus,
  Layers,
  ShieldAlert,
  AlertTriangle,
  Trash2,
  Loader2,
  Link2,
  Link2Off,
} from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useEnvGroups,
  useEnvGroupMutations,
  classifyEnvGroupError,
} from "@/features/env-groups/hooks/use-env-groups";
import { CenteredState } from "@/features/services/components/centered-state";
import { isValidEnvGroupName } from "@/features/env-groups/lib/validation";
import type { EnvGroupView } from "@/features/env-groups/types";

/**
 * The service Environment tab's Environment Groups section (Render dashboard
 * shape): lists all env groups, shows each group's env-var keys + secret-file
 * names (read-only), and links/unlinks the CURRENT service to a group — all over
 * bex-api's env-groups GraphQL. A group is a reusable bundle shared across
 * services, so the list is service-independent; only membership is per-service.
 */
export function EnvGroupsPanel({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const { groups, loading, error, refetch } = useEnvGroups();
  const { createGroup, deleteGroup, linkGroup, unlinkGroup, busy } =
    useEnvGroupMutations(refetch);

  const errorKind = classifyEnvGroupError(error);
  const initialLoading = loading && groups.length === 0 && !error;
  const gated = errorKind === "unavailable" || errorKind === "forbidden";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.envGroupsTitle")}</CardTitle>
        <CardDescription>{t("services.envGroupsDescription")}</CardDescription>
        <CardAction>
          <CreateGroupButton
            createGroup={createGroup}
            disabled={gated || busy}
          />
        </CardAction>
      </CardHeader>
      <CardContent>
        {errorKind ? (
          <StatePanel kind={errorKind} />
        ) : initialLoading ? (
          <ListSkeleton />
        ) : groups.length === 0 ? (
          <EnvGroupsEmptyState />
        ) : (
          <ul className="divide-y">
            {groups.map((group) => (
              <EnvGroupItem
                key={group.id}
                group={group}
                serviceId={serviceId}
                onLink={linkGroup}
                onUnlink={unlinkGroup}
                onDelete={deleteGroup}
                busy={busy}
              />
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

/** One env-group list item: name, its contents preview, link/unlink + delete. */
function EnvGroupItem({
  group,
  serviceId,
  onLink,
  onUnlink,
  onDelete,
  busy,
}: {
  group: EnvGroupView;
  serviceId: string;
  onLink: (id: string, serviceId: string) => Promise<boolean>;
  onUnlink: (id: string, serviceId: string) => Promise<boolean>;
  onDelete: (id: string) => Promise<boolean>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const linked = group.serviceLinks.includes(serviceId);

  return (
    <li className="flex items-start justify-between gap-4 py-4 first:pt-0 last:pb-0">
      <div className="min-w-0 space-y-2">
        <div className="flex items-center gap-2">
          <span className="font-medium break-all">{group.name}</span>
          {linked && (
            <Badge variant="success">{t("services.envGroupLinked")}</Badge>
          )}
        </div>
        {group.envVarKeys.length === 0 && group.secretFileNames.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {t("services.envGroupEmptyContents")}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1">
            {group.envVarKeys.map((key) => (
              <Badge key={`k-${key}`} variant="secondary" className="font-mono">
                {key}
              </Badge>
            ))}
            {group.secretFileNames.map((name) => (
              <Badge key={`f-${name}`} variant="outline" className="font-mono">
                {name}
              </Badge>
            ))}
          </div>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1">
        <Button
          variant="outline"
          size="sm"
          disabled={busy}
          onClick={() =>
            void (linked
              ? onUnlink(group.id, serviceId)
              : onLink(group.id, serviceId))
          }
        >
          {busy ? (
            <Loader2 className="animate-spin" />
          ) : linked ? (
            <Link2Off />
          ) : (
            <Link2 />
          )}
          {linked ? t("services.envGroupUnlink") : t("services.envGroupLink")}
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              size="icon"
              variant="ghost"
              aria-label={t("services.envGroupDelete")}
              disabled={busy}
            >
              <Trash2 className="text-destructive" />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("services.envGroupDeleteConfirmTitle", { name: group.name })}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("services.envGroupDeleteConfirmBody")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("services.envCancel")}</AlertDialogCancel>
              <AlertDialogAction onClick={() => void onDelete(group.id)}>
                {t("services.envGroupDelete")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </li>
  );
}

/** The "Create group" affordance: a button that opens an inline name form. */
function CreateGroupButton({
  createGroup,
  disabled,
}: {
  createGroup: (name: string) => Promise<string | null>;
  disabled: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [invalid, setInvalid] = useState(false);
  const [saving, setSaving] = useState(false);

  function reset() {
    setName("");
    setInvalid(false);
    setOpen(false);
  }

  async function submit() {
    if (!isValidEnvGroupName(name)) {
      setInvalid(true);
      return;
    }
    setSaving(true);
    const ok = await createGroup(name.trim());
    setSaving(false);
    if (ok) reset();
  }

  if (!open) {
    return (
      <Button
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => setOpen(true)}
      >
        <Plus /> {t("services.envGroupCreate")}
      </Button>
    );
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-2">
        <Input
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            setInvalid(false);
          }}
          placeholder={t("services.envGroupNamePlaceholder")}
          aria-label={t("services.envGroupNameLabel")}
          aria-invalid={invalid}
          className="w-48 text-sm"
          autoFocus
          onKeyDown={(e) => {
            if (e.key === "Enter") void submit();
            if (e.key === "Escape") reset();
          }}
        />
        <Button size="sm" disabled={saving} onClick={() => void submit()}>
          {t("services.envGroupCreateSubmit")}
        </Button>
        <Button size="sm" variant="ghost" onClick={reset}>
          {t("services.envCancel")}
        </Button>
      </div>
      {invalid && (
        <p className="text-destructive text-xs">
          {t("services.envGroupInvalidName")}
        </p>
      )}
    </div>
  );
}

function ListSkeleton() {
  return (
    <div className="space-y-4">
      {[0, 1].map((i) => (
        <div key={i} className="flex items-center justify-between gap-4">
          <div className="flex-1 space-y-2">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-4 w-64" />
          </div>
          <Skeleton className="h-8 w-20" />
        </div>
      ))}
    </div>
  );
}

function EnvGroupsEmptyState() {
  const { t } = useTranslations();
  return (
    <CenteredState
      icon={<Layers />}
      title={t("services.envGroupsEmptyTitle")}
      body={t("services.envGroupsEmptyBody")}
    />
  );
}

/** The unavailable (503) / forbidden (403) / generic error states. */
function StatePanel({
  kind,
}: {
  kind: "unavailable" | "forbidden" | "generic";
}) {
  const { t } = useTranslations();
  const copy = {
    unavailable: {
      icon: <AlertTriangle />,
      title: t("services.envGroupsUnavailableTitle"),
      body: t("services.envGroupsUnavailableBody"),
    },
    forbidden: {
      icon: <ShieldAlert />,
      title: t("services.envGroupsForbiddenTitle"),
      body: t("services.envGroupsForbiddenBody"),
    },
    generic: {
      icon: <AlertTriangle />,
      title: t("services.envGroupsErrorTitle"),
      body: t("services.envGroupsErrorBody"),
    },
  }[kind];
  return <CenteredState icon={copy.icon} title={copy.title} body={copy.body} />;
}

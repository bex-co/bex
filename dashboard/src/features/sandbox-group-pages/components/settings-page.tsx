import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { refusalReason } from "@/common/lib/graphql-error";
import { regionLabel } from "@/common/lib/region-label";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DeleteSandboxGroupDocument,
  SandboxGroupsDocument,
  SetDefaultSandboxGroupDocument,
  UpdateSandboxGroupLimitDocument,
} from "@/graphql/definitions";
import { SandboxGroupCredentialsPanel } from "@/features/sandbox-groups/components/sandbox-group-credentials-panel";
import { useSandboxGroupDetail } from "@/features/sandbox-group-pages/lib/group-detail";
import { DetailRow } from "./parts";

/**
 * A sandbox group's Settings page (w1/m658), as Render's holds the group's
 * settings and its delete: the group's region, concurrency limit and default
 * flag, its brokered credentials, and Delete sandbox group with Render's list
 * of what goes with it. bex serves no rename of a group (w1/m658 t001).
 */
export function SandboxGroupSettingsPage() {
  const { t } = useTranslations();
  const { group, refetch } = useSandboxGroupDetail();
  const navigate = useNavigate();
  const [limit, setLimit] = useState(String(group.concurrencyLimit));
  const [confirming, setConfirming] = useState(false);
  const [saveLimit, saving] = useMutation(UpdateSandboxGroupLimitDocument);
  const [makeDefault, defaulting] = useMutation(
    SetDefaultSandboxGroupDocument,
    {
      refetchQueries: [SandboxGroupsDocument],
    },
  );
  const [deleteGroup, deleting] = useMutation(DeleteSandboxGroupDocument, {
    refetchQueries: [SandboxGroupsDocument],
  });
  const parsed = Number(limit);
  const limitValid = Number.isInteger(parsed) && parsed >= 1;

  const onSaveLimit = async () => {
    const { error } = await saveLimit({
      variables: {
        id: group.id,
        ownerId: group.ownerId,
        concurrencyLimit: parsed,
      },
    });
    if (error) {
      toast.error(refusalReason(error));
      return;
    }
    toast.success(t("sandboxGroupPages.limitSaved"));
    refetch();
  };

  const onMakeDefault = async () => {
    const { error } = await makeDefault({
      variables: { id: group.id, ownerId: group.ownerId },
    });
    if (error) {
      toast.error(refusalReason(error));
      return;
    }
    toast.success(t("sandboxGroupPages.madeDefault", { name: group.name }));
    refetch();
  };

  const onDelete = async () => {
    const { error } = await deleteGroup({
      variables: { id: group.id, ownerId: group.ownerId },
    });
    setConfirming(false);
    if (error) {
      toast.error(refusalReason(error));
      return;
    }
    toast.success(t("sandboxGroupPages.groupDeleted", { name: group.name }));
    void navigate({ to: "/sandbox-groups" });
  };

  const goes = [
    t("sandboxGroupPages.navSandboxes"),
    t("sandboxGroupPages.navSnapshots"),
    t("sandboxGroupPages.navImages"),
    t("sandboxGroupPages.brokeredCredentials"),
  ];

  return (
    <div className="space-y-6">
      <Card data-skeleton-region="group-general" className="gap-3 py-4">
        <CardHeader className="px-4">
          <CardTitle className="text-base">
            {t("sandboxGroupPages.general")}
          </CardTitle>
        </CardHeader>
        <CardContent className="px-4">
          <dl className="divide-y">
            <DetailRow label={t("sandboxGroups.fieldName")}>
              {group.name}
            </DetailRow>
            <DetailRow label={t("sandboxGroups.fieldRegion")}>
              {regionLabel(group.region, t)}
            </DetailRow>
            <DetailRow label={t("sandboxGroups.fieldDefault")}>
              <span className="inline-flex items-center gap-2">
                {group.isDefault
                  ? t("sandboxGroups.defaultYes")
                  : t("sandboxGroups.defaultNo")}
                {!group.isDefault ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={defaulting.loading}
                    onClick={() => void onMakeDefault()}
                  >
                    {t("sandboxGroupPages.makeDefault")}
                  </Button>
                ) : null}
              </span>
            </DetailRow>
          </dl>
          <form
            className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-end"
            onSubmit={(e) => {
              e.preventDefault();
              if (limitValid) void onSaveLimit();
            }}
          >
            <label className="flex flex-1 flex-col gap-1 text-sm">
              {t("sandboxGroups.fieldConcurrencyLimit")}
              <Input
                type="number"
                min={1}
                value={limit}
                onChange={(e) => setLimit(e.target.value)}
                className="h-8 sm:max-w-40"
              />
            </label>
            <Button
              type="submit"
              size="sm"
              disabled={
                !limitValid ||
                saving.loading ||
                parsed === group.concurrencyLimit
              }
            >
              {t("sandboxGroupPages.save")}
            </Button>
          </form>
        </CardContent>
      </Card>

      <div data-skeleton-region="credentials-panel">
        <SandboxGroupCredentialsPanel
          sandboxGroupId={group.id}
          ownerId={group.ownerId}
        />
      </div>

      <Card
        data-skeleton-region="danger-zone"
        className="border-destructive/40 gap-3 py-4"
      >
        <CardHeader className="px-4">
          <CardTitle className="text-base">
            {t("sandboxGroupPages.deleteGroup")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 px-4">
          <p className="text-muted-foreground text-sm">
            {group.isDefault
              ? t("sandboxGroupPages.deleteGroupDefault")
              : t("sandboxGroupPages.deleteGroupHelp")}
          </p>
          <Button
            variant="destructive"
            size="sm"
            disabled={group.isDefault}
            onClick={() => setConfirming(true)}
          >
            {t("sandboxGroupPages.deleteGroup")}
          </Button>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("sandboxGroupPages.deleteGroupTitle")}
        description={
          <span className="block space-y-2">
            <span className="block">
              {t("sandboxGroupPages.deleteGroupDescription", {
                name: group.name,
              })}
            </span>
            <span className="block">
              {goes.map((item) => (
                <span key={item} className="block">
                  • {item}
                </span>
              ))}
            </span>
            <span className="block">
              {t("sandboxGroupPages.deleteGroupStops")}
            </span>
          </span>
        }
        confirmLabel={t("sandboxGroupPages.deleteGroup")}
        destructive
        phrase={`sudo delete sandbox group ${group.name}`}
        pending={deleting.loading}
        closeOnConfirm={false}
        onConfirm={() => void onDelete()}
      />
    </div>
  );
}

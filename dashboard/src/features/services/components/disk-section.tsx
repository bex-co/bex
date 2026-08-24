import { HardDrive, Pencil } from "lucide-react";
import { useState } from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { PanelCenteredState } from "@/common/components/panel-states";
import { CardSkeleton } from "@/common/components/detail-skeletons";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  type DiskView,
  useDisk,
  useDiskMutations,
  useDiskSnapshots,
} from "@/features/services/hooks/use-disks";
import { DatastoreMetricsPanel } from "@/features/metrics/components/datastore-metrics-panel";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The Disk tab (docs/ADR082-persistent-disks.md D6).
 *
 * Deliberately mirrors the Render page captured in
 * docs/render-artifacts/disks.md: an empty-state card quoting the rate, an
 * inline add form (mount path + size, no name field), the five-bullet warning
 * list, and 1/5/10/50/100 GB quick-select chips defaulting to 10. The one
 * divergence is the price — bex's own $0.175/GB-month rather than Render's
 * $0.25.
 */

/**
 * The phrase a tenant must type to run an irreversible disk action, in the same
 * `sudo <verb> <what>` shape the service-delete card uses
 * (features/services/lib/service-type.ts). The mount path identifies the disk:
 * it is what the tab shows, and a service has at most one disk, so there is
 * nothing else to confuse it with. ADR082 D6 requires this on restore — a
 * restore silently discards every byte written since the snapshot, and unlike
 * a delete there is no second signal (no vanished resource) that it happened.
 */
function diskSudoPhrase(verb: "restore" | "delete", disk: DiskView): string {
  return `sudo ${verb} disk ${disk.mountPath}`;
}

/** Render's quick-select sizes, in the order its form shows them. */
const SIZE_CHIPS = [1, 5, 10, 50, 100] as const;
const DEFAULT_SIZE_GB = 10;
const MAX_SIZE_GB = 10000;

/** The service types validateDisk accepts (lego/backend/internal/apps/disks.go). */
const DISK_ELIGIBLE_TYPES = ["web_service", "private_service", "background_worker"];

export function DiskSection({
  serviceId,
  plan,
  serviceType,
}: {
  serviceId: string;
  plan: string | null;
  serviceType: string | null;
}) {
  const { t } = useTranslations();
  const { disk, loading, error, refetch } = useDisk(serviceId);
  const mutations = useDiskMutations(serviceId, refetch);
  const { canCreate } = useCapabilities();
  // Two independent refusals the API makes, both surfaced up front rather than
  // as a failed submit: a disk needs a long-running instance to mount on (so
  // no cron job or static site) and a paid instance type. Type is checked
  // first because it is the one the tenant cannot fix by upgrading.
  const wrongType = serviceType !== null && !DISK_ELIGIBLE_TYPES.includes(serviceType);
  const eligible = !wrongType && plan !== null && plan !== "free";
  const ineligibleReason = wrongType
    ? t("services.diskUnsupportedType")
    : t("services.diskPaidOnly");

  // Render's Disk tab is FOUR cards in a fixed order — Recent Metrics, Disk
  // Configuration, Snapshots, Delete Disk (live capture 2026-08-24, the
  // with-disk state docs/render-artifacts/disks.md could not walk before a disk
  // existed). Metrics lead because "is it filling up?" is the question a tenant
  // opens this tab to answer; delete trails in its own card because it destroys
  // data and should not sit one mis-click from the size field.
  //
  // A diskless service collapses to the single add card: there is no usage to
  // chart, nothing to snapshot, and nothing to delete.
  if (loading && !disk) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t("services.diskTitle")}</CardTitle>
            <CardDescription>{t("services.diskDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <CardSkeleton />
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!disk) {
    return (
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t("services.diskTitle")}</CardTitle>
            <CardDescription>{t("services.diskDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            {error ? (
              <PanelCenteredState
                icon={<HardDrive className="size-6" />}
                title={t("services.diskLoadErrorTitle")}
                body={error.message}
              />
            ) : (
              <AddDiskForm
                eligible={eligible}
                ineligibleReason={ineligibleReason}
                canCreate={canCreate}
                busy={mutations.busy}
                addError={mutations.addError}
                clearAddError={mutations.clearAddError}
                onAdd={mutations.addDisk}
              />
            )}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {(
        <>
          {/* The series is the same kubelet volume-stats pair a managed
              datastore's disk chart reads, so it reuses that panel rather than
              growing a second chart stack (ADR082 D6). Usage is observability
              only — billing meters PROVISIONED GB (D9), which is why the
              provisioned size rides in the header beside the chart, exactly as
              Render prints "Size 1 GB" above its own. */}
          <DatastoreMetricsPanel
            kind="service"
            resource={serviceId}
            title={t("services.diskUsageTitle")}
            description={t("services.diskUsageDescription")}
            diskHeaderExtra={
              <span className="text-xs text-muted-foreground">
                {t("services.diskProvisionedLabel", { size: String(disk.sizeGB) })}
              </span>
            }
          />
          <Card>
            <CardHeader>
              <CardTitle>{t("services.diskConfigTitle")}</CardTitle>
              <CardDescription>{t("services.diskConfigDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <AttachedDisk disk={disk} mutations={mutations} canCreate={canCreate} />
            </CardContent>
          </Card>
          <SnapshotsCard disk={disk} mutations={mutations} canCreate={canCreate} />
          {canCreate ? (
            <DeleteDiskCard disk={disk} onDelete={mutations.deleteDisk} busy={mutations.busy} />
          ) : null}
        </>
      )}
    </div>
  );
}

/** The five things Render tells you before you attach a disk, verbatim in meaning. */
function DiskWarnings() {
  const { t } = useTranslations();
  return (
    <div className="rounded-md border border-amber-500/40 bg-amber-500/5 p-4 text-sm">
      <p className="font-medium">{t("services.diskWarningsTitle")}</p>
      <ul className="mt-2 list-disc space-y-1 pl-5 text-muted-foreground">
        <li>{t("services.diskWarningZeroDowntime")}</li>
        <li>{t("services.diskWarningSingleInstance")}</li>
        <li>{t("services.diskWarningOnePerService")}</li>
        <li>{t("services.diskWarningMountPathOnly")}</li>
        <li>{t("services.diskWarningNoSharing")}</li>
      </ul>
    </div>
  );
}

function AddDiskForm({
  eligible,
  ineligibleReason,
  canCreate,
  busy,
  addError,
  clearAddError,
  onAdd,
}: {
  eligible: boolean;
  /** Why the form is unavailable — shown in place of the invitation copy. */
  ineligibleReason: string;
  canCreate: boolean;
  busy: boolean;
  addError: string | null;
  clearAddError: () => void;
  onAdd: (input: { mountPath: string; sizeGB: number }) => Promise<boolean>;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const [mountPath, setMountPath] = useState("");
  const [sizeGB, setSizeGB] = useState<number>(DEFAULT_SIZE_GB);
  const [invalid, setInvalid] = useState<string | null>(null);

  if (!open) {
    return (
      <div className="space-y-4">
        <PanelCenteredState
          icon={<HardDrive className="size-6" />}
          title={t("services.diskEmptyTitle")}
          body={eligible ? t("services.diskEmptyBody") : ineligibleReason}
        />
        {canCreate ? (
          <div className="flex justify-center">
            <Button onClick={() => setOpen(true)} disabled={!eligible || busy}>
              {t("services.diskAddAction")}
            </Button>
          </div>
        ) : null}
      </div>
    );
  }

  const submit = async () => {
    const path = mountPath.trim();
    const problem = validateMountPath(path, t);
    if (problem) {
      setInvalid(problem);
      return;
    }
    setInvalid(null);
    if (await onAdd({ mountPath: path, sizeGB })) {
      setOpen(false);
      setMountPath("");
      setSizeGB(DEFAULT_SIZE_GB);
    }
  };

  return (
    <div className="space-y-4">
      <DiskWarnings />
      <div className="space-y-2">
        <Label htmlFor="disk-mount-path">{t("services.diskMountPathLabel")}</Label>
        <p className="text-sm text-muted-foreground">{t("services.diskMountPathHint")}</p>
        <Input
          id="disk-mount-path"
          placeholder="/var/data"
          value={mountPath}
          onChange={(e) => {
            setMountPath(e.target.value);
            setInvalid(null);
            clearAddError();
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") void submit();
          }}
          aria-invalid={invalid !== null || addError !== null}
          aria-describedby={invalid || addError ? "disk-mount-path-error" : undefined}
        />
        {invalid || addError ? (
          <p id="disk-mount-path-error" role="alert" className="text-sm text-destructive">
            {invalid ?? addError}
          </p>
        ) : null}
      </div>
      <div className="space-y-2">
        <Label htmlFor="disk-size">{t("services.diskSizeLabel")}</Label>
        <p className="text-sm text-muted-foreground">{t("services.diskSizeHint")}</p>
        <div className="flex flex-wrap items-center gap-2">
          {SIZE_CHIPS.map((size) => (
            <Button
              key={size}
              type="button"
              variant={sizeGB === size ? "default" : "outline"}
              size="sm"
              onClick={() => setSizeGB(size)}
            >
              {t("services.diskSizeChip", { size: String(size) })}
            </Button>
          ))}
          <Input
            id="disk-size"
            type="number"
            min={1}
            max={MAX_SIZE_GB}
            className="w-28"
            value={sizeGB}
            onChange={(e) => setSizeGB(Number(e.target.value))}
          />
        </div>
      </div>
      <div className="flex gap-2">
        <Button variant="outline" onClick={() => setOpen(false)} disabled={busy}>
          {t("common.cancel")}
        </Button>
        <Button onClick={() => void submit()} disabled={busy}>
          {t("services.diskAddAction")}
        </Button>
      </div>
    </div>
  );
}

/**
 * The same mount-path rules the CRD enforces, checked before submit so the
 * common mistakes read as guidance rather than as a server rejection.
 */
function validateMountPath(path: string, t: (key: string) => string): string | null {
  if (path === "") return t("services.diskMountPathRequired");
  if (!path.startsWith("/")) return t("services.diskMountPathAbsolute");
  if (path === "/" || path.endsWith("/")) return t("services.diskMountPathNotRoot");
  const reserved = [
    "/etc",
    "/etc/secrets",
    "/home",
    "/home/render",
    "/opt",
    "/opt/render",
    "/opt/render/project",
    "/opt/render/project/src",
  ];
  if (reserved.includes(path)) return t("services.diskMountPathReserved");
  return null;
}

function AttachedDisk({
  disk,
  mutations,
  canCreate,
}: {
  disk: DiskView;
  mutations: ReturnType<typeof useDiskMutations>;
  canCreate: boolean;
}) {
  const { t } = useTranslations();
  const [size, setSize] = useState(disk.sizeGB);
  // Render keeps both fields inert until you press Edit, so the size box is
  // never a live control you can nudge by accident on a page you opened to read.
  // Growing a volume is irreversible — you cannot shrink it back — so the extra
  // click is the point, not friction to design away.
  const [editing, setEditing] = useState(false);

  const cancelEdit = () => {
    setSize(disk.sizeGB);
    setEditing(false);
  };

  return (
    <div className="space-y-6">
      <ConfigRow
        label={t("services.diskMountPathLabel")}
        help={t("services.diskMountPathHint")}
        htmlFor="disk-mount-path-value"
      >
        {/* Read-only, like Render's: the mount path is baked into the running
            pod's volume mount, so changing it is a detach and re-attach, not an
            edit. Rendering it as a disabled field rather than plain text says
            "this is a setting, and it is fixed" in one glance. */}
        <Input id="disk-mount-path-value" value={disk.mountPath} readOnly className="font-mono" />
      </ConfigRow>

      <ConfigRow
        label={t("services.diskSizeLabel")}
        help={t("services.diskSizeHint")}
        htmlFor="disk-grow"
      >
        <div className="flex items-center gap-2">
          <Input
            id="disk-grow"
            type="number"
            // Grow-only: the control cannot even express a shrink, which the
            // API, the store, and the CRD would each refuse anyway.
            min={disk.sizeGB}
            max={MAX_SIZE_GB}
            disabled={!editing || mutations.busy}
            value={size}
            onChange={(e) => setSize(Number(e.target.value))}
          />
          <span className="text-sm text-muted-foreground">{t("services.diskSizeUnit")}</span>
        </div>
        {canCreate ? (
          <div className="mt-2 flex justify-end gap-2">
            {editing ? (
              <>
                <Button variant="ghost" size="sm" onClick={cancelEdit} disabled={mutations.busy}>
                  {t("common.cancel")}
                </Button>
                <Button
                  size="sm"
                  onClick={async () => {
                    if (await mutations.growDisk(disk.id, size)) setEditing(false);
                  }}
                  disabled={mutations.busy || size <= disk.sizeGB}
                >
                  {t("services.diskGrowAction")}
                </Button>
              </>
            ) : (
              <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
                <Pencil className="size-3.5" />
                {t("services.diskEditAction")}
              </Button>
            )}
          </div>
        ) : null}
      </ConfigRow>

      <p className="text-sm text-muted-foreground">{t("services.diskGrowHint")}</p>
    </div>
  );
}

/**
 * Render's settings row: the label and its explanation sit in a left column,
 * the control in a wider right one. Reading down the left column tells you what
 * the page can configure without your eye crossing into the values — which is
 * why Render uses it for settings and a plain dl for read-only facts.
 */
function ConfigRow({
  label,
  help,
  htmlFor,
  children,
}: {
  label: string;
  help: string;
  htmlFor: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid gap-2 sm:grid-cols-[minmax(0,18rem)_1fr] sm:gap-6">
      <div>
        <Label htmlFor={htmlFor} className="font-medium">
          {label}
        </Label>
        <p className="mt-1 text-sm text-muted-foreground">{help}</p>
      </div>
      <div>{children}</div>
    </div>
  );
}

/**
 * Render gives deletion its own card at the bottom of the page, with a sentence
 * saying what is lost, rather than an icon button in a header. Both the
 * distance and the explanation are deliberate: this button destroys a volume
 * and every byte on it.
 */
function DeleteDiskCard({
  disk,
  onDelete,
  busy,
}: {
  disk: DiskView;
  onDelete: (id: string) => Promise<boolean>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.diskDeleteCardTitle")}</CardTitle>
        <CardDescription>{t("services.diskDeleteCardDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <DeleteDiskButton disk={disk} onDelete={onDelete} busy={busy} />
      </CardContent>
    </Card>
  );
}

function DeleteDiskButton({
  disk,
  onDelete,
  busy,
}: {
  disk: DiskView;
  onDelete: (id: string) => Promise<boolean>;
  busy: boolean;
}) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const phrase = diskSudoPhrase("delete", disk);
  return (
    <>
      <Button variant="destructive" size="sm" onClick={() => setOpen(true)} disabled={busy}>
        {t("services.diskDeleteAction")}
      </Button>
      <SudoConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={t("services.diskDeleteTitle")}
        description={t("services.diskDeleteWarning")}
        phrase={phrase}
        confirmLabel={t("services.diskDeleteConfirm")}
        onConfirm={() => void onDelete(disk.id)}
      />
    </>
  );
}

/**
 * An AlertDialog whose confirm stays disabled until the exact phrase is typed.
 * Local to the disk tab rather than shared: the dashboard's 27 other confirm
 * dialogs are plain AlertDialogs and folding them all into one primitive is
 * its own piece of work (.pm/w1/075.md), so this converges the two dialogs
 * that need a phrase without forking a house-wide pattern for the rest.
 */
function SudoConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  phrase,
  confirmLabel,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  title: string;
  description: string;
  phrase: string;
  confirmLabel: string;
  onConfirm: () => void;
}) {
  const { t } = useTranslations();
  const [typed, setTyped] = useState("");
  const matches = typed === phrase;

  // Clearing on close means reopening never starts pre-armed with a phrase the
  // tenant typed for a previous, possibly different, disk.
  const setOpenAndReset = (next: boolean) => {
    if (!next) setTyped("");
    onOpenChange(next);
  };

  return (
    <AlertDialog open={open} onOpenChange={setOpenAndReset}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <div className="space-y-2">
          <Label htmlFor="disk-sudo-phrase">
            {t("services.diskSudoPrompt", { phrase })}
          </Label>
          <Input
            id="disk-sudo-phrase"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
          />
        </div>
        <AlertDialogFooter>
          <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
          <AlertDialogAction
            disabled={!matches}
            onClick={() => {
              onConfirm();
              setTyped("");
            }}
          >
            {confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function SnapshotsCard({
  disk,
  mutations,
  canCreate,
}: {
  disk: DiskView;
  mutations: ReturnType<typeof useDiskMutations>;
  canCreate: boolean;
}) {
  const { t } = useTranslations();
  const { snapshots, loading, error } = useDiskSnapshots(disk.id);
  const [pending, setPending] = useState<string | null>(null);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.diskSnapshotsTitle")}</CardTitle>
        <CardDescription>{t("services.diskSnapshotsDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Render's own warning, and the most important thing on this page. */}
        <p className="rounded-md border border-amber-500/40 bg-amber-500/5 p-3 text-sm text-muted-foreground">
          {t("services.diskRestoreDatabaseWarning")}
        </p>
        {loading && snapshots.length === 0 ? (
          <CardSkeleton />
        ) : error ? (
          <PanelCenteredState
            icon={<HardDrive className="size-6" />}
            title={t("services.diskSnapshotsUnavailableTitle")}
            body={error.message}
          />
        ) : snapshots.length === 0 ? (
          <PanelCenteredState
            icon={<HardDrive className="size-6" />}
            title={t("services.diskSnapshotsEmptyTitle")}
            body={t("services.diskSnapshotsEmptyBody")}
          />
        ) : (
          <ul className="divide-y">
            {snapshots.map((snapshot) => (
              <li key={snapshot.snapshotKey} className="flex items-center justify-between gap-4 py-3">
                <span className="text-sm">{new Date(snapshot.createdAt).toLocaleString()}</span>
                {canCreate ? (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPending(snapshot.snapshotKey)}
                    disabled={mutations.busy}
                  >
                    {t("services.diskRestoreAction")}
                  </Button>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
      <SudoConfirmDialog
        open={pending !== null}
        onOpenChange={(next) => !next && setPending(null)}
        title={t("services.diskRestoreTitle")}
        description={t("services.diskRestoreWarning")}
        phrase={diskSudoPhrase("restore", disk)}
        confirmLabel={t("services.diskRestoreConfirm")}
        onConfirm={() => {
          if (pending) void mutations.restoreSnapshot(disk.id, pending);
          setPending(null);
        }}
      />
    </Card>
  );
}

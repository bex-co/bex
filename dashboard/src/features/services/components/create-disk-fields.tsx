import { useEffect, useId, useRef } from "react";

import { Button } from "@/common/components/ui/button";
import { CreateField, CreateTextField } from "@/common/components/create-form";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import {
  DiskSizeControl,
  DiskWarnings,
} from "@/features/services/components/disk-fields";
import {
  DISK_DEFAULT_SIZE_GB,
  DISK_MAX_SIZE_GB,
  DISK_MIN_SIZE_GB,
} from "@/features/services/lib/disk";
import type { DiskDraft } from "@/features/services/lib/create-service-input";

const ADD_ID = "svc-disk-add";
const MOUNT_PATH_ID = "svc-disk-mount-path";

/**
 * Render's Disk in the create form's Advanced card (w4/m652): Add Disk, then
 * the Disk tab's notes, mount path and size, with Remove Disk to drop it
 * (docs/render-artifacts/new-service-wizard.md § The Advanced section). The
 * page renders it only where a disk is allowed (`BuildShape.showDisk`).
 */
export function CreateDiskFields({
  disk,
  onChange,
  mountPathError,
  sizeError,
}: {
  disk: DiskDraft | null;
  onChange: (disk: DiskDraft | null) => void;
  mountPathError?: keyof typeof en;
  sizeError?: keyof typeof en;
}) {
  const { t } = useTranslations();
  const sizeErrorId = useId();
  // The button pressed is replaced by the fields (or they by it), so focus
  // moves to what took its place instead of falling back to the page.
  const focusNext = useRef<string | null>(null);
  useEffect(() => {
    if (focusNext.current === null) return;
    document.getElementById(focusNext.current)?.focus();
    focusNext.current = null;
  });

  const add = () => {
    focusNext.current = MOUNT_PATH_ID;
    onChange({ mountPath: "", sizeGB: DISK_DEFAULT_SIZE_GB });
  };
  const remove = () => {
    focusNext.current = ADD_ID;
    onChange(null);
  };

  return (
    <>
      <CreateField
        label={t("services.navDisk")}
        optional
        description={t("services.diskDescription")}
      >
        {disk ? (
          <div className="space-y-3">
            <DiskWarnings />
            <Button type="button" variant="outline" onClick={remove}>
              {t("services.createDiskRemove")}
            </Button>
          </div>
        ) : (
          <div>
            <Button id={ADD_ID} type="button" variant="outline" onClick={add}>
              {t("services.diskAddAction")}
            </Button>
          </div>
        )}
      </CreateField>
      {disk ? (
        <>
          {/* An empty path is held by the action bar's reason, not marked
              red the moment Add Disk is pressed. */}
          <CreateTextField
            id={MOUNT_PATH_ID}
            label={t("services.diskMountPathLabel")}
            required
            mono
            value={disk.mountPath}
            onChange={(mountPath) => onChange({ ...disk, mountPath })}
            placeholder="/var/data"
            description={t("services.diskMountPathHint")}
            error={
              mountPathError && disk.mountPath.trim() !== ""
                ? t(mountPathError)
                : undefined
            }
          />
          <CreateField
            label={t("services.diskSizeLabel")}
            htmlFor="svc-disk-size"
            description={t("services.diskSizeHint")}
          >
            <DiskSizeControl
              id="svc-disk-size"
              value={disk.sizeGB}
              onChange={(sizeGB) => onChange({ ...disk, sizeGB })}
              invalid={Boolean(sizeError)}
              describedBy={sizeError ? sizeErrorId : undefined}
            />
            {sizeError ? (
              <p
                id={sizeErrorId}
                role="alert"
                className="text-destructive text-sm"
              >
                {t(sizeError, {
                  min: String(DISK_MIN_SIZE_GB),
                  max: String(DISK_MAX_SIZE_GB),
                })}
              </p>
            ) : null}
          </CreateField>
        </>
      ) : null}
    </>
  );
}

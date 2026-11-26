import { useRef, useState, type ChangeEvent, type ReactNode } from "react";
import { Pencil } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { CropAvatarDialog } from "@/features/avatars/components/crop-avatar-dialog";
import { EditMonogramDialog } from "@/features/avatars/components/edit-monogram-dialog";
import {
  useAvatarActions,
  type AvatarType,
} from "@/features/avatars/hooks/use-avatar-actions";
import {
  AVATAR_FILE_TYPES,
  MAX_AVATAR_DATA_URL_LENGTH,
  cropToJpeg,
  monogramToJpeg,
  readAsDataURL,
  type CropView,
} from "@/features/avatars/lib/avatar-image";
import type { Monogram } from "@/features/avatars/lib/avatar-utils";
import { AVATAR_ROW_CONTROL_CLASS } from "@/features/avatars/components/avatar-row-layout";

/**
 * Render's Avatar row control (w1/m648, `AvatarRow-*.js`): the avatar, then
 * an Edit menu with Upload image, Change monogram and, only while the avatar
 * is an upload, Reset to default. Without permission the Edit button is
 * disabled and its tooltip says why.
 *
 * Upload image takes a PNG or JPEG under 20 MB, crops it round and uploads a
 * 128x128 JPEG; Change monogram uploads the drawn monogram the same way, or
 * resets when it is the default. Every failure is one of Render's toasts.
 */
export function AvatarEditor({
  ownerId,
  avatarType,
  avatar,
  isUpload,
  defaultMonogram,
  canEdit,
  noPermissionMessage,
}: {
  ownerId: string;
  avatarType: AvatarType;
  /** The owner's avatar at xlarge, as the row shows it. */
  avatar: ReactNode;
  /** Whether the avatar showing is an upload (a saved monogram is one). */
  isUpload: boolean;
  defaultMonogram: Monogram;
  canEdit: boolean;
  noPermissionMessage?: string;
}) {
  const { t } = useTranslations();
  const { upload, reset } = useAvatarActions(ownerId, avatarType);
  const fileInput = useRef<HTMLInputElement>(null);
  const [cropSource, setCropSource] = useState<string | null>(null);
  const [monogramOpen, setMonogramOpen] = useState(false);

  const failed = (title: string, body: string) =>
    toast.error(title, { description: body });
  const uploadFailed = () =>
    failed(t("avatars.uploadFailedTitle"), t("avatars.connectionFailedBody"));

  const doReset = async () => {
    // Render resets nothing on the server unless an upload is showing.
    if (!isUpload) return;
    try {
      await reset();
    } catch {
      failed(t("avatars.deleteFailedTitle"), t("avatars.connectionFailedBody"));
    }
  };

  const onFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!(AVATAR_FILE_TYPES as readonly string[]).includes(file.type)) {
      failed(
        t("avatars.invalidFileTypeTitle"),
        t("avatars.invalidFileTypeBody"),
      );
      return;
    }
    let dataURL: string;
    try {
      dataURL = await readAsDataURL(file);
    } catch {
      failed(t("avatars.unreadableFileTitle"), t("avatars.unreadableFileBody"));
      return;
    }
    if (dataURL.length > MAX_AVATAR_DATA_URL_LENGTH) {
      failed(t("avatars.fileTooLargeTitle"), t("avatars.fileTooLargeBody"));
      return;
    }
    setCropSource(dataURL);
  };

  const saveCrop = async (view: CropView) => {
    if (!cropSource) return;
    try {
      await upload(await cropToJpeg(cropSource, view));
      setCropSource(null);
    } catch {
      uploadFailed();
    }
  };

  const saveMonogram = async (monogram: Monogram) => {
    const isDefault =
      monogram.letter === defaultMonogram.letter &&
      monogram.colorIndex === defaultMonogram.colorIndex;
    if (isDefault) {
      await doReset();
      setMonogramOpen(false);
      return;
    }
    let image: Blob;
    try {
      image = await monogramToJpeg(monogram);
    } catch {
      failed(t("avatars.monogramFailedTitle"), t("avatars.monogramFailedBody"));
      return;
    }
    try {
      await upload(image);
      setMonogramOpen(false);
    } catch {
      uploadFailed();
    }
  };

  const editButton = (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      disabled={!canEdit}
      data-testid="avatar-edit-button"
    >
      <Pencil aria-hidden="true" />
      {t("avatars.edit")}
    </Button>
  );

  return (
    <div className={AVATAR_ROW_CONTROL_CLASS}>
      {avatar}
      {canEdit ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>{editButton}</DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuItem onSelect={() => fileInput.current?.click()}>
              {t("avatars.uploadImage")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setMonogramOpen(true)}>
              {t("avatars.changeMonogram")}
            </DropdownMenuItem>
            {isUpload ? (
              <DropdownMenuItem
                variant="destructive"
                onSelect={() => void doReset()}
              >
                {t("avatars.resetToDefault")}
              </DropdownMenuItem>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : (
        <PermissionTooltip reason={noPermissionMessage}>
          {editButton}
        </PermissionTooltip>
      )}
      <input
        ref={fileInput}
        type="file"
        accept={AVATAR_FILE_TYPES.join(", ")}
        hidden
        data-testid="avatar-file-input"
        onChange={(event) => void onFile(event)}
      />
      <CropAvatarDialog
        imageDataURL={cropSource}
        onCancel={() => setCropSource(null)}
        onSave={saveCrop}
      />
      <EditMonogramDialog
        open={monogramOpen}
        initial={defaultMonogram}
        onCancel={() => setMonogramOpen(false)}
        onSave={saveMonogram}
      />
    </div>
  );
}

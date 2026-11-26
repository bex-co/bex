import { useId, useState } from "react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { useCardRadioGroup } from "@/common/hooks/use-card-radio-group";
import { MonogramAvatar } from "@/features/avatars/components/avatar";
import {
  MONOGRAM_COLORS,
  isSavableMonogram,
  monogramLetter,
  type Monogram,
} from "@/features/avatars/lib/avatar-utils";

const COLOR_IDS = MONOGRAM_COLORS.map((color) => color.id);

/**
 * The dialog's form. It mounts each time the dialog opens, so it starts on
 * `initial` every time without an effect.
 */
function MonogramForm({
  initial,
  onCancel,
  onSave,
}: {
  initial: Monogram;
  onCancel: () => void;
  onSave: (monogram: Monogram) => Promise<void>;
}) {
  const { t } = useTranslations();
  const textId = useId();
  const colorLabelId = useId();
  const [draft, setDraft] = useState(initial);
  const [saving, setSaving] = useState(false);

  const save = async () => {
    setSaving(true);
    try {
      await onSave(draft);
    } finally {
      setSaving(false);
    }
  };

  // The colors are one radio group: the arrow keys move the choice and the
  // focus together, and only the chosen swatch is in the tab order.
  const swatch = useCardRadioGroup(
    COLOR_IDS,
    MONOGRAM_COLORS[draft.colorIndex]?.id,
    (id) => setDraft((d) => ({ ...d, colorIndex: COLOR_IDS.indexOf(id) })),
  );

  return (
    <form
      noValidate
      className="mx-auto w-full max-w-[320px] space-y-8 py-2"
      onSubmit={(event) => {
        event.preventDefault();
        if (isSavableMonogram(draft) && !saving) void save();
      }}
    >
      <div className="flex justify-center">
        <MonogramAvatar monogram={draft} size="xlarge" />
      </div>
      <div className="space-y-2">
        <Label htmlFor={textId}>{t("avatars.monogramText")}</Label>
        <Input
          id={textId}
          value={draft.letter}
          autoFocus
          spellCheck={false}
          autoComplete="off"
          onChange={(event) =>
            setDraft((d) => ({
              ...d,
              letter: monogramLetter(event.target.value),
            }))
          }
        />
      </div>
      <div className="space-y-2">
        <p id={colorLabelId} className="text-sm leading-none font-medium">
          {t("avatars.monogramColor")}
        </p>
        <div
          role="radiogroup"
          aria-labelledby={colorLabelId}
          className="flex justify-between gap-2"
        >
          {MONOGRAM_COLORS.map((color, index) => {
            const checked = draft.colorIndex === index;
            return (
              <button
                key={color.id}
                {...swatch(color.id)}
                type="button"
                role="radio"
                aria-checked={checked}
                aria-label={t(color.labelKey)}
                onClick={() => setDraft((d) => ({ ...d, colorIndex: index }))}
                className={cn(
                  "size-[30px] shrink-0 rounded-full border-2 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                  checked ? "border-foreground" : "border-transparent",
                )}
                style={{ backgroundColor: color.background }}
              />
            );
          })}
        </div>
      </div>
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          disabled={saving}
        >
          {t("avatars.cancel")}
        </Button>
        <Button type="submit" disabled={!isSavableMonogram(draft) || saving}>
          {t("avatars.save")}
        </Button>
      </DialogFooter>
    </form>
  );
}

/**
 * Render's "Edit Monogram" dialog: a 64 px preview, a Text field that keeps
 * its first character upper-cased, and the eight colors as a radio group.
 * It opens on the owner's default monogram. Save hands the choice to
 * `onSave`, which resets to the default when the choice is the default and
 * uploads it as an image otherwise.
 */
export function EditMonogramDialog({
  open,
  initial,
  onCancel,
  onSave,
}: {
  open: boolean;
  initial: Monogram;
  onCancel: () => void;
  onSave: (monogram: Monogram) => Promise<void>;
}) {
  const { t } = useTranslations();
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onCancel();
      }}
    >
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{t("avatars.monogramTitle")}</DialogTitle>
        </DialogHeader>
        <MonogramForm initial={initial} onCancel={onCancel} onSave={onSave} />
      </DialogContent>
    </Dialog>
  );
}

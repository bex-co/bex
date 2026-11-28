import { useCallback, useEffect, useId, useRef, useState } from "react";
import { Loader2, Pencil } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Card } from "@/common/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { Switch } from "@/common/components/ui/switch";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { useTheme } from "@/common/providers/theme-provider/hooks";
import { useUnsavedChangesWarning } from "@/common/providers/unsaved-changes-context";
import {
  LOG_THEME_SETTINGS,
  THEMES,
  type LogThemeSetting,
} from "@/common/providers/theme-provider/settings";
import type { Theme } from "@/common/providers/theme-provider/type";
import { READ_MODE_CLASS } from "@/features/services/components/editable-field-row";
import {
  APPEARANCE_ACTIONS_CLASS,
  APPEARANCE_CARD_CLASS,
  APPEARANCE_CONTROL_CLASS,
  APPEARANCE_SELECT_CLASS,
  APPEARANCE_SWITCH_CLASS,
  AppearanceRow,
} from "./appearance-row";

/**
 * One of the two theme rows, as Render's `RowInput` draws it (w3/097): the
 * saved choice in a select that stays shut until a ghost Edit opens it, with
 * Cancel and Save changes beneath. Only Save changes writes the account, and
 * reads Saving until the account answers; a refused save keeps the row open.
 * `onPreview` hears each pick, and null when Cancel ends the edit. An unsaved
 * pick holds a navigation to another path with Render's Discard unsaved
 * changes? (w3/118).
 */
function ThemeChoiceRow<T extends string>({
  label,
  value,
  options,
  labels,
  ready,
  onSave,
  onPreview,
  onEditingChange,
}: {
  label: string;
  value: T;
  options: readonly T[];
  labels: Record<T, string>;
  ready: boolean;
  onSave: (value: T) => Promise<boolean>;
  onPreview?: (value: T | null) => void;
  onEditingChange?: (editing: boolean) => void;
}) {
  const { t } = useTranslations();
  const labelId = useId();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const editRef = useRef<HTMLButtonElement>(null);
  // Edit hands focus to the select it opens, and Cancel or a save back to
  // Edit, once the row has rendered, so the keyboard is never dropped on
  // <body> as the actions swap.
  const refocus = useRef(false);
  useEffect(() => {
    if (!refocus.current) return;
    refocus.current = false;
    (editing ? triggerRef : editRef).current?.focus();
  }, [editing]);

  const edit = (next: boolean) => {
    refocus.current = true;
    setEditing(next);
    onEditingChange?.(next);
  };
  const control = useCallback(() => triggerRef.current, []);
  useUnsavedChangesWarning(
    editing && draft !== value && !saving,
    label,
    control,
  );
  const save = async () => {
    setSaving(true);
    const saved = await onSave(draft);
    setSaving(false);
    if (saved) edit(false);
  };

  return (
    <AppearanceRow labelId={labelId} label={label}>
      <div
        role="group"
        aria-labelledby={labelId}
        className={APPEARANCE_CONTROL_CLASS}
      >
        <Select
          value={ready ? (editing ? draft : value) : ""}
          onValueChange={(next) => {
            setDraft(next as T);
            onPreview?.(next as T);
          }}
          disabled={!ready || !editing || saving}
        >
          <SelectTrigger
            ref={triggerRef}
            className={cn(
              APPEARANCE_SELECT_CLASS,
              ready && !editing && READ_MODE_CLASS,
            )}
            aria-label={label}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {options.map((option) => (
              <SelectItem key={option} value={option}>
                {labels[option]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className={APPEARANCE_ACTIONS_CLASS}>
          {editing ? (
            <>
              <Button
                variant="secondary"
                size="sm"
                disabled={saving}
                onClick={() => {
                  onPreview?.(null);
                  edit(false);
                }}
              >
                {t("auth.appearanceCancel")}
              </Button>
              <Button
                size="sm"
                disabled={saving || draft === value}
                onClick={() => void save()}
              >
                {saving && <Loader2 className="animate-spin" />}
                {saving
                  ? t("auth.appearanceSaving")
                  : t("auth.appearanceSaveChanges")}
              </Button>
            </>
          ) : (
            <Button
              ref={editRef}
              variant="ghost"
              size="sm"
              disabled={!ready}
              onClick={() => {
                setDraft(value);
                edit(true);
              }}
            >
              <Pencil />
              {t("auth.appearanceEdit")}
            </Button>
          )}
        </div>
      </div>
    </AppearanceRow>
  );
}

/**
 * Render's Appearance (w3/m528, w3/m531, w3/097): Dashboard Theme (System
 * (Default), Light, Dark), Log Explorer Theme (Match Dashboard (Default),
 * Light, Dark) and High Contrast Mode, in Render's order. All three are saved
 * on the account, so every browser it signs in to opens in them. The two
 * themes change behind Edit and are written only by Save changes; Dashboard
 * Theme shows each pick on the page while it is edited, and Cancel, or
 * leaving the page unsaved (after Discard unsaved changes?), puts the saved
 * theme back. High Contrast Mode is written as it is switched, keeps
 * Dashboard Theme as it is, and waits while Dashboard Theme is being edited,
 * as Render's does. The account menu's theme submenu changes Dashboard Theme
 * at once.
 */
export function AppearanceCard() {
  const { t } = useTranslations();
  const {
    theme,
    setTheme,
    previewTheme,
    highContrast,
    setHighContrast,
    logTheme,
    setLogTheme,
    ready,
  } = useTheme();
  const highContrastLabelId = useId();
  const [editingTheme, setEditingTheme] = useState(false);
  // Leaving the page with Dashboard Theme still being edited shows the saved
  // theme again.
  useEffect(() => () => previewTheme(null), [previewTheme]);
  // The stored settings are read after hydration; until then the controls
  // show none, so the server's render and the first client render agree.
  const highContrastOn = ready && highContrast;
  return (
    <Card data-testid="appearance-card" className={APPEARANCE_CARD_CLASS}>
      <ThemeChoiceRow<Theme>
        label={t("auth.dashboardTheme")}
        value={theme}
        options={THEMES}
        labels={{
          system: t("auth.dashboardThemeSystem"),
          light: t("common.userMenuThemeLight"),
          dark: t("common.userMenuThemeDark"),
        }}
        ready={ready}
        onSave={setTheme}
        onPreview={previewTheme}
        onEditingChange={setEditingTheme}
      />
      <ThemeChoiceRow<LogThemeSetting>
        label={t("auth.logExplorerTheme")}
        value={logTheme}
        options={LOG_THEME_SETTINGS}
        labels={{
          MATCH_APP: t("auth.logExplorerThemeMatch"),
          ALWAYS_LIGHT: t("common.userMenuThemeLight"),
          ALWAYS_DARK: t("common.userMenuThemeDark"),
        }}
        ready={ready}
        onSave={setLogTheme}
      />
      <AppearanceRow
        labelId={highContrastLabelId}
        label={t("auth.highContrastMode")}
        description={t("auth.highContrastModeDescription")}
      >
        <label className={APPEARANCE_SWITCH_CLASS}>
          <Switch
            checked={highContrastOn}
            onCheckedChange={(on) => void setHighContrast(on)}
            disabled={!ready || editingTheme}
            aria-labelledby={highContrastLabelId}
          />
          {highContrastOn
            ? t("auth.highContrastEnabled")
            : t("auth.highContrastDisabled")}
        </label>
      </AppearanceRow>
    </Card>
  );
}

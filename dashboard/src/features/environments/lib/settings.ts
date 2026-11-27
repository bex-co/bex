import { LockKeyhole, ShieldOff, type LucideIcon } from "lucide-react";
import type { Translate } from "@/common/hooks/use-translations";
import { proFeaturesPlan } from "@/features/workspaces/types";

/** The environment settings page's sections in Render's order; each is its anchor. */
export type EnvironmentSettingsSection =
  | "name"
  | "permissions"
  | "cross-environment-connections"
  | "environment-groups"
  | "networking"
  | "delete-environment";

/**
 * The sections a caller sees: Render shows Permissions, Connections and
 * Networking only to a member who may change them, which on bex is an Admin
 * (`can_manage`). While the role is unknown every section shows, and the
 * server refuses a change it does not allow.
 */
export function environmentSettingsSections(
  adminDenied: boolean,
): EnvironmentSettingsSection[] {
  return adminDenied
    ? ["name", "environment-groups", "delete-environment"]
    : [
        "name",
        "permissions",
        "cross-environment-connections",
        "environment-groups",
        "networking",
        "delete-environment",
      ];
}

/**
 * Whether blocking cross-environment connections is behind Render's plan
 * gate: below Pro (ADR030), unless the environment is already isolated, which
 * bex always lets turn off.
 */
export function isolationGated(
  plan: string | null | undefined,
  isolated: boolean,
): boolean {
  return !isolated && !proFeaturesPlan(plan);
}

/** The environment settings page's route, which the chrome recognizes it by. */
export const ENVIRONMENT_SETTINGS_ROUTE_ID =
  "/project/$projectId/environment/$environmentId/settings";

type TranslationKey = Parameters<Translate>[0];

export type Protection = "unprotected" | "protected";
export type Connections = "allowed" | "blocked";

export interface ChoiceOptionCopy<T extends string> {
  value: T;
  titleKey: TranslationKey;
  hintKey: TranslationKey;
  icon?: LucideIcon;
}

/** Render's Permissions options, in its order. */
export const PROTECTION_OPTIONS: Array<ChoiceOptionCopy<Protection>> = [
  {
    value: "unprotected",
    titleKey: "environments.permissionsDefault",
    hintKey: "environments.permissionsDefaultHint",
  },
  {
    value: "protected",
    titleKey: "environments.permissionsProtected",
    hintKey: "environments.permissionsProtectedHint",
    icon: LockKeyhole,
  },
];

/** Render's Cross-Environment Connections options, in its order. */
export const CONNECTION_OPTIONS: Array<ChoiceOptionCopy<Connections>> = [
  {
    value: "allowed",
    titleKey: "environments.connectionsAllowed",
    hintKey: "environments.connectionsAllowedHint",
  },
  {
    value: "blocked",
    titleKey: "environments.connectionsBlocked",
    hintKey: "environments.connectionsBlockedHint",
    icon: ShieldOff,
  },
];

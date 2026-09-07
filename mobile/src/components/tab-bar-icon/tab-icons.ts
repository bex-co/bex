import type { ComponentProps } from "react";
import type { Ionicons } from "@expo/vector-icons";

type IoniconName = ComponentProps<typeof Ionicons>["name"];
export type TabRouteName = "index" | "activity" | "sessions" | "notifications";

export const tabIcons = {
  index: { active: "pulse", inactive: "pulse-outline" },
  activity: { active: "time", inactive: "time-outline" },
  sessions: { active: "sparkles", inactive: "sparkles-outline" },
  notifications: { active: "notifications", inactive: "notifications-outline" },
} as const satisfies Record<
  TabRouteName,
  { active: IoniconName; inactive: IoniconName }
>;

// Distinct outline/filled symbols keep focus visible without relying on tint.
export const nativeTabIcons = {
  index: {
    default: "waveform.path.ecg.rectangle",
    selected: "waveform.path.ecg.rectangle.fill",
  },
  activity: { default: "clock", selected: "clock.fill" },
  sessions: {
    default: "sparkles.rectangle.stack",
    selected: "sparkles.rectangle.stack.fill",
  },
  notifications: { default: "bell", selected: "bell.fill" },
} as const satisfies Record<
  TabRouteName,
  { default: string; selected: string }
>;

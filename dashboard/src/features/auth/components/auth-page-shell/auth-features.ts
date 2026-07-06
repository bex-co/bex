import { Lock, BarChart3, Heart } from "lucide-react";
import type { AuthFeature } from "./index";

export const AUTH_FEATURES: AuthFeature[] = [
  {
    icon: Lock,
    title: "Secure by default",
    description:
      "Sessions are managed by Ory Kratos — battle-tested identity infrastructure, not a hand-rolled auth system.",
    iconColor: "text-blue-400 dark:text-blue-300",
    iconBg: "bg-blue-500/10 dark:bg-blue-400/10",
  },
  {
    icon: BarChart3,
    title: "One dashboard for every service",
    description:
      "Deploy, monitor, and manage everything running on bex from a single place.",
    iconColor: "text-green-400 dark:text-green-300",
    iconBg: "bg-green-500/10 dark:bg-green-400/10",
  },
  {
    icon: Heart,
    title: "Built in the open",
    description: "bex is the open-source Render alternative.",
    iconColor: "text-purple-400 dark:text-purple-300",
    iconBg: "bg-purple-500/10 dark:bg-purple-400/10",
  },
];

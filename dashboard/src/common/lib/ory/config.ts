import type { OryClientConfiguration } from "@ory/elements-react";

/** Ory Kratos public API base URL (docs/auth.md). */
export const KRATOS_PUBLIC_URL =
  import.meta.env.VITE_KRATOS_PUBLIC_URL || "https://auth.bex.co";

/**
 * Drives both Ory Elements' own internal flow submissions (`sdk.url`) and
 * where each flow's UI is expected to live in this app (`project.*_ui_url`).
 */
export const oryConfig: OryClientConfiguration = {
  sdk: {
    url: KRATOS_PUBLIC_URL,
    options: {
      // Kratos and this dashboard are deployed under different hosts, so
      // cross-origin requests need the session/CSRF cookies forwarded.
      credentials: "include",
    },
  },
  project: {
    name: "bex",
    default_redirect_url: "/",
    error_ui_url: "/auth/error",
    login_ui_url: "/auth/login",
    registration_ui_url: "/auth/sign-up",
    registration_enabled: true,
    recovery_ui_url: "/auth/forgot-password",
    recovery_enabled: true,
    settings_ui_url: "/settings",
    verification_ui_url: "/auth/verification",
    verification_enabled: false,
  },
};

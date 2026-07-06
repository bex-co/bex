import type { OryClientConfiguration } from "@ory/elements-react";

/** Ory Kratos public API base URL, reachable from the browser (docs/auth.md). */
export const KRATOS_PUBLIC_URL =
  import.meta.env.VITE_KRATOS_PUBLIC_URL || "https://auth.bex.co";

/**
 * Kratos URL for server-side (SSR) calls, e.g. the in-cluster Service DNS
 * name (`http://kratos-public.auth.svc.cluster.local`) — the dashboard's own
 * pod reaches Kratos directly, it doesn't need to go back out through the
 * public ingress. Falls back to KRATOS_PUBLIC_URL (e.g. local dev, where SSR
 * runs on the same laptop as the browser).
 */
export const KRATOS_SSR_URL =
  import.meta.env.VITE_KRATOS_SSR_URL || KRATOS_PUBLIC_URL;

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

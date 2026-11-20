/// <reference types="vite/client" />

declare global {
  interface ImportMetaEnv {
    /** Public API URL used by the browser (CSR) */
    readonly VITE_API_URL: string;
    /** Optional internal API URL used during SSR (defaults to VITE_API_URL) */
    readonly VITE_SSR_API_URL?: string;
    /** Ory Kratos public API base URL, reachable from the browser (self-service login/registration/recovery/settings) */
    readonly VITE_KRATOS_PUBLIC_URL?: string;
    /** Optional in-cluster Kratos URL used during SSR (defaults to VITE_KRATOS_PUBLIC_URL) */
    readonly VITE_KRATOS_SSR_URL?: string;
    /** Hydra public issuer (token endpoint base) for API-key exchange copy */
    readonly VITE_OAUTH_ISSUER?: string;
    /** Per-build id stamped by vite.config.ts; guards the one preload-error reload (w4/651) */
    readonly VITE_BEX_BUILD_ID?: string;
    /** GA4 measurement ID (G-…); unset loads no Google Analytics */
    readonly VITE_GA_MEASUREMENT_ID?: string;
  }

  interface ImportMeta {
    readonly env: ImportMetaEnv;
  }

  interface Window {
    /** GA4 gtag.js, present only when VITE_GA_MEASUREMENT_ID loaded it */
    gtag?: (...args: unknown[]) => void;
  }
}

export {};

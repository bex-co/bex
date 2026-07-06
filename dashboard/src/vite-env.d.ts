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
  }

  interface ImportMeta {
    readonly env: ImportMetaEnv;
  }

  interface Window {
    __THEME__?: "light" | "dark"; // SSR-injected theme (always resolved, never "system")
  }
}

export {};

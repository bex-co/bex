/**
 * A Postgres database's admin apps — Render's Postgres "Apps" page (w4/m551,
 * docs/render-artifacts/postgres-admin-apps.md). bex-api answers the catalog
 * and each app's deployed service; this module holds what Render's page keeps
 * in the browser: each app's copy and docs link, and the credentials its
 * dialog generates.
 */

/** Render's catalog ids, the order its page shows them. */
export type AdminAppId = "pgadmin" | "pghero";

export interface AdminAppCopy {
  /** Translation key of the card's description. */
  descriptionKey: string;
  /** Render's "Read the docs" link, when the app has one. */
  docsUrl?: string;
}

export const ADMIN_APP_COPY: Record<AdminAppId, AdminAppCopy> = {
  pgadmin: {
    descriptionKey: "databases.adminAppPgAdminDescription",
    docsUrl: "https://www.pgadmin.org/docs/",
  },
  pghero: { descriptionKey: "databases.adminAppPgHeroDescription" },
};

export function isAdminAppId(id: string | null | undefined): id is AdminAppId {
  return id === "pgadmin" || id === "pghero";
}

/** `length` random hex characters, Render's generator (crypto.getRandomValues). */
export function randomHex(length: number): string {
  const bytes = new Uint8Array(Math.ceil(length / 2));
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0"))
    .join("")
    .slice(0, length);
}

export interface AdminAppCredentials {
  username: string;
  password: string;
}

/**
 * The dialog's prefilled login, as Render generates it: PgHero's username is
 * `pghero-` and 8 hex characters; pgAdmin logs in by email, so its username is
 * the signed-in user's email (the generated name when the session has none);
 * the password is 32 hex characters.
 */
export function generateAdminAppCredentials(
  app: AdminAppId,
  email: string | undefined,
): AdminAppCredentials {
  const generated = `${app}-${randomHex(8)}`;
  return {
    username: app === "pgadmin" && email ? email : generated,
    password: randomHex(32),
  };
}

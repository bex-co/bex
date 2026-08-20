export const MOBILE_CLIENT_ID = "bex-mobile";
export const MOBILE_REDIRECT_URI = "https://dashboard.bex.co/oauth2redirect";
export const DEFAULT_OAUTH_AUDIENCE = "https://api.bex.co/mcp";

export type MobileConfig = {
  apiOrigin: string;
  graphqlUrl: string;
  oauthIssuer: string;
  oauthClientId: string;
  oauthAudience: string;
  oauthRedirectUri: string;
  easProjectId?: string | null;
};

type Environment = Record<string, string | undefined>;

function parseOrigin(
  name: string,
  raw: string,
  allowLocalHttp: boolean,
): string {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`${name} must be an absolute URL`);
  }
  const local = url.hostname === "localhost" || url.hostname === "127.0.0.1";
  if (url.protocol !== "https:" && !(allowLocalHttp && local)) {
    throw new Error(`${name} must use HTTPS`);
  }
  if (url.username || url.password || url.search || url.hash) {
    throw new Error(`${name} must not include credentials, query, or fragment`);
  }
  return url.origin;
}

function parseResource(name: string, raw: string, allowLocalHttp: boolean) {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`${name} must be an absolute URL`);
  }
  const local = url.hostname === "localhost" || url.hostname === "127.0.0.1";
  if (url.protocol !== "https:" && !(allowLocalHttp && local)) {
    throw new Error(`${name} must use HTTPS`);
  }
  if (url.username || url.password || url.search || url.hash) {
    throw new Error(`${name} must not include credentials, query, or fragment`);
  }
  return `${url.origin}${url.pathname === "/" ? "" : url.pathname.replace(/\/$/, "")}`;
}

export function readMobileConfig(
  env: Environment = process.env,
  allowLocalHttp = typeof __DEV__ !== "undefined" && __DEV__,
): MobileConfig {
  const apiOrigin = parseOrigin(
    "EXPO_PUBLIC_BEX_API_URL",
    env.EXPO_PUBLIC_BEX_API_URL ?? "https://api.bex.co",
    allowLocalHttp,
  );
  const oauthIssuer = parseOrigin(
    "EXPO_PUBLIC_BEX_OAUTH_ISSUER",
    env.EXPO_PUBLIC_BEX_OAUTH_ISSUER ?? "https://oauth.bex.co",
    allowLocalHttp,
  );
  const oauthClientId = env.EXPO_PUBLIC_BEX_OAUTH_CLIENT_ID ?? MOBILE_CLIENT_ID;
  if (!/^[A-Za-z0-9._~-]{1,128}$/.test(oauthClientId)) {
    throw new Error("EXPO_PUBLIC_BEX_OAUTH_CLIENT_ID is invalid");
  }
  const oauthAudience = parseResource(
    "EXPO_PUBLIC_BEX_OAUTH_AUDIENCE",
    env.EXPO_PUBLIC_BEX_OAUTH_AUDIENCE ?? `${apiOrigin}/mcp`,
    allowLocalHttp,
  );
  return {
    apiOrigin,
    graphqlUrl: `${apiOrigin}/graphql`,
    oauthIssuer,
    oauthClientId,
    oauthAudience,
    oauthRedirectUri: MOBILE_REDIRECT_URI,
    easProjectId: env.EXPO_PUBLIC_EAS_PROJECT_ID?.trim() || null,
  };
}

export const mobileConfig = readMobileConfig();

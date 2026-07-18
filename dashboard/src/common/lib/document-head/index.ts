import type { TOptions } from "i18next";
import i18n from "@/i18n/init";
import type { SupportedLanguage } from "@/i18n";

export { DashboardDocumentTitle } from "./document-title";
export { getDashboardOrigin } from "./origin";

export const DASHBOARD_NAME = "bex Dashboard";
export const TITLE_SEPARATOR = " ・ ";

type MetaTag =
  | { title: string }
  | { charSet: "utf-8" }
  | { name: string; content: string }
  | { property: string; content: string };

export interface DashboardHead {
  meta: MetaTag[];
}

export interface DashboardHeadMatch {
  status?: string;
}

export type RouteResource<T> =
  | { state: "ready"; resource: T }
  | { state: "not-found" }
  | { state: "error" };

/** One Render-shaped formatter for every static and resource title. */
export function formatDashboardTitle(
  ...parts: Array<string | null | undefined>
) {
  const prefix = parts
    .map((part) => part?.trim())
    .filter(Boolean)
    .join(TITLE_SEPARATOR);
  return prefix
    ? `${prefix}${TITLE_SEPARATOR}${DASHBOARD_NAME}`
    : DASHBOARD_NAME;
}

export function translatedText(key: string, options?: TOptions): string {
  return i18n.t(key, options);
}

export function translatedTitle(key: string, options?: TOptions): string {
  return formatDashboardTitle(translatedText(key, options));
}

function hasRenderedFallback(
  matchOrMatches?: DashboardHeadMatch | readonly DashboardHeadMatch[],
) {
  const matches = Array.isArray(matchOrMatches)
    ? matchOrMatches
    : matchOrMatches
      ? [matchOrMatches]
      : [];
  return matches.some(
    (match) => match.status === "error" || match.status === "notFound",
  );
}

export function titleHead(
  title: string,
  matchOrMatches?: DashboardHeadMatch | readonly DashboardHeadMatch[],
): DashboardHead {
  if (hasRenderedFallback(matchOrMatches)) return { meta: [] };
  return { meta: [{ title }] };
}

export function translatedTitleHead(
  key: string,
  matchOrMatches?: DashboardHeadMatch | readonly DashboardHeadMatch[],
  options?: TOptions,
): DashboardHead {
  return titleHead(translatedTitle(key, options), matchOrMatches);
}

export function stateTitle(
  resource: RouteResource<unknown> | undefined,
): string {
  if (!resource) return translatedTitle("common.loading");
  if (resource.state === "not-found") {
    return translatedTitle("common.notFoundTitle");
  }
  if (resource.state === "error") return translatedTitle("common.errorTitle");
  return formatDashboardTitle();
}

export function routeResourceTitle<T>(
  result: RouteResource<T> | undefined,
  parts: (resource: T) => Array<string | null | undefined>,
): string {
  return result?.state === "ready"
    ? formatDashboardTitle(...parts(result.resource))
    : stateTitle(result);
}

/**
 * Resolve a route resource without throwing away the distinction between a
 * missing row and a failed authenticated query. The serializable result is
 * safe to dehydrate with TanStack Router; Apollo retains the full query in its
 * normalized cache for the page component.
 */
export async function loadRouteResource<TData, TResource>(
  query: () => Promise<{ data?: TData; error?: unknown }>,
  select: (data: TData | undefined) => TResource | null | undefined,
  isNotFound: (error: unknown) => boolean = isNotFoundError,
): Promise<RouteResource<TResource>> {
  try {
    const result = await query();
    const resource = select(result.data);
    if (resource) return { state: "ready", resource };
    if (!result.error || isNotFound(result.error))
      return { state: "not-found" };
    return { state: "error" };
  } catch (error) {
    return isNotFound(error) ? { state: "not-found" } : { state: "error" };
  }
}

export function isNotFoundError(error: unknown): boolean {
  return error instanceof Error && /\bnot[ -]?found\b/i.test(error.message);
}

export function normalizeDashboardOrigin(
  origin: string | null | undefined,
): string | null {
  if (!origin) return null;
  try {
    const url = new URL(origin);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    return url.origin;
  } catch {
    return null;
  }
}

export function absoluteMetadataUrl(
  origin: string | null | undefined,
  path: `/${string}`,
): string | null {
  const normalized = normalizeDashboardOrigin(origin);
  return normalized ? new URL(path, normalized).toString() : null;
}

const OPEN_GRAPH_LOCALE: Record<SupportedLanguage, string> = {
  en: "en_US",
  zh: "zh_CN",
};

/**
 * Generic installation metadata. Route names deliberately never enter this
 * function: private state changes only the document title, just as it does in
 * Render's authenticated dashboard shell.
 */
export function globalMetadata(
  origin: string | null | undefined,
  language: SupportedLanguage,
): DashboardHead {
  const description = i18n.t("common.headDescription", { lng: language });
  const imageAlt = i18n.t("common.headImageAlt", { lng: language });
  const normalizedOrigin = normalizeDashboardOrigin(origin);
  const image = absoluteMetadataUrl(normalizedOrigin, "/logo.png");
  const meta: MetaTag[] = [
    { charSet: "utf-8" },
    {
      name: "viewport",
      content: "width=device-width, initial-scale=1, shrink-to-fit=no",
    },
    { name: "description", content: description },
    { property: "og:type", content: "website" },
    { property: "og:title", content: DASHBOARD_NAME },
    { property: "og:description", content: description },
    { property: "og:site_name", content: DASHBOARD_NAME },
    { property: "og:locale", content: OPEN_GRAPH_LOCALE[language] },
    { name: "twitter:card", content: "summary_large_image" },
    { name: "twitter:title", content: DASHBOARD_NAME },
    { name: "twitter:description", content: description },
  ];

  if (normalizedOrigin)
    meta.push({ property: "og:url", content: normalizedOrigin });
  if (image) {
    meta.push(
      { property: "og:image", content: image },
      { property: "og:image:alt", content: imageAlt },
      { name: "twitter:image", content: image },
      { name: "twitter:image:alt", content: imageAlt },
    );
  }

  return { meta };
}

import type { en } from "@/i18n";
import type {
  StaticHeaderView,
  StaticRouteView,
} from "@/features/services/types";

/**
 * The per-field rules bex-api enforces on static-site edge rules
 * (lego/backend/internal/apps/service.go validateStaticRoutes/Headers), checked
 * in the row so an invalid rule is marked where it is, before Save, instead of
 * a toast naming "Routes[1]" by its zero-based wire index (w4/145). The server
 * stays authoritative; these are the rules a row can check on its own.
 */
export type RuleFieldErrors<T> = Partial<Record<keyof T, keyof typeof en>>;

// The server trims surrounding whitespace before checking (w4/136), so the row
// does too.
function pathError(raw: string): keyof typeof en | undefined {
  const value = raw.trim();
  if (value === "") return "services.staticRuleRequired";
  if (!value.startsWith("/")) return "services.staticRulePathSlash";
  return undefined;
}

export function routeErrors(
  route: StaticRouteView,
): RuleFieldErrors<StaticRouteView> {
  const errors: RuleFieldErrors<StaticRouteView> = {};
  const source = pathError(route.source);
  if (source) errors.source = source;
  const destination = pathError(route.destination);
  if (destination) {
    errors.destination = destination;
  } else if (route.destination.trim().startsWith("//")) {
    // A network-path reference would leave the site (the open-redirect guard,
    // ADR029).
    errors.destination = "services.staticRuleLocalPath";
  }
  return errors;
}

export function headerErrors(
  header: StaticHeaderView,
): RuleFieldErrors<StaticHeaderView> {
  const errors: RuleFieldErrors<StaticHeaderView> = {};
  const path = pathError(header.path);
  if (path) errors.path = path;
  if (header.name.trim() === "") errors.name = "services.staticRuleRequired";
  return errors;
}

/**
 * Rewrites the server's wire index ("routes[1].destination …", which reaches
 * the toast capitalized as "Routes[1]") into the row a person counts ("Row 2:
 * destination …"). A message without an index passes through unchanged.
 */
export function humanizeRuleError(
  message: string,
  rowLabel: (row: number) => string,
): string {
  return message.replace(
    /\b(?:routes|headers)\[(\d+)\]\.?/i,
    (_, index: string) => `${rowLabel(Number(index) + 1)} `,
  );
}

import { isValidDnsLabel } from "@/common/lib/utils/dns-label";

// The resource name bex-api derives from a free-form service name on create
// (w5/m153), the same mapping a Blueprint name gets: blueprintResourceSlug in
// lego/backend/internal/apps/blueprint_names.go. Keep the two in step.

const MAX_RESOURCE_NAME_LENGTH = 30;
const HASH_LENGTH = 6;

/** Lowercases one code point the way Go's unicode.ToLower does — JS's full
 *  case mapping turns U+0130 into "i" plus a combining dot. */
function lowerCodePoint(c: string): string {
  return c === "\u0130" ? "i" : c.toLowerCase();
}

/** The slug before any length cap: accents fold onto their base letters
 *  (NFKD, combining marks dropped), runs of non-[a-z0-9] collapse to one
 *  hyphen, trimmed at both ends; "" when nothing is left. */
export function uncappedSlug(name: string): string {
  let slug = "";
  let dash = false;
  const folded = [...name]
    .map(lowerCodePoint)
    .join("")
    .normalize("NFKD")
    .replace(/\p{M}/gu, "");
  for (const c of folded) {
    if ((c >= "a" && c <= "z") || (c >= "0" && c <= "9")) {
      slug += c;
      dash = false;
    } else if (!dash && slug.length > 0) {
      slug += "-";
      dash = true;
    }
  }
  return slug.replace(/-+$/, "");
}

/**
 * The derived resource name when it needs no hash, synchronously; `null` when
 * the slug is empty or longer than 30 characters and so takes a sha256 suffix
 * (use {@link resourceSlug}).
 */
export function shortResourceSlug(name: string): string | null {
  if (isValidDnsLabel(name)) return name;
  const slug = uncappedSlug(name);
  return slug === "" || slug.length > MAX_RESOURCE_NAME_LENGTH ? null : slug;
}

/** The resource name bex-api derives from `name`. */
export async function resourceSlug(name: string): Promise<string> {
  const short = shortResourceSlug(name);
  if (short !== null) return short;
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(name),
  );
  const sum = Array.from(new Uint8Array(digest).slice(0, HASH_LENGTH / 2))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  const slug = uncappedSlug(name);
  // Nothing in a-z or 0-9 left: a name-specific fallback, so two different
  // non-Latin names stay distinct (w4/385).
  if (slug === "") return `resource-${sum}`;
  const head = slug
    .slice(0, MAX_RESOURCE_NAME_LENGTH - HASH_LENGTH - 1)
    .replace(/-+$/, "");
  return `${head}-${sum}`;
}

// Render's avatar rules (w1/m648, docs/render-artifacts/avatars.md, read from
// its `avatar-utils-*.js` and `Avatar-*.js`): the eight monogram colors and
// the string hash that picks one, the default monogram of a person (their
// email's first letter) and of a workspace (its name's), and the Gravatar URL
// with its five-minute miss cache.

import { config } from "@/config/config";
import { sha256Hex } from "@/common/lib/utils/sha256";

/** Render's avatar sizes, in CSS pixels. */
export const AVATAR_PIXELS = {
  tiny: 12,
  mini: 16,
  small: 20,
  medium: 24,
  large: 32,
  xlarge: 64,
} as const;

export type AvatarSize = keyof typeof AVATAR_PIXELS;

/** Box and letter size per avatar size: Render's `w-N h-N type-avatar-*`. */
export const AVATAR_SIZE_CLASS: Record<AvatarSize, string> = {
  tiny: "size-3 text-[0.4375rem]",
  mini: "size-4 text-[0.5625rem]",
  small: "size-5 text-[0.6875rem]",
  medium: "size-6 text-xs",
  large: "size-8 text-sm",
  xlarge: "size-16 text-[1.75rem]",
};

/**
 * Render's palette, in its order: Tailwind's `bg-<color>-100 text-<color>-700`
 * pairs, kept as literal colors so the monogram the dashboard draws into a JPEG
 * is the very one it shows.
 */
export const MONOGRAM_COLORS = [
  {
    id: "blue",
    labelKey: "avatars.colorBlue",
    background: "#dbeafe",
    foreground: "#1d4ed8",
  },
  {
    id: "green",
    labelKey: "avatars.colorGreen",
    background: "#dcfce7",
    foreground: "#15803d",
  },
  {
    id: "lime-green",
    labelKey: "avatars.colorLimeGreen",
    background: "#ecfccb",
    foreground: "#4d7c0f",
  },
  {
    id: "orange",
    labelKey: "avatars.colorOrange",
    background: "#ffedd5",
    foreground: "#c2410c",
  },
  {
    id: "pink",
    labelKey: "avatars.colorPink",
    background: "#fce7f3",
    foreground: "#be185d",
  },
  {
    id: "purple",
    labelKey: "avatars.colorPurple",
    background: "#f3e8ff",
    foreground: "#7e22ce",
  },
  {
    id: "red",
    labelKey: "avatars.colorRed",
    background: "#fee2e2",
    foreground: "#b91c1c",
  },
  {
    id: "yellow",
    labelKey: "avatars.colorYellow",
    background: "#fef9c3",
    foreground: "#a16207",
  },
] as const;

/** The palette color at index, the first for an index outside it. */
export function monogramColor(index: number) {
  return MONOGRAM_COLORS[index] ?? MONOGRAM_COLORS[0];
}

/**
 * Render's color index for a seed: Java's string hash over UTF-16 code units
 * (`h = (h << 5) - h + c`, kept to 32 bits), then `Math.abs(h % 8)`.
 */
export function monogramColorIndex(seed: string): number {
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = (hash << 5) - hash + seed.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash % MONOGRAM_COLORS.length);
}

/** A monogram: one letter on one palette color. */
export interface Monogram {
  letter: string;
  colorIndex: number;
}

/** Render's rule for the Edit Monogram dialog's Save: exactly one
 *  character, on a palette color. */
export function isSavableMonogram(monogram: Monogram): boolean {
  return (
    Array.from(monogram.letter.trim()).length === 1 &&
    monogram.colorIndex >= 0 &&
    monogram.colorIndex < MONOGRAM_COLORS.length
  );
}

/** Render's email key: trimmed and lower-cased. */
export function normalizeEmail(email: string | null | undefined): string {
  return (email ?? "").trim().toLocaleLowerCase();
}

/** The first character of value (a whole code point), upper-cased. */
export function monogramLetter(value: string): string {
  return (Array.from(value.trim())[0] ?? "").toLocaleUpperCase();
}

/** A person's default monogram: their email's first letter, colored by it. */
export function emailMonogram(email: string | null | undefined): Monogram {
  const key = normalizeEmail(email);
  return { letter: monogramLetter(key), colorIndex: monogramColorIndex(key) };
}

/** A workspace's default monogram: its name's first letter, colored by its
 *  id (Render falls back to the workspace's email, which bex's have none of). */
export function workspaceMonogram(
  id: string,
  name: string | null | undefined,
): Monogram {
  return {
    letter: monogramLetter(name ?? ""),
    colorIndex: monogramColorIndex(id),
  };
}

/** Render's Gravatar URL for an email's SHA-256 hash, `d=404` so a miss is one. */
export function gravatarUrl(
  emailHash: string,
  pixels: number,
  retina = false,
): string {
  const params = new URLSearchParams({
    s: String(retina ? pixels * 2 : pixels),
    r: "x",
    d: "404",
  });
  return `https://en.gravatar.com/avatar/${emailHash}?${params.toString()}`;
}

/** Whether this screen is high-DPI, as Render's Gravatar size reads it. */
export function isRetinaScreen(): boolean {
  return typeof window !== "undefined" && (window.devicePixelRatio ?? 1) > 1.25;
}

const emailHashes = new Map<string, Promise<string>>();

/** Gravatar's key for an email: the SHA-256 of the trimmed, lower-cased
 *  address, worked out once per email however many avatars show it. */
export function gravatarHash(email: string): Promise<string> {
  const key = normalizeEmail(email);
  let hash = emailHashes.get(key);
  if (!hash) {
    hash = sha256Hex(key);
    emailHashes.set(key, hash);
  }
  return hash;
}

/** How long a Gravatar that failed to load is not asked for again. */
export const GRAVATAR_MISS_TTL_MS = 5 * 60 * 1000;

type GravatarState =
  | { found: true }
  | { found: false; failedAt: number }
  | { pending: Promise<boolean> };
const gravatars = new Map<string, GravatarState>();

function loadImage(url: string): Promise<boolean> {
  return new Promise((resolve) => {
    const image = new Image();
    image.onload = () => resolve(true);
    image.onerror = () => resolve(false);
    image.src = url;
  });
}

/**
 * Whether the email with this hash has a Gravatar. A hit is remembered for the
 * page's life and a miss for five minutes, as Render's are, per email rather
 * than per size; while one probe of `url` is in flight, every other avatar of
 * the same email waits for it instead of asking again.
 */
export function hasGravatar(
  hash: string,
  url: string,
  now = Date.now(),
  probe: (url: string) => Promise<boolean> = loadImage,
): Promise<boolean> {
  const known = gravatars.get(hash);
  if (known) {
    if ("pending" in known) return known.pending;
    if (known.found) return Promise.resolve(true);
    if (now - known.failedAt < GRAVATAR_MISS_TTL_MS) {
      return Promise.resolve(false);
    }
  }
  const pending = probe(url).then((found) => {
    gravatars.set(hash, found ? { found } : { found, failedAt: Date.now() });
    return found;
  });
  gravatars.set(hash, { pending });
  return pending;
}

/** Remembers that a shown Gravatar failed after all. */
export function forgetGravatar(hash: string) {
  gravatars.set(hash, { found: false, failedAt: Date.now() });
}

/** Test seam: forget every Gravatar result. */
export function forgetGravatars() {
  gravatars.clear();
}

/**
 * An avatar URL as bex-api answered it, made loadable: a root-relative path
 * (bex-api without BEX_API_PUBLIC_URL) is on the API origin, not the
 * dashboard's.
 */
export function resolveAvatarUrl(
  url: string | null | undefined,
): string | null {
  if (!url) return null;
  if (url.startsWith("/") && !url.startsWith("//")) {
    return `${config.apiBaseUrl ?? ""}${url}`;
  }
  return url;
}

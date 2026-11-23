import type { AnsiSpan } from "./ansi";

// Fast pre-check so lines without http(s):// skip matching and allocation
// entirely — most app log lines contain no URL and stay a single text node.
const HAS_HTTP_URL = /https?:\/\//i;

// Match http:// or https:// URLs preceded by start-of-string or a non-scheme
// character (so `myhttps://…` and `git+https://…` stay plain text), stopping
// at whitespace, quotes, angle brackets, backticks, or backslashes.
const HTTP_URL_RE = /(?<![A-Za-z0-9+.-])https?:\/\/[^\s<>"'`\\]+/gi;

const TRAILING_PUNCTUATION = new Set([".", ",", ";", ":", "!", "?"]);
const CLOSING_DELIMITERS: Record<string, string> = {
  ")": "(",
  "]": "[",
  "}": "{",
};

interface UrlRange {
  start: number;
  end: number;
  href: string;
}

export type LinkifiedTextSegment =
  { kind: "text"; text: string } | { kind: "link"; text: string; href: string };

export type LinkifiedSpanSegment =
  | { kind: "text"; spans: AnsiSpan[] }
  | { kind: "link"; spans: AnsiSpan[]; href: string };

function trimControlTail(raw: string): string {
  for (let i = 0; i < raw.length; i++) {
    const code = raw.charCodeAt(i);
    if (code <= 0x1f || code === 0x7f) return raw.slice(0, i);
  }
  return raw;
}

function countChar(text: string, target: string): number {
  let count = 0;
  for (let i = 0; i < text.length; i++) {
    if (text[i] === target) count++;
  }
  return count;
}

function trimTrailingSyntax(raw: string): string {
  let out = trimControlTail(raw);
  while (out.length > 0) {
    const last = out[out.length - 1];
    if (TRAILING_PUNCTUATION.has(last)) {
      out = out.slice(0, -1);
      continue;
    }
    const opener = CLOSING_DELIMITERS[last];
    if (opener && countChar(out, last) > countChar(out, opener)) {
      out = out.slice(0, -1);
      continue;
    }
    break;
  }
  return out;
}

function isSafeHttpUrl(candidate: string): boolean {
  let parsed: URL;
  try {
    parsed = new URL(candidate);
  } catch {
    return false;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return false;
  }
  // Reject bare punctuation hosts like `https://---` while accepting domains,
  // `localhost`, IPv4 (`0.0.0.0`), and IPv6 (`[::1]`).
  return /[A-Za-z0-9]/.test(parsed.hostname);
}

function findHttpUrls(text: string): UrlRange[] {
  if (!HAS_HTTP_URL.test(text)) return [];
  HTTP_URL_RE.lastIndex = 0;
  const ranges: UrlRange[] = [];
  let match: RegExpExecArray | null;
  while ((match = HTTP_URL_RE.exec(text)) !== null) {
    const candidate = trimTrailingSyntax(match[0]);
    if (!isSafeHttpUrl(candidate)) continue;
    ranges.push({
      start: match.index,
      end: match.index + candidate.length,
      href: candidate,
    });
  }
  return ranges;
}

/**
 * Split a plain log message into text and safe `http://` / `https://` link
 * segments, preserving the exact displayed characters (never appending a
 * trailing slash). Returns `null` when no safe HTTP(S) URL is present so the
 * caller can keep rendering the bare string.
 */
export function linkifyText(text: string): LinkifiedTextSegment[] | null {
  const ranges = findHttpUrls(text);
  if (ranges.length === 0) return null;
  const segments: LinkifiedTextSegment[] = [];
  let pos = 0;
  for (const range of ranges) {
    if (range.start > pos) {
      segments.push({ kind: "text", text: text.slice(pos, range.start) });
    }
    segments.push({
      kind: "link",
      text: text.slice(range.start, range.end),
      href: range.href,
    });
    pos = range.end;
  }
  if (pos < text.length) {
    segments.push({ kind: "text", text: text.slice(pos) });
  }
  return segments;
}

/**
 * Split ANSI-parsed spans into text and safe `http://` / `https://` link
 * segments while preserving every span's styling — including URLs that cross
 * an ANSI style boundary (such as Vite's bold port inside a cyan URL).
 * Returns `null` when no safe HTTP(S) URL is present.
 */
export function linkifySpans(
  spans: readonly AnsiSpan[],
): LinkifiedSpanSegment[] | null {
  let fullText = "";
  for (const span of spans) fullText += span.text;
  const ranges = findHttpUrls(fullText);
  if (ranges.length === 0) return null;

  const segments: LinkifiedSpanSegment[] = [];
  let spanIndex = 0;
  let spanOffset = 0;
  let globalPos = 0;

  const takeSpans = (until: number): AnsiSpan[] => {
    const out: AnsiSpan[] = [];
    while (globalPos < until && spanIndex < spans.length) {
      const span = spans[spanIndex];
      const remaining = span.text.length - spanOffset;
      const take = Math.min(remaining, until - globalPos);
      if (take > 0) {
        const slice =
          spanOffset === 0 && take === span.text.length
            ? span
            : {
                text: span.text.slice(spanOffset, spanOffset + take),
                className: span.className,
                style: span.style,
              };
        out.push(slice);
        globalPos += take;
        spanOffset += take;
      }
      if (spanOffset >= span.text.length) {
        spanIndex++;
        spanOffset = 0;
      }
    }
    return out;
  };

  for (const range of ranges) {
    if (range.start > globalPos) {
      segments.push({ kind: "text", spans: takeSpans(range.start) });
    }
    segments.push({
      kind: "link",
      href: range.href,
      spans: takeSpans(range.end),
    });
  }
  if (globalPos < fullText.length) {
    segments.push({ kind: "text", spans: takeSpans(fullText.length) });
  }
  return segments;
}

// The audit row's short form of the user agent an event was recorded with
// (w9/m450): the browser and its operating system for a browser, else the
// client's first product token ("render-cli/2.27.0", "curl/8.4.0"). The full
// value stays one hover away in the cell's title, so this is a label, not a
// parser: anything it does not recognize reads as its first token.

/** Checked in order: Edge and Opera also name Chrome, Chrome names Safari. */
const BROWSERS: [RegExp, string][] = [
  [/\bEdg(?:e|A|iOS)?\/(\d+)/, "Edge"],
  [/\bOPR\/(\d+)/, "Opera"],
  [/\bFirefox\/(\d+)/, "Firefox"],
  [/\bFxiOS\/(\d+)/, "Firefox"],
  [/\bCriOS\/(\d+)/, "Chrome"],
  [/\bChrome\/(\d+)/, "Chrome"],
  [/\bVersion\/(\d+)[\d.]* (?:Mobile\/\S+ )?Safari\//, "Safari"],
];

/** Checked in order: iOS and Android user agents also name macOS and Linux. */
const SYSTEMS: [RegExp, string][] = [
  [/\b(?:iPhone|iPad|iPod)\b/, "iOS"],
  [/\bAndroid\b/, "Android"],
  [/\bWindows\b/, "Windows"],
  [/\bMac OS X\b|\bMacintosh\b/, "macOS"],
  [/\bCrOS\b/, "ChromeOS"],
  [/\bLinux\b/, "Linux"],
];

export function shortUserAgent(userAgent: string): string {
  const ua = userAgent.trim();
  if (!ua) return "";
  for (const [pattern, browser] of BROWSERS) {
    const match = pattern.exec(ua);
    if (!match) continue;
    const system = SYSTEMS.find(([os]) => os.test(ua))?.[1];
    const name = `${browser} ${match[1]}`;
    return system ? `${name} · ${system}` : name;
  }
  return ua.split(/\s/, 1)[0];
}

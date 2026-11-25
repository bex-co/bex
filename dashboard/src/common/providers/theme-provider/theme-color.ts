/**
 * The page colour of each theme (w3/098), for `<meta name="theme-color">`:
 * the colour a phone's browser bar and an installed app's title bar take from
 * the page. Render's theme provider gives it the current theme's page colour
 * (`page-primary`, docs/render-artifacts/account-appearance.md § The browser
 * copy and other tabs); bex's is the theme's `--background` (style.css) in
 * sRGB, which High Contrast Mode leaves as it is. The pre-paint script sets it
 * from these values before the first paint (theme-script.js.txt) and the
 * ThemeProvider keeps it in step with the applied theme.
 */
export const THEME_COLORS = { light: "#f8f8fa", dark: "#040608" } as const;

/** Gives the document's one `<meta name="theme-color">` the page colour of
 *  `theme`, adding the tag when the document has none. */
export function applyThemeColor(theme: keyof typeof THEME_COLORS) {
  let meta = document.head.querySelector<HTMLMetaElement>(
    'meta[name="theme-color"]',
  );
  if (!meta) {
    meta = document.createElement("meta");
    meta.name = "theme-color";
    document.head.appendChild(meta);
  }
  meta.content = THEME_COLORS[theme];
}

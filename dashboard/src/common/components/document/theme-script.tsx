import { THEME_COLORS } from "@/common/providers/theme-provider/theme-color";
import source from "./theme-script.js.txt?raw";

/** The pre-paint script, with each theme's page colour filled in. */
// eslint-disable-next-line react-refresh/only-export-components -- the script's text, for its test
export const themeScript = source.replace(
  "__THEME_COLORS__",
  JSON.stringify(THEME_COLORS),
);

export function ThemeScript() {
  return <script dangerouslySetInnerHTML={{ __html: themeScript }} />;
}

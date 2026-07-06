import script from "./theme-script.js.txt?raw";

export function ThemeScript() {
  return <script dangerouslySetInnerHTML={{ __html: script }} />;
}

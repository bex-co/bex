import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { Appearance } from "react-native";
import {
  getSystemColorScheme,
  themes,
  ThemeProvider as BaseThemeProvider,
} from "@/common/theme";
import {
  loadThemeMode,
  resolveScheme,
  saveThemeMode,
  type ThemeMode,
} from "@/common/preferences/preferences";

type ThemeModeContextValue = {
  /** The user's preference: an explicit scheme, or "system" to track the OS. */
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
};

const ThemeModeContext = createContext<ThemeModeContextValue>({
  mode: "system",
  setMode: () => {},
});

/** Read/set the persisted color-scheme preference from the settings screen. */
export const useThemeMode = () => useContext(ThemeModeContext);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [scheme, setScheme] = useState(getSystemColorScheme());
  const [mode, setModeState] = useState<ThemeMode>("system");

  useEffect(() => {
    const subscription = Appearance.addChangeListener(({ colorScheme }) => {
      setScheme(colorScheme === "dark" ? "dark" : "light");
    });
    return () => subscription.remove();
  }, []);

  // Restore the saved preference once on mount; until it resolves we render the
  // system scheme, so the first paint never flashes the wrong colors.
  useEffect(() => {
    let active = true;
    void loadThemeMode().then((stored) => {
      if (active && stored) setModeState(stored);
    });
    return () => {
      active = false;
    };
  }, []);

  const setMode = useMemo(
    () => (next: ThemeMode) => {
      setModeState(next);
      void saveThemeMode(next);
    },
    [],
  );

  const value = useMemo(() => ({ mode, setMode }), [mode, setMode]);
  const effective = resolveScheme(mode, scheme);

  return (
    <ThemeModeContext.Provider value={value}>
      <BaseThemeProvider theme={themes[effective]}>
        {children}
      </BaseThemeProvider>
    </ThemeModeContext.Provider>
  );
}

import { createContext } from "react";
import type { ThemeProviderContextType } from "./type.ts";

const initialState: ThemeProviderContextType = {
  theme: "system",
  setTheme: () => null,
};

export const ThemeProviderContext =
  createContext<ThemeProviderContextType>(initialState);

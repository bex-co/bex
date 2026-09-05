// Mock dependencies BEFORE any imports
const Module = require("module");
const originalRequire = Module.prototype.require;

let mockColorScheme: string | null | undefined = "light";

Module.prototype.require = function (this: NodeModule, id: string) {
  // Mock react-native
  if (id === "react-native") {
    return {
      Appearance: {
        getColorScheme: () => mockColorScheme,
      },
      Platform: {
        OS: "ios",
        select: (options: { ios?: unknown; default?: unknown }) =>
          "ios" in options ? options.ios : options.default,
      },
    };
  }

  // Mock @callstack/react-theme-provider
  if (id === "@callstack/react-theme-provider") {
    return {
      createTheming: (theme: unknown) => ({
        ThemeProvider: theme,
        withTheme: (component: unknown) => component,
        useTheme: () => theme,
      }),
    };
  }

  return originalRequire.apply(this, arguments);
};

import { getSystemColorScheme, themes, space, gutter } from "../index";

describe("getSystemColorScheme", () => {
  it("returns light when system color scheme is light", () => {
    mockColorScheme = "light";
    const result = getSystemColorScheme();
    expect(result).toBe("light");
  });

  it("returns dark when system color scheme is dark", () => {
    mockColorScheme = "dark";
    const result = getSystemColorScheme();
    expect(result).toBe("dark");
  });

  it("returns light when system color scheme is null", () => {
    mockColorScheme = null;
    const result = getSystemColorScheme();
    expect(result).toBe("light");
  });

  it("returns light when system color scheme is undefined", () => {
    mockColorScheme = undefined;
    const result = getSystemColorScheme();
    expect(result).toBe("light");
  });
});

describe("themes", () => {
  it("contains light theme definition", () => {
    expect(themes.light).toBeTruthy();
    expect(themes.light.name).toBe("light");
    expect(themes.light.colorTheme).toBeTruthy();
  });

  it("contains dark theme definition", () => {
    expect(themes.dark).toBeTruthy();
    expect(themes.dark.name).toBe("dark");
    expect(themes.dark.colorTheme).toBeTruthy();
  });

  it("light theme has expected color properties", () => {
    const { colorTheme } = themes.light;
    expect(colorTheme.white).toBe("#ffffff");
    expect(colorTheme.black).toBe("#202420");
    expect(colorTheme.primary).toBe("#2f7d32");
  });

  it("dark theme has expected color properties", () => {
    const { colorTheme } = themes.dark;
    expect(colorTheme.white).toBe("#191d19");
    expect(colorTheme.black).toBe("#f1f4f1");
    expect(colorTheme.primary).toBe("#74c875");
  });

  it("exposes an ascending 4pt-based spacing scale", () => {
    const scale = [
      space.xxs,
      space.xs,
      space.sm,
      space.md,
      space.lg,
      space.xl,
      space.xxl,
    ];
    expect(scale).toEqual([2, 4, 8, 12, 16, 24, 32]);
    // Strictly ascending, so tokens can't silently collide.
    const isAscending = scale.every(
      (value, i) => i === 0 || value > scale[i - 1],
    );
    expect(isAscending).toBe(true);
  });

  it("gutter is the canonical 16pt horizontal inset", () => {
    expect(gutter).toBe(16);
    expect(gutter).toBe(space.lg);
  });

  it("both themes have error colors", () => {
    expect(themes.light.colorTheme.error).toBe("#c63f36");
    expect(themes.dark.colorTheme.error).toBe("#ec7168");
  });

  it("both themes have success colors", () => {
    expect(themes.light.colorTheme.success).toBe("#27833f");
    expect(themes.dark.colorTheme.success).toBe("#62c87b");
  });

  it("both themes have warning colors", () => {
    expect(themes.light.colorTheme.warning).toBe("#b26a12");
    expect(themes.dark.colorTheme.warning).toBe("#e0a14a");
  });

  it("light theme has white background", () => {
    expect(themes.light.colorTheme.navBg).toBe("#ffffff");
    expect(themes.light.colorTheme.activeBackgroundColor).toBe("#ffffff");
  });

  it("dark theme has charcoal background", () => {
    expect(themes.dark.colorTheme.navBg).toBe("#191d19");
    expect(themes.dark.colorTheme.activeBackgroundColor).toBe("#191d19");
  });
});

describe("primary action legibility", () => {
  const luminance = (hex: string) => {
    const values = [1, 3, 5]
      .map((offset) => parseInt(hex.slice(offset, offset + 2), 16) / 255)
      .map((value) =>
        value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4,
      );
    return values[0] * 0.2126 + values[1] * 0.7152 + values[2] * 0.0722;
  };
  it("keeps primary button text above 4.5:1 in both color schemes", () => {
    for (const theme of Object.values(themes)) {
      const foreground = luminance(theme.colorTheme.onPrimary);
      const background = luminance(theme.colorTheme.primary);
      expect(
        (Math.max(foreground, background) + 0.05) /
          (Math.min(foreground, background) + 0.05) >=
          4.5,
      ).toBe(true);
    }
  });
});

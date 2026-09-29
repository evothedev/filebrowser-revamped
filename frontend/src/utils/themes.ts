// The themes the UI offers, in the order they are offered.
//
// Each theme names the CSS class that activates it (see css/_themes.css) and
// declares whether it is light or dark. The mode is not cosmetic: the editor
// fallback and the epub reader both need to know which ace theme to fall back
// to, and a theme that only listed colours could not tell them.
//
// A theme with a sibling in the other mode sets "counterpart", which is where
// the light/dark toggle goes. Without one — Dracula, which has no light variant
// — the toggle falls back to the plain light/dark pair.

export type ThemeMode = "light" | "dark";

export interface Theme {
  // The value persisted in settings.branding.theme, and the class set on <html>.
  name: string;
  mode: ThemeMode;
  counterpart?: string;
  // Enough of the palette to render a recognisable preview without activating
  // the theme: the page background, a raised surface, and the accent.
  swatch: {
    background: string;
    surface: string;
    accent: string;
    text: string;
  };
}

export const THEMES: readonly Theme[] = [
  {
    name: "light",
    mode: "light",
    counterpart: "dark",
    swatch: {
      background: "#FAFAFA",
      surface: "#FFFFFF",
      accent: "#2196F3",
      text: "#333333",
    },
  },
  {
    name: "dark",
    mode: "dark",
    counterpart: "light",
    swatch: {
      background: "#141D24",
      surface: "#20292F",
      accent: "#2196F3",
      text: "#FFFFFF",
    },
  },
  {
    name: "nord-light",
    mode: "light",
    counterpart: "nord-dark",
    swatch: {
      background: "#ECEFF4",
      surface: "#FFFFFF",
      accent: "#5E81AC",
      text: "#2E3440",
    },
  },
  {
    name: "nord-dark",
    mode: "dark",
    counterpart: "nord-light",
    swatch: {
      background: "#2E3440",
      surface: "#3B4252",
      accent: "#88C0D0",
      text: "#ECEFF4",
    },
  },
  {
    name: "solarized-light",
    mode: "light",
    counterpart: "solarized-dark",
    swatch: {
      background: "#FDF6E3",
      surface: "#EEE8D5",
      accent: "#268BD2",
      text: "#586E75",
    },
  },
  {
    name: "solarized-dark",
    mode: "dark",
    counterpart: "solarized-light",
    swatch: {
      background: "#002B36",
      surface: "#073642",
      accent: "#268BD2",
      text: "#EEE8D5",
    },
  },
  {
    name: "gruvbox-light",
    mode: "light",
    counterpart: "gruvbox-dark",
    swatch: {
      background: "#F2E5BC",
      surface: "#FBF1C7",
      accent: "#458588",
      text: "#3C3836",
    },
  },
  {
    name: "gruvbox-dark",
    mode: "dark",
    counterpart: "gruvbox-light",
    swatch: {
      background: "#282828",
      surface: "#3C3836",
      accent: "#83A598",
      text: "#EBDBB2",
    },
  },
  {
    name: "dracula",
    mode: "dark",
    swatch: {
      background: "#282A36",
      surface: "#343746",
      accent: "#BD93F9",
      text: "#F8F8F2",
    },
  },
];

const BY_NAME = new Map(THEMES.map((theme) => [theme.name, theme]));

export function getThemeDefinition(name: string): Theme | undefined {
  return BY_NAME.get(name);
}

export function isKnownTheme(name: string): boolean {
  return BY_NAME.has(name);
}

// getThemeMode reports whether a theme is dark. An unknown name is reported as
// light, which is what an unstyled :root would render, so a name the server
// sends that this build does not know about degrades to the light appearance
// rather than to an unreadable one.
export function getThemeMode(name: string): ThemeMode {
  return BY_NAME.get(name)?.mode ?? "light";
}

// getThemeCounterpart is where the light/dark toggle goes from a theme. A theme
// with no sibling in the other mode goes to the plain theme of that mode.
export function getThemeCounterpart(name: string): string {
  const theme = BY_NAME.get(name);
  if (!theme) {
    return "light";
  }

  if (theme.counterpart && BY_NAME.has(theme.counterpart)) {
    return theme.counterpart;
  }

  return theme.mode === "dark" ? "light" : "dark";
}

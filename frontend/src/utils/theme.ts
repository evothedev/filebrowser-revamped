import { theme } from "./constants";
import { getThemeCounterpart, getThemeMode, isKnownTheme } from "./themes";
import "ace-builds";
import { themesByName } from "ace-builds/src-noconflict/ext-themelist";

export const getTheme = (): UserTheme => {
  return (document.documentElement.className as UserTheme) || theme;
};

export const setTheme = (theme: UserTheme) => {
  const html = document.documentElement;
  if (!theme) {
    html.className = getMediaPreference();
  } else if (isKnownTheme(theme)) {
    html.className = theme;
  } else {
    // The theme name is whatever the server has stored, and an older build may
    // have stored a name a newer one dropped. An unknown class would leave the
    // page unthemed, so fall back to the light appearance rather than applying
    // a class that does not exist.
    html.className = getMediaPreference();
  }
};

export const toggleTheme = (): void => {
  setTheme(getThemeCounterpart(getTheme()) as UserTheme);
};

// isDarkTheme is for the places that need the mode rather than the name: the
// editor's fallback theme, and the epub reader's text colour.
export const isDarkTheme = (): boolean => {
  return getThemeMode(getTheme()) === "dark";
};

export const getMediaPreference = (): UserTheme => {
  const hasDarkPreference = window.matchMedia(
    "(prefers-color-scheme: dark)"
  ).matches;
  if (hasDarkPreference) {
    return "dark";
  } else {
    return "light";
  }
};

export const getEditorTheme = (themeName: string) => {
  if (!themeName.startsWith("ace/theme/")) {
    themeName = `ace/theme/${themeName}`;
  }
  const themeKey = themeName.replace("ace/theme/", "");
  if (themesByName[themeKey] !== undefined) {
    return themeName;
  } else if (isDarkTheme()) {
    return "ace/theme/twilight";
  } else {
    return "ace/theme/chrome";
  }
};

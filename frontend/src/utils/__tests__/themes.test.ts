import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import {
  getThemeCounterpart,
  getThemeDefinition,
  getThemeMode,
  isKnownTheme,
  THEMES,
} from "../themes";

const readCss = (file: string) =>
  readFileSync(resolve(__dirname, "../../css", file), "utf8");

const variablesCss = readCss("_variables.css");
const themesCss = readCss("_themes.css");

// Every custom property :root declares. A named theme has to set all of them:
// partial overrides would silently inherit the default theme's colours, which
// is the kind of bug that only shows up on one palette in one dropdown state.
function rootVariables(): string[] {
  const root = variablesCss.slice(
    variablesCss.indexOf(":root {"),
    variablesCss.indexOf(":root.dark")
  );
  return [...root.matchAll(/^\s*(--[\w-]+)\s*:/gm)].map((m) => m[1]);
}

// The declarations inside one `:root.<name> { ... }` block.
function themeBlock(name: string): string {
  const start = themesCss.indexOf(`:root.${name} {`);
  expect(start, `no CSS block for theme "${name}"`).toBeGreaterThan(-1);
  const end = themesCss.indexOf("}", start);
  return themesCss.slice(start, end);
}

function declaredVariables(block: string): string[] {
  return [...block.matchAll(/^\s*(--[\w-]+)\s*:/gm)].map((m) => m[1]);
}

describe("theme registry", () => {
  it("has no duplicate names", () => {
    const names = THEMES.map((t) => t.name);
    expect(new Set(names).size).toBe(names.length);
  });

  it("keeps the built-in light and dark themes first", () => {
    expect(THEMES.slice(0, 2).map((t) => t.name)).toEqual(["light", "dark"]);
  });

  it("offers Nord, Dracula, Solarized and Gruvbox in both modes where they have them", () => {
    const names = THEMES.map((t) => t.name);
    expect(names).toEqual(
      expect.arrayContaining([
        "nord-light",
        "nord-dark",
        "solarized-light",
        "solarized-dark",
        "gruvbox-light",
        "gruvbox-dark",
        "dracula",
      ])
    );
  });

  it("agrees with the UserTheme union", () => {
    const userDts = readFileSync(
      resolve(__dirname, "../../types/user.d.ts"),
      "utf8"
    );
    const declaration = userDts.slice(userDts.indexOf("type UserTheme"));
    const declared = [...declaration.matchAll(/\|\s*"([\w-]*)"/g)].map(
      (m) => m[1]
    );

    for (const theme of THEMES) {
      expect(declared).toContain(theme.name);
    }
    // Every member is either a theme or the empty "follow the OS" value.
    expect(declared.filter((n) => n !== "")).toHaveLength(THEMES.length);
  });

  it("gives every theme a label in en.json", () => {
    const en = JSON.parse(readCss("../i18n/en.json"));
    for (const theme of THEMES) {
      expect(
        en.settings.themes[theme.name],
        `missing label for "${theme.name}"`
      ).toBeTruthy();
    }
    expect(en.settings.themes.default).toBeTruthy();
  });
});

describe("theme CSS", () => {
  const required = rootVariables();

  // light and dark are the base: light is :root in _variables.css and dark
  // only overrides what it changes, inheriting the rest. Every *named* theme
  // has to be self-contained, because it inherits from :root, not from dark.
  const named = THEMES.filter((t) => t.name !== "light" && t.name !== "dark");

  it("declares a usable number of variables to override", () => {
    expect(required.length).toBeGreaterThan(20);
  });

  it.each(named.map((t) => t.name))(
    "defines every variable for %s so nothing leaks from the default theme",
    (name) => {
      const declared = declaredVariables(themeBlock(name));
      const missing = required.filter((v) => !declared.includes(v));
      expect(missing, `${name} is missing variables`).toEqual([]);
    }
  );

  it("only contains blocks for registered themes, one each", () => {
    const blocks = [...themesCss.matchAll(/^:root\.([\w-]+)\s*\{/gm)].map(
      (m) => m[1]
    );
    // Order in the stylesheet is irrelevant to CSS, and the file groups each
    // palette dark-first, so compare as sets and separately reject duplicates.
    expect(new Set(blocks).size).toBe(blocks.length);
    expect([...blocks].sort()).toEqual(named.map((t) => t.name).sort());
  });

  it("leaves the built-in themes in _variables.css", () => {
    expect(variablesCss).toContain(":root {");
    expect(variablesCss).toContain(":root.dark {");
  });

  it("is imported after the variables it overrides", () => {
    const order = readCss("styles.css");
    expect(order.indexOf("_themes.css")).toBeGreaterThan(
      order.indexOf("_variables.css")
    );
  });
});

describe("getThemeMode", () => {
  it.each([
    ["light", "light"],
    ["nord-light", "light"],
    ["solarized-light", "light"],
    ["gruvbox-light", "light"],
    ["dark", "dark"],
    ["nord-dark", "dark"],
    ["solarized-dark", "dark"],
    ["gruvbox-dark", "dark"],
    ["dracula", "dark"],
  ] as const)("reports %s as %s", (name, mode) => {
    expect(getThemeMode(name)).toBe(mode);
  });

  it("treats an unknown name as light, which is what an unstyled :root renders", () => {
    expect(getThemeMode("a-theme-this-build-does-not-know")).toBe("light");
  });
});

describe("getThemeCounterpart", () => {
  it("stays within a palette", () => {
    expect(getThemeCounterpart("nord-light")).toBe("nord-dark");
    expect(getThemeCounterpart("nord-dark")).toBe("nord-light");
    expect(getThemeCounterpart("solarized-light")).toBe("solarized-dark");
    expect(getThemeCounterpart("gruvbox-dark")).toBe("gruvbox-light");
    expect(getThemeCounterpart("light")).toBe("dark");
    expect(getThemeCounterpart("dark")).toBe("light");
  });

  it("falls back to the plain light theme for Dracula, which has no light variant", () => {
    expect(getThemeCounterpart("dracula")).toBe("light");
  });

  it("round trips for every theme that has a counterpart", () => {
    for (const theme of THEMES) {
      if (!theme.counterpart) {
        continue;
      }
      expect(getThemeCounterpart(getThemeCounterpart(theme.name))).toBe(
        theme.name
      );
    }
  });
});

describe("lookups", () => {
  it("finds known themes and rejects unknown ones", () => {
    expect(isKnownTheme("dracula")).toBe(true);
    expect(isKnownTheme("")).toBe(false);
    expect(isKnownTheme("nope")).toBe(false);
    expect(getThemeDefinition("nord-dark")?.mode).toBe("dark");
    expect(getThemeDefinition("nope")).toBeUndefined();
  });
});

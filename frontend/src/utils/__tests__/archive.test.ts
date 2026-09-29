import { describe, expect, it } from "vitest";
import { archiveBaseName, isArchive } from "../archive";

function item(name: string, isDir = false): ResourceItem {
  return {
    path: `/${name}`,
    name,
    size: 1024,
    extension: name.slice(name.lastIndexOf(".")),
    modified: "2024-01-01T00:00:00Z",
    mode: 420,
    isDir,
    isSymlink: false,
    type: "blob",
    url: `/files/${name}`,
    index: 0,
  };
}

describe("isArchive", () => {
  it.each(["notes.txt", "photos.jpg", "noextension", "README"])(
    "does not offer to extract %s",
    (name) => {
      expect(isArchive(item(name))).toBe(false);
    }
  );

  it.each([
    "bundle.zip",
    "BUNDLE.ZIP",
    "logs.tar",
    "logs.tar.gz",
    "logs.tar.bz2",
    "logs.tar.xz",
    "logs.tar.zst",
    "app.jar",
    "photos.7z",
    "backup.rar",
    "notes.gz",
  ])("offers to extract %s", (name) => {
    expect(isArchive(item(name))).toBe(true);
  });

  it("treats a directory named like an archive as a directory", () => {
    expect(isArchive(item("bundle.zip", true))).toBe(false);
  });

  it("does not mistake a dotfile for an extension", () => {
    expect(isArchive(item(".zip"))).toBe(false);
  });
});

describe("archiveBaseName", () => {
  it.each([
    ["bundle.zip", "bundle"],
    ["logs.tar.gz", "logs"],
    ["logs.tar.bz2", "logs"],
    ["logs.tar", "logs"],
    ["app.jar", "app"],
    // Nothing that looks like a suffix to strip: the name is left alone rather
    // than trimmed to something the user did not ask for.
    ["archive", "archive"],
  ])("names the folder for %s as %s", (name, expected) => {
    expect(archiveBaseName(name)).toBe(expected);
  });

  it("keeps the case of the name it was given", () => {
    expect(archiveBaseName("Bundle.TAR.GZ")).toBe("Bundle");
  });
});

// The suffixes the server is able to extract, mirroring the formats
// github.com/mholt/archives can identify.
//
// The tar formats are normally named with a compound suffix ("x.tar.gz"), but
// only the last dot-separated piece is an extension, and every compound suffix
// ends in a compression format that is itself listed here. So matching on the
// last piece alone already covers them, and ".tar.bz2" is matched by ".bz2".
const ARCHIVE_EXTENSIONS = new Set([
  ".zip",
  ".jar",
  ".war",
  ".ear",
  ".apk",
  ".whl",
  ".ipa",
  ".epub",
  ".xpi",
  ".crx",
  ".vsix",
  ".nupkg",
  ".tar",
  ".gz",
  ".tgz",
  ".bz2",
  ".tbz",
  ".tbz2",
  ".xz",
  ".txz",
  ".lz4",
  ".tlz4",
  ".sz",
  ".tsz",
  ".br",
  ".tbr",
  ".zst",
  ".tzst",
  ".rar",
  ".7z",
]);

// isArchive reports whether the item is something the Extract action can be
// offered for.
//
// The listing cannot tell an archive from any other opaque file: the server
// types a ".zip" as "blob" like any other binary. So the name is the only thing
// to go on, and a file that slips through the list is rejected by the server
// with a clear error rather than by this check.
export function isArchive(item: ResourceItem): boolean {
  if (item.isDir) {
    return false;
  }

  const name = item.name.toLowerCase();
  const dot = name.lastIndexOf(".");

  // A leading dot marks a hidden file, not an extension, so ".zip" on its own
  // is a dotfile and not an archive.
  return dot > 0 && ARCHIVE_EXTENSIONS.has(name.slice(dot));
}

// archiveBaseName strips the archive suffixes from name to get the name a new
// folder for its contents would take. "bundle.zip" gives "bundle", and
// "backup.tar.gz" gives "backup" rather than "backup.tar".
export function archiveBaseName(name: string): string {
  const lower = name.toLowerCase();

  let suffix = "";
  for (const ext of ARCHIVE_EXTENSIONS) {
    if (
      ext.length > suffix.length &&
      lower.length > ext.length &&
      lower.endsWith(ext)
    ) {
      suffix = ext;
    }
  }

  if (suffix === "") {
    return name;
  }

  const stem = name.slice(0, name.length - suffix.length);

  // What is left is the compression suffix's own tar wrapper, which is part of
  // the archive name rather than part of what it contains.
  if (stem.length > 4 && stem.toLowerCase().endsWith(".tar")) {
    return stem.slice(0, -".tar".length);
  }

  return stem;
}

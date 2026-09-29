package fbhttp

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/users"
)

// zipEntry is one member to place in a test archive.
type zipEntry struct {
	name    string
	content string
}

func extractKey() []byte { return []byte("test-signing-key") }

// extractTestStorage is scopedUserStorage with modes that let the test read back
// what it wrote, plus any rules the case needs.
func extractTestStorage(t *testing.T, userScope string, perm users.Permissions, globalRules []rules.Rule) *storage.Storage {
	t.Helper()

	st := scopedUserStorage(t, userScope, perm, extractKey())
	if err := st.Settings.Save(&settings.Settings{
		Key:      extractKey(),
		FileMode: 0o644,
		DirMode:  0o755,
		Rules:    globalRules,
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	return st
}

func writeTestZip(t *testing.T, path string, entries []zipEntry) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("failed to add %s: %v", e.name, err)
		}
		if _, err := w.Write([]byte(e.content)); err != nil {
			t.Fatalf("failed to write %s: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close the archive: %v", err)
	}
}

func writeTestTarGz(t *testing.T, path string, entries []zipEntry) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		body := []byte(e.content)
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if strings.HasSuffix(e.name, "/") {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to add %s: %v", e.name, err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(body); err != nil {
				t.Fatalf("failed to write %s: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close the tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close the gzip stream: %v", err)
	}
}

// extract runs the PATCH action and returns the response.
func extract(t *testing.T, st *storage.Storage, token, target string) *httptest.ResponseRecorder {
	t.Helper()

	req, _ := http.NewRequest(http.MethodPatch, target, http.NoBody)
	req.Header.Set("X-Auth", token)
	rec := httptest.NewRecorder()
	handle(resourcePatchHandler(diskcache.NewNoOp()), "", st, &settings.Server{}).ServeHTTP(rec, req)

	return rec
}

func extractResultBody(t *testing.T, rec *httptest.ResponseRecorder) extractResult {
	t.Helper()

	var result extractResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}

	return result
}

func readInScope(t *testing.T, userScope, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(userScope, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("expected %s to exist: %v", rel, err)
	}

	return string(data)
}

func writeInScope(t *testing.T, userScope, rel, content string) {
	t.Helper()

	p := filepath.Join(userScope, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("failed to create the parent of %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", rel, err)
	}
}

func TestExtractsZipIntoTheArchivesOwnDirectory(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "notes.txt", content: "hello"},
		{name: "docs/", content: ""},
		{name: "docs/readme.md", content: "# readme"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	// No destination: the archive's own directory.
	rec := extract(t, st, token, "/bundle.zip?action=extract")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q; want 200", rec.Code, rec.Body.String())
	}

	if got := readInScope(t, userScope, "notes.txt"); got != "hello" {
		t.Errorf("notes.txt = %q; want %q", got, "hello")
	}
	if got := readInScope(t, userScope, "docs/readme.md"); got != "# readme" {
		t.Errorf("docs/readme.md = %q; want %q", got, "# readme")
	}

	result := extractResultBody(t, rec)
	if result.Destination != "/" {
		t.Errorf("destination = %q; want %q", result.Destination, "/")
	}
	if result.Files != 2 || result.Dirs != 1 {
		t.Errorf("files = %d dirs = %d; want 2 and 1", result.Files, result.Dirs)
	}
}

func TestExtractsZipIntoANewFolder(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "notes.txt", content: "hello"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/bundle.zip?action=extract&destination=/bundle")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q; want 200", rec.Code, rec.Body.String())
	}

	if got := readInScope(t, userScope, "bundle/notes.txt"); got != "hello" {
		t.Errorf("bundle/notes.txt = %q; want %q", got, "hello")
	}

	if result := extractResultBody(t, rec); result.Destination != "/bundle" {
		t.Errorf("destination = %q; want %q", result.Destination, "/bundle")
	}
}

// A destination that is already taken is a conflict, so a second extraction
// into the same new folder cannot silently merge with the first.
func TestExtractRefusesAnExistingNamedDestination(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "notes.txt", content: "hello"},
	})
	if err := os.MkdirAll(filepath.Join(userScope, "bundle"), 0o755); err != nil {
		t.Fatal(err)
	}

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/bundle.zip?action=extract&destination=/bundle")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %q; want 409", rec.Code, rec.Body.String())
	}
}

func TestExtractsTarGz(t *testing.T) {
	userScope := t.TempDir()
	writeTestTarGz(t, filepath.Join(userScope, "logs.tar.gz"), []zipEntry{
		{name: "logs/", content: ""},
		{name: "logs/today.txt", content: "entries"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/logs.tar.gz?action=extract")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q; want 200", rec.Code, rec.Body.String())
	}

	if got := readInScope(t, userScope, "logs/today.txt"); got != "entries" {
		t.Errorf("logs/today.txt = %q; want %q", got, "entries")
	}
}

func TestExtractRefusesANonArchive(t *testing.T) {
	userScope := t.TempDir()
	writeInScope(t, userScope, "plain.txt", "not an archive")

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/plain.txt?action=extract")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %q; want 400", rec.Code, rec.Body.String())
	}
}

func TestExtractRefusesADirectory(t *testing.T) {
	userScope := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userScope, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/folder/?action=extract")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %q; want 400", rec.Code, rec.Body.String())
	}
}

// The classic zip-slip: an entry that climbs out of the destination with "..".
// The request must fail and nothing may be written outside the destination.
func TestExtractRejectsTraversalEntries(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "evil.zip"), []zipEntry{
		{name: "first.txt", content: "written before the bad entry"},
		{name: "../escaped.txt", content: "VULNERABLE"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/evil.zip?action=extract")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %q; want 400", rec.Code, rec.Body.String())
	}

	// The entry that came before the bad one must have been unwound, not left
	// behind as a half-finished extraction, and nothing may have escaped the
	// destination either.
	for _, survivor := range []string{"first.txt", "escaped.txt"} {
		if _, err := os.Stat(filepath.Join(userScope, survivor)); err == nil {
			t.Fatalf("a rejected archive left %s behind instead of unwinding", survivor)
		}
	}
	if _, err := os.Stat(filepath.Join(userScope, "..", "escaped.txt")); err == nil {
		t.Fatal("VULNERABLE: escaped.txt was written outside the destination")
	}
}

func TestExtractRejectsAbsoluteEntries(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "evil.zip"), []zipEntry{
		{name: "/etc/absolute.txt", content: "VULNERABLE"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/evil.zip?action=extract")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %q; want 400", rec.Code, rec.Body.String())
	}
}

// A rule can deny a path deep inside the destination while allowing the
// destination itself, so the extraction has to check every entry rather than
// trusting the authorization the handler did for the archive.
func TestExtractChecksRulesForEveryEntry(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "allowed.txt", content: "fine"},
		{name: "denied/secret.txt", content: "VULNERABLE"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	deny := rules.Rule{Path: "/denied", Allow: false}
	st := extractTestStorage(t, userScope, perm, []rules.Rule{deny})
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/bundle.zip?action=extract")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %q; want 403", rec.Code, rec.Body.String())
	}

	if _, err := os.Stat(filepath.Join(userScope, "denied")); err == nil {
		t.Error("VULNERABLE: a denied directory was created")
	}
	// Nothing survives a rejected extraction, including the entry that was
	// allowed.
	if _, err := os.Stat(filepath.Join(userScope, "allowed.txt")); err == nil {
		t.Error("a rejected archive left allowed.txt behind instead of unwinding")
	}
}

func TestExtractLeavesExistingFilesAloneUnlessAskedToOverwrite(t *testing.T) {
	entries := []zipEntry{{name: "notes.txt", content: "from the archive"}}

	t.Run("without override", func(t *testing.T) {
		userScope := t.TempDir()
		writeTestZip(t, filepath.Join(userScope, "bundle.zip"), entries)
		writeInScope(t, userScope, "notes.txt", "already here")

		perm := users.Permissions{Create: true, Modify: true}
		st := extractTestStorage(t, userScope, perm, nil)
		token := signToken(t, perm, extractKey())

		rec := extract(t, st, token, "/bundle.zip?action=extract")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body = %q; want 200", rec.Code, rec.Body.String())
		}

		if got := readInScope(t, userScope, "notes.txt"); got != "already here" {
			t.Errorf("notes.txt = %q; want the existing file to be left alone", got)
		}
		if result := extractResultBody(t, rec); result.Skipped != 1 || result.Files != 0 {
			t.Errorf("skipped = %d files = %d; want 1 and 0", result.Skipped, result.Files)
		}
	})

	t.Run("with override", func(t *testing.T) {
		userScope := t.TempDir()
		writeTestZip(t, filepath.Join(userScope, "bundle.zip"), entries)
		writeInScope(t, userScope, "notes.txt", "already here")

		perm := users.Permissions{Create: true, Modify: true}
		st := extractTestStorage(t, userScope, perm, nil)
		token := signToken(t, perm, extractKey())

		rec := extract(t, st, token, "/bundle.zip?action=extract&override=true")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d body = %q; want 200", rec.Code, rec.Body.String())
		}

		if got := readInScope(t, userScope, "notes.txt"); got != "from the archive" {
			t.Errorf("notes.txt = %q; want it replaced", got)
		}
	})

	t.Run("override needs the modify permission", func(t *testing.T) {
		userScope := t.TempDir()
		writeTestZip(t, filepath.Join(userScope, "bundle.zip"), entries)
		writeInScope(t, userScope, "notes.txt", "already here")

		perm := users.Permissions{Create: true}
		st := extractTestStorage(t, userScope, perm, nil)
		token := signToken(t, perm, extractKey())

		rec := extract(t, st, token, "/bundle.zip?action=extract&override=true")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d body = %q; want 403", rec.Code, rec.Body.String())
		}
		if got := readInScope(t, userScope, "notes.txt"); got != "already here" {
			t.Errorf("notes.txt = %q; want the existing file to be left alone", got)
		}
	})
}

func TestExtractRequiresTheCreatePermission(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "notes.txt", content: "VULNERABLE"},
	})

	perm := users.Permissions{Download: true, Modify: true}
	st := extractTestStorage(t, userScope, perm, nil)
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/bundle.zip?action=extract")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %q; want 403", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(userScope, "notes.txt")); err == nil {
		t.Error("VULNERABLE: extracted without the create permission")
	}
}

// Extracting into a directory the rules deny must be refused even though the
// archive's own path is allowed.
func TestExtractChecksTheDestinationAgainstTheRules(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "notes.txt", content: "VULNERABLE"},
	})

	perm := users.Permissions{Create: true, Modify: true}
	deny := rules.Rule{Path: "/out", Allow: false}
	st := extractTestStorage(t, userScope, perm, []rules.Rule{deny})
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/bundle.zip?action=extract&destination=/out")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %q; want 403", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(userScope, "out")); err == nil {
		t.Error("VULNERABLE: created a destination the rules deny")
	}
}

// Entries the rules deny are refused, but only up to the point the rule bites:
// the extraction is unwound, so nothing half-written survives either.
func TestExtractUnwindsTheNewFolderItCreated(t *testing.T) {
	userScope := t.TempDir()
	writeTestZip(t, filepath.Join(userScope, "bundle.zip"), []zipEntry{
		{name: "notes.txt", content: "fine"},
	})

	// A rule that matches nothing during the pre-checks but that the extraction
	// trips over: denying the file the archive would create.
	perm := users.Permissions{Create: true, Modify: true}
	deny := rules.Rule{Path: "/out/notes.txt", Allow: false}
	st := extractTestStorage(t, userScope, perm, []rules.Rule{deny})
	token := signToken(t, perm, extractKey())

	rec := extract(t, st, token, "/bundle.zip?action=extract&destination=/out")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %q; want 403", rec.Code, rec.Body.String())
	}

	if _, err := os.Stat(filepath.Join(userScope, "out")); err == nil {
		t.Error("the destination folder created by the failed extraction was left behind")
	}
}

func TestSafeEntryName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		in    string
		want  string
		valid bool
	}{
		{"plain", "notes.txt", "notes.txt", true},
		{"nested", "docs/readme.md", "docs/readme.md", true},
		{"trailing slash marks a directory", "docs/", "docs", true},
		{"backslash is a separator on windows", `a\b.txt`, "a/b.txt", true},

		{"empty", "", "", false},
		{"absolute", "/etc/passwd", "", false},
		{"parent", "../escape.txt", "", false},
		{"parent in the middle", "docs/../../escape.txt", "", false},
		{"bare parent", "..", "", false},
		{"current directory", "./notes.txt", "", false},
		{"empty component", "docs//notes.txt", "", false},
		{"nul byte", "notes.txt\x00.png", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := safeEntryName(tc.in)
			if ok != tc.valid {
				t.Fatalf("safeEntryName(%q) valid = %v; want %v", tc.in, ok, tc.valid)
			}
			if got != tc.want {
				t.Errorf("safeEntryName(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

package fbhttp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	gopath "path"
	"strings"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/mholt/archives"
)

const (
	// extractMaxEntries caps how many entries a single archive may contribute, so
	// a small archive that declares millions of members cannot turn one request
	// into millions of filesystem operations.
	extractMaxEntries = 50000

	// extractMaxBytes caps how much a single extraction may write. A zip bomb
	// is tiny on the wire and enormous once expanded, so the total is what
	// bounds the damage, not the size of the archive on disk.
	extractMaxBytes = 16 << 30 // 16 GiB
)

// extractResult reports what an extraction actually put on disk, so the client
// can tell the user where the files landed and what was deliberately left out.
type extractResult struct {
	Destination string `json:"destination"`
	Files       int    `json:"files"`
	Dirs        int    `json:"dirs"`
	// Entries counts every member the archive declared, including the ones that
	// were not written.
	Entries int `json:"entries"`
	// Bytes is how much was written, before any compression the archive applied.
	Bytes int64 `json:"bytes"`
	// Skipped counts entries whose target path already held a file, which is
	// left alone unless the caller explicitly asked to overwrite.
	Skipped int `json:"skipped"`
	// Links counts entries that carry no content to write: symbolic and hard
	// links, devices, sockets and fifos.
	Links int `json:"links"`
}

// extractArchive unpacks src into dst, or into the directory that already holds
// src when dst is empty. It reports what was written.
//
// Archive entry names are data chosen by whoever produced the archive, so they
// are treated as hostile: names that are absolute, traverse upward or contain a
// NUL are rejected rather than repaired, and every resolved path is checked
// against the user's rules before anything is created. A failure part-way
// through unwinds the paths this call brought into existence, so a rejected
// archive does not leave a half-unpacked tree behind.
func extractArchive(ctx context.Context, d *data, src, dst string, override bool, fileCache FileCache) (*extractResult, error) {
	info, err := d.user.Fs.Stat(src)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: %s is a directory", fberrors.ErrNotAnArchive, src)
	}

	root := dst
	if root == "" {
		root = gopath.Dir(src)
	}
	root = gopath.Clean("/" + root)

	// The archive itself is authorized by the request, but the directory it is
	// unpacked into is named either by the caller or derived here, and neither
	// of those has been through the rules.
	if !d.CheckRules(root) {
		return nil, fberrors.ErrPermissionDenied
	}

	fd, err := d.user.Fs.Open(src)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	// Identify peeks at the stream to recognize the format, so the reader it
	// hands back is the one to read from, not fd.
	format, reader, err := archives.Identify(ctx, gopath.Base(src), fd)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", fberrors.ErrNotAnArchive, err)
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return nil, fmt.Errorf("%w: %s is not extractable", fberrors.ErrNotAnArchive, format.Extension())
	}

	// created holds, in creation order, only the paths this call brought into
	// existence. Anything that was already there is absent from it, so unwinding
	// can never remove something the user had before the request.
	var created []string
	unwind := func() {
		for i := len(created) - 1; i >= 0; i-- {
			if rmErr := d.user.Fs.RemoveAll(created[i]); rmErr != nil {
				log.Printf("WARNING: failed to remove %s while unwinding the extraction of %s: %v", created[i], src, rmErr)
			}
		}
	}

	mkdir := func(target string) error {
		var missing []string
		for p := target; p != "/" && p != ""; p = gopath.Dir(p) {
			if _, statErr := d.user.Fs.Stat(p); statErr == nil {
				break
			}
			missing = append([]string{p}, missing...)
		}
		if len(missing) == 0 {
			return nil
		}
		if mkErr := d.user.Fs.MkdirAll(target, d.settings.DirMode); mkErr != nil {
			return mkErr
		}
		// Record the whole chain, not just the leaf: MkdirAll creates every
		// missing level, and only the leaf is the path that was asked for.
		created = append(created, missing...)
		return nil
	}

	if err := mkdir(root); err != nil {
		return nil, err
	}

	result := &extractResult{Destination: root}

	err = extractor.Extract(ctx, reader, func(_ context.Context, f archives.FileInfo) error {
		result.Entries++
		if result.Entries > extractMaxEntries {
			return fmt.Errorf("%w: more than %d entries", fberrors.ErrArchiveTooLarge, extractMaxEntries)
		}

		name, ok := safeEntryName(f.NameInArchive)
		if !ok {
			return fmt.Errorf("%w: %q", fberrors.ErrUnsafeArchiveEntry, f.NameInArchive)
		}
		target := gopath.Join(root, name)

		// A member can declare a name without any metadata behind it, and every
		// field below reads that metadata.
		if f.FileInfo == nil {
			return fmt.Errorf("%w: %q has no file metadata", fberrors.ErrUnsafeArchiveEntry, f.NameInArchive)
		}

		// The handler only authorized the archive, not what is inside it, so
		// every entry is authorized on its own before it is created.
		if !d.CheckRules(target) {
			return fberrors.ErrPermissionDenied
		}

		if f.IsDir() {
			if mkErr := mkdir(target); mkErr != nil {
				return mkErr
			}
			result.Dirs++
			return nil
		}

		if isArchiveLink(f) {
			// Links, devices, sockets and fifos have no content to write, and
			// recreating a link whose target the archive chose is how an
			// extraction turns into a write outside the destination.
			result.Links++
			return nil
		}

		if mkErr := mkdir(gopath.Dir(target)); mkErr != nil {
			return mkErr
		}

		existed := false
		if _, statErr := d.user.Fs.Stat(target); statErr == nil {
			existed = true
			if !override {
				// Writing over a file the user never offered up would destroy
				// data the request did not ask to touch, so the entry is left
				// alone and reported instead.
				result.Skipped++
				return nil
			}
			if err := invalidateThumbs(ctx, d, fileCache, target); err != nil {
				return err
			}
		}

		remaining := extractMaxBytes - result.Bytes
		if remaining <= 0 {
			return fmt.Errorf("%w: more than %d bytes", fberrors.ErrArchiveTooLarge, int64(extractMaxBytes))
		}

		entry, err := f.Open()
		if err != nil {
			return err
		}

		// One extra byte so a file that would cross the limit is detected as
		// such rather than silently truncated.
		written, err := writeFile(d.user.Fs, target, io.LimitReader(entry, remaining+1), d.settings.FileMode, d.settings.DirMode)
		_ = entry.Close()
		if err != nil {
			return err
		}

		if written.Size() > remaining {
			return fmt.Errorf("%w: more than %d bytes", fberrors.ErrArchiveTooLarge, int64(extractMaxBytes))
		}

		if !existed {
			created = append(created, target)
		}
		result.Bytes += written.Size()
		result.Files++
		return nil
	})
	if err != nil {
		unwind()
		return nil, err
	}

	return result, nil
}

// invalidateThumbs drops the cached previews of a file that is about to be
// replaced, so the listing does not keep showing the previous contents.
func invalidateThumbs(ctx context.Context, d *data, fileCache FileCache, target string) error {
	info, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       target,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: false,
		Checker:    d,
	})
	if err != nil {
		return err
	}

	return delThumbs(ctx, fileCache, info)
}

// isArchiveLink reports whether an archive entry is a link or some other
// special node rather than a plain file.
//
// The library flags both symbolic and hard links with LinkTarget, which is more
// dependable than the mode bits, and those are what matter here. Everything else
// that is not a directory is treated as a regular file: archive formats do not
// reliably record Unix mode bits, so a plain file stored by a Windows archiver
// can carry a mode with no file-type bits at all, and refusing to write it would
// silently drop ordinary content.
func isArchiveLink(f archives.FileInfo) bool {
	if f.LinkTarget != "" {
		return true
	}

	mode := f.Mode()
	return mode&os.ModeSymlink != 0 ||
		mode&os.ModeDevice != 0 ||
		mode&os.ModeCharDevice != 0 ||
		mode&os.ModeNamedPipe != 0 ||
		mode&os.ModeSocket != 0 ||
		mode&os.ModeIrregular != 0
}

// safeEntryName validates an archive entry name and returns it in the
// "/"-separated form used to build destination paths.
//
// An unusable name is rejected rather than repaired. Repairing it would write a
// path the archive never asked for, and that path could be a file the archive
// has no business touching. So absolute names, any "..", "." and empty
// components, and embedded NULs are all refused.
//
// "\" is folded to "/" because it is an ordinary filename character on POSIX but
// a separator on Windows, and the destination has to name the same file on both.
// Folding only ever deepens a path, so it cannot reach outside the destination.
func safeEntryName(name string) (string, bool) {
	if name == "" || strings.ContainsRune(name, 0) {
		return "", false
	}

	folded := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(folded, "/") {
		return "", false
	}

	// A trailing separator is how archive formats mark a directory entry. It
	// adds nothing once the name is joined onto the destination, so drop it
	// before checking the components.
	folded = strings.TrimRight(folded, "/")
	if folded == "" {
		return "", false
	}

	for _, part := range strings.Split(folded, "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}

	return folded, true
}

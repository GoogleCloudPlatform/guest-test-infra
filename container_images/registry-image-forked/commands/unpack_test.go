package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type tarEntry struct {
	name     string
	typeflag byte
	body     string
	linkname string
	mode     int64
}

func makeLayer(t *testing.T, entries []tarEntry) v1.Layer {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			if e.typeflag == tar.TypeDir {
				mode = 0755
			} else {
				mode = 0644
			}
		}
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Mode:     mode,
			Size:     int64(len(e.body)),
			Linkname: e.linkname,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q): %v", e.name, err)
		}
		if len(e.body) > 0 {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("Write(%q): %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Writer.Close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip.Writer.Close: %v", err)
	}
	raw := buf.Bytes()
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	})
	if err != nil {
		t.Fatalf("LayerFromOpener: %v", err)
	}
	return layer
}

func makeImage(t *testing.T, layers ...[]tarEntry) v1.Image {
	t.Helper()
	img := empty.Image
	v1Layers := make([]v1.Layer, 0, len(layers))
	for _, entries := range layers {
		v1Layers = append(v1Layers, makeLayer(t, entries))
	}
	img, err := mutate.AppendLayers(img, v1Layers...)
	if err != nil {
		t.Fatalf("AppendLayers: %v", err)
	}
	return img
}

func TestExtractLayer_RejectsLexicalPathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name: "regular file traversal",
			entries: []tarEntry{
				{name: "../../outside/victim", typeflag: tar.TypeReg, body: "overwritten"},
			},
		},
		{
			name: "leading slash traversal",
			entries: []tarEntry{
				{name: "/../../outside/victim", typeflag: tar.TypeReg, body: "overwritten"},
			},
		},
		{
			name: "nested dotdot traversal",
			entries: []tarEntry{
				{name: "sub/../../../outside/victim", typeflag: tar.TypeReg, body: "overwritten"},
			},
		},
		{
			name: "regular whiteout traversal",
			entries: []tarEntry{
				{name: "../../outside/.wh.victim", typeflag: tar.TypeReg},
			},
		},
		{
			name: "opaque whiteout traversal",
			entries: []tarEntry{
				{name: "../../outside/.wh..wh..opq", typeflag: tar.TypeReg},
			},
		},
		{
			name: "whiteout targeting parent directory",
			entries: []tarEntry{
				{name: ".wh...", typeflag: tar.TypeReg},
			},
		},
		{
			name: "whiteout targeting nested parent directory",
			entries: []tarEntry{
				{name: "sub/.wh...", typeflag: tar.TypeReg},
			},
		},
		{
			name: "empty whiteout target",
			entries: []tarEntry{
				{name: ".wh.", typeflag: tar.TypeReg},
			},
		},
		{
			name: "root overwrite with regular file",
			entries: []tarEntry{
				{name: ".", typeflag: tar.TypeReg, body: "nope"},
			},
		},
		{
			name: "dotdot back to root overwrite with regular file",
			entries: []tarEntry{
				{name: "sub/..", typeflag: tar.TypeReg, body: "nope"},
			},
		},
		{
			name: "slash-dot root overwrite with regular file",
			entries: []tarEntry{
				{name: "/.", typeflag: tar.TypeReg, body: "nope"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			outsideDir := filepath.Join(base, "outside")
			if err := os.MkdirAll(outsideDir, 0755); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(outsideDir, "victim")
			if err := os.WriteFile(victim, []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}

			dest := filepath.Join(base, "work", "rootfs")
			img := makeImage(t, tc.entries)
			if err := unpackImage(dest, img, true, io.Discard); err == nil {
				t.Fatalf("unpackImage() succeeded, want error")
			}

			got, err := os.ReadFile(victim)
			if err != nil {
				t.Fatalf("victim file missing or unreadable after rejected unpack: %v", err)
			}
			if string(got) != "original" {
				t.Fatalf("victim content = %q, want %q", string(got), "original")
			}
		})
	}
}

func TestExtractLayer_SymlinkWriteContainedInDest(t *testing.T) {
	tests := []struct {
		name     string
		relative bool
	}{
		{name: "absolute symlink target", relative: false},
		{name: "relative escaping symlink target", relative: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			outsideDir := filepath.Join(base, "outside")
			if err := os.MkdirAll(outsideDir, 0755); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(outsideDir, "victim")
			if err := os.WriteFile(victim, []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}

			linkTarget := outsideDir
			if tc.relative {
				linkTarget = "../../../../../../../../" + filepath.ToSlash(outsideDir)
			}

			dest := filepath.Join(base, "work", "rootfs")
			img := makeImage(t, []tarEntry{
				{name: "escape", typeflag: tar.TypeSymlink, linkname: linkTarget},
				{name: "escape/victim", typeflag: tar.TypeReg, body: "pwned"},
				{name: "escape/created", typeflag: tar.TypeReg, body: "evil"},
			})

			if err := unpackImage(dest, img, true, io.Discard); err != nil {
				t.Fatalf("unpackImage() unexpected error: %v", err)
			}

			got, err := os.ReadFile(victim)
			if err != nil {
				t.Fatalf("outside victim unreadable: %v", err)
			}
			if string(got) != "original" {
				t.Fatalf("outside victim modified: got %q, want %q", string(got), "original")
			}
			if _, err := os.Lstat(filepath.Join(outsideDir, "created")); !os.IsNotExist(err) {
				t.Fatalf("file was created outside dest: err=%v", err)
			}
		})
	}
}

func TestExtractLayer_SymlinkWhiteoutContainedInDest(t *testing.T) {
	tests := []struct {
		name         string
		whiteoutName string
	}{
		{name: "regular whiteout through symlink", whiteoutName: "escape/.wh.victim"},
		{name: "opaque whiteout through symlink", whiteoutName: "escape/.wh..wh..opq"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			outsideDir := filepath.Join(base, "outside")
			if err := os.MkdirAll(outsideDir, 0755); err != nil {
				t.Fatal(err)
			}
			victim := filepath.Join(outsideDir, "victim")
			if err := os.WriteFile(victim, []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}

			dest := filepath.Join(base, "work", "rootfs")
			img := makeImage(t, []tarEntry{
				{name: "escape", typeflag: tar.TypeSymlink, linkname: outsideDir},
				{name: tc.whiteoutName, typeflag: tar.TypeReg},
			})

			if err := unpackImage(dest, img, true, io.Discard); err != nil {
				t.Fatalf("unpackImage() unexpected error: %v", err)
			}

			got, err := os.ReadFile(victim)
			if err != nil {
				t.Fatalf("outside victim was deleted: %v", err)
			}
			if string(got) != "original" {
				t.Fatalf("outside victim content = %q, want %q", string(got), "original")
			}
		})
	}
}

func TestExtractLayer_ReplaceSymlinkDoesNotTouchTarget(t *testing.T) {
	base := t.TempDir()
	outsideFile := filepath.Join(base, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(base, "rootfs")
	img := makeImage(t,
		[]tarEntry{
			{name: "link", typeflag: tar.TypeSymlink, linkname: outsideFile},
		},
		[]tarEntry{
			{name: "link", typeflag: tar.TypeReg, body: "replacement"},
		},
	)

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	gotOutside, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("outside file missing: %v", err)
	}
	if string(gotOutside) != "original" {
		t.Fatalf("outside file content = %q, want %q", string(gotOutside), "original")
	}

	gotInside, err := os.ReadFile(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatalf("dest link file unreadable: %v", err)
	}
	if string(gotInside) != "replacement" {
		t.Fatalf("dest link content = %q, want %q", string(gotInside), "replacement")
	}
}

func TestExtractLayer_HardlinkContainment(t *testing.T) {
	base := t.TempDir()
	siblingDir := filepath.Join(base, "rootfs2")
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatal(err)
	}
	siblingVictim := filepath.Join(siblingDir, "secret")
	if err := os.WriteFile(siblingVictim, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name: "hardlink traversal with dotdot",
			entries: []tarEntry{
				{name: "stolen", typeflag: tar.TypeLink, linkname: "../rootfs2/secret"},
			},
		},
		{
			name: "hardlink targeting root",
			entries: []tarEntry{
				{name: "rootlink", typeflag: tar.TypeLink, linkname: "."},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dest := filepath.Join(base, "rootfs")
			os.RemoveAll(dest)
			img := makeImage(t, tc.entries)
			if err := unpackImage(dest, img, true, io.Discard); err == nil {
				t.Fatalf("unpackImage() succeeded, want error")
			}
			if _, err := os.Lstat(filepath.Join(dest, "stolen")); !os.IsNotExist(err) {
				t.Fatalf("unexpected hardlink created at dest/stolen: err=%v", err)
			}
		})
	}
}

func TestExtractLayer_HardlinkThroughEscapingSymlinkScopedToDest(t *testing.T) {
	base := t.TempDir()
	siblingDir := filepath.Join(base, "rootfs2")
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatal(err)
	}
	siblingVictim := filepath.Join(siblingDir, "secret")
	if err := os.WriteFile(siblingVictim, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(base, "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "rootfs2/secret", typeflag: tar.TypeReg, body: "inside"},
		{name: "escape", typeflag: tar.TypeSymlink, linkname: "../rootfs2"},
		{name: "stolen", typeflag: tar.TypeLink, linkname: "escape/secret"},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "stolen"))
	if err != nil {
		t.Fatalf("ReadFile(dest/stolen): %v", err)
	}
	if string(got) != "inside" {
		t.Fatalf("dest/stolen content = %q, want %q", string(got), "inside")
	}

	gotSibling, err := os.ReadFile(siblingVictim)
	if err != nil || string(gotSibling) != "secret" {
		t.Fatalf("sibling victim = (%q, %v), want (%q, nil)", string(gotSibling), err, "secret")
	}
	fi, err := os.Lstat(siblingVictim)
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatalf("sibling victim mode = (%o, %v), want (0600, nil)", fi.Mode().Perm(), err)
	}
}

func TestExtractLayer_AllowsValidOCISymlinksAndUsrmerge(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rootfs")
	img := makeImage(t,
		[]tarEntry{
			{name: "usr/lib", typeflag: tar.TypeDir},
			{name: "lib", typeflag: tar.TypeSymlink, linkname: "/usr/lib"},
			{name: "bin", typeflag: tar.TypeDir},
			{name: "bin/sh", typeflag: tar.TypeSymlink, linkname: "/bin/busybox"},
		},
		[]tarEntry{
			{name: "lib/libfoo.so", typeflag: tar.TypeReg, body: "elf"},
		},
	)

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	target, err := os.Readlink(filepath.Join(dest, "bin", "sh"))
	if err != nil {
		t.Fatalf("Readlink(bin/sh): %v", err)
	}
	if target != "/bin/busybox" {
		t.Fatalf("bin/sh target = %q, want %q", target, "/bin/busybox")
	}

	got, err := os.ReadFile(filepath.Join(dest, "usr", "lib", "libfoo.so"))
	if err != nil {
		t.Fatalf("ReadFile(usr/lib/libfoo.so): %v", err)
	}
	if string(got) != "elf" {
		t.Fatalf("usr/lib/libfoo.so = %q, want %q", string(got), "elf")
	}
}

func TestExtractLayer_WhiteoutsWithinDest(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rootfs")
	img := makeImage(t,
		[]tarEntry{
			{name: "dir", typeflag: tar.TypeDir},
			{name: "dir/keep", typeflag: tar.TypeReg, body: "keep"},
			{name: "dir/remove", typeflag: tar.TypeReg, body: "gone"},
			{name: "opq", typeflag: tar.TypeDir},
			{name: "opq/old", typeflag: tar.TypeReg, body: "old"},
			{name: "opq/sub", typeflag: tar.TypeDir},
			{name: "opq/sub/nested", typeflag: tar.TypeReg, body: "nested"},
		},
		[]tarEntry{
			{name: "dir/.wh.remove", typeflag: tar.TypeReg},
			{name: "opq/.wh..wh..opq", typeflag: tar.TypeReg},
			{name: "opq/new", typeflag: tar.TypeReg, body: "new"},
			{name: "nonexistent/.wh..wh..opq", typeflag: tar.TypeReg},
		},
	)

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "dir", "keep")); err != nil {
		t.Fatalf("dir/keep missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "dir", "remove")); !os.IsNotExist(err) {
		t.Fatalf("dir/remove should be deleted, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "opq", "old")); !os.IsNotExist(err) {
		t.Fatalf("opq/old should be deleted, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "opq", "sub")); !os.IsNotExist(err) {
		t.Fatalf("opq/sub should be deleted, got err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "opq", "new"))
	if err != nil || string(got) != "new" {
		t.Fatalf("opq/new = (%q, %v), want (%q, nil)", string(got), err, "new")
	}
}

func TestExtractLayer_RootOpaqueWhiteoutPreservesDest(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "rootfs")
	img := makeImage(t,
		[]tarEntry{
			{name: "oldfile", typeflag: tar.TypeReg, body: "old"},
			{name: "olddir", typeflag: tar.TypeDir},
			{name: "olddir/file", typeflag: tar.TypeReg, body: "nested"},
		},
		[]tarEntry{
			{name: ".wh..wh..opq", typeflag: tar.TypeReg},
			{name: "newfile", typeflag: tar.TypeReg, body: "new"},
		},
	)

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "oldfile")); !os.IsNotExist(err) {
		t.Fatalf("oldfile should be deleted, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "olddir")); !os.IsNotExist(err) {
		t.Fatalf("olddir should be deleted, got err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "newfile"))
	if err != nil || string(got) != "new" {
		t.Fatalf("newfile = (%q, %v), want (%q, nil)", string(got), err, "new")
	}
}

func TestCleanTarPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: "."},
		{name: "dot", input: ".", want: "."},
		{name: "slash", input: "/", want: "."},
		{name: "relative file", input: "./etc/hosts", want: filepath.Join("etc", "hosts")},
		{name: "leading slash file", input: "/etc/hosts", want: filepath.Join("etc", "hosts")},
		{name: "leading dotdot in filename", input: "..hidden", want: "..hidden"},
		{name: "dotdot escape", input: "../etc/shadow", wantErr: true},
		{name: "leading slash dotdot escape", input: "/../../etc/shadow", wantErr: true},
		{name: "nested dotdot escape", input: "a/../../etc/shadow", wantErr: true},
		{name: "bare dotdot", input: "..", wantErr: true},
		{name: "NUL byte", input: "bad\x00name", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cleanTarPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("cleanTarPath(%q) = %q, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("cleanTarPath(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("cleanTarPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestExtractLayer_ValidHardlinkWithinDest(t *testing.T) {
	base := t.TempDir()
	outsideVictim := filepath.Join(base, "victim")
	if err := os.WriteFile(outsideVictim, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(base, "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "orig", typeflag: tar.TypeReg, body: "hello"},
		{name: "hard", typeflag: tar.TypeLink, linkname: "orig"},
		{name: "sym", typeflag: tar.TypeSymlink, linkname: outsideVictim},
		{name: "hardsym", typeflag: tar.TypeLink, linkname: "sym", mode: 0777},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hard"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("hard = (%q, %v), want (%q, nil)", string(got), err, "hello")
	}
	target, err := os.Readlink(filepath.Join(dest, "hardsym"))
	if err != nil || target != outsideVictim {
		t.Fatalf("Readlink(hardsym) = (%q, %v), want (%q, nil)", target, err, outsideVictim)
	}

	gotOutside, err := os.ReadFile(outsideVictim)
	if err != nil || string(gotOutside) != "outside" {
		t.Fatalf("outside victim content = (%q, %v), want (%q, nil)", string(gotOutside), err, "outside")
	}
	fi, err := os.Lstat(outsideVictim)
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatalf("outside victim mode = (%o, %v), want (0600, nil)", fi.Mode().Perm(), err)
	}
}

func TestExtractLayer_DirOverEscapingSymlink(t *testing.T) {
	base := t.TempDir()
	outsideDir := filepath.Join(base, "outside")
	if err := os.MkdirAll(outsideDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outsideDir, 0700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outsideDir, "victim")
	if err := os.WriteFile(victim, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(base, "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "link", typeflag: tar.TypeSymlink, linkname: outsideDir},
		{name: "link/", typeflag: tar.TypeDir, mode: 0777},
		{name: "link/victim", typeflag: tar.TypeReg, body: "inside"},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	linkInfo, err := os.Lstat(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatalf("Lstat(dest/link): %v", err)
	}
	if !linkInfo.IsDir() || linkInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("dest/link mode = %v, want real directory", linkInfo.Mode())
	}

	gotInside, err := os.ReadFile(filepath.Join(dest, "link", "victim"))
	if err != nil || string(gotInside) != "inside" {
		t.Fatalf("dest/link/victim = (%q, %v), want (%q, nil)", string(gotInside), err, "inside")
	}

	outsideDirInfo, err := os.Lstat(outsideDir)
	if err != nil || outsideDirInfo.Mode().Perm() != 0700 {
		t.Fatalf("outsideDir mode = (%o, %v), want (0700, nil)", outsideDirInfo.Mode().Perm(), err)
	}
	gotOutside, err := os.ReadFile(victim)
	if err != nil || string(gotOutside) != "original" {
		t.Fatalf("outside victim content = (%q, %v), want (%q, nil)", string(gotOutside), err, "original")
	}
	victimInfo, err := os.Lstat(victim)
	if err != nil || victimInfo.Mode().Perm() != 0600 {
		t.Fatalf("outside victim mode = (%o, %v), want (0600, nil)", victimInfo.Mode().Perm(), err)
	}
}

func TestExtractLayer_DirReplacedBySymlinkMidLayer(t *testing.T) {
	base := t.TempDir()
	outsideDir := filepath.Join(base, "outside")
	outsideSub := filepath.Join(outsideDir, "sub")
	if err := os.MkdirAll(outsideSub, 0755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outsideDir, "victim")
	if err := os.WriteFile(victim, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(outsideSub, "nested")
	if err := os.WriteFile(nested, []byte("nested"), 0600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(base, "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "d/", typeflag: tar.TypeDir},
		{name: "d/x", typeflag: tar.TypeReg, body: "x"},
		{name: "d", typeflag: tar.TypeSymlink, linkname: outsideDir},
		{name: "d/victim", typeflag: tar.TypeReg, body: "pwned"},
		{name: "d/sub/.wh.nested", typeflag: tar.TypeReg},
		{name: "d/sub/.wh..wh..opq", typeflag: tar.TypeReg},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	gotVictim, err := os.ReadFile(victim)
	if err != nil || string(gotVictim) != "original" {
		t.Fatalf("outside victim = (%q, %v), want (%q, nil)", string(gotVictim), err, "original")
	}
	victimInfo, err := os.Lstat(victim)
	if err != nil || victimInfo.Mode().Perm() != 0600 {
		t.Fatalf("outside victim mode = (%o, %v), want (0600, nil)", victimInfo.Mode().Perm(), err)
	}

	gotNested, err := os.ReadFile(nested)
	if err != nil || string(gotNested) != "nested" {
		t.Fatalf("outside nested = (%q, %v), want (%q, nil)", string(gotNested), err, "nested")
	}
	nestedInfo, err := os.Lstat(nested)
	if err != nil || nestedInfo.Mode().Perm() != 0600 {
		t.Fatalf("outside nested mode = (%o, %v), want (0600, nil)", nestedInfo.Mode().Perm(), err)
	}
}

func TestExtractLayer_Fifo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fifos not supported on windows")
	}
	dest := filepath.Join(t.TempDir(), "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "fifo", typeflag: tar.TypeFifo, mode: 0640},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	fi, err := os.Lstat(filepath.Join(dest, "fifo"))
	if err != nil {
		t.Fatalf("Lstat(fifo): %v", err)
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("fifo mode = %v, want named pipe", fi.Mode())
	}
	if fi.Mode().Perm() != 0640 {
		t.Fatalf("fifo perm = %o, want 0640", fi.Mode().Perm())
	}
}

func TestExtractLayer_Setuid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("setuid not supported on windows")
	}
	dest := filepath.Join(t.TempDir(), "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "bin/su", typeflag: tar.TypeReg, body: "su", mode: 04755},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	fi, err := os.Lstat(filepath.Join(dest, "bin", "su"))
	if err != nil {
		t.Fatalf("Lstat(bin/su): %v", err)
	}
	if fi.Mode()&os.ModeSetuid == 0 {
		t.Fatalf("bin/su mode = %v, want setuid bit set", fi.Mode())
	}
	if fi.Mode().Perm() != 0755 {
		t.Fatalf("bin/su perm = %o, want 0755", fi.Mode().Perm())
	}
}

func TestExtractLayer_OpaqueWhiteoutReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX read-only directory semantics differ on windows")
	}
	dest := filepath.Join(t.TempDir(), "rootfs")
	roDir := filepath.Join(dest, "ro")
	t.Cleanup(func() {
		os.Chmod(roDir, 0755)
	})

	img := makeImage(t,
		[]tarEntry{
			{name: "ro/old", typeflag: tar.TypeReg, body: "old"},
			{name: "ro/", typeflag: tar.TypeDir, mode: 0555},
		},
		[]tarEntry{
			{name: "ro/.wh..wh..opq", typeflag: tar.TypeReg},
		},
	)

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(roDir, "old")); !os.IsNotExist(err) {
		t.Fatalf("ro/old should be deleted, got err=%v", err)
	}
	fi, err := os.Lstat(roDir)
	if err != nil {
		t.Fatalf("Lstat(ro): %v", err)
	}
	if fi.Mode().Perm() != 0555 {
		t.Fatalf("ro perm = %o, want 0555", fi.Mode().Perm())
	}
}

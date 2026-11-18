package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
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
		{
			name: "hardlink through escaping symlink",
			entries: []tarEntry{
				{name: "escape", typeflag: tar.TypeSymlink, linkname: siblingDir},
				{name: "stolen", typeflag: tar.TypeLink, linkname: "escape/secret"},
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
	dest := filepath.Join(t.TempDir(), "rootfs")
	img := makeImage(t, []tarEntry{
		{name: "orig", typeflag: tar.TypeReg, body: "hello"},
		{name: "hard", typeflag: tar.TypeLink, linkname: "orig"},
		{name: "sym", typeflag: tar.TypeSymlink, linkname: "/etc/hosts"},
		{name: "hardsym", typeflag: tar.TypeLink, linkname: "sym"},
	})

	if err := unpackImage(dest, img, true, io.Discard); err != nil {
		t.Fatalf("unpackImage() unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hard"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("hard = (%q, %v), want (%q, nil)", string(got), err, "hello")
	}
	target, err := os.Readlink(filepath.Join(dest, "hardsym"))
	if err != nil || target != "/etc/hosts" {
		t.Fatalf("Readlink(hardsym) = (%q, %v), want (%q, nil)", target, err, "/etc/hosts")
	}
}

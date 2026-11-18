package commands

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/fatih/color"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

const whiteoutPrefix = ".wh."
const whiteoutOpaqueDir = whiteoutPrefix + whiteoutPrefix + ".opq"

func unpackImage(dest string, img v1.Image, debug bool, out io.Writer) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}

	chown := os.Getuid() == 0

	if debug {
		out = ioutil.Discard
	}

	progress := mpb.New(mpb.WithOutput(out))

	bars := make([]*mpb.Bar, len(layers))

	for i, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			return err
		}

		digest, err := layer.Digest()
		if err != nil {
			return err
		}

		bars[i] = progress.AddBar(
			size,
			mpb.PrependDecorators(decor.Name(color.HiBlackString(digest.Hex[0:12]))),
			mpb.AppendDecorators(decor.CountersKibiByte("%.1f/%.1f")),
		)
	}

	// iterate over layers in reverse order; no need to write things files that
	// are modified by later layers anyway
	for i, layer := range layers {
		logrus.Debugf("extracting layer %d of %d", i+1, len(layers))

		err := extractLayer(dest, layer, bars[i], chown)
		if err != nil {
			return err
		}
	}

	progress.Wait()

	return nil
}

func extractLayer(dest string, layer v1.Layer, bar *mpb.Bar, chown bool) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	dest, err = filepath.EvalSymlinks(absDest)
	if err != nil {
		return err
	}

	r, err := layer.Compressed()
	if err != nil {
		return err
	}
	defer r.Close()

	gr, err := gzip.NewReader(bar.ProxyReader(r))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}

		dir, path, base, isRoot, err := resolvePathInDest(dest, hdr.Name)
		if err != nil {
			return err
		}

		log := logrus.WithFields(logrus.Fields{
			"Name": hdr.Name,
		})

		log.Debug("unpacking")

		if base == whiteoutOpaqueDir {
			fi, err := os.Lstat(dir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			if !fi.IsDir() {
				continue
			}

			log.Debugf("removing contents of %s", dir)

			if err := removeDirContents(dir, fi.Mode()); err != nil {
				return err
			}

			continue
		} else if strings.HasPrefix(base, whiteoutPrefix) {
			// layer has marked a file as deleted
			name := strings.TrimPrefix(base, whiteoutPrefix)
			if name == "" || name == "." || name == ".." || strings.ContainsRune(name, os.PathSeparator) || strings.ContainsRune(name, '/') {
				return fmt.Errorf("invalid whiteout entry %q", hdr.Name)
			}
			removedPath := filepath.Join(dir, name)
			if !isWithinDest(dest, removedPath) || removedPath == dest {
				return fmt.Errorf("whiteout entry %q escapes destination directory", hdr.Name)
			}

			log.Debugf("removing %s", removedPath)

			if err := os.RemoveAll(removedPath); err != nil {
				return err
			}

			continue
		}

		if isRoot {
			if hdr.Typeflag == tar.TypeDir {
				continue
			}
			return fmt.Errorf("tar entry %q cannot overwrite destination root", hdr.Name)
		}

		if hdr.Typeflag == tar.TypeBlock || hdr.Typeflag == tar.TypeChar {
			// devices can't be created in a user namespace
			log.Debugf("skipping device %s", hdr.Name)
			continue
		}

		if hdr.Typeflag == tar.TypeSymlink {
			log.Debugf("symlinking to %s", hdr.Linkname)
		}

		if hdr.Typeflag == tar.TypeLink {
			log.Debugf("hardlinking to %s", hdr.Linkname)
		}

		if fi, err := os.Lstat(path); err == nil {
			if !(fi.IsDir() && hdr.Typeflag == tar.TypeDir) {
				log.Debugf("removing existing path")
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}

		if err := extractEntry(hdr, dest, dir, path, tr, chown); err != nil {
			log.Debugf("extracting")
			return err
		}
	}

	err = gr.Close()
	if err != nil {
		return err
	}

	err = r.Close()
	if err != nil {
		return err
	}

	bar.SetTotal(bar.Current(), true)

	return nil
}

func cleanTarPath(name string) (string, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", fmt.Errorf("tar entry %q contains NUL byte", name)
	}
	trimmed := strings.TrimLeft(name, "/")
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" {
		return "", fmt.Errorf("tar entry %q escapes destination directory", name)
	}
	return clean, nil
}

func isWithinDest(dest, target string) bool {
	rel, err := filepath.Rel(dest, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func resolvePathInDest(dest, tarName string) (dir, path, base string, isRoot bool, err error) {
	clean, err := cleanTarPath(tarName)
	if err != nil {
		return "", "", "", false, err
	}
	if clean == "." {
		return dest, dest, ".", true, nil
	}

	relDir := filepath.Dir(clean)
	base = filepath.Base(clean)

	dir, err = securejoin.SecureJoin(dest, relDir)
	if err != nil {
		return "", "", "", false, fmt.Errorf("resolve parent directory for %q: %w", tarName, err)
	}
	if !isWithinDest(dest, dir) {
		return "", "", "", false, fmt.Errorf("tar entry %q parent escapes destination directory", tarName)
	}

	path = filepath.Join(dir, base)
	if !isWithinDest(dest, path) || path == dest {
		return "", "", "", false, fmt.Errorf("tar entry %q escapes destination directory", tarName)
	}
	return dir, path, base, false, nil
}

func removeDirContents(dir string, mode os.FileMode) error {
	if mode&0200 == 0 {
		if err := os.Chmod(dir, mode|0200); err != nil {
			return err
		}
		defer os.Chmod(dir, mode)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(hdr *tar.Header, dest, dir, path string, input io.Reader, chown bool) error {
	fileMode := hdr.FileInfo().Mode()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	switch hdr.Typeflag {
	case tar.TypeLink:
		_, targetPath, _, targetIsRoot, err := resolvePathInDest(dest, hdr.Linkname)
		if err != nil {
			return fmt.Errorf("resolve hardlink target for %q: %w", hdr.Name, err)
		}
		if targetIsRoot {
			return fmt.Errorf("hardlink %q cannot target destination root", hdr.Name)
		}
		if err := os.Link(targetPath, path); err != nil {
			return err
		}

	case tar.TypeSymlink:
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeDir:
		if err := os.MkdirAll(path, fileMode); err != nil {
			return err
		}

	case tar.TypeReg, tar.TypeRegA:
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode.Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(file, input); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}

	case tar.TypeFifo:
		if err := mkfifoEntry(hdr, path); err != nil {
			return err
		}

	default:
		return fmt.Errorf("%s: unsupported entry type (%c)", hdr.Name, hdr.Typeflag)
	}

	if runtime.GOOS != "windows" && chown {
		if err := os.Lchown(path, hdr.Uid, hdr.Gid); err != nil {
			return err
		}
	}

	if err := lchmod(hdr, path, fileMode); err != nil {
		return err
	}

	return lchtimes(hdr, path)
}

func lchmod(hdr *tar.Header, path string, fileMode os.FileMode) error {
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink == 0 {
			return os.Chmod(path, fileMode)
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		return os.Chmod(path, fileMode)
	}
	return nil
}

func lchtimes(hdr *tar.Header, path string) error {
	aTime := hdr.AccessTime
	mTime := hdr.ModTime
	if aTime.Before(mTime) {
		aTime = mTime
	}

	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink == 0 {
			return os.Chtimes(path, aTime, mTime)
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		return os.Chtimes(path, aTime, mTime)
	}
	return nil
}

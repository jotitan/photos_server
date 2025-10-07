//go:build ignore
// +build ignore

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var dryRunFlag bool
	flag.BoolVar(&dryRunFlag, "dry-run", false, "Log actions without performing them")
	flag.BoolVar(&dryRunFlag, "n", false, "Alias for -dry-run")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <root-directory>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	rootDir := flag.Arg(0)
	if err := processRootDirectory(rootDir, dryRunFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func processRootDirectory(rootDir string, dryRun bool) error {
	rootInfo, err := os.Stat(rootDir)
	if err != nil {
		return fmt.Errorf("stat root: %w", err)
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("root is not a directory: %s", rootDir)
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return fmt.Errorf("read root dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdirPath := filepath.Join(rootDir, entry.Name())
		if err := processSubdirectory(subdirPath, dryRun); err != nil {
			return fmt.Errorf("process subdirectory %s: %w", subdirPath, err)
		}
	}

	return nil
}

func processSubdirectory(subdirPath string, dryRun bool) error {
	// 1) Move contents of hd/ up one level into subdir
	hdPath := filepath.Join(subdirPath, "hd")
	if info, err := os.Stat(hdPath); err == nil && info.IsDir() {
		if err := moveContentsUpOneLevel(hdPath, subdirPath, dryRun); err != nil {
			return fmt.Errorf("move hd contents for %s: %w", subdirPath, err)
		}
		// Remove now-empty hd directory
		if dryRun {
			fmt.Printf("Would remove directory: %s\n", hdPath)
		} else {
			_ = os.Remove(hdPath)
		}
	}

	// 2) Delete sd/ and ld/
	for _, name := range []string{"sd", "ld"} {
		p := filepath.Join(subdirPath, name)
		if _, err := os.Stat(p); err == nil {
			if dryRun {
				fmt.Printf("Would remove directory: %s\n", p)
			} else {
				if err := os.RemoveAll(p); err != nil {
					return fmt.Errorf("remove %s: %w", p, err)
				}
			}
		}
	}

	// 3) Remove any Thumbs.db files within this subdir tree
	if err := removeThumbsDB(subdirPath, dryRun); err != nil {
		return fmt.Errorf("remove Thumbs.db in %s: %w", subdirPath, err)
	}

	return nil
}

func moveContentsUpOneLevel(fromDir, toDir string, dryRun bool) error {
	entries, err := os.ReadDir(fromDir)
	if err != nil {
		return fmt.Errorf("read fromDir: %w", err)
	}
	for _, entry := range entries {
		src := filepath.Join(fromDir, entry.Name())
		// If this is a Thumbs.db, delete it instead of moving
		if strings.EqualFold(entry.Name(), "Thumbs.db") {
			if dryRun {
				fmt.Printf("Would remove file: %s\n", src)
			} else {
				if remErr := os.Remove(src); remErr != nil && !errors.Is(remErr, os.ErrNotExist) {
					return fmt.Errorf("remove %s: %w", src, remErr)
				}
			}
			continue
		}
		dst := filepath.Join(toDir, entry.Name())
		resolvedDst, err := resolveConflictPath(dst)
		if err != nil {
			return err
		}
		if dryRun {
			fmt.Printf("Would move %s -> %s\n", src, resolvedDst)
		} else {
			if err := movePath(src, resolvedDst); err != nil {
				return fmt.Errorf("move %s -> %s: %w", src, resolvedDst, err)
			}
		}
	}
	return nil
}

func resolveConflictPath(dst string) (string, error) {
	if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
		return dst, nil
	}
	base := filepath.Base(dst)
	dir := filepath.Dir(dst)
	name := base
	ext := ""
	if dot := strings.LastIndex(base, "."); dot > 0 {
		name = base[:dot]
		ext = base[dot:]
	}
	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not resolve conflict for %s", dst)
}

func movePath(src, dst string) error {
	// Try simple rename first
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDeviceLinkError(err) {
		// If it's not a cross-device error, try to handle with copy as fallback anyway
		// but keep original error context if copy also fails
		if copyErr := copyPath(src, dst); copyErr != nil {
			return fmt.Errorf("rename failed (%v) and copy failed (%v)", err, copyErr)
		}
		// remove source only if copy succeeded
		return os.RemoveAll(src)
	}

	// Cross-device: copy then remove
	if err := copyPath(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func isCrossDeviceLinkError(err error) bool {
	if err == nil {
		return false
	}
	// Portable detection via substring match
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cross-device link") || strings.Contains(msg, "invalid cross-device link")
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst, info)
	}
	return copyFile(src, dst, info)
}

func copyFile(src, dst string, info os.FileInfo) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	dstF, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstF.Close()

	if _, err := io.Copy(dstF, srcF); err != nil {
		return err
	}
	return nil
}

func copyDir(src, dst string, info os.FileInfo) error {
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		childSrc := filepath.Join(src, e.Name())
		childDst := filepath.Join(dst, e.Name())
		if err := copyPath(childSrc, childDst); err != nil {
			return err
		}
	}
	return nil
}

func removeThumbsDB(root string, dryRun bool) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "Thumbs.db") {
			if dryRun {
				fmt.Printf("Would remove file: %s\n", path)
			} else {
				if remErr := os.Remove(path); remErr != nil && !errors.Is(remErr, os.ErrNotExist) {
					return fmt.Errorf("remove %s: %w", path, remErr)
				}
			}
		}
		return nil
	})
}

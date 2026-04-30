package updater

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

// swapBinary replaces the on-disk launcher executable with the new file.
// On Windows the running .exe cannot be overwritten, so we rename it
// out of the way first and let the OS clean it up on next boot.
func swapBinary(currentPath, newPath string) error {
	if runtime.GOOS == "windows" {
		old := currentPath + ".old"
		_ = os.Remove(old) // best-effort
		if err := os.Rename(currentPath, old); err != nil {
			return fmt.Errorf("rename current: %w", err)
		}
		if err := copyFile(newPath, currentPath); err != nil {
			// Try to restore the original on failure.
			_ = os.Rename(old, currentPath)
			return fmt.Errorf("copy new binary: %w", err)
		}
		return nil
	}
	// On macOS/Linux a regular rename works because the kernel keeps the
	// running process's file open via the inode.
	if err := copyFile(newPath, currentPath); err != nil {
		return err
	}
	return os.Chmod(currentPath, 0o755)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		// Some filesystems (Windows on FUSE) fail rename across files.
		os.Remove(tmp)
		return errors.New("could not rename new binary into place")
	}
	return nil
}

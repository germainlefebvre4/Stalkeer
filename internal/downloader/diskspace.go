package downloader

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// DiskSpace represents available disk space information
type DiskSpace struct {
	Available uint64  // Available bytes for unprivileged users
	Free      uint64  // Free bytes on filesystem
	Total     uint64  // Total bytes on filesystem
	UsedPct   float64 // Percentage of space used
}

// GetDiskSpace returns disk space information for the given path
func GetDiskSpace(path string) (*DiskSpace, error) {
	// Ensure path exists or use parent directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find existing directory in path
	checkPath := absPath
	for {
		if _, err := os.Stat(checkPath); err == nil {
			break
		}
		parent := filepath.Dir(checkPath)
		if parent == checkPath {
			// Reached root
			return nil, fmt.Errorf("no existing directory found in path")
		}
		checkPath = parent
	}

	var stat unix.Statfs_t
	if err := unix.Statfs(checkPath, &stat); err != nil {
		return nil, fmt.Errorf("failed to get filesystem stats: %w", err)
	}

	// Calculate space in bytes
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	usedPct := float64(used) / float64(total) * 100

	return &DiskSpace{
		Available: available,
		Free:      free,
		Total:     total,
		UsedPct:   usedPct,
	}, nil
}

// HasEnoughSpace checks if there's enough available disk space for the given size
func HasEnoughSpace(path string, requiredBytes uint64) (bool, *DiskSpace, error) {
	space, err := GetDiskSpace(path)
	if err != nil {
		return false, nil, err
	}

	return space.Available >= requiredBytes, space, nil
}

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CheckDiskSpaceBeforeDownload validates there's enough space before starting a download
func CheckDiskSpaceBeforeDownload(destPath string, estimatedSize uint64, minFreeSpaceBytes uint64) error {
	space, err := GetDiskSpace(destPath)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	// Add buffer (10% or minimum free space requirement)
	requiredSpace := estimatedSize
	if minFreeSpaceBytes > 0 && space.Available < minFreeSpaceBytes+requiredSpace {
		return fmt.Errorf(
			"insufficient disk space: available=%s, required=%s (download) + %s (min free) = %s",
			FormatBytes(space.Available),
			FormatBytes(estimatedSize),
			FormatBytes(minFreeSpaceBytes),
			FormatBytes(estimatedSize+minFreeSpaceBytes),
		)
	}

	if space.Available < requiredSpace {
		return fmt.Errorf(
			"insufficient disk space: available=%s, required=%s",
			FormatBytes(space.Available),
			FormatBytes(requiredSpace),
		)
	}

	return nil
}

package downloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDiskSpace(t *testing.T) {
	// Use current directory for testing
	wd, err := os.Getwd()
	require.NoError(t, err)

	space, err := GetDiskSpace(wd)
	require.NoError(t, err)
	assert.NotNil(t, space)

	// Basic sanity checks
	assert.Greater(t, space.Total, uint64(0))
	assert.Greater(t, space.Available, uint64(0))
	assert.LessOrEqual(t, space.Available, space.Total)
	assert.GreaterOrEqual(t, space.UsedPct, 0.0)
	assert.LessOrEqual(t, space.UsedPct, 100.0)
}

func TestGetDiskSpace_NonExistentPath(t *testing.T) {
	// Test with non-existent path - should check parent directory
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "does", "not", "exist", "yet")

	space, err := GetDiskSpace(nonExistent)
	require.NoError(t, err)
	assert.NotNil(t, space)
	assert.Greater(t, space.Available, uint64(0))
}

func TestGetDiskSpace_TempDir(t *testing.T) {
	tempDir := t.TempDir()

	space, err := GetDiskSpace(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, space)
	assert.Greater(t, space.Available, uint64(0))
}

func TestHasEnoughSpace(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		requiredBytes uint64
		expectEnough  bool
	}{
		{
			name:          "1 KB should have space",
			requiredBytes: 1024,
			expectEnough:  true,
		},
		{
			name:          "1 MB should have space",
			requiredBytes: 1024 * 1024,
			expectEnough:  true,
		},
		{
			name:          "extremely large should not have space",
			requiredBytes: 1024 * 1024 * 1024 * 1024 * 1024, // 1 PB
			expectEnough:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasSpace, space, err := HasEnoughSpace(tempDir, tt.requiredBytes)
			require.NoError(t, err)
			assert.NotNil(t, space)
			assert.Equal(t, tt.expectEnough, hasSpace)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536 * 1024 * 1024, "1.5 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckDiskSpaceBeforeDownload(t *testing.T) {
	tempDir := t.TempDir()

	// Get actual available space
	space, err := GetDiskSpace(tempDir)
	require.NoError(t, err)

	tests := []struct {
		name              string
		estimatedSize     uint64
		minFreeSpaceBytes uint64
		expectError       bool
	}{
		{
			name:              "small download with no minimum",
			estimatedSize:     1024 * 1024, // 1 MB
			minFreeSpaceBytes: 0,
			expectError:       false,
		},
		{
			name:              "small download with reasonable minimum",
			estimatedSize:     1024 * 1024,       // 1 MB
			minFreeSpaceBytes: 100 * 1024 * 1024, // 100 MB
			expectError:       space.Available < (101 * 1024 * 1024),
		},
		{
			name:              "unreasonably large download",
			estimatedSize:     1024 * 1024 * 1024 * 1024 * 1024, // 1 PB
			minFreeSpaceBytes: 0,
			expectError:       true,
		},
		{
			name:              "unreasonably large minimum free space",
			estimatedSize:     1024,                             // 1 KB
			minFreeSpaceBytes: 1024 * 1024 * 1024 * 1024 * 1024, // 1 PB
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckDiskSpaceBeforeDownload(tempDir, tt.estimatedSize, tt.minFreeSpaceBytes)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckDiskSpaceBeforeDownload_NonExistentPath(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "future", "download", "path")

	// Should not error even if path doesn't exist yet
	err := CheckDiskSpaceBeforeDownload(nonExistent, 1024*1024, 0)
	assert.NoError(t, err)
}

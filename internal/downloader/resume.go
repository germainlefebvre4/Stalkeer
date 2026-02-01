package downloader

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/glefebvre/stalkeer/internal/errors"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/models"
)

// ResumeSupport handles resumable download logic
type ResumeSupport struct {
	stateManager *StateManager
}

// NewResumeSupport creates a new resume support handler
func NewResumeSupport(stateManager *StateManager) *ResumeSupport {
	return &ResumeSupport{
		stateManager: stateManager,
	}
}

// CheckServerSupport checks if server supports HTTP range requests
func (rs *ResumeSupport) CheckServerSupport(ctx context.Context, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * 1000000000, // 10 seconds
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send HEAD request: %w", err)
	}
	defer resp.Body.Close()

	// Check for Accept-Ranges header
	acceptRanges := resp.Header.Get("Accept-Ranges")
	if acceptRanges == "bytes" {
		return true, nil
	}

	// Some servers support ranges but don't advertise it
	// We'll need to try a range request to be sure
	return false, nil
}

// ValidatePartialFile validates that a partial download file exists and has expected size
func (rs *ResumeSupport) ValidatePartialFile(partialPath string, expectedBytes int64) (bool, int64, error) {
	log := logger.AppLogger()

	// Check if file exists
	info, err := os.Stat(partialPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to stat partial file: %w", err)
	}

	actualBytes := info.Size()

	// Validate file size
	if actualBytes == 0 {
		log.WithFields(map[string]interface{}{
			"path": partialPath,
		}).Debug("partial file is empty, will restart download")
		return false, 0, nil
	}

	if actualBytes > expectedBytes {
		log.WithFields(map[string]interface{}{
			"path":           partialPath,
			"actual_bytes":   actualBytes,
			"expected_bytes": expectedBytes,
		}).Warn("partial file is larger than expected, will restart download")
		// Remove corrupted file
		os.Remove(partialPath)
		return false, 0, nil
	}

	if actualBytes == expectedBytes {
		// File is complete
		return true, actualBytes, nil
	}

	// File is valid partial download
	log.WithFields(map[string]interface{}{
		"path":             partialPath,
		"bytes_downloaded": actualBytes,
		"total_bytes":      expectedBytes,
		"progress_pct":     fmt.Sprintf("%.1f%%", float64(actualBytes)/float64(expectedBytes)*100),
	}).Info("found valid partial download")

	return true, actualBytes, nil
}

// GetPartialFilePath returns the path for a partial download file
func (rs *ResumeSupport) GetPartialFilePath(tempDir, fileName string) string {
	return filepath.Join(tempDir, fileName+".partial")
}

// BuildResumeRequest creates an HTTP request with Range header for resuming
func (rs *ResumeSupport) BuildResumeRequest(ctx context.Context, url string, startByte int64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Range header (RFC 7233)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))

	return req, nil
}

// HandleResumeResponse processes HTTP response for resumed download
func (rs *ResumeSupport) HandleResumeResponse(resp *http.Response, expectedStartByte int64) error {
	// Check status code
	// 206 Partial Content = server honors range request
	// 200 OK = server ignores range request (full download)
	if resp.StatusCode == http.StatusPartialContent {
		// Validate Content-Range header
		contentRange := resp.Header.Get("Content-Range")
		if contentRange == "" {
			return fmt.Errorf("server returned 206 but no Content-Range header")
		}

		// Parse Content-Range (format: "bytes START-END/TOTAL")
		// Example: "bytes 1024-2047/2048"
		var start, end, total int64
		_, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &total)
		if err != nil {
			return fmt.Errorf("failed to parse Content-Range header: %s", contentRange)
		}

		if start != expectedStartByte {
			return fmt.Errorf("server returned unexpected start byte: expected %d, got %d", expectedStartByte, start)
		}

		logger.AppLogger().WithFields(map[string]interface{}{
			"start_byte": start,
			"end_byte":   end,
			"total":      total,
		}).Debug("resuming download with partial content")

		return nil
	}

	if resp.StatusCode == http.StatusOK {
		// Server doesn't support ranges or is sending full file
		logger.AppLogger().Warn("server doesn't support range requests, restarting download")
		return errors.ValidationError("server does not support resume")
	}

	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

// ShouldAttemptResume determines if we should try to resume a download
func (rs *ResumeSupport) ShouldAttemptResume(download *models.DownloadInfo, partialPath string) bool {
	// Must have partial download info
	if !download.HasPartialDownload() {
		return false
	}

	// Check if partial file exists and is valid
	valid, _, err := rs.ValidatePartialFile(partialPath, *download.BytesDownloaded)
	if err != nil {
		logger.AppLogger().WithFields(map[string]interface{}{
			"error": err,
			"path":  partialPath,
		}).Warn("failed to validate partial file")
		return false
	}

	return valid
}

// GetResumeInfo extracts resume information from download record
type ResumeInfo struct {
	BytesDownloaded int64
	TotalBytes      int64
	ResumeToken     string
	PartialPath     string
}

// ExtractResumeInfo gets resume information from a download record
func (rs *ResumeSupport) ExtractResumeInfo(download *models.DownloadInfo, tempDir string) (*ResumeInfo, error) {
	if !download.HasPartialDownload() {
		return nil, errors.ValidationError("no partial download information")
	}

	info := &ResumeInfo{
		BytesDownloaded: *download.BytesDownloaded,
		TotalBytes:      *download.TotalBytes,
	}

	if download.ResumeToken != nil {
		info.ResumeToken = *download.ResumeToken
	}

	// Construct partial file path
	if download.DownloadPath != nil {
		fileName := filepath.Base(*download.DownloadPath)
		// Remove extension and add .partial
		fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
		info.PartialPath = rs.GetPartialFilePath(tempDir, fileName)
	}

	return info, nil
}

// CleanupPartialFile removes a partial download file
func (rs *ResumeSupport) CleanupPartialFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove partial file: %w", err)
	}
	return nil
}

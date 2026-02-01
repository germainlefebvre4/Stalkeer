package downloader

import (
	"context"
	"sync"
	"time"
)

// DownloadJob represents a single download job
type DownloadJob struct {
	ID      int
	Options DownloadOptions
}

// DownloadJobResult contains the result of a download job
type DownloadJobResult struct {
	JobID  int
	Result *DownloadResult
	Error  error
}

// ParallelDownloader manages parallel download operations
type ParallelDownloader struct {
	downloader  *Downloader
	concurrency int
}

// NewParallel creates a new parallel downloader
func NewParallel(timeout time.Duration, retryAttempts int, concurrency int) *ParallelDownloader {
	if concurrency <= 0 {
		concurrency = 3 // Default concurrency
	}

	return &ParallelDownloader{
		downloader:  New(timeout, retryAttempts),
		concurrency: concurrency,
	}
}

// DownloadBatch downloads multiple files in parallel
// Returns a channel of results and starts processing immediately
func (pd *ParallelDownloader) DownloadBatch(ctx context.Context, jobs []DownloadJob) <-chan DownloadJobResult {
	results := make(chan DownloadJobResult, len(jobs))
	jobQueue := make(chan DownloadJob, len(jobs))

	// Fill the job queue
	for _, job := range jobs {
		jobQueue <- job
	}
	close(jobQueue)

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < pd.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobQueue {
				select {
				case <-ctx.Done():
					results <- DownloadJobResult{
						JobID: job.ID,
						Error: ctx.Err(),
					}
					return
				default:
					result, err := pd.downloader.Download(ctx, job.Options)
					results <- DownloadJobResult{
						JobID:  job.ID,
						Result: result,
						Error:  err,
					}
				}
			}
		}(i)
	}

	// Close results channel when all workers complete
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// DownloadBatchSync downloads multiple files in parallel and waits for all to complete
func (pd *ParallelDownloader) DownloadBatchSync(ctx context.Context, jobs []DownloadJob) []DownloadJobResult {
	resultsChan := pd.DownloadBatch(ctx, jobs)

	var results []DownloadJobResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

// DownloadBatchWithProgress downloads multiple files in parallel with progress tracking
func (pd *ParallelDownloader) DownloadBatchWithProgress(
	ctx context.Context,
	jobs []DownloadJob,
	onProgress func(completed, total int),
) []DownloadJobResult {
	total := len(jobs)
	completed := 0
	var mu sync.Mutex

	resultsChan := pd.DownloadBatch(ctx, jobs)

	var results []DownloadJobResult
	for result := range resultsChan {
		results = append(results, result)

		mu.Lock()
		completed++
		if onProgress != nil {
			onProgress(completed, total)
		}
		mu.Unlock()
	}

	return results
}

// GetConcurrency returns the current concurrency level
func (pd *ParallelDownloader) GetConcurrency() int {
	return pd.concurrency
}

// SetConcurrency updates the concurrency level
func (pd *ParallelDownloader) SetConcurrency(concurrency int) {
	if concurrency > 0 {
		pd.concurrency = concurrency
	}
}

package redis

type URLStatus string

const (
	StatusQueue      URLStatus = "queue"
	StatusProcessing URLStatus = "processing"
	StatusRetry      URLStatus = "retry"
	StatusProcessed  URLStatus = "processed"
	StatusFailure    URLStatus = "failure"
	StatusManual     URLStatus = "manual"
)

type URLInfo struct {
	Status              URLStatus `json:"status"`
	WorkerID            string    `json:"worker_id"`
	ProcessingStartTime int64     `json:"processing_start_time"` // UnixMilli format
	Tries               int       `json:"retries"`
	NextRetryTime       int64     `json:"next_retry_time"` // UnixMilli format
}

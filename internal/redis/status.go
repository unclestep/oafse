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
	Status              URLStatus
	WorkerID            int64
	ProcessingStartTime int64 // UnixNano format
	Tries               int
	NextRetryTime       int64 // UnixNano format
}

package redis

const (
	PrefixProcessingQueue = "crawler:processing:queue"
	KeyProcessingIndex    = "crawler:processing:index"
	KeyURLStatus          = "crawler:url:status"
	KeyQueue              = "crawler:queue"
	KeyRetry              = "crawler:retry"
)

func WorkerKey(workerID string) string {
	return PrefixProcessingQueue + ":" + workerID
}

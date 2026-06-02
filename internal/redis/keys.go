package redis

const (
	prefixProcessingQueue = "crawler:processing:queue"
	keyProcessingIndex    = "crawler:processing:index"
	keyURLStatus          = "crawler:url:status"
	keyQueue              = "crawler:queue"
	keyRetry              = "crawler:retry"
)

func workerKey(workerID string) string {
	return prefixProcessingQueue + ":" + workerID
}

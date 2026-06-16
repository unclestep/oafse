package embedder

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "oafse/pkg/proto/embedder"
)

type Embedder struct {
	conn *grpc.ClientConn
	pb   pb.EmbedderClient
}

func NewEmbedder(addr string) (*Embedder, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("embedder client: %w", err)
	}

	return &Embedder{
		conn: conn,
		pb:   pb.NewEmbedderClient(conn),
	}, nil
}

func (e *Embedder) Close() error {
	return e.conn.Close()
}

const maxBatchSize = 3_500_000

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.pb.Embed(ctx, &pb.EmbedRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	return resp.Vector, nil
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	vectors := make([][]float32, 0, len(texts))
	curBatchSize := 0
	start := 0

	for i := 0; i < len(texts); {
		textSize := len(texts[i])
		if curBatchSize+textSize > maxBatchSize && start == i {
			log.Printf("[WARN] text %.10s... is too long to be vectorized", texts[i])
			vectors = append(vectors, []float32{})

			i++
			start = i
		} else if curBatchSize+textSize > maxBatchSize {
			chunk, err := e.embedBatch(ctx, texts[start:i])
			if err != nil {
				return nil, fmt.Errorf("embed batch chunk [%d:%d): %w", start, i, err)
			}

			vectors = append(vectors, chunk...)
			start = i
			curBatchSize = 0
		} else {
			curBatchSize += textSize
			i++
		}
	}

	if start < len(texts) {
		chunk, err := e.embedBatch(ctx, texts[start:])
		if err != nil {
			return nil, fmt.Errorf("embed batch chunk [%d:%d): %w", start, len(texts), err)
		}
		vectors = append(vectors, chunk...)
	}

	return vectors, nil
}

func (e *Embedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := e.pb.EmbedBatch(ctx, &pb.EmbedBatchRequest{Texts: texts})
	if err != nil {
		return nil, err
	}

	vectors := make([][]float32, len(resp.Vectors))
	for i, v := range resp.Vectors {
		vectors[i] = v.Values
	}

	return vectors, nil
}

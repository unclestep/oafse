# OAF Search Engine

OAF search engine is a semantic search engine over crawled web pages, built in Go with a Python sidecar for SBERT inference.

A crawler collects pages into PostgreSQL, an indexer vectorizes them via SBERT, and a search server performs semantic similarity search using pgvector's inner dot-product operator on L2-normalized vectors. That's all instrumented with Prometheus metrics and a Grafana dashboards.

## Architecture

1. **Crawl Seeding:** A site is added at runtime via `POST /crawl?url=...` on the crawler's own REST control API; no restart needed to start crawling a new domain.
2. **Crawler (Go):** A long-running crawler service that pulls URLs from Redis, fetches and parses pages, and writes pages and links into PostgreSQL.
3. **Redis:** Backs the crawler with a main queue (`LIST`), a retry queue (`ZSET` scored by next-retry timestamp), and dedup/processing sets. Workers wake via a scheduled retry timer raced against a keyspace-notification Pub/Sub signal on new pushes.
4. **PostgreSQL:** Stores pages and links; an insert triggers a `pg_notify` event.
5. **Indexer (Go):** Subscribes to the Postgres `NOTIFY` channel, batches unvectorized pages, and requests embeddings for them.
6. **Embedder (Python / gRPC):** Serves SBERT (`all-MiniLM-L6-v2`) embeddings over gRPC, supporting both single and batched requests.
7. **PostgreSQL + pgvector:** The indexer writes the returned vectors back into a `vector(384)` column, L2-normalizing them.
8. **Search Server (Go):** Exposes `GET /search?q=...`. It embeds the query via the same embedder, L2-normalizes it, and then queries Postgres for the `top-k` most similar pages using dot product, filtered by a minimum similarity threshold.
8. **Observability:** Every service exposes `/metrics`; Prometheus scrapes all of them, and four Grafana dashboards are provisioned automatically on startup.

## Getting Started

```bash
cp .env.example .env
docker-compose up
```

## API

The crawler runs as a persistent service - seed it via its own control API:

```bash
curl -X POST "http://localhost:14443/crawl?url=https://books.toscrape.com"
```
Crawl control API docs: `http://localhost:14443/swagger/index.html`

Note: The Search API is available during indexing; however, unindexed pages will not appear in results.

```bash
curl "http://localhost:14444/search?q=ernest+hemingway"
```
Search API docs: `http://localhost:14444/swagger/index.html`

Example result for `https://books.toscrape.com`:
<img width="1280" height="906" alt="image" src="https://github.com/user-attachments/assets/da2424a2-aa8b-4396-81c8-b078e363b764" />

## Observability

Each service exposes `/metrics` (Prometheus format, port `9100`); Prometheus scrapes all of them, and four Grafana dashboards are provisioned automatically on startup (`OAFSE / Crawler`, `OAFSE / Indexer`, `OAFSE / Postgres`, `OAFSE / Search Server`).

Crawler throughput, fetch latency, SPA fallback rate:
<img width="2388" height="1546" alt="image" src="https://github.com/user-attachments/assets/58333347-2804-4063-b26e-c16c51459f5d" />

Indexer backlog and embedding latency:
<img width="2404" height="1530" alt="image" src="https://github.com/user-attachments/assets/b7bd0181-f2d0-47ad-9fa1-890aac271243" />

## Benchmark Results

Observed metrics from the latest crawl + index + search run over `books.toscrape.com` and `quotes.toscrape.com`. These are production-pipeline observations, not a controlled load test - see [Future Plans](#future-plans).

| Metric                                  | Value              |
| --------------------------------------- | ------------------ |
| Pages fetched (total attempts)          | 1,456              |
| Fetch success rate                      | 100%               |
| Total bytes fetched                     | ~28.3 MB           |
| Avg HTTP fetch latency                  | 266 ms             |
| p95 HTTP fetch latency                  | 587 ms             |
| Pages successfully embedded             | 1,449              |
| Embedding errors                        | 2 (99.86% success) |
| Avg embed batch duration                | 732 ms / batch     |
| Avg search request latency              | 56 ms              |
| Postgres pool acquire latency (indexer) | ~5 µs              |
| Postgres pool acquire latency (crawler) | ~64 µs             |
| Postgres pool acquire latency (server)  | ~6.9 ms            |

## Design Decisions

- Two-tier Redis Queue (LIST + ZSET): Avoids priority starvation by separating the main queue (LIST) from the retry queue (ZSET with timestamps). Workers drain retries only when the main queue is empty, mimicking Go’s local/global scheduler model.
- Atomic Lua Scripts: Encapsulates push (dedup + enqueue) and pop (move to processing) operations in single Redis Lua scripts to eliminate race conditions among concurrent workers.
- In-Database Vector Search: Delegates similarity matching to PostgreSQL via pgvector using the <#> negative dot product operator. Since vectors are L2-normalized, this effectively computes optimized cosine similarity without loading the corpus into Go memory.
- Minimum Similarity Threshold: Discards search results below an empirically calibrated inner-product threshold instead of forcing a fixed limit of irrelevant matches.
- gRPC for Embedding: Uses Protobuf binary encoding between the indexer and embedder to eliminate per-call JSON marshaling overhead during high-frequency batch polling.
- LISTEN/NOTIFY for Indexing: Replaces interval-based polling with Postgres NOTIFY triggers on page inserts, reducing idle database load and dropping indexing latency to near-zero.
- Hybrid Worker Wake-up (Timer + Pub/Sub): Workers race a scheduled retry sleep against Redis keyspace notifications (LPUSH). Pub/Sub ensures near-zero latency for fresh URLs, while the timer acts as a reliability backstop against missed events.

## Testing

Integration tests run against real PostgreSQL and Redis instances via `testcontainers-go`, with per-test transaction rollback isolation for Postgres. Coverage includes the fetcher (httptest), extractor, worker pool, Redis queue/processing/retry logic, and use cases via mocked ports.

## Future Plans

1. Semantic Query Cache: Caches recent query vectors in Redis and matches new requests via cosine similarity threshold, allowing semantically equivalent queries to share a cache without exact string matches.
2. Cross-Modal Search (CLIP): Embeds page images alongside text into a shared vector space, enabling cross-modal text-to-image and image-to-text search with metadata filtering.
3. GNN Reranking (GAT): Trains a Graph Attention Network in PyTorch Geometric using SBERT node features to compute structural authority. Reranks results using score = cosine_sim * log(1 + gnn_score) via the existing links table.
4. Load Testing & Benchmarking: Uses k6/wrk to stress-test /search latency, benchmark crawler throughput under varying worker pool sizes, and validate HNSW index performance at scale.
5. Anti-Bot Resilience: Implements proxy pooling with per-domain cooldowns, cookie-jar sessions (chromedp to HTTP fallback), and User-Agent rotation to bypass active scraping defenses.

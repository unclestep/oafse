CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE pages
    ALTER COLUMN vector TYPE vector(384)
    USING l2_normalize(vector::vector(384));

CREATE INDEX pages_vector_hnsw_idx ON pages USING hnsw (vector vector_ip_ops);

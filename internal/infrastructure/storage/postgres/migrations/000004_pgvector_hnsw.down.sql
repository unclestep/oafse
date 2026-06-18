DROP INDEX IF EXISTS pages_vector_hnsw_idx;

ALTER TABLE pages
    ALTER COLUMN vector TYPE real[]
    USING vector::real[];

DROP EXTENSION IF EXISTS vector;

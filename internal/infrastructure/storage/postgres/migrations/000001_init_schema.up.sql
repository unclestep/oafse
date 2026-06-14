CREATE TABLE IF NOT EXISTS pages(
    id BIGSERIAL PRIMARY KEY,
    url TEXT UNIQUE NOT NULL,
    title TEXT,
    description TEXT,
    content TEXT,
    crawled_at TIMESTAMP DEFAULT now()
);

CREATE TABLE IF NOT EXISTS links(
    src_page_id BIGINT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    dst_url TEXT NOT NULL,
    PRIMARY KEY(src_page_id, dst_url)
);

CREATE INDEX IF NOT EXISTS idx_links_src ON links(src_page_id);
CREATE INDEX IF NOT EXISTS idx_links_dst ON links(dst_url);

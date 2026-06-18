CREATE OR REPLACE FUNCTION notify_page_inserted()
RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('page_inserted', '');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER pages_inserted
AFTER INSERT ON pages
FOR EACH STATEMENT EXECUTE FUNCTION notify_page_inserted();

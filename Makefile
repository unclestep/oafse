all:
	CGO_ENABLED=0 go build -o crawler cmd/crawler/main.go
	CGO_ENABLED=0 go build -o crawler cmd/indexer/main.go

setup:
	@echo "Setting up Chrome/Chromium for SPA tests..."
	@if command -v google-chrome >/dev/null 2>&1 || command -v google-chrome-stable >/dev/null 2>&1; then \
		echo "Google Chrome already installed."; \
	elif command -v chromium >/dev/null 2>&1 && chromium --version >/dev/null 2>&1; then \
		echo "Chromium already installed."; \
	elif [ "$$(uname)" = "Darwin" ]; then \
		echo "macOS detected, installing Chromium..."; \
		brew install --cask chromium; \
	elif command -v apk >/dev/null 2>&1; then \
		echo "Alpine detected, installing Chromium..."; \
		apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont; \
	elif command -v apt-get >/dev/null 2>&1; then \
		echo "Debian/Ubuntu detected, installing Google Chrome..."; \
		wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb -O /tmp/_chrome.deb \
		&& sudo apt-get install -y /tmp/_chrome.deb \
		&& sudo ln -sf /usr/bin/google-chrome /usr/bin/chromium-browser \
		&& rm -f /tmp/_chrome.deb; \
	elif command -v dnf >/dev/null 2>&1; then \
		echo "Fedora detected, installing Chromium..."; \
		sudo dnf install -y chromium; \
	else \
		echo "ERROR: Unsupported OS, install Chrome/Chromium manually"; exit 1; \
	fi

test: setup
	go test ./... -timeout 120s -short

db-check:
	@. ./.env && psql "$$POSTGRES_LOCAL_DSN" -c "SELECT url, title, crawled_at FROM pages ORDER BY crawled_at DESC LIMIT 20;" \
	                -c "SELECT COUNT(*) AS total_pages FROM pages;" \
	                -c "SELECT COUNT(*) AS total_links FROM links;"

PYTHON := $(shell command -v python3 2>/dev/null || command -v python 2>/dev/null)

proto:
	protoc \
		--go_out=pkg/proto/embedder \
		--go_opt=paths=source_relative \
		--go-grpc_out=pkg/proto/embedder \
		--go-grpc_opt=paths=source_relative \
		--proto_path=proto/embedder \
		proto/embedder/embedder.proto
	$(PYTHON) -m grpc_tools.protoc \
		--python_out=embedder/proto \
		--grpc_python_out=embedder/proto \
		--proto_path=proto/embedder \
		proto/embedder/embedder.proto

.PHONY: all setup test db-check proto

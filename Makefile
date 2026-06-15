all:
	CGO_ENABLED=0 go build -o crawler cmd/main.go

setup:
	@echo "Installing Chromium for SPA tests..."
	@if command -v chromium >/dev/null 2>&1 || \
		command -v chromium-browser >/dev/null 2>&1 || \
		command -v google-chrome >/dev/null 2>&1 || \
    	command -v google-chrome-stable >/dev/null 2>&1; then \
			echo "Browser found, installation is not required."; \
	elif [ "$$(uname)" = "Darwin" ]; then \
			echo "Detected MacOS. Installing..."; \
			brew install --cask chromium; \
	elif command -v apk >/dev/null 2>&1; then \
			echo "Detected Alpine. Installing..."; \
			apk add --no-cache chromium chromium-chromedriver nss freetype freetype-dev harfbuzz ca-certificates ttf-freefont; \
	elif command -v apt-get >/dev/null 2>&1; then \
			echo "Detected Ubuntu/Debian. Installing..."; \
			sudo apt-get update && sudo apt-get install -y chromium; \
	elif command -v dnf >/dev/null 2>&1; then \
			echo "Detected Fedora. Installing..."; \
			sudo dnf install -y chromium; \
	else \
			echo "ERROR: Unknown OS"; exit 1; \
	fi

test: setup
	go test ./... -timeout 120s -short

.PHONE: all setup test

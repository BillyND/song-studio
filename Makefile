# Quê Hương Studio — a Mac app (Swift window + Go server) for the suno-que-huong skill.
VERSION ?= $(shell cat VERSION)
PREFIX  ?= $(HOME)/.local
APPDIR  ?= $(HOME)/Applications/Quê Hương Studio.app
SKILL   ?= $(HOME)/.claude/skills/suno-que-huong
BIN      = bin/que-huong-studio
SPARKLE  = 2.10.0
# release notes travel to packaging/release.sh in the environment (they may span lines)
export NOTES

.PHONY: build test run app install release sparkle-tools uninstall clean

# build: the Go server alone (also usable from a terminal: bin/que-huong-studio)
build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) .

test:
	gofmt -l . | (! grep .) && go vet ./... && go test ./...

run: build
	./$(BIN)

# app: out/Quê Hương Studio.app with the server, Sparkle and a snapshot of the skill
app: test
	bash packaging/build-app.sh "$(VERSION)" "$(SKILL)"

# install: the app into ~/Applications (quits a running copy first) and the server on PATH
install: app
	-osascript -e 'tell application id "local.que-huong-studio" to quit' >/dev/null 2>&1
	rm -rf "$(APPDIR)"
	ditto "out/Quê Hương Studio.app" "$(APPDIR)"
	install -d "$(PREFIX)/bin"
	ln -sf "$(APPDIR)/Contents/MacOS/que-huong-studio" "$(PREFIX)/bin/que-huong-studio"
	@echo "Installed $(VERSION). Open it with Spotlight: Quê Hương Studio"

# release: bump VERSION first; publishes to the public releases repo, and every installed app updates itself.
# NOTES (optional, may span lines) reaches the script through the environment, not the command line.
release: test sparkle-tools
	bash packaging/release.sh "$(VERSION)" "$(SKILL)"

sparkle-tools:
	@test -x packaging/sparkle-tools/bin/sign_update || (mkdir -p packaging/sparkle-tools && \
	  curl -sSfL https://github.com/sparkle-project/Sparkle/releases/download/$(SPARKLE)/Sparkle-$(SPARKLE).tar.xz | tar -xJ -C packaging/sparkle-tools)

uninstall:
	rm -f "$(PREFIX)/bin/que-huong-studio"
	rm -rf "$(APPDIR)"

clean:
	rm -rf bin out macapp/.build

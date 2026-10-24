# Quê Hương Studio — local workflow web for the suno-que-huong skill.
VERSION ?= 1.0.0
PREFIX  ?= $(HOME)/.local
APPDIR  ?= $(HOME)/Applications/Quê Hương Studio.app
BIN      = bin/que-huong-studio

.PHONY: build test run install uninstall clean

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) .

test:
	gofmt -l . | (! grep .) && go vet ./... && go test ./...

run: build
	./$(BIN)

# install puts the binary in ~/.local/bin and a launchable app in ~/Applications
install: test build
	install -d "$(PREFIX)/bin"
	install -m 755 $(BIN) "$(PREFIX)/bin/que-huong-studio"
	rm -rf "$(APPDIR)"
	install -d "$(APPDIR)/Contents/MacOS" "$(APPDIR)/Contents/Resources"
	install -m 644 packaging/Info.plist "$(APPDIR)/Contents/Info.plist"
	sed "s#__STUDIO_PATH__#$$PATH#" packaging/launcher.sh > "$(APPDIR)/Contents/MacOS/launcher"
	chmod 755 "$(APPDIR)/Contents/MacOS/launcher"
	install -m 644 packaging/AppIcon.icns "$(APPDIR)/Contents/Resources/AppIcon.icns"
	@echo "Installed. Open it with Spotlight: Quê Hương Studio (or run: que-huong-studio)"

uninstall:
	rm -f "$(PREFIX)/bin/que-huong-studio"
	rm -rf "$(APPDIR)"

clean:
	rm -rf bin

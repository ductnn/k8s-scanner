.PHONY: build-linux build-mac build-windows build-all

# Build with CGO disabled for compatibility with older systems (CentOS 7)
LINUX=env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -v
MAC=env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -v
WINDOWS=env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -v

build-linux:
	@mkdir -p bin/linux
	$(LINUX) -o bin/linux/k8s-scanner cmd/scanner/main.go

build-mac:
	@mkdir -p bin/darwin
	$(MAC) -o bin/darwin/k8s-scanner cmd/scanner/main.go

build-windows:
	@mkdir -p bin/windows
	$(WINDOWS) -o bin/windows/k8s-scanner.exe cmd/scanner/main.go

build-all: build-linux build-mac build-windows
	@echo "Built for all platforms: linux, darwin, windows"
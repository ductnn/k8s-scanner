.PHONY: build-linux build-mac build-windows build-all

LINUX=env GOOS=linux GOARCH=amd64 go build -v
MAC=env GOOS=darwin GOARCH=amd64 go build -v
WINDOWS=env GOOS=windows GOARCH=amd64 go build -v

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
.PHONY: build-linux build-mac

LINUX=env GOOS=linux GOARCH=amd64 go build -v
MAC=env GOOS=darwin GOARCH=amd64 go build -v

build-linux:
	$(LINUX) -o bin/linux/k8s-scanner cmd/scanner/main.go

build-mac:
	$(MAC) -o bin/darwin/k8s-scanner cmd/scanner/main.go
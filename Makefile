.PHONY: build
build:
	go build -o build/cloudreve-uploader main.go

build-all: build
	env GOOS=linux GOARCH=amd64 go build -o build/cloudreve-uploader.linux.amd64 main.go
	env GOOS=linux GOARCH=arm64 go build -o build/cloudreve-uploader.linux.arm64 main.go
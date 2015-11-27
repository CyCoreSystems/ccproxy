GOPATH = $(shell pwd)/vendor:$(shell pwd)

all: test build

docker: build push_docker

test:
	gb test

build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -extld ld -extldflags -static' -a -x -o bin/ccproxy src/cmd/ccproxy/main.go
	docker build -t quay.io/cycore/ccproxy ./
#gb build

push_docker:
	docker push quay.io/cycore/ccproxy

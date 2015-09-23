all: test build

docker: build push_docker

test:
	gb test

build:
	gb build
	docker build -t quay.io/cycore/ccproxy ./

push_docker:
	docker push quay.io/cycore/ccproxy

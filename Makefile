VERSION?=$(shell cat VERSION)
IMAGE_TAG?=v$(VERSION)

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run

push_image:
	docker build -t yutachaos/kube-job-notifier:$(IMAGE_TAG) .
    docker push yutachaos/argocd-notifications:$(IMAGE_TAG)
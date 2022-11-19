.PHONY: setup-local-env teardown-local-env watch test tidy run build

natsContainer?=nats

setup-local-env:
	docker run -d \
		--name $(natsContainer) \
		--rm \
		-p 4222:4222 \
		-p 8222:8222 \
			nats \
				--http_port 8222

teardown-local-env:
	docker kill $(natsContainer) || true

watch:
	modd -f modd.conf

test:
	go test -timeout 1s ./...

build:
	go build -o dist/stage ./cmd/stage

run: build
	./dist/stage upstream

tidy:
	go mod tidy
	go fmt ./...

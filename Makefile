.PHONY: generate test watch tidy \
	grpc_gen

generate: internal/protocol/protocol_grpc.pb.go
	go build ./...

internal/protocol/protocol_grpc.pb.go: internal/protocol/protocol.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/protocol/protocol.proto

test: generate
	go test ./...

watch:
	modd

tidy:
	go fmt ./...
	go mod tidy

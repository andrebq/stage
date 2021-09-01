.PHONY: generate tinygo

generate: ./wasm_programs/ping.wasm

./wasm_programs/ping.wasm: ./wasm_programs/ping.go
	docker run --rm -v $(PWD):$(PWD) -w $(PWD) tinygo/tinygo:0.19.0 tinygo build -opt=s -o $(PWD)/example/scripted/wasm/wasm_programs/ping.wasm -target wasm $(PWD)/example/scripted/wasm/wasm_programs/ping.go

run: ./wasm_programs/ping.wasm
	go build .
	./wasm

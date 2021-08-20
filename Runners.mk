.PHONY: run-exchange

run-exchange: dist
	./dist/stage exchange

run-bridge: dist
	./dist/stage -debug exchange -bridge

msg?='{"marco":"polo"}'
target?="actor"
echo: dist
	./dist/stage helper cat -target=$(target) <<<"$(msg)"

delay?=1
annoying-echo:
	while true; do ./dist/stage helper cat -target=$(target) <<<$(msg); sleep $(delay); done

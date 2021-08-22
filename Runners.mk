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

actorid?=hello-actor
behaviour?=index.js
jtb: dist
	go build -o ./dist/jtb ./example/scripted/jtb
	./dist/jtb -code ./example/scripted/jtb/$(behaviour) -actorID "$(actorid)"

jtb-ping: dist
	./dist/jtb -code ./example/scripted/jtb/ping.js -actorID "pinger"

jtb-pong: dist
	./dist/jtb -code ./example/scripted/jtb/pong.js -actorID "pong"

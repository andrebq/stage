let io = require("@stdio")
let actor = require("@actor")

io.println("waiting for the first message");
io.println("sending my salutes to pong actor");


while (true) {
	actor.write("pong", JSON.stringify({ hello: "world" }));
	io.println("waiting for the next message...");
	io.println(JSON.stringify(actor.read()));
}

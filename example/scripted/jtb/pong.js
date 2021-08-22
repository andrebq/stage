let io = require("@stdio")
let actor = require("@actor")

io.println("Welcome to the pong actor");
io.println(" anytime I get a new message I'll simply reply with the same payload");
io.println(" that way the sender will know that I exist");

function getMessage(reader) {
	let msg = reader();
	try {
		return { sender: msg.sender, payload: JSON.parse(msg.payload) };
	} catch (ex) {
		console.error("Unable to process message", ex.toString());
		return;
	}
}

while (true) {
	let msg = getMessage(actor.read);
	if (!msg) {
		console.error("no message from the bridge")
		continue;
	}
	io.println("got: ", JSON.stringify(msg));
	actor.write(msg.sender, JSON.stringify(msg.payload));
}

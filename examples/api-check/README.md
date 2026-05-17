# GoMeshin API Check TUI

Separate Go module that talks to the `gomeshind` daemon over HTTP/SSE and builds a Bubble Tea TUI on top of the daemon API.

Start the daemon from the repo root:

```bash
go run ./cmd/gomeshind -port /dev/ttyUSB0 -db gomeshin.db -listen 127.0.0.1:8080
```

Then run the TUI from this directory:

```bash
go run . -api http://127.0.0.1:8080
```

The TUI no longer opens the serial port directly. The daemon owns the radio and database; the TUI is just a client.

Live incoming messages are received from `/events`. Sends, node lists, channels, and traceroute use the JSON HTTP endpoints.

Controls:

- Type a message and press `enter` to send.
- Press `tab` / `shift+tab` to switch channels.
- Press `esc` to open the main menu.
- In menus, use arrow keys to choose an option and `enter` to select.
- The main menu contains messages, nodes, channels, tools, channel editing, and exit.
- In tools, press `enter` to open the traceroute target picker.
- In the traceroute picker, type to search by short name, long name, or node number.
- Use arrow keys to choose a filtered node and `enter` to request traceroute.
- Exit is an explicit main menu option. `ctrl+c` also quits.

Messages are filtered to the selected channel.

Channel add/remove uses the GoMeshin `mesh.AddChannel` and `mesh.RemoveChannel` API calls, which send local Meshtastic admin channel updates to the attached radio.

Traceroute sends the request, waits for the matching route reply, and renders the forward and return paths when the radio receives them.

Traceroute waits up to 90 seconds in the TUI.

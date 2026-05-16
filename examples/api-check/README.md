# API Check Bubble Tea TUI

Separate Go module that imports GoMeshin like an external project and builds a Bubble Tea TUI on top of the public API.

It uses:

```go
import "meshin/mesh"
```

The local development import is wired with:

```go
replace meshin => ../..
```

Run from this directory:

```bash
go run . -port /dev/ttyUSB0
```

By default it stores messages, nodes, and channels in `gomeshin-api-check.db`.

```bash
go run . -port /dev/ttyUSB0 -db rugged-node.db
```

Controls:

- Type a message and press `enter` to send.
- Press `tab` / `shift+tab` to switch channels.
- Press `esc` to open the main menu.
- In menus, use arrow keys to choose an option and `enter` to select.
- The main menu contains messages, nodes, channels, tools, channel editing, and exit.
- In tools, use arrow keys to select a node and `enter` to request traceroute.
- Exit is an explicit main menu option. `ctrl+c` also quits.

Messages are filtered to the selected channel.

Channel add/remove uses the GoMeshin `mesh.AddChannel` and `mesh.RemoveChannel` API calls, which send local Meshtastic admin channel updates to the attached radio.

Traceroute sends the request, waits for the matching route reply, and renders the forward and return paths when the radio receives them.

Traceroute waits up to 90 seconds in the TUI.

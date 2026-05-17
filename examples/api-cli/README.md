# GoMeshin API CLI Example

Small command-line client for the `gomeshind` daemon. It uses HTTP for commands
and Server-Sent Events for live received messages.

Start the daemon from the repo root:

```bash
go run ./cmd/gomeshind -port /dev/ttyUSB0 -db gomeshin.db -listen 127.0.0.1:8080
```

Run commands from this directory:

```bash
go run . -api http://127.0.0.1:8080 -nodes
go run . -api http://127.0.0.1:8080 -positions
go run . -api http://127.0.0.1:8080 -weather
go run . -api http://127.0.0.1:8080 -channels
go run . -api http://127.0.0.1:8080 -messages
go run . -api http://127.0.0.1:8080 -listen
```

`-listen` prints live text messages, position updates, and weather/environment
telemetry updates.

Send a message:

```bash
go run . -api http://127.0.0.1:8080 -channel Primary -send "hello from api-cli"
```

Send a direct message:

```bash
go run . -api http://127.0.0.1:8080 -to '!12345678' -send "direct hello" -ack
```

Run traceroute:

```bash
go run . -api http://127.0.0.1:8080 -traceroute '!12345678' -channel Primary
```

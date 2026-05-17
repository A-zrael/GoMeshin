# GoMeshin

Small Go API for building a controller around a Meshtastic radio.

The public API is the `mesh` package. It owns the radio listener, tracks known messages, nodes, and channels, and exposes calls such as `Send`.

The lower-level `meshtasticapi` package is the serial/protobuf driver.

## Radio Setup

The radio must have `Security -> Serial enabled` turned on. The serial module is not required.

Use a normal client role while developing, such as `CLIENT` or `CLIENT_MUTE`.

## CLI

Listen for messages:

```bash
go run . -port /dev/ttyUSB0
```

Examples:

- `examples/api-check`: Bubble Tea TUI that talks to `gomeshind`.
- `examples/api-cli`: small command-line client that talks to `gomeshind`.
- `examples/web-client`: static HTML browser client that talks to `gomeshind`.

## Daemon API

`cmd/gomeshind` runs GoMeshin as a long-lived local service. It owns the serial
connection, keeps the SQLite store open, and exposes a JSON API for CLI tools,
TUIs, web UIs, scripts, and other clients.

Run it directly while developing:

```bash
go run ./cmd/gomeshind \
  -port /dev/ttyUSB0 \
  -db gomeshin.db \
  -listen 127.0.0.1:8080
```

Use a Unix socket instead of a TCP port:

```bash
go run ./cmd/gomeshind \
  -port /dev/ttyUSB0 \
  -db gomeshin.db \
  -unix /tmp/gomeshind.sock
```

Serve the web client from the daemon:

```bash
go run ./cmd/gomeshind \
  -port /dev/ttyUSB0 \
  -db gomeshin.db \
  -listen 127.0.0.1:8080 \
  -web-dir examples/web-client
```

Or run the browser example as a separate web server:

```bash
go run ./cmd/gomeshind \
  -port /dev/ttyUSB0 \
  -db gomeshin.db \
  -listen 127.0.0.1:8080 \
  -cors-origin http://127.0.0.1:8090
```

Then, in another terminal:

```bash
cd examples/web-client
python3 -m http.server 8090 --bind 127.0.0.1
```

Open `http://127.0.0.1:8090` and keep the API field set to
`http://127.0.0.1:8080`.

Useful endpoints:

```text
GET    /health
GET    /nodes
GET    /channels
POST   /channels
DELETE /channels/{name}
GET    /messages
GET    /messages?channel=Primary
POST   /messages
POST   /traceroute
GET    /events
GET    /events?channel=Primary
```

Send a message:

```bash
curl -X POST http://127.0.0.1:8080/messages \
  -H 'content-type: application/json' \
  -d '{"channel":"Primary","text":"hello mesh"}'
```

Send to a specific node:

```bash
curl -X POST http://127.0.0.1:8080/messages \
  -H 'content-type: application/json' \
  -d '{"to":"!12345678","channel":"Primary","text":"direct hello","wantAck":true}'
```

Watch live received messages with Server-Sent Events:

```bash
curl -N http://127.0.0.1:8080/events
```

Run traceroute:

```bash
curl -X POST http://127.0.0.1:8080/traceroute \
  -H 'content-type: application/json' \
  -d '{"to":"!12345678","channel":"Primary","hopLimit":3,"timeoutSeconds":90}'
```

Add and remove a secondary channel:

```bash
curl -X POST http://127.0.0.1:8080/channels \
  -H 'content-type: application/json' \
  -d '{"name":"ops"}'

curl -X DELETE http://127.0.0.1:8080/channels/ops
```

For browser clients served from a different port or host, `-cors-origin` must be
the web page origin, not the daemon API URL. For example, if the page is loaded
from `http://127.0.0.1:8090` and calls `http://127.0.0.1:8080`, use
`-cors-origin http://127.0.0.1:8090`. Multiple origins can be comma-separated.
Keep the daemon bound to `127.0.0.1` unless you deliberately want other machines
on the network to reach it.

List configured channels:

```bash
go run . -port /dev/ttyUSB0 -channels
```

List known nodes:

```bash
go run . -port /dev/ttyUSB0 -nodes
```

Send one broadcast text message:

```bash
go run . -port /dev/ttyUSB0 -send "hello mesh"
```

Send on a named channel:

```bash
go run . -port /dev/ttyUSB0 -channel ops -send "hello ops"
```

Interactive send and receive:

```bash
go run . -port /dev/ttyUSB0
```

## API

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

meshNode, err := mesh.Open(ctx, mesh.Config{
    Port: "/dev/ttyUSB0",
    Baud: 115200,
})
if err != nil {
    log.Fatal(err)
}
defer meshNode.Close()

messages, unsubscribe := meshNode.Subscribe(64)
defer unsubscribe()

go func() {
    for message := range messages {
        fmt.Println(message.From.ShortName, message.Channel.Name, message.Text)
    }
}()

id, err := meshNode.Send("hello mesh", mesh.SendOptions{})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("sent %08x\n", id)
```

`SendOptions{}` broadcasts on the primary channel.

```go
channels, err := meshNode.Channels(ctx)
if err != nil {
    log.Fatal(err)
}
for _, channel := range channels {
    fmt.Printf("%d %s %s\n", channel.Index, channel.Role, channel.Name)
}

_, err = meshNode.Send("hello ops", mesh.SendOptions{
    Channel: "ops",
})
```

Channels can also be edited through local admin messages:

```go
err = meshNode.AddChannel(ctx, mesh.ChannelOptions{Name: "ops"})
err = meshNode.RemoveChannel(ctx, "ops")
```

Traceroute requests can be sent to a known node number:

```go
route, err := meshNode.TraceRoute(ctx, mesh.TraceRouteOptions{
    To:      0x12345678,
    Channel: "ops",
})
fmt.Println(route.Towards)
```

`mesh.MemoryStore` is the default storage backend. Implement `mesh.Store` to persist messages, nodes, and channels to SQLite or another database.

The repo includes a SQLite store:

```go
store, err := sqlitestore.Open(ctx, "gomeshin.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

meshNode, err := mesh.Open(ctx, mesh.Config{
    Port:  "/dev/ttyUSB0",
    Baud:  115200,
    Store: store,
})
```

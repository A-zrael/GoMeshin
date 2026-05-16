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

The Bubble Tea TUI example lives in `examples/api-check`.

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

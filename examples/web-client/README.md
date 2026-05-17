# GoMeshin Web Client Example

Static browser client for the `gomeshind` daemon. It uses JSON HTTP requests for
commands and Server-Sent Events for live received messages.

Start the daemon from the repo root and let it serve this web client:

```bash
go run ./cmd/gomeshind \
  -port /dev/ttyUSB0 \
  -db gomeshin.db \
  -listen 127.0.0.1:8080 \
  -web-dir examples/web-client
```

Open:

```text
http://127.0.0.1:8080
```

Serving the page from `gomeshind` keeps the browser and API on the same origin,
so CORS is not involved.

## Split Server Mode

You can also run the web page and daemon as two separate servers. Start the
daemon with the web page origin allowed:

```bash
go run ./cmd/gomeshind \
  -port /dev/ttyUSB0 \
  -db gomeshin.db \
  -listen 127.0.0.1:8080 \
  -cors-origin http://127.0.0.1:8090
```

Then serve this directory:

```bash
python3 -m http.server 8090 --bind 127.0.0.1
```

Open:

```text
http://127.0.0.1:8090
```

Keep the API field set to:

```text
http://127.0.0.1:8080
```

For CORS, the origin is the web page URL, including the port. If the page is
loaded from `http://localhost:8090`, either open `http://127.0.0.1:8090`
instead or start the daemon with:

```bash
-cors-origin http://127.0.0.1:8090,http://localhost:8090
```

The page can list nodes/channels/messages, send channel messages, receive live
messages, and run traceroute through the daemon API.

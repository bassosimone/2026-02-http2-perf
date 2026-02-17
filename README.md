# 2026-02-http2-perf

This repository contains code to investigate HTTP/2 performance compared
to HTTP/1.1 and WebSocket, using both Go and Rust implementations.

## Setup

We use [LXC](https://linuxcontainers.org/) to create a three-container
network: client, router, and server. The router sits between client and
server, providing a realistic topology where we can later add netem
shaping. All benchmarks are orchestrated using the `lxs` tool.

```bash
go build -v ./cmd/lxs
./lxs create
```

## Running benchmarks

Verify the baseline bandwidth of the LXC topology:

```bash
./lxs iperf        # download
./lxs iperf -R     # upload
```

All protocol benchmarks follow the same pattern — start a server in one
terminal, then measure from another:

```bash
./lxs serve gohttp1              # start server
./lxs measure gohttp1            # download (GET)
./lxs measure gohttp1 -X PUT     # upload (PUT)
```

Available benchmarks:

| Command | What it tests |
|---|---|
| `gohttp1` | HTTP/1.1 cleartext (Go `net/http`) |
| `gohttp2` | HTTP/1.1 or HTTP/2 over TLS (Go `net/http` + `x/net/http2`). Use `-2` for HTTP/2. |
| `gohttp2c` | HTTP/2 cleartext / h2c (Go `x/net/http2/h2c`) |
| `ndt7` | ndt7 protocol (WebSocket over TLS, using `gorilla/websocket`) |
| `rusthttp2` | HTTP/2 over TLS (Rust, `hyper` + `axum` + `rustls`). Use `--no-tls` for h2c. |

## Results

Measured on an Intel Core i5 laptop, through the three-container LXC
topology (client -- router -- server, connected via veth pairs).

| Stack | Download | Upload | Notes |
|---|---|---|---|
| iperf3 (raw TCP) | 52 Gbit/s | 53 Gbit/s | Baseline link capacity |
| Go HTTP/1.1 cleartext | 47 Gbit/s | 45 Gbit/s | `net/http` overhead only |
| Go HTTP/1.1 + TLS | 20 Gbit/s | 20 Gbit/s | TLS is ~2.5x cost |
| ndt7 (WebSocket + TLS) | 20 Gbit/s | 19 Gbit/s | Same ceiling as HTTP/1.1+TLS |
| Rust h2c (no TLS) | 18 Gbit/s | 21 Gbit/s | h2 framing cost in Rust |
| Rust HTTP/2 + TLS | 11 Gbit/s | 12 Gbit/s | h2 + TLS combined |
| Go h2c (no TLS) | 5 Gbit/s | 8 Gbit/s | h2 framing cost in Go |
| Go HTTP/2 + TLS | 4 Gbit/s | 7 Gbit/s | h2 + TLS combined |

## Key observations

1. **TLS costs ~2.5x** regardless of protocol. Go's `crypto/tls` reduces
   throughput from ~47 to ~20 Gbit/s. This ceiling is the same for
   HTTP/1.1, WebSocket, and Rust h2c.

2. **Go's `x/net/http2` is the bottleneck.** Even without TLS (h2c),
   Go HTTP/2 only reaches 5-8 Gbit/s — the same as with TLS. The
   download path is probably limited by a single writer goroutine that
   serializes all frame writes.

3. **Rust's h2 is ~3x faster than Go's** for the same protocol,
   but still ~2x slower than HTTP/1.1 cleartext, showing that h2
   framing and flow control have inherent per-byte overhead.

4. **Switching from WebSocket to HTTP/2 is a throughput regression.**
   ndt7 over WebSocket+TLS achieves ~20 Gbit/s. Any HTTP/2-based
   protocol would probably achieve 4-12 Gbit/s depending on implementation.

5. **Further tuning and optimization may still be possible.** This testing
   is not exhaustive. I eventually timed out and chose to publish this initial
   version. Additionally, my Rust skills are fairly basic — the Rust benchmarks
   may have significant room for optimization that I missed.

## Open Questions / Future Work

Building on observation 5 above, these are genuinely open questions that could
be investigated with additional benchmarks using the same LXC testbed:

- **Production HTTP/2 reverse proxies**: Would placing nginx, Cloudflare's
  [pingora](https://github.com/cloudflare/pingora), or Google's
  [QUICHE](https://github.com/google/quiche) in front of a simple backend
  outperform Go's `x/net/http2`? These are battle-tested, heavily optimized
  HTTP/2 implementations that might not have the same single-writer bottleneck.
  In particular, would serving large static files through these proxies perform
  even better, since that is their primary optimization target (sendfile,
  zero-copy, mmap)?

- **HTTP/3 (QUIC)**: Does QUIC change the picture? It eliminates TCP head-of-line
  blocking and has its own flow control. Worth testing with QUICHE or nginx's
  experimental HTTP/3 support.

- **Browser as client**: All current benchmarks use Go or Rust clients. Does
  a browser's HTTP/2 stack (Chromium's, Firefox's) perform differently as the
  receiving end? This matters because the real-world ndt8 client will be
  JavaScript running in a browser. Note that both Chromium and Firefox separate
  the network stack (Chromium's "network service," Firefox's "necko") from the
  renderer process where JavaScript runs. Data must cross an IPC boundary (Mojo
  in Chromium) to reach JavaScript. It would be important to understand whether
  this IPC hop is itself a throughput bottleneck, and how the browser's network
  stack alone performs when not limited by the renderer. This last point is
  probably best investigated in a separate, browser-focused effort.

## Single-Connection Constraint

Both HTTP/2 ([RFC 9113 section 9.1](https://www.rfc-editor.org/rfc/rfc9113#section-9.1))
and HTTP/3 ([RFC 9114](https://www.rfc-editor.org/rfc/rfc9114)) recommend
against opening more than one connection to the same host. RFC 9113 uses
"SHOULD NOT" (a strong recommendation in RFC 2119 language).

This has architectural implications for network measurement:

- **Parallel measurements**: Running multiple concurrent TCP-based tests to the
  same server (e.g., as MSAK *could* do) conflicts with this recommendation.
  With HTTP/2, multiple requests become streams multiplexed over the same
  connection, sharing flow control budget.

- **Responsiveness probes**: Sending lightweight probe requests to measure RTT
  *during* a bulk data transfer is harder when both share the same connection —
  the probe competes with data frames for flow control window space.

- **Flow control defaults**: HTTP/2's default initial flow control window is
  only 65,535 bytes (64 KB). Browsers increase this via WINDOW_UPDATE, but how
  aggressively they scale varies across implementations. These defaults, combined
  with the single-connection constraint, may be a fundamental throughput
  bottleneck in both HTTP/2 and HTTP/3 for high-bandwidth measurement scenarios.

## Cleanup

```bash
./lxs destroy
```

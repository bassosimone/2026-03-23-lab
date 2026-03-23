# Censorship Lab

A self-contained simulation of a tiny subset of the internet,
built to explore how internet censorship works and how to detect it.

Everything runs locally in your browser and a Go server.
No real network traffic leaves your machine.

## Architecture

The Go server creates a simulated internet powered by
[gVisor](https://gvisor.dev/), a userspace network stack. The
simulation contains a client, a router with a configurable DPI
(Deep Packet Inspection) engine, DNS servers, and HTTP servers.

A browser-based SPA provides:

- **Network** — topology map with packet replay, directional
  arrows, and sonification.
- **Console** — run `curl`, `dig`, and `host` inside the
  simulated network.
- **Packets** — inspect every packet that crossed the network,
  filter by perspective, view headers, or download as PCAP.
- **Censorship** — switch DPI presets to simulate DNS spoofing,
  IP blocking, RST injection, or throttling.
- **Focus** — interactive presentation guide.

## Building

```sh
go build .
```

## Running

```sh
./2026-03-23-lab serve -b
```

The `-b` flag opens the lab in your default browser.

## History

The router, DPI engine, and packet dissection code in `internal/vis/`
are adapted from [ooni/netem](https://github.com/ooni/netem). The TLS
ClientHello dissector in `internal/dissector/` is also derived from
the same codebase.

## License

```
SPDX-License-Identifier: GPL-3.0-or-later
```

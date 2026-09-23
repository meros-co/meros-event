# Meros Event (Go)

The shared Go emitter for the **Meros event envelope (v1)** — every Meros Go product
(Sluice first) uses it to emit events to a collector (a local Imperio, the Meros
cloud, both, or neither). Stdlib-only, dependency-free.

- **Module:** `github.com/meros-co/meros-event`
- **Package identifier:** `merosevent` (Go identifiers can't contain a hyphen, so the
  import path is `meros-event` but the package you reference is `merosevent`)

> **Status:** this directory is the source for the **public** `meros-co/meros-event`
> repository, which is being created (public because open-source products import it —
> no auth needed). Until it is published + tagged, vendor these files; after that,
> `go get github.com/meros-co/meros-event@<tag>`.

## Use

```go
import merosevent "github.com/meros-co/meros-event"

e := merosevent.New("sluice", "0.6.0", "e7c1a9f2",
    merosevent.WithCollectors("https://meros.co"),
    merosevent.WithToken("mst_…"),          // a site ingest token
    merosevent.WithEdition("appliance"),
)

e.Emit("sluice.route.media_lost", merosevent.Error,
    merosevent.WithSubject("route", "snd-3f2a>rcv-91c0", "Pulpit Mic → Lobby Amp"),
    merosevent.WithAttrs(map[string]any{"rtp_seen": false}),
    merosevent.WithSeq(10428),
)
```

- `Envelope(...)` builds a conformant envelope (pure — hand it to your own transport
  if you prefer). `Emit(...)` builds and fire-and-forgets over HTTP.
- **Never breaks your hot path:** a down/slow/misconfigured collector is swallowed;
  **zero collectors is a valid no-op.**
- Optional fields (`seq`, `subject`, `actor`, `attrs`, `trace`) are omitted when unset.

## Contract

The envelope this produces validates against the Meros event envelope JSON Schema
(`spec/schema/event-envelope-1.json` in the meros repo) — the conformance authority.
Keep this module's version in step with the envelope version it targets (currently
`envelope: 1`).

## Test

```sh
go test ./...
```

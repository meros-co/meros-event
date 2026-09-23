# Meros Event (Go)

The Go emitter for the **Meros event envelope (v1)** — Meros Go products use it to
emit events to a collector (a local Imperio, the Meros cloud, both, or neither).
Stdlib-only, dependency-free.

- **Module:** `github.com/meros-co/meros-event`
- **Package:** `merosevent` (Go identifiers can't contain a hyphen, so the import path
  is `meros-event` but the package you reference is `merosevent`)

## Install

```sh
go get github.com/meros-co/meros-event
```

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

## Envelope contract

Each event is a flat JSON object with a fixed set of keys — `envelope`, `id` (ULID),
`occurred_at`, `source{product,version,instance,…}`, `type` (`product.subject.verb`),
`severity`, and optional `seq`/`subject`/`actor`/`attrs`/`trace`. This module
implements version `1`; keep it in step with the envelope version you target.

## Test

```sh
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).

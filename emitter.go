// Package merosevent is the Meros event emitter for the Meros event envelope (v1) —
// the shared Go module every Meros Go product uses to emit events. Published as
// github.com/meros-co/meros-event (the package identifier is merosevent because Go
// identifiers can't contain a hyphen). Stdlib-only. It does two things:
//
//  1. Envelope() builds a conformant v1 envelope (pure) — generating the ULID id and
//     stamping occurred_at in UTC millisecond RFC3339.
//  2. Emit() fire-and-forgets that envelope to every configured collector over HTTP.
//     It never returns an error into your product's hot path: a collector being
//     down, slow, or misconfigured must not break the product. Zero collectors is a
//     valid configuration and is a silent no-op.
//
// The envelope this produces validates against the Meros event envelope schema
// (spec/schema/event-envelope-1.json in the meros repo). See docs/event-envelope.md.
package merosevent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Severity levels (the schema's enum).
const (
	Debug    = "debug"
	Info     = "info"
	Notice   = "notice"
	Warning  = "warning"
	Error    = "error"
	Critical = "critical"
)

// Source identifies the emitter.
type Source struct {
	Product  string `json:"product"`
	Version  string `json:"version"`
	Instance string `json:"instance"`
	Site     string `json:"site,omitempty"`
	Edition  string `json:"edition,omitempty"`
}

// Subject is what an event is about (optional).
type Subject struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Actor is who/what caused an event (optional). Kind is user|system|device|external.
type Actor struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Envelope is one Meros event. seq/subject/actor/attrs/trace are optional and
// omitted when empty; everything else is required by the schema.
type Envelope struct {
	Envelope   int            `json:"envelope"`
	ID         string         `json:"id"`
	OccurredAt string         `json:"occurred_at"`
	Seq        uint64         `json:"seq,omitempty"`
	Source     Source         `json:"source"`
	Type       string         `json:"type"`
	Severity   string         `json:"severity"`
	Subject    *Subject       `json:"subject,omitempty"`
	Actor      *Actor         `json:"actor,omitempty"`
	Attrs      map[string]any `json:"attrs,omitempty"`
	Trace      string         `json:"trace,omitempty"`
}

// EventOption decorates an event with optional fields.
type EventOption func(*Envelope)

// WithSubject sets the subject.
func WithSubject(kind, id, name string) EventOption {
	return func(e *Envelope) { e.Subject = &Subject{Kind: kind, ID: id, Name: name} }
}

// WithActor sets the actor.
func WithActor(kind, id, name string) EventOption {
	return func(e *Envelope) { e.Actor = &Actor{Kind: kind, ID: id, Name: name} }
}

// WithAttrs sets the product-specific body.
func WithAttrs(attrs map[string]any) EventOption {
	return func(e *Envelope) { e.Attrs = attrs }
}

// WithSeq sets the monotonic per-instance sequence.
func WithSeq(seq uint64) EventOption {
	return func(e *Envelope) { e.Seq = seq }
}

// WithTrace sets the correlation id.
func WithTrace(trace string) EventOption {
	return func(e *Envelope) { e.Trace = trace }
}

// Emitter builds and sends envelopes for one source.
type Emitter struct {
	source     Source
	collectors []string
	token      string
	client     *http.Client
}

// Option configures an Emitter.
type Option func(*Emitter)

// WithCollectors sets zero or more collector base URLs. Zero is valid (no-op).
func WithCollectors(urls ...string) Option {
	return func(e *Emitter) {
		for _, u := range urls {
			if u != "" {
				e.collectors = append(e.collectors, u)
			}
		}
	}
}

// WithToken sets the bearer token presented to the collector.
func WithToken(token string) Option { return func(e *Emitter) { e.token = token } }

// WithSite sets the optional local operator site label.
func WithSite(site string) Option { return func(e *Emitter) { e.source.Site = site } }

// WithEdition sets the optional build tier (desktop|server|appliance).
func WithEdition(edition string) Option { return func(e *Emitter) { e.source.Edition = edition } }

// WithHTTPClient overrides the HTTP client (e.g. a shorter timeout).
func WithHTTPClient(c *http.Client) Option { return func(e *Emitter) { e.client = c } }

// New builds an Emitter for a product (slug), its version, and a stable per-install
// instance id.
func New(product, version, instance string, opts ...Option) *Emitter {
	e := &Emitter{
		source: Source{Product: product, Version: version, Instance: instance},
		client: &http.Client{Timeout: 1500 * time.Millisecond},
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Envelope builds a conformant v1 envelope. Pure — no network.
func (e *Emitter) Envelope(typ, severity string, opts ...EventOption) Envelope {
	env := Envelope{
		Envelope:   1,
		ID:         ulid(),
		OccurredAt: now(),
		Source:     e.source,
		Type:       typ,
		Severity:   severity,
	}
	for _, o := range opts {
		o(&env)
	}
	return env
}

// Emit builds and fire-and-forgets an event to every configured collector, and
// returns the envelope that was sent. Never returns an error; delivery failures are
// swallowed by design. Synchronous but bounded by the client timeout; wrap in `go`
// to move it fully off the hot path.
func (e *Emitter) Emit(typ, severity string, opts ...EventOption) Envelope {
	env := e.Envelope(typ, severity, opts...)
	e.send(env)
	return env
}

func (e *Emitter) send(env Envelope) {
	if len(e.collectors) == 0 {
		return
	}
	body, err := json.Marshal(env)
	if err != nil {
		return
	}
	for _, base := range e.collectors {
		e.post(strings.TrimRight(base, "/")+"/v1/events", body)
	}
}

func (e *Emitter) post(url string, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), e.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// now returns UTC, millisecond precision, trailing Z — matches occurred_at.
func now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"
}

// ulid returns a Crockford base32 ULID: 48 bits of ms time + 80 bits of randomness.
func ulid() string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var b [26]byte

	t := uint64(time.Now().UnixMilli())
	for i := 9; i >= 0; i-- {
		b[i] = alphabet[t%32]
		t /= 32
	}

	var r [16]byte
	_, _ = rand.Read(r[:])
	for i := 0; i < 16; i++ {
		b[10+i] = alphabet[int(r[i])%32]
	}

	return string(b[:])
}

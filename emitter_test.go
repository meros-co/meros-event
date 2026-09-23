package merosevent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"
)

var (
	ulidRe = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
	tsRe   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)
)

func TestEnvelopeShape(t *testing.T) {
	e := New("sluice", "0.6.0", "e7c1a9f2", WithSite("auditorium"), WithEdition("desktop"))
	env := e.Envelope("sluice.route.confirmed", Info,
		WithSeq(10427),
		WithSubject("route", "snd-3f2a>rcv-91c0", "Pulpit Mic → Lobby Amp"),
		WithActor("user", "u_4412", "David"),
		WithAttrs(map[string]any{"state": "confirmed"}),
		WithTrace("recall-88"),
	)

	if env.Envelope != 1 {
		t.Fatalf("envelope = %d, want 1", env.Envelope)
	}
	if !ulidRe.MatchString(env.ID) {
		t.Errorf("id %q is not a ULID", env.ID)
	}
	if !tsRe.MatchString(env.OccurredAt) {
		t.Errorf("occurred_at %q is not UTC ms RFC3339", env.OccurredAt)
	}
	if env.Source.Product != "sluice" || env.Source.Instance != "e7c1a9f2" || env.Source.Edition != "desktop" {
		t.Errorf("source wrong: %+v", env.Source)
	}
	if env.Seq != 10427 || env.Subject == nil || env.Actor == nil {
		t.Errorf("optional fields not set: %+v", env)
	}
}

func TestMinimalEventOmitsOptionalKeys(t *testing.T) {
	e := New("imperio", "2.0.0", "site-hq")
	env := e.Envelope("imperio.update.applied", Info)

	b, _ := json.Marshal(env)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"seq", "subject", "actor", "attrs", "trace"} {
		if _, ok := m[k]; ok {
			t.Errorf("%q should be omitted when unset", k)
		}
	}
	// The source object must never omit its required fields.
	src := m["source"].(map[string]any)
	for _, k := range []string{"product", "version", "instance"} {
		if _, ok := src[k]; !ok {
			t.Errorf("source.%s missing", k)
		}
	}
}

func TestIDsAreUnique(t *testing.T) {
	e := New("sluice", "0.6.0", "e7c1a9f2")
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := e.Envelope("sluice.device.offline", Warning).ID
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestEmitPostsToCollector(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.URL.Path != "/v1/events" {
			t.Errorf("path = %q, want /v1/events", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sekret" {
			t.Errorf("missing bearer token")
		}
		var env Envelope
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			t.Errorf("body did not decode: %v", err)
		}
		if env.Type != "rfdeck.link.dropout" {
			t.Errorf("type = %q", env.Type)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	e := New("rfdeck", "1.2.0", "rack-2", WithCollectors(srv.URL), WithToken("sekret"))
	e.Emit("rfdeck.link.dropout", Warning, WithSubject("transmitter", "tx-7", ""))

	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("collector hit %d times, want 1", hits)
	}
}

func TestEmitWithZeroCollectorsIsNoop(t *testing.T) {
	e := New("basin", "0.3.1", "basin-mixer-a")
	env := e.Emit("basin.recording.stopped", Critical)
	if env.Type != "basin.recording.stopped" {
		t.Fatalf("type = %q", env.Type)
	}
}

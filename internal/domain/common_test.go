package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeJSON(t *testing.T) {
	ts := Time{time.Date(2024, 5, 1, 10, 20, 30, 123e6, time.UTC)}
	b, err := json.Marshal(ts)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `"2024-05-01T10:20:30.123+00:00"`; got != want {
		t.Fatalf("marshal = %s, want %s", got, want)
	}
	for _, in := range []string{`"2024-05-01T10:20:30.123+00:00"`, `"2024-05-01T12:20:30.123+02:00"`, `1714558830123`} {
		var got Time
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("unmarshal %s: %v", in, err)
		}
		if !got.Equal(ts.Time) {
			t.Fatalf("unmarshal %s = %v, want %v", in, got, ts)
		}
	}
}

func TestNewID(t *testing.T) {
	id := NewID()
	if len(id) != 6 {
		t.Fatalf("len(%q) = %d, want 6", id, len(id))
	}
}

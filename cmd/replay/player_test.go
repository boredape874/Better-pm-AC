package replay

import (
	"path/filepath"
	"testing"
)

func TestPlayer_Counts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "round_trip.bpac")

	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	// Write 5 entries: 3 client (dir=0), 2 server (dir=1).
	writes := []struct {
		dir  uint8
		data []byte
	}{
		{0, []byte("client-pkt-1")},
		{1, []byte("server-pkt-1")},
		{0, []byte("client-pkt-2")},
		{0, []byte("client-pkt-3")},
		{1, []byte("server-pkt-2")},
	}
	for _, w := range writes {
		if err := rec.Record(w.dir, w.data); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pl := NewPlayer(path)
	counts, err := pl.Play()
	if err != nil {
		t.Fatalf("Play: %v", err)
	}

	if counts["total"] != 5 {
		t.Errorf("total: got %d want 5", counts["total"])
	}
	if counts["client"] != 3 {
		t.Errorf("client: got %d want 3", counts["client"])
	}
	if counts["server"] != 2 {
		t.Errorf("server: got %d want 2", counts["server"])
	}
}

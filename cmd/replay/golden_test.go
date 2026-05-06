package replay

import (
	"path/filepath"
	"testing"
)

func TestGoldenFixtures(t *testing.T) {
	dir := t.TempDir()

	fixtures := []struct {
		name    string
		count   int
		dir     uint8
		payload []byte
	}{
		{"idle.bpac", 10, 0, []byte("idle-tick")},
		{"walk.bpac", 10, 0, []byte("walk-tick")},
		{"sprint_jump.bpac", 10, 0, []byte("sprint-tick")},
		{"combat.bpac", 5, 0, []byte("attack")},
		{"fall.bpac", 10, 0, []byte("fall-tick")},
	}

	// Write all synthetic fixture files.
	for _, fx := range fixtures {
		path := filepath.Join(dir, fx.name)
		rec, err := NewRecorder(path)
		if err != nil {
			t.Fatalf("%s: NewRecorder: %v", fx.name, err)
		}
		for i := 0; i < fx.count; i++ {
			if err := rec.Record(fx.dir, fx.payload); err != nil {
				rec.Close()
				t.Fatalf("%s: Record[%d]: %v", fx.name, i, err)
			}
		}
		if err := rec.Close(); err != nil {
			t.Fatalf("%s: Close: %v", fx.name, err)
		}
	}

	// Read each back with Player and verify counts.
	for _, fx := range fixtures {
		path := filepath.Join(dir, fx.name)
		pl := NewPlayer(path)
		counts, err := pl.Play()
		if err != nil {
			t.Fatalf("%s: Play: %v", fx.name, err)
		}
		if counts["total"] != fx.count {
			t.Errorf("%s: total got %d want %d", fx.name, counts["total"], fx.count)
		}
	}
}

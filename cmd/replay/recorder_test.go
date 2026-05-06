package replay

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestRecorder_Format(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bpac")

	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	entries := []struct {
		dir  uint8
		data []byte
	}{
		{0, []byte("hello")},
		{1, []byte("world!")},
		{0, []byte("third-entry-data")},
	}

	for _, e := range entries {
		if err := rec.Record(e.dir, e.data); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-read raw bytes and verify format.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	offset := 0
	for i, e := range entries {
		if offset+13+len(e.data) > len(raw) {
			t.Fatalf("entry %d: not enough bytes (offset=%d, total=%d)", i, offset, len(raw))
		}
		// 8-byte timestamp (just verify it's non-zero)
		ts := binary.LittleEndian.Uint64(raw[offset : offset+8])
		if ts == 0 {
			t.Errorf("entry %d: timestamp is zero", i)
		}
		offset += 8

		// 1-byte dir
		if raw[offset] != e.dir {
			t.Errorf("entry %d: dir got %d want %d", i, raw[offset], e.dir)
		}
		offset++

		// 4-byte length
		length := binary.LittleEndian.Uint32(raw[offset : offset+4])
		if int(length) != len(e.data) {
			t.Errorf("entry %d: length got %d want %d", i, length, len(e.data))
		}
		offset += 4

		// data
		got := string(raw[offset : offset+int(length)])
		want := string(e.data)
		if got != want {
			t.Errorf("entry %d: data got %q want %q", i, got, want)
		}
		offset += int(length)
	}

	if offset != len(raw) {
		t.Errorf("trailing bytes: offset=%d total=%d", offset, len(raw))
	}
}

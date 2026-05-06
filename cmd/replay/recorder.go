package replay

import (
	"encoding/binary"
	"os"
	"time"
)

// RecordEntry is one recorded packet frame.
type RecordEntry struct {
	TimestampNs int64
	Dir         uint8 // 0=client→server, 1=server→client
	Data        []byte
}

// Recorder writes RecordEntries to a .bpac file.
type Recorder struct {
	f *os.File
}

func NewRecorder(path string) (*Recorder, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Recorder{f: f}, nil
}

// Record writes one entry: 8-byte ts, 1-byte dir, 4-byte len, data
func (r *Recorder) Record(dir uint8, data []byte) error {
	buf := make([]byte, 13+len(data))
	binary.LittleEndian.PutUint64(buf[0:8], uint64(time.Now().UnixNano()))
	buf[8] = dir
	binary.LittleEndian.PutUint32(buf[9:13], uint32(len(data)))
	copy(buf[13:], data)
	_, err := r.f.Write(buf)
	return err
}

func (r *Recorder) Close() error { return r.f.Close() }

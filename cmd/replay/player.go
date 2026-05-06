package replay

import (
	"encoding/binary"
	"io"
	"os"
)

// Player reads a .bpac file produced by Recorder.
type Player struct{ path string }

func NewPlayer(path string) *Player { return &Player{path: path} }

// Play reads all entries and returns a map of counts by dir (0 or 1).
func (p *Player) Play() (map[string]int, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	counts := map[string]int{"client": 0, "server": 0, "total": 0}
	for {
		var ts uint64
		var dir uint8
		var length uint32
		if err := binary.Read(f, binary.LittleEndian, &ts); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		binary.Read(f, binary.LittleEndian, &dir)
		binary.Read(f, binary.LittleEndian, &length)
		data := make([]byte, length)
		io.ReadFull(f, data)
		_ = ts
		_ = data
		if dir == 0 {
			counts["client"]++
		} else {
			counts["server"]++
		}
		counts["total"]++
	}
	return counts, nil
}

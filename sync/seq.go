package sync

import (
	"crypto/sha256"
	"io"
	"os"
)

type SeqProducer struct {
	buf [S64k]byte
}

func (p *SeqProducer) Start(cnt int) int {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		return -1
	}
	defer f.Close()
	res := 0
	for range cnt {
		if _, err := io.ReadFull(f, p.buf[:]); err != nil {
			return -1
		}
		sum := sha256.Sum256(p.buf[:])
		for i := range len(sum) {
			res += int(sum[i])
		}
	}
	return res
}

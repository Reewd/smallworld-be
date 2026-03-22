package foundation

import (
	"fmt"
	"sync/atomic"
	"time"
)

type IDGenerator interface {
	New(prefix string) string
}

type AtomicIDGenerator struct {
	seq atomic.Uint64
}

func (g *AtomicIDGenerator) New(prefix string) string {
	value := g.seq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), value)
}

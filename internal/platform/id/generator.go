package id

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync/atomic"
	"time"
)

type Generator struct {
	prefix  string
	counter atomic.Uint64
}

func New(prefix string) *Generator { return &Generator{prefix: prefix} }

func (g *Generator) NewID() string {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err == nil {
		return g.prefix + "_" + hex.EncodeToString(random)
	}
	return g.prefix + "_" + time.Now().UTC().Format("20060102150405") + "_" + strconv.FormatUint(g.counter.Add(1), 10)
}

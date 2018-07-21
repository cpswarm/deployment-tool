// Package buffer implements a first-in-first-out (FIFO) fixed-capacity list
package buffer

import (
	"sync"
)

func NewBuffer(capacity uint8) *buffer {
	return &buffer{
		capacity: capacity,
	}
}

type buffer struct {
	sync.RWMutex
	list     []string
	capacity uint8
	index    uint8
}

func (b *buffer) Insert(line string) {
	b.RLock()
	defer b.RUnlock()

	if uint8(len(b.list)) < b.capacity { // buffer expanding
		b.list = append(b.list, line)
	} else { // buffer full
		if b.index == uint8(len(b.list)) {
			b.index = 0
		}
		b.list[b.index] = line
		b.index++
	}
}

func (b *buffer) Collect() []string {
	b.Lock()
	defer b.Unlock()

	return append(b.list[b.index:], b.list[:b.index]...)
}

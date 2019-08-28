// Package buffer implements a first-in-first-out (FIFO) fixed-capacity list
package buffer

import (
	"sync"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

func NewBuffer(capacity uint8) Buffer {
	return Buffer{
		capacity: capacity,
	}
}

type Buffer struct {
	mutex    sync.RWMutex
	list     []model.Log
	capacity uint8
	index    uint8
}

func (b *Buffer) Insert(line model.Log) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

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

func (b *Buffer) Size() uint8 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return uint8(len(b.list))
}

func (b *Buffer) Collect() []model.Log {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return append(b.list[b.index:], b.list[:b.index]...)
}

func (b *Buffer) Flush() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.list = nil
	b.index = 0
}

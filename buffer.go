package main

import (
	"fmt"
	"go.uber.org/zap"
)

type logBuffer struct {
	size int
	pointer int
	capacity int
	buffer []logEntry
	logger *zap.SugaredLogger
}

func newLogBuffer(log *zap.SugaredLogger, size int) *logBuffer {
	log.Debugf("Initializing ring buffer of size %d", size)
	return &logBuffer{
		size: size,
		pointer: 0,
		capacity: 0,
		buffer: make([]logEntry, size, size),
		logger: log,
	}
}

func (lb *logBuffer) Append(entry logEntry) {
	// update pointer to oldest entry
	lb.logger.Debugf("Appending into index: %d", lb.pointer)
	lb.buffer[lb.pointer] = entry
	lb.pointer = (lb.pointer+1) % lb.size
	// TODO: Limit capacity?
	lb.capacity = lb.capacity+1
	lb.logger.Debugf("New pointer: %d", lb.pointer)
	lb.logger.Debugf("New capacity: %d", lb.capacity)
}

// TODO: Limit it to only X tokens
func (lb logBuffer) Dump() []logEntry {
	lb.logger.Debugf("Dump capacity: %d", lb.capacity)
	if lb.capacity > lb.size {
		// loop around entire slice from here
		composeSlice := append(lb.buffer[lb.pointer:], lb.buffer[0:lb.pointer]...)
		lb.logger.Debugf("Dump (Max capacity): %s\n", stringifyLogs(composeSlice))
		return composeSlice
	}
	// TODO: Avoid special case
	if lb.pointer == 0 && lb.capacity > 0 {
		// Buffer is full and pointer wrapped around
		lb.logger.Debugf("Dump: %s\n", stringifyLogs(lb.buffer))
		return lb.buffer
	}
	lb.logger.Debugf("Dump: %s\n", stringifyLogs(lb.buffer[0:lb.pointer]))
	return lb.buffer[0:lb.pointer]
}

func (lb logBuffer) String() string {
	return fmt.Sprintf("%v", lb.Dump())
}

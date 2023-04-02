package main

import (
	"fmt"
)

type logBuffer struct {
	size int
	pointer int
	capacity int
	buffer []logEntry
}

func newLogBuffer(size int) *logBuffer {
	return &logBuffer{
		size: size,
		pointer: 0,
		capacity: 0,
		buffer: make([]logEntry, size, size),
	}
}

func (lb *logBuffer) Append(entry logEntry) {
	// update pointer to oldest entry
	lb.buffer[lb.pointer] = entry
	lb.pointer = (lb.pointer+1) % lb.size
	if lb.capacity < lb.size {
		lb.capacity = lb.capacity+1
	}
}

func (lb logBuffer) Dump() []logEntry {
	if lb.capacity == lb.size {
		// loop around entire slice from here
		composeSlice := append(lb.buffer[lb.pointer:], lb.buffer[0:lb.size-lb.pointer]...)
		fmt.Printf("DEBUG: DUMP (COMPOSED): %s\n", stringifyLogs(composeSlice))
		return composeSlice
	}
	fmt.Printf("DEBUG: DUMP: %s\n", stringifyLogs(lb.buffer[0:lb.pointer]))
	return lb.buffer[0:lb.pointer]
}

func (lb logBuffer) String() string {
	return fmt.Sprintf("%v", lb.Dump())
}

func reverse(s []logEntry) []logEntry {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}	
	return s
}

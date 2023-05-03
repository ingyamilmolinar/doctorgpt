package buffer

import (
	"fmt"
	"go.uber.org/zap"

	"github.com/ingyamilmolinar/doctorgpt/agent/internal/parser"
)

type LogBuffer struct {
	size      int
	maxTokens int
	pointer   int
	capacity  int
	buffer    []parser.LogEntry
	logger    *zap.SugaredLogger
}

func NewLogBuffer(log *zap.SugaredLogger, size, maxTokens int) *LogBuffer {
	log.Debugf("Initializing ring buffer of size %d and max tokens %d", size, maxTokens)
	return &LogBuffer{
		size:      size,
		maxTokens: maxTokens,
		pointer:   0,
		capacity:  0,
		buffer:    make([]parser.LogEntry, size, size),
		logger:    log,
	}
}

func (lb *LogBuffer) Append(entry parser.LogEntry) {
	// update pointer to oldest entry
	lb.logger.Debugf("Appending into index: %d", lb.pointer)
	lb.buffer[lb.pointer] = entry
	lb.pointer = (lb.pointer + 1) % lb.size
	// TODO: It is weird that capacity can be > size
	if lb.capacity <= lb.size {
		lb.capacity = lb.capacity + 1
	}
	lb.logger.Debugf("New pointer: %d", lb.pointer)
	lb.logger.Debugf("New capacity: %d", lb.capacity)
}

func (lb LogBuffer) Dump() []parser.LogEntry {
	lb.logger.Debugf("Dump capacity: %d", lb.capacity)
	if lb.capacity > lb.size {
		// loop around entire slice from here
		composeSlice := append(lb.buffer[lb.pointer:], lb.buffer[0:lb.pointer]...)
		trimmedSlice := trimSlice(lb.logger, composeSlice, lb.maxTokens)
		lb.logger.Debugf("Dump (Max capacity): %s", parser.Stringify(trimmedSlice))
		return trimmedSlice
	}
	// TODO: Avoid special case
	if lb.pointer == 0 && lb.capacity > 0 {
		// Buffer is full and pointer wrapped around
		trimmedSlice := trimSlice(lb.logger, lb.buffer, lb.maxTokens)
		lb.logger.Debugf("Dump: %s", parser.Stringify(trimmedSlice))
		return trimmedSlice
	}
	trimmedSlice := trimSlice(lb.logger, lb.buffer[0:lb.pointer], lb.maxTokens)
	lb.logger.Debugf("Dump: %s", parser.Stringify(trimmedSlice))
	return trimmedSlice
}

func (lb *LogBuffer) Clear() {
	lb.pointer = 0
	lb.capacity = 0
	lb.buffer = make([]parser.LogEntry, lb.size, lb.size)
}

func (lb LogBuffer) String() string {
	return fmt.Sprintf("%v", lb.Dump())
}

func trimSlice(log *zap.SugaredLogger, entries []parser.LogEntry, maxTokens int) []parser.LogEntry {
	tokens := 0
	// Go from most recent logs into oldest logs
	var i int
	for i = len(entries) - 1; i >= 0; i-- {
		logEntry := entries[i]
		tokens += getTokens(logEntry.Text)
		if tokens > maxTokens {
			// Ignore the rest of the older entries
			log.Debugf("Skipping oldest lines including: (%s)", logEntry.Text)
			break
		}
		log.Debugf("Including (%s)", logEntry.Text)
		log.Debugf("Tokens so far: %d, Max tokens: %d", tokens, maxTokens)
	}
	return entries[i+1:]
}

// https://help.openai.com/en/articles/4936856-what-are-tokens-and-how-to-count-them
func getTokens(s string) int {
	return len(s) / 4
}

package buffer

import (
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/ingyamilmolinar/doctorgpt/agent/internal/parser"
	"go.uber.org/zap"
)

var logger, _ = zap.NewDevelopment()

func TestBuffer(t *testing.T) {
	// Dump() will only return the latest 30 characters
	buffer := NewLogBuffer(logger.Sugar(), 3, 30/4)
	entry1 := parser.LogEntry{
		Text:   "0123456789", // 10 chars
		LineNo: 1,
	}
	entries := []parser.LogEntry{
		entry1,
	}
	buffer.Append(entry1)
	require.Equal(t, entries, buffer.Dump())
	entry2 := parser.LogEntry{
		Text:   "abcdefghij", // 10 chars
		LineNo: 2,
	}
	buffer.Append(entry2)
	entries = append(entries, entry2)
	require.Equal(t, entries, buffer.Dump())
	entry3 := parser.LogEntry{
		Text:   "klmnopqrst", // 10 chars
		LineNo: 3,
	}
	buffer.Append(entry3)
	entries = append(entries, entry3)
	require.Equal(t, entries, buffer.Dump())
	entry4 := parser.LogEntry{
		Text:   "uvwxyz-./;", // 10 chars
		LineNo: 4,
	}
	buffer.Append(entry4)
	// It circled around
	expectedBuffer := []parser.LogEntry{
		entry4, // <- replaced first entry
		entry2,
		entry3,
	}
	entries = append(entries, entry4)
	require.Equal(t, expectedBuffer, buffer.buffer)
	require.Equal(t, entries[1:], buffer.Dump())
	entry5 := parser.LogEntry{
		Text:   "abcdefghijklmnopqrst", // 20 chars
		LineNo: 5,
	}
	buffer.Append(entry5)
	// It circled around
	expectedBuffer = []parser.LogEntry{
		entry4,
		entry5, // <- replaced second entry
		entry3,
	}
	entries = append(entries, entry5)
	require.Equal(t, expectedBuffer, buffer.buffer)
	// Dump should skip entry3 since entry4 + entry5 == 30 chars
	require.Equal(t, entries[3:], buffer.Dump())

	buffer.Clear()
	require.Equal(t, make([]parser.LogEntry, 3, 3), buffer.buffer)
	require.Equal(t, 0, buffer.pointer)
	require.Equal(t, 0, buffer.capacity)
}

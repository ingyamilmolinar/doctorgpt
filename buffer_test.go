package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBuffer(t *testing.T) {
	// Dump() will only return the latest 30 characters
	buffer := newLogBuffer(logger.Sugar(), 3, 30/4)
	entry1 := logEntry{
		Text:   "0123456789", // 10 chars
		LineNo: 1,
	}
	entries := []logEntry{
		entry1,
	}
	buffer.Append(entry1)
	require.Equal(t, entries, buffer.Dump())
	entry2 := logEntry{
		Text:   "abcdefghij", // 10 chars
		LineNo: 2,
	}
	buffer.Append(entry2)
	entries = append(entries, entry2)
	require.Equal(t, entries, buffer.Dump())
	entry3 := logEntry{
		Text:   "klmnopqrst", // 10 chars
		LineNo: 3,
	}
	buffer.Append(entry3)
	entries = append(entries, entry3)
	require.Equal(t, entries, buffer.Dump())
	entry4 := logEntry{
		Text:   "uvwxyz-./;", // 10 chars
		LineNo: 4,
	}
	buffer.Append(entry4)
	// It circled around
	expectedBuffer := []logEntry{
		entry4, // <- replaced first entry
		entry2,
		entry3,
	}
	entries = append(entries, entry4)
	require.Equal(t, expectedBuffer, buffer.buffer)
	require.Equal(t, entries[1:], buffer.Dump())
	entry5 := logEntry{
		Text:   "abcdefghijklmnopqrst", // 20 chars
		LineNo: 5,
	}
	buffer.Append(entry5)
	// It circled around
	expectedBuffer = []logEntry{
		entry4,
		entry5, // <- replaced second entry
		entry3,
	}
	entries = append(entries, entry5)
	require.Equal(t, expectedBuffer, buffer.buffer)
	// Dump should skip entry3 since entry4 + entry5 == 30 chars
	require.Equal(t, entries[3:], buffer.Dump())

	buffer.Clear()
	require.Equal(t, make([]logEntry, 3, 3), buffer.buffer)
	require.Equal(t, 0, buffer.pointer)
	require.Equal(t, 0, buffer.capacity)
}

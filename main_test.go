package main

import (
	"github.com/hpcloud/tail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sync"
	"testing"
	"time"
)

var logger, _ = zap.NewDevelopment()

var nodeLogParser, _ = newParser(logger.Sugar(), "^\\[(?P<LEVEL>\\w+)\\]\\s+(?P<MESSAGE>.*)$", []variableMatcher{}, []variableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
})

var allLineParser, _ = newParser(logger.Sugar(), "^(?P<MESSAGE>.*)$", []variableMatcher{}, []variableMatcher{})

type expectedEntry struct {
	logEntry      logEntry
	parserMatched int
}

var expectedEntries = []expectedEntry{
	{
		logEntry: logEntry{
			Parser:    &allLineParser,
			Triggered: false,
			LineNo:    1,
			Text:      "yarn run v1.22.19",
			Variables: map[string]string{
				"MESSAGE": "yarn run v1.22.19",
			},
		},
		parserMatched: 1,
	},
	{
		logEntry: logEntry{
			Parser:    &allLineParser,
			Triggered: false,
			LineNo:    2,
			Text:      "$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts",
			Variables: map[string]string{
				"MESSAGE": "$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts",
			},
		},
		parserMatched: 1,
	},
	{
		logEntry: logEntry{
			Parser:    &nodeLogParser,
			Triggered: false,
			LineNo:    3,
			Text:      "[INFO] 15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)",
			Variables: map[string]string{
				"LEVEL":   "INFO",
				"MESSAGE": "15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)",
			},
		},
		parserMatched: 0,
	},
	{
		logEntry: logEntry{
			Parser:    &nodeLogParser,
			Triggered: true,
			LineNo:    4,
			Text:      "[ERROR]  PrismaClientKnownRequestError:",
			Variables: map[string]string{
				"LEVEL":   "ERROR",
				"MESSAGE": "PrismaClientKnownRequestError:",
			},
		},
		parserMatched: 0,
	},
}

func TestParsers(t *testing.T) {
	parsers := []parser{
		nodeLogParser,
		allLineParser,
	}

	tailConfig := tail.Config{
		Follow:    false,
		MustExist: true,
	}

	f, err := tail.TailFile("testlogs/prisma.log", tailConfig)
	require.NoError(t, err)

	i := 0
	for line := range f.Lines {
		entry, parserMatched, err := parseLogEntry(logger.Sugar(), parsers, line.Text, i+1)
		assert.NoError(t, err)
		assert.Equal(t, expectedEntries[i].logEntry, entry)
		assert.Equal(t, expectedEntries[i].parserMatched, parserMatched)
		i++
	}
}

var dropboxParser, _ = newParser(logger.Sugar(), "^\\[(\\d{4}\\/\\d{6}\\.\\d{6}):(?P<LEVEL>\\w+):([\\w\\.\\_]+)\\(\\d+\\)\\]\\s+(?P<MESSAGE>.*)$", []variableMatcher{}, []variableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
})

func TestDropboxLogExample(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
		Parser:    &dropboxParser,
		Triggered: true,
		Text:      "[1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000",
		LineNo:    2,
		Variables: map[string]string{
			"LEVEL":   "ERROR",
			"MESSAGE": "Unable to move cache folder GPUCache to old_GPUCache_000",
		},
	}
	expectedContext := []logEntry{
		{
			Parser:    &dropboxParser,
			Triggered: false,
			Text:      "[1217/070353.692622:WARNING:dns_config_service_posix.cc(335)] Failed to read DnsConfig.",
			LineNo:    1,
			Variables: map[string]string{
				"LEVEL":   "WARNING",
				"MESSAGE": "Failed to read DnsConfig.",
			},
		},
		expectedEntry,
		{
			Parser:    &dropboxParser,
			Triggered: true,
			Text:      "[1217/201832.973523:ERROR:disk_cache.cc(184)] Unable to create cache",
			LineNo:    3,
			Variables: map[string]string{
				"LEVEL":   "ERROR",
				"MESSAGE": "Unable to create cache",
			},
		},
		{
			Parser:    &dropboxParser,
			Triggered: true,
			Text:      "[1217/201832.973606:ERROR:shader_disk_cache.cc(622)] Shader Cache Creation failed: -2",
			LineNo:    4,
			Variables: map[string]string{
				"LEVEL":   "ERROR",
				"MESSAGE": "Shader Cache Creation failed: -2",
			},
		},
	}
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		wg.Done()
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		monitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser{
			dropboxParser,
			allLineParser,
		}, handler, 100*time.Millisecond)
	}(t)
	// Wait until handler executes
	wg.Wait()
}

var dropboxParserWithFilters, _ = newParser(logger.Sugar(), "^\\[(\\d{4}\\/\\d{6}\\.\\d{6}):(?P<LEVEL>\\w+):([\\w\\.\\_]+)\\(\\d+\\)\\]\\s+(?P<MESSAGE>.*)$", []variableMatcher{
	{
		// We will skip the first error lines
		Variable: "MESSAGE",
		Regex:    "Unable",
	},
}, []variableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
})

func TestDropboxLogExampleWithFilters(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
		Parser:    &dropboxParserWithFilters,
		Filtered:  false,
		Triggered: true,
		Text:      "[1217/201832.973606:ERROR:shader_disk_cache.cc(622)] Shader Cache Creation failed: -2",
		LineNo:    4,
		Variables: map[string]string{
			"LEVEL":   "ERROR",
			"MESSAGE": "Shader Cache Creation failed: -2",
		},
	}
	expectedContext := []logEntry{
		{
			Parser:    &dropboxParserWithFilters,
			Filtered:  false,
			Triggered: false,
			Text:      "[1217/070353.692622:WARNING:dns_config_service_posix.cc(335)] Failed to read DnsConfig.",
			LineNo:    1,
			Variables: map[string]string{
				"LEVEL":   "WARNING",
				"MESSAGE": "Failed to read DnsConfig.",
			},
		},
		{
			Parser:    &dropboxParserWithFilters,
			Filtered:  true,
			Triggered: true,
			Text:      "[1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000",
			LineNo:    2,
			Variables: map[string]string{
				"LEVEL":   "ERROR",
				"MESSAGE": "Unable to move cache folder GPUCache to old_GPUCache_000",
			},
		},
		{
			Parser:    &dropboxParserWithFilters,
			Filtered:  true,
			Triggered: true,
			Text:      "[1217/201832.973523:ERROR:disk_cache.cc(184)] Unable to create cache",
			LineNo:    3,
			Variables: map[string]string{
				"LEVEL":   "ERROR",
				"MESSAGE": "Unable to create cache",
			},
		},
		expectedEntry,
	}
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		wg.Done()
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		monitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser{
			dropboxParserWithFilters,
			allLineParser,
		}, handler, 100*time.Millisecond)
	}(t)
	// Wait until handler executes
	wg.Wait()
}

// We do not match the hash in a variable on purpose
var photosParser, _ = newParser(logger.Sugar(), "^(?P<DATE>[^ ]+)\\s+(?P<TIME>[^ ]+)\\s+[^ ]+\\s+(?P<LEVEL>[^ ]+)\\s+(?P<PID>[^ ]+)\\s+(?P<PROCNAME>[^ ]+)\\s+(?P<FILEANDLINENO>[^ ]+)\\s+(?P<MESSAGE>.*)$", []variableMatcher{}, []variableMatcher{
	{
		Variable: "MESSAGE",
		Regex:    "error", // will match line 2
	},
	{
		Variable: "MESSAGE",
		Regex:    "Error:", // will  match lines 4-5
	},
})

func TestPhotosLogExampleMultipleMatchers(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
		Parser:    &photosParser,
		Filtered:  false,
		Triggered: true,
		Text:      "2022-01-27 21:37:36.776 0x2eb3     Default       511 photolibraryd: PLModelMigration.m:314   Creating sqlite error indicator file",
		LineNo:    2,
		Variables: map[string]string{
			"DATE":          "2022-01-27",
			"TIME":          "21:37:36.776",
			"LEVEL":         "Default",
			"PID":           "511",
			"PROCNAME":      "photolibraryd:",
			"FILEANDLINENO": "PLModelMigration.m:314",
			"MESSAGE":       "Creating sqlite error indicator file",
		},
	}
	expectedContext := []logEntry{
		{
			Parser:    &photosParser,
			Filtered:  false,
			Triggered: false,
			Text:      "2022-01-27 21:37:36.774 0x2eb3     Info          511 photolibraryd: PLModelMigration.m:290   Store has incompatible model version 14300, will attempt migration to current version 15331.",
			LineNo:    1,
			Variables: map[string]string{
				"DATE":          "2022-01-27",
				"TIME":          "21:37:36.774",
				"LEVEL":         "Info",
				"PID":           "511",
				"PROCNAME":      "photolibraryd:",
				"FILEANDLINENO": "PLModelMigration.m:290",
				"MESSAGE":       "Store has incompatible model version 14300, will attempt migration to current version 15331.",
			},
		},
		expectedEntry,
	}
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		wg.Done()
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		monitorLogLoop(logger.Sugar(), "testlogs/photos.log", "", "", "", 10, 8000, []parser{
			photosParser,
			allLineParser,
		}, handler, 100*time.Millisecond)
	}(t)
	// Wait until handler executes
	wg.Wait()
}

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
}

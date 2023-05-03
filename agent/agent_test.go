package main

import (
	"github.com/hpcloud/tail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sync"
	"testing"
	"time"

	"github.com/ingyamilmolinar/doctorgpt/agent/internal/common"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/config"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/parser"
)

var logger, _ = zap.NewDevelopment()

var nodeLogParser, _ = parser.NewParser(logger.Sugar(), "^\\[(?P<LEVEL>\\w+)\\]\\s+(?P<MESSAGE>.*)$", []config.VariableMatcher{}, []config.VariableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
}, []config.VariableMatcher{})

var allLineParser, _ = parser.NewParser(logger.Sugar(), "^(?P<MESSAGE>.*)$", []config.VariableMatcher{}, []config.VariableMatcher{}, []config.VariableMatcher{})

type expectedEntry struct {
	logEntry      parser.LogEntry
	parserMatched int
}

var expectedEntries = []expectedEntry{
	{
		logEntry: parser.LogEntry{
			Parser:    &allLineParser,
			Triggered: false,
			LineNo:    1,
			Text:      "yarn run v1.22.19",
			Variables: map[string]string{
				"LINENO":  "1",
				"MESSAGE": "yarn run v1.22.19",
			},
		},
		parserMatched: 1,
	},
	{
		logEntry: parser.LogEntry{
			Parser:    &allLineParser,
			Triggered: false,
			LineNo:    2,
			Text:      "$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts",
			Variables: map[string]string{
				"LINENO":  "2",
				"MESSAGE": "$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts",
			},
		},
		parserMatched: 1,
	},
	{
		logEntry: parser.LogEntry{
			Parser:    &nodeLogParser,
			Triggered: false,
			LineNo:    3,
			Text:      "[INFO] 15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)",
			Variables: map[string]string{
				"LINENO":  "3",
				"LEVEL":   "INFO",
				"MESSAGE": "15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)",
			},
		},
		parserMatched: 0,
	},
	{
		logEntry: parser.LogEntry{
			Parser:    &nodeLogParser,
			Triggered: true,
			LineNo:    4,
			Text:      "[ERROR]  PrismaClientKnownRequestError:",
			Variables: map[string]string{
				"LINENO":  "4",
				"LEVEL":   "ERROR",
				"MESSAGE": "PrismaClientKnownRequestError:",
			},
		},
		parserMatched: 0,
	},
}

func TestParsers(t *testing.T) {
	parsers := []parser.Parser{
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
		entry, parserMatched, err := parser.ParseLogEntry(logger.Sugar(), parsers, line.Text, i+1)
		assert.NoError(t, err)
		assert.Equal(t, expectedEntries[i].logEntry, entry)
		assert.Equal(t, expectedEntries[i].parserMatched, parserMatched)
		i++
	}
}

var dropboxParser, _ = parser.NewParser(logger.Sugar(), "^\\[(\\d{4}\\/\\d{6}\\.\\d{6}):(?P<LEVEL>\\w+):([\\w\\.\\_]+)\\(\\d+\\)\\]\\s+(?P<MESSAGE>.*)$", []config.VariableMatcher{}, []config.VariableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
}, []config.VariableMatcher{})

func TestDropboxLogExample(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := parser.LogEntry{
		Parser:    &dropboxParser,
		Triggered: true,
		Text:      "[1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000",
		LineNo:    2,
		Variables: map[string]string{
			"LINENO":  "2",
			"LEVEL":   "ERROR",
			"MESSAGE": "Unable to move cache folder GPUCache to old_GPUCache_000",
		},
	}
	expectedContext := []parser.LogEntry{
		{
			Parser:    &dropboxParser,
			Triggered: false,
			Text:      "[1217/070353.692622:WARNING:dns_config_service_posix.cc(335)] Failed to read DnsConfig.",
			LineNo:    1,
			Variables: map[string]string{
				"LINENO":  "1",
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
				"LINENO":  "3",
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
				"LINENO":  "4",
				"LEVEL":   "ERROR",
				"MESSAGE": "Shader Cache Creation failed: -2",
			},
		},
	}
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose parser.LogEntry, logContext []parser.LogEntry) error {
		defer wg.Done()
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		MonitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser.Parser{
			dropboxParser,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	common.WaitWithTimeout(t, &wg, 1*time.Second)
}

var dropboxParserWithFilters, _ = parser.NewParser(logger.Sugar(), "^\\[(\\d{4}\\/\\d{6}\\.\\d{6}):(?P<LEVEL>\\w+):([\\w\\.\\_]+)\\(\\d+\\)\\]\\s+(?P<MESSAGE>.*)$", []config.VariableMatcher{
	{
		// We will skip the first error lines
		Variable: "MESSAGE",
		Regex:    "Unable",
	},
}, []config.VariableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
}, []config.VariableMatcher{})

func TestDropboxLogExampleWithFilters(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := parser.LogEntry{
		Parser:    &dropboxParserWithFilters,
		Filtered:  false,
		Triggered: true,
		Text:      "[1217/201832.973606:ERROR:shader_disk_cache.cc(622)] Shader Cache Creation failed: -2",
		LineNo:    4,
		Variables: map[string]string{
			"LINENO":  "4",
			"LEVEL":   "ERROR",
			"MESSAGE": "Shader Cache Creation failed: -2",
		},
	}
	expectedContext := []parser.LogEntry{
		{
			Parser:    &dropboxParserWithFilters,
			Filtered:  false,
			Triggered: false,
			Text:      "[1217/070353.692622:WARNING:dns_config_service_posix.cc(335)] Failed to read DnsConfig.",
			LineNo:    1,
			Variables: map[string]string{
				"LINENO":  "1",
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
				"LINENO":  "2",
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
				"LINENO":  "3",
				"LEVEL":   "ERROR",
				"MESSAGE": "Unable to create cache",
			},
		},
		expectedEntry,
	}
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose parser.LogEntry, logContext []parser.LogEntry) error {
		defer wg.Done()
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		MonitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser.Parser{
			dropboxParserWithFilters,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	common.WaitWithTimeout(t, &wg, 1*time.Second)
}

var dropboxParserWithExcludes, _ = parser.NewParser(logger.Sugar(), "^\\[(\\d{4}\\/\\d{6}\\.\\d{6}):(?P<LEVEL>\\w+):([\\w\\.\\_]+)\\(\\d+\\)\\]\\s+(?P<MESSAGE>.*)$", []config.VariableMatcher{}, []config.VariableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
}, []config.VariableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "WARNING",
	},
})

func TestDropboxLogExampleWithExcludes(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := parser.LogEntry{
		Parser:    &dropboxParserWithExcludes,
		Triggered: true,
		Excluded:  false,
		Text:      "[1217/201832.950515:ERROR:cache_util.cc(140)] Unable to move cache folder GPUCache to old_GPUCache_000",
		LineNo:    2,
		Variables: map[string]string{
			"LINENO":  "2",
			"LEVEL":   "ERROR",
			"MESSAGE": "Unable to move cache folder GPUCache to old_GPUCache_000",
		},
	}
	expectedContext := []parser.LogEntry{
		expectedEntry,
		{
			Parser:    &dropboxParserWithExcludes,
			Triggered: true,
			Excluded:  false,
			Text:      "[1217/201832.973523:ERROR:disk_cache.cc(184)] Unable to create cache",
			LineNo:    3,
			Variables: map[string]string{
				"LINENO":  "3",
				"LEVEL":   "ERROR",
				"MESSAGE": "Unable to create cache",
			},
		},
		{
			Parser:    &dropboxParserWithExcludes,
			Triggered: true,
			Excluded:  false,
			Text:      "[1217/201832.973606:ERROR:shader_disk_cache.cc(622)] Shader Cache Creation failed: -2",
			LineNo:    4,
			Variables: map[string]string{
				"LINENO":  "4",
				"LEVEL":   "ERROR",
				"MESSAGE": "Shader Cache Creation failed: -2",
			},
		},
	}
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose parser.LogEntry, logContext []parser.LogEntry) error {
		defer wg.Done()
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		MonitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser.Parser{
			dropboxParserWithExcludes,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	common.WaitWithTimeout(t, &wg, 1*time.Second)
}

// We do not match the hash in a variable on purpose
var photosParser, _ = parser.NewParser(logger.Sugar(), "^(?P<DATE>[^ ]+)\\s+(?P<TIME>[^ ]+)\\s+[^ ]+\\s+(?P<LEVEL>[^ ]+)\\s+(?P<PID>[^ ]+)\\s+(?P<PROCNAME>[^ ]+)\\s+(?P<FILEANDLINENO>[^ ]+)\\s+(?P<MESSAGE>.*)$", []config.VariableMatcher{}, []config.VariableMatcher{
	{
		Variable: "MESSAGE",
		Regex:    "error", // will match line 2
	},
	{
		Variable: "MESSAGE",
		Regex:    "Error:", // will  match lines 4-5
	},
}, []config.VariableMatcher{})

func TestPhotosLogExampleMultipleMatchers(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := parser.LogEntry{
		Parser:    &photosParser,
		Filtered:  false,
		Triggered: true,
		Text:      "2022-01-27 21:37:36.776 0x2eb3     Default       511 photolibraryd: PLModelMigration.m:314   Creating sqlite error indicator file",
		LineNo:    2,
		Variables: map[string]string{
			"LINENO":        "2",
			"DATE":          "2022-01-27",
			"TIME":          "21:37:36.776",
			"LEVEL":         "Default",
			"PID":           "511",
			"PROCNAME":      "photolibraryd:",
			"FILEANDLINENO": "PLModelMigration.m:314",
			"MESSAGE":       "Creating sqlite error indicator file",
		},
	}
	expectedContext := []parser.LogEntry{
		{
			Parser:    &photosParser,
			Filtered:  false,
			Triggered: false,
			Text:      "2022-01-27 21:37:36.774 0x2eb3     Info          511 photolibraryd: PLModelMigration.m:290   Store has incompatible model version 14300, will attempt migration to current version 15331.",
			LineNo:    1,
			Variables: map[string]string{
				"LINENO":        "1",
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

	expectedEntry2 := parser.LogEntry{
		Parser:    &photosParser,
		Filtered:  false,
		Triggered: true,
		Text:      "2023-02-28 18:55:19.381 0x7f70b    Default      2750 photolibraryd: PLModelMigrationActionUtility.m:69    Failed updating attributes. Error: Error Domain=com.apple.photos.error Code=41004 \"Missing metadata for asset E59700B1-CF52-47FD-86B5-6835F995AAF8. File not on disk\" UserInfo={NSLocalizedDescription=Missing metadata for asset E59700B1-CF52-47FD-86B5-6835F995AAF8. File not on disk}",
		LineNo:    4,
		Variables: map[string]string{
			"LINENO":        "4",
			"DATE":          "2023-02-28",
			"TIME":          "18:55:19.381",
			"LEVEL":         "Default",
			"PID":           "2750",
			"PROCNAME":      "photolibraryd:",
			"FILEANDLINENO": "PLModelMigrationActionUtility.m:69",
			"MESSAGE":       "Failed updating attributes. Error: Error Domain=com.apple.photos.error Code=41004 \"Missing metadata for asset E59700B1-CF52-47FD-86B5-6835F995AAF8. File not on disk\" UserInfo={NSLocalizedDescription=Missing metadata for asset E59700B1-CF52-47FD-86B5-6835F995AAF8. File not on disk}",
		},
	}
	expectedContext2 := []parser.LogEntry{
		{
			Parser:    &photosParser,
			Filtered:  false,
			Triggered: false,
			Text:      "2022-01-27 21:37:36.777 0x2eb3     Default       511 photolibraryd: PLModelMigration.m:350   Starting migration stage from version 14300 to 15054, with model /System/Library/PrivateFrameworks/PhotoLibraryServices.framework/Resources/photos-15054-STAGED.mom.",
			LineNo:    3,
			Variables: map[string]string{
				"LINENO":        "3",
				"DATE":          "2022-01-27",
				"TIME":          "21:37:36.777",
				"LEVEL":         "Default",
				"PID":           "511",
				"PROCNAME":      "photolibraryd:",
				"FILEANDLINENO": "PLModelMigration.m:350",
				"MESSAGE":       "Starting migration stage from version 14300 to 15054, with model /System/Library/PrivateFrameworks/PhotoLibraryServices.framework/Resources/photos-15054-STAGED.mom.",
			},
		},
		expectedEntry2,
	}
	// create validation function
	// we expect the logs to produce two rounds of error diagnosis
	round := 1
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose parser.LogEntry, logContext []parser.LogEntry) error {
		defer wg.Done()
		if round == 1 {
			require.Equal(t, expectedEntry, entryToDiagnose)
			require.Equal(t, expectedContext, logContext)
		} else if round == 2 {
			require.Equal(t, expectedEntry2, entryToDiagnose)
			require.Equal(t, expectedContext2, logContext)
		}
		round++
		return nil
	}
	// Send process for a spin.
	wg.Add(2)
	go func(t *testing.T) {
		MonitorLogLoop(logger.Sugar(), "testlogs/photos.log", "", "", "", 10, 8000, []parser.Parser{
			photosParser,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	common.WaitWithTimeout(t, &wg, 1*time.Second)
}

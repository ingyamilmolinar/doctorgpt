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
}, []variableMatcher{})

var allLineParser, _ = newParser(logger.Sugar(), "^(?P<MESSAGE>.*)$", []variableMatcher{}, []variableMatcher{}, []variableMatcher{})

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
				"LINENO":  "1",
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
				"LINENO":  "2",
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
				"LINENO":  "3",
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
				"LINENO":  "4",
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
}, []variableMatcher{})

func TestDropboxLogExample(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
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
	expectedContext := []logEntry{
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
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
		defer wg.Done()
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		monitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser{
			dropboxParser,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	waitWithTimeout(t, &wg, 1*time.Second)
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
}, []variableMatcher{})

func TestDropboxLogExampleWithFilters(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
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
	expectedContext := []logEntry{
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
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
		defer wg.Done()
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		monitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser{
			dropboxParserWithFilters,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	waitWithTimeout(t, &wg, 1*time.Second)
}

var dropboxParserWithExcludes, _ = newParser(logger.Sugar(), "^\\[(\\d{4}\\/\\d{6}\\.\\d{6}):(?P<LEVEL>\\w+):([\\w\\.\\_]+)\\(\\d+\\)\\]\\s+(?P<MESSAGE>.*)$", []variableMatcher{}, []variableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "ERROR",
	},
}, []variableMatcher{
	{
		Variable: "LEVEL",
		Regex:    "WARNING",
	},
})

func TestDropboxLogExampleWithExcludes(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
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
	expectedContext := []logEntry{
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
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
		defer wg.Done()
		require.Equal(t, expectedEntry, entryToDiagnose)
		require.Equal(t, expectedContext, logContext)
		return nil
	}
	// Send process for a spin.
	wg.Add(1)
	go func(t *testing.T) {
		monitorLogLoop(logger.Sugar(), "testlogs/dropbox.log", "", "", "", 10, 8000, []parser{
			dropboxParserWithExcludes,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	waitWithTimeout(t, &wg, 1*time.Second)
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
}, []variableMatcher{})

func TestPhotosLogExampleMultipleMatchers(t *testing.T) {
	var wg sync.WaitGroup
	expectedEntry := logEntry{
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
	expectedContext := []logEntry{
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

	expectedEntry2 := logEntry{
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
	expectedContext2 := []logEntry{
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
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose logEntry, logContext []logEntry) error {
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
		monitorLogLoop(logger.Sugar(), "testlogs/photos.log", "", "", "", 10, 8000, []parser{
			photosParser,
			allLineParser,
		}, handler, 100*time.Millisecond, true)
	}(t)
	// Wait until handler executes
	waitWithTimeout(t, &wg, 1*time.Second)
}

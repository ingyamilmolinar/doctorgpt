package main

import (
	"testing"
	"github.com/hpcloud/tail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var logLevelParser = newParser("^\\[(?P<LEVEL>\\w+)\\]\\s+(?P<MESSAGE>.*)$")
var allLineParser = newParser("^(?P<MESSAGE>.*)$")

type expectedEntry struct {
	logEntry logEntry
	matchedDefault bool
}

var expectedEntries = []expectedEntry{
	{
		logEntry: logEntry{
			LineNo: 1,
			Text: "yarn run v1.22.19",
			Message: "yarn run v1.22.19",
		},
		matchedDefault: true,
	},
	{
		logEntry: logEntry{
			LineNo: 2,
			Text: "$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts",
			Message: "$ tsnd --respawn --transpile-only --no-notify --ignore-watch node_modules src/index.ts",
		},
		matchedDefault: true,
	},
	{
		logEntry: logEntry{
			LineNo: 3,
			Text: "[INFO] 15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)",
			Message: "15:20:25 ts-node-dev ver. 2.0.0 (using ts-node ver. 10.8.0, typescript ver. 4.8.4)",
			Level: "INFO",
		},
		matchedDefault: false,
	},
	{
		logEntry: logEntry{
			LineNo: 4,
			Text: "[INFO]  DB ready",
			Message: "DB ready",
			Level: "INFO",
		},
		matchedDefault: false,
	},
	{
		logEntry: logEntry{
			LineNo: 5,
			Text: "[INFO]  Auth ready",
			Message: "Auth ready",
			Level: "INFO",
		},
		matchedDefault: false,
	},
	{
		logEntry: logEntry{
			LineNo: 6,
			Text: "[INFO]  Apollo setup",
			Message: "Apollo setup",
			Level: "INFO",
		},
		matchedDefault: false,
	},
	{
		logEntry: logEntry{
			LineNo: 7,
			Text: "[INFO]  Server started at http://localhost:5555/graphql 🚀",
			Message: "Server started at http://localhost:5555/graphql 🚀",
			Level: "INFO",
		},
		matchedDefault: false,
	},
}

func Test(t *testing.T) {
	parsers := []parser{
		logLevelParser,
		allLineParser,
	}

	tailConfig := tail.Config{
		Follow:   false,
	}

	f, err := tail.TailFile("testlogs/error.log", tailConfig)
	require.NoError(t, err)

	i := 0
	for line := range f.Lines {
		entry, matchedDefault, err := parseLogEntry(parsers, line.Text, i+1)
		assert.NoError(t, err)
		assert.Equal(t, expectedEntries[i].logEntry, entry)
		assert.Equal(t, expectedEntries[i].matchedDefault, matchedDefault)
		i++
	}
}

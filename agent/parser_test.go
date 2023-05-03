package main

import (
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sync"
	"testing"
	"time"

	"github.com/ingyamilmolinar/doctorgpt/agent/internal/common"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/config"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/parser"
)

func TestAndroidParser(t *testing.T) {
	androidParser, err := parser.NewParser(logger.Sugar(), "^(?P<DATE>\\d{2}-\\d{2})\\s(?P<TIME>\\d{2}:\\d{2}:\\d{2}.\\d{3})\\s+(?P<PID>\\d+)\\s+(?P<TID>\\d+)\\s+(?P<LEVEL>[A-Z])\\s+(?P<TAG>[^:]+):\\s(?P<MESSAGE>.+)$",
		[]config.VariableMatcher{}, []config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		}, []config.VariableMatcher{})
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &androidParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "03-17 16:16:09.141  1702  1820 D DisplayPowerController: Animating brightness: target=38, rate=200",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":  "2000",
			"DATE":    "03-17",
			"TIME":    "16:16:09.141",
			"PID":     "1702",
			"TID":     "1820",
			"LEVEL":   "D",
			"TAG":     "DisplayPowerController",
			"MESSAGE": "Animating brightness: target=38, rate=200",
		},
	}

	testParser(t, androidParser, expectedEntry, 2000, "testlogs/Android_2k.log")
}

func TestApacheParser(t *testing.T) {
	apacheParser, err := parser.NewParser(logger.Sugar(), "^\\[(?P<DATE>\\w{3} \\w{3} \\d{2} \\d{2}:\\d{2}:\\d{2} \\d{4})\\] \\[(?P<SEVERITY>\\w+)\\] (?P<MESSAGE>.*)$", []config.VariableMatcher{}, []config.VariableMatcher{
		{
			Variable: "LINENO",
			Regex:    "2000",
		},
	}, []config.VariableMatcher{})
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &apacheParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "[Mon Dec 05 19:15:57 2005] [error] mod_jk child workerEnv in error state 6",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":   "2000",
			"DATE":     "Mon Dec 05 19:15:57 2005",
			"SEVERITY": "error",
			"MESSAGE":  "mod_jk child workerEnv in error state 6",
		},
	}

	testParser(t, apacheParser, expectedEntry, 2000, "testlogs/Apache_2k.log")
}

func TestHDFSParser(t *testing.T) {
	hdfsParser, err := parser.NewParser(logger.Sugar(), "^(?P<DATE>\\d{6})\\s(?P<TIME>\\d{6})\\s(?P<PID>\\d+)\\s(?P<LEVEL>\\w+)\\s(?P<CLASS>[^\\s]+):\\s(?P<MESSAGE>.*)$",
		[]config.VariableMatcher{}, []config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		}, []config.VariableMatcher{})
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &hdfsParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "081111 102017 26347 INFO dfs.DataNode$DataXceiver: Receiving block blk_4343207286455274569 src: /10.250.9.207:59759 dest: /10.250.9.207:50010",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":  "2000",
			"DATE":    "081111",
			"TIME":    "102017",
			"PID":     "26347",
			"LEVEL":   "INFO",
			"CLASS":   "dfs.DataNode$DataXceiver",
			"MESSAGE": "Receiving block blk_4343207286455274569 src: /10.250.9.207:59759 dest: /10.250.9.207:50010",
		},
	}

	testParser(t, hdfsParser, expectedEntry, 2000, "testlogs/HDFS_2k.log")
}

func TestHadoopParser(t *testing.T) {
	hadoopParser, err := parser.NewParser(logger.Sugar(), "^(?P<TIMESTAMP>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2},\\d{3})\\s+(?P<LEVEL>[A-Z]+)\\s+\\[(?P<THREAD>[^\\]]+)\\] (?P<CLASS>[^:]+): (?P<MESSAGE>.+)$",
		[]config.VariableMatcher{}, []config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		}, []config.VariableMatcher{})
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &hadoopParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "2015-10-18 18:10:55,202 WARN [LeaseRenewer:msrabi@msra-sa-41:9000] org.apache.hadoop.ipc.Client: Address change detected. Old: msra-sa-41/10.190.173.170:9000 New: msra-sa-41:9000",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":    "2000",
			"TIMESTAMP": "2015-10-18 18:10:55,202",
			"LEVEL":     "WARN",
			"THREAD":    "LeaseRenewer:msrabi@msra-sa-41:9000",
			"CLASS":     "org.apache.hadoop.ipc.Client",
			"MESSAGE":   "Address change detected. Old: msra-sa-41/10.190.173.170:9000 New: msra-sa-41:9000",
		},
	}

	testParser(t, hadoopParser, expectedEntry, 2000, "testlogs/Hadoop_2k.log")
}

func TestLinuxParser(t *testing.T) {
	linuxParser, err := parser.NewParser(logger.Sugar(), "^(?P<DATE>[A-Z][a-z]{2}\\s+\\d{1,2})\\s+(?P<TIME>\\d{2}:\\d{2}:\\d{2})\\s+(?P<HOST>\\S+)\\s+(?P<PROCESS>[^:]+)(\\[(?P<PID>\\d+)\\])?:\\s+(?P<MESSAGE>.+)$",
		[]config.VariableMatcher{}, []config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		}, []config.VariableMatcher{})
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &linuxParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "Jul 27 14:42:00 combo kernel: Linux agpgart interface v0.100 (c) Dave Jones",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":  "2000",
			"DATE":    "Jul 27",
			"TIME":    "14:42:00",
			"HOST":    "combo",
			"PROCESS": "kernel",
			"PID":     "",
			"MESSAGE": "Linux agpgart interface v0.100 (c) Dave Jones",
		},
	}

	testParser(t, linuxParser, expectedEntry, 2000, "testlogs/Linux_2k.log")
}

func TestMacParser(t *testing.T) {
	macParser, err := parser.NewParser(logger.Sugar(),
		"^(?P<MONTH>[A-Z][a-z]{2})\\s+(?P<DAY>\\d{1,2})\\s(?P<TIME>(?:\\d{2}:){2}\\d{2})\\s(?P<HOST>[^\\s]+)\\s(?P<PROCESS>[^\\[]+)\\[(?P<PID>\\d+)\\]:?(?:\\s\\((?P<PID2>\\d+)\\))?:?\\s(?P<MESSAGE>.*)$",
		[]config.VariableMatcher{},
		[]config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		},
		[]config.VariableMatcher{},
	)
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &macParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "Jul  8 08:10:46 calvisitor-10-105-162-124 kernel[0]: AppleCamIn::wakeEventHandlerThread",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":  "2000",
			"MONTH":   "Jul",
			"DAY":     "8",
			"TIME":    "08:10:46",
			"HOST":    "calvisitor-10-105-162-124",
			"PROCESS": "kernel",
			"PID":     "0",
			"PID2":    "",
			"MESSAGE": "AppleCamIn::wakeEventHandlerThread",
		},
	}

	testParser(t, macParser, expectedEntry, 2000, "testlogs/Mac_2k.log")
}

func TestSparkParser(t *testing.T) {
	sparkParser, err := parser.NewParser(logger.Sugar(),
		"^(?P<DATE>\\d{2}\\/\\d{2}\\/\\d{2}) (?P<TIME>\\d{2}:\\d{2}:\\d{2}) (?P<LEVEL>[A-Z]+) (?P<CLASS>[a-zA-Z0-9\\.]+): (?P<MESSAGE>.+)$",
		[]config.VariableMatcher{},
		[]config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		},
		[]config.VariableMatcher{},
	)
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &sparkParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "17/06/09 20:11:11 INFO storage.BlockManager: Found block rdd_42_32 locally",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":  "2000",
			"DATE":    "17/06/09",
			"TIME":    "20:11:11",
			"LEVEL":   "INFO",
			"CLASS":   "storage.BlockManager",
			"MESSAGE": "Found block rdd_42_32 locally",
		},
	}

	testParser(t, sparkParser, expectedEntry, 2000, "testlogs/Spark_2k.log")
}

func TestWindowsParser(t *testing.T) {
	windowsParser, err := parser.NewParser(logger.Sugar(),
		"^(?P<DATE>\\d{4}-\\d{2}-\\d{2}) (?P<TIME>\\d{2}:\\d{2}:\\d{2}),\\s+(?P<LEVEL>[A-Z][a-z]+)\\s+(?P<CLASS>[A-Za-z]+)\\s+(?P<MESSAGE>.*)$",
		[]config.VariableMatcher{},
		[]config.VariableMatcher{
			{
				Variable: "LINENO",
				Regex:    "2000",
			},
		},
		[]config.VariableMatcher{},
	)
	require.NoError(t, err)

	expectedEntry := parser.LogEntry{
		Parser:    &windowsParser,
		Filtered:  false,
		Triggered: true,
		Excluded:  false,
		Text:      "2016-09-29 02:04:40, Info                  CBS    Read out cached package applicability for package: Package_for_KB2928120~31bf3856ad364e35~amd64~~6.1.1.2, ApplicableState: 0, CurrentState:0",
		LineNo:    2000,
		Variables: map[string]string{
			"LINENO":  "2000",
			"DATE":    "2016-09-29",
			"TIME":    "02:04:40",
			"LEVEL":   "Info",
			"CLASS":   "CBS",
			"MESSAGE": "Read out cached package applicability for package: Package_for_KB2928120~31bf3856ad364e35~amd64~~6.1.1.2, ApplicableState: 0, CurrentState:0",
		},
	}

	testParser(t, windowsParser, expectedEntry, 2000, "testlogs/Windows_2k.log")
}

func testParser(t *testing.T, mainParser parser.Parser, expectedLastEntry parser.LogEntry, logLines int, filePath string) {
	var wg sync.WaitGroup
	// create validation function
	handler := func(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, entryToDiagnose parser.LogEntry, logContext []parser.LogEntry) error {
		defer wg.Done()
		require.Equal(t, expectedLastEntry, entryToDiagnose)
		require.Equal(t, logLines, len(logContext))
		// Verify that all entries in the context were parsed with the first parser
		for _, entry := range logContext {
			require.Equal(t, mainParser.Regex, entry.Parser.Regex, "Line (%d) was not parsed correctly", entry.LineNo)
		}
		return nil
	}
	// Monitor log (will finish and not tail)
	wg.Add(1)
	go func() {
		MonitorLogLoop(logger.Sugar(), filePath, "", "", "", logLines, 999999, []parser.Parser{
			mainParser,
			allLineParser,
		}, handler, 100*time.Millisecond, false)
	}()
	common.WaitWithTimeout(t, &wg, 2*time.Second)
}

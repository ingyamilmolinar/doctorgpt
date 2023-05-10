package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/buffer"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/config"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/diagnose"
	"github.com/ingyamilmolinar/doctorgpt/agent/internal/parser"
	"go.uber.org/zap"
)

func main() {
	_, err := fmt.Println("Beginning start-up sequence")
	if err != nil {
		panic(err)
	}

	// Parse command-line arguments
	// TODO: Monitor all logs in a directory
	logFilePath := flag.String("logfile", "", "path to log file")
	outputDir := flag.String("outdir", "", "path to output directory")
	configFilePath := flag.String("configfile", "", "path to config file")
	// String instead of bool due to: https://stackoverflow.com/questions/27411691/how-to-pass-boolean-arguments-to-go-flags
	debugFlag := flag.String("debug", "true", "log debug flag")
	logBundlingTimeoutInSecs := flag.Int("bundlingtimeoutseconds", 5, "log bundling timeout duration in seconds")
	bufferSize := flag.Int("buffersize", 100, "max log entries per ring-buffer")
	maxTokens := flag.Int("maxtokens", 8000, "max tokens for context per API request")
	gptModel := flag.String("gptmodel", "gpt-4", "GPT model to use for diagnosis")
	flag.Parse()

	// Init logger
	var logger *zap.Logger
	var logConfig zap.Config
	if *debugFlag == "false" {
		logConfig = zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
			Development:      false,
			Encoding:         "json",
			EncoderConfig:    zap.NewProductionEncoderConfig(),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stdout"},
		}
		fmt.Println("Initializing logger in production mode")
	} else {
		logConfig = zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Development:      true,
			Encoding:         "console",
			EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stdout"},
		}
		fmt.Println("Initializing logger in debug mode")
	}
	logger, err = logConfig.Build()
	if err != nil {
		fmt.Printf("Failed to init logger: %v", err)
		os.Exit(1)
	}
	// TODO: Handle a kill event gracefully
	defer logger.Sync()

	go func(log *zap.Logger) {
		flushLoggerTick := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-flushLoggerTick.C:
				err := logger.Sync()
				// https://github.com/uber-go/zap/issues/328
				if err != nil && !strings.Contains(err.Error(), "inappropriate ioctl for device") && !strings.Contains(err.Error(), "invalid argument") {
					log.Sugar().Debugf("%v\n", err)
				}
			}
		}
	}(logger)
	log := logger.Sugar()

	if *logFilePath == "" {
		log.Fatal("Log file path is required")
	}

	if *outputDir == "" {
		log.Fatal("Output directory path is required")
	}

	if *configFilePath == "" {
		log.Fatal("Config file path is required")
	}

	// Get ChatGPT API key from environment variable
	apiKey := os.Getenv("OPENAI_KEY")
	if apiKey == "" {
		log.Fatal("ChatGPT API key is required")
	}

	// Setup and build parsers
	parsers, err := setup(log, *configFilePath, *outputDir, config.FileConfigProvider)
	if err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	// This will effectively never end (it doesn't handle EOF)
	timeoutDuration := time.Duration(*logBundlingTimeoutInSecs) * time.Second
	MonitorLogLoop(log, *logFilePath, *outputDir, apiKey, *gptModel, *bufferSize, *maxTokens, parsers, diagnose.HandleTrigger, timeoutDuration, true)
}

func setup(log *zap.SugaredLogger, configFile, outputDir string, configProvider config.ConfigProvider) ([]parser.Parser, error) {
	cfg, err := configProvider(log, configFile)
	if err != nil {
		return nil, fmt.Errorf("config provider failed: %w", err)
	}
	if cfg.SystemPrompt != "" {
		config.SystemPrompt = cfg.SystemPrompt
	}
	if cfg.Prompt != "" {
		config.UserPrompt = cfg.Prompt
	}

	var parsers []parser.Parser
	for _, p := range cfg.Parsers {
		parser, err := parser.NewParser(log, p.Regex, p.Filters, p.Triggers, p.Excludes)
		if err != nil {
			return nil, fmt.Errorf("invalid config file: %w", err)
		}
		log.Debugf("Appending parser (%s)", parser.Regex)
		parsers = append(parsers, parser)
	}
	log.Infof("Initialized (%d) parsers", len(parsers))

	// Create dir if not exists
	exists, err := exists(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open output directory: %w", err)
	}
	// TODO: If exists, check permissions
	if !exists {
		err = os.Mkdir(outputDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	return parsers, nil
}

func MonitorLogLoop(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, bufferSize, maxTokens int, parsers []parser.Parser, handler diagnose.Handler, timeout time.Duration, follow bool) {
	// Set up tail object to read log file
	tailConfig := tail.Config{
		Follow: follow,
		ReOpen: follow,
	}
	t, err := tail.TailFile(fileName, tailConfig)
	if err != nil {
		log.Fatalf("Failed to tail log file: %v", err)
	}

	// Map of log buffers, keyed by thread ID or routine name
	logBuffers := make(map[string]*buffer.LogBuffer)

	// Loop to read new lines from the log file
	lineNum := 0
	for line := range t.Lines {
		lineNum++
	top:
		// Parse the log entry
		entry, parserMatched, err := parser.ParseLogEntry(log, parsers, line.Text, lineNum)
		if err != nil {
			log.Fatalf("Error parsing log entry (%s)", line)
		}

		// If entry is excluded, ignore it
		if entry.Excluded {
			continue
		}

		// TODO: Divide into buffers depending on granularity
		key := "DEFAULT"
		log.Debugf("Process key (%s)", key)

		// Create a new buffer if necessary
		if _, ok := logBuffers[key]; !ok {
			logBuffers[key] = buffer.NewLogBuffer(log, bufferSize, maxTokens-len(config.SystemPrompt)-len(config.UserPrompt))
		}

		// Buffer the log entry
		log.Debugf("Appending to buffer: (%s)", line)
		buffer := logBuffers[key]
		buffer.Append(entry)

		// Check if the log entry indicates an error
		log.Debugf("Should filter: %v", entry.Filtered)
		log.Debugf("Should diagnose: %v", !entry.Filtered && entry.Triggered)
		if !entry.Filtered && entry.Triggered {
			entryToDiagnose := entry
			log.Infof("Entry to diagnose: %s", entryToDiagnose.Text)
			// Append subsequent log entries to the buffer until a new log level is detected
			// Wait for input or timeout in N seconds
			timec := time.After(timeout)
		outer:
			for {
				select {
				case <-timec:
					log.Info("Timeout!")
					break outer // timed out
				// Process previous entry if exist
				case l, ok := <-t.Lines:
					if !ok {
						log.Debug("Log line channel closed or empty")
						break outer
					}
					// increment line number
					lineNum++
					// Parse lines until we hit a known log line that's not the generic one
					var matched int
					entry, matched, err = parser.ParseLogEntry(log, parsers, l.Text, lineNum)
					if err != nil {
						log.Fatalf("Error parsing log entry (%s)", l.Text)
					}

					// If entry is excluded, ignore it
					if entry.Excluded {
						continue
					}

					// TODO: Have an optional "bundle" line limit to avoid packing too much context after the error
					// TODO: Do not rely on location for the default parser
					if matched == len(parsers)-1 || (matched == parserMatched && !entry.Filtered && entry.Triggered) {
						// Matched default parser OR
						// Matched the same parser and it was triggered
						log.Debugf("Default parser matched: (%v)", matched == len(parsers)-1)
						log.Debugf("Appending to buffer: (%v)", entry)
						buffer := logBuffers[key]
						buffer.Append(entry)
					} else {
						// Spoof line and go back to top
						log.Debugf("Spoofing: (%s)", l.Text)
						line = l

						// TODO: Deduplicate this logic
						// dump log context buffer and clear
						dumpedBuffer := logBuffers[key].Dump()
						logBuffers[key].Clear()
						go func() {
							err := handler(log, fileName, outputDir, apiKey, model, entryToDiagnose, dumpedBuffer)
							if err != nil {
								log.Errorf("Handler failed: %v", err)
							}
						}()
						goto top
					}
				}
			}

			// dump log context buffer and clear
			dumpedBuffer := logBuffers[key].Dump()
			logBuffers[key].Clear()

			// Async call the ChatGPT API
			// TODO: We need persistance to make sure all errors are reported
			// TODO: Expose N prompts and N diagnosis per error configuration
			go func() {
				err := handler(log, fileName, outputDir, apiKey, model, entryToDiagnose, dumpedBuffer)
				if err != nil {
					log.Errorf("Handler failed: %v", err)
				}
			}()
		}
	}
}

// exists returns whether the given file or directory exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

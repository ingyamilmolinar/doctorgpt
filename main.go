package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hpcloud/tail"
	"go.uber.org/zap"
)

func main() {
	// Parse command-line arguments
	// TODO: Monitor all logs in a directory
	logFilePath := flag.String("logfile", "", "path to log file")
	outputDir := flag.String("outdir", "", "path to output directory")
	configFilePath := flag.String("configfile", "", "path to config file")
	debugFlag := flag.Bool("debug", true, "log debug flag")
	logBundlingTimeoutInSecs := flag.Int("bundlingtimeoutseconds", 5, "log bundling timeout duration in seconds")
	bufferSize := flag.Int("buffersize", 100, "max log entries per ring-buffer")
	maxTokens := flag.Int("maxtokens", 8000, "max tokens for context per API request")
	gptModel := flag.String("gptmodel", "gpt-4", "GPT model to use for diagnosis")
	flag.Parse()

	// Init logger
	var err error
	var logger *zap.Logger
	if *debugFlag {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("Failed to init logger: %v", err)
		os.Exit(1)
	}
	// TODO: Can this cause issues?
	// TODO: Handle a kill event gracefully
	defer logger.Sync()
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
	parsers, err := setup(log, *configFilePath, *outputDir, fileConfigProvider)
	if err != nil {
		log.Fatal("Setup failed: %v", err)
	}

	// This will effectively never end (it doesn't handle EOF)
	timeoutDuration := time.Duration(*logBundlingTimeoutInSecs) * time.Second
	monitorLogLoop(log, *logFilePath, *outputDir, apiKey, *gptModel, *bufferSize, *maxTokens, parsers, handleTrigger, timeoutDuration)
}

func setup(log *zap.SugaredLogger, configFile, outputDir string, configProvider configProvider) ([]parser, error) {
	config, err := configProvider(log, configFile)
	if err != nil {
		return nil, fmt.Errorf("config provider failed: %w", err)
	}
	if config.Prompt != "" {
		basePrompt = config.Prompt
	}

	var parsers []parser
	for _, parser := range config.Parsers {
		parser, err := newParser(log, parser.Regex, parser.Filters, parser.Triggers)
		if err != nil {
			return nil, fmt.Errorf("invalid config file: %w", err)
		}
		log.Debugf("Appending parser (%s)", parser.regex)
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

func monitorLogLoop(log *zap.SugaredLogger, fileName, outputDir, apiKey, model string, bufferSize, maxTokens int, parsers []parser, handler handler, timeout time.Duration) {
	// Set up tail object to read log file
	tailConfig := tail.Config{
		Follow: true,
		ReOpen: true,
	}
	t, err := tail.TailFile(fileName, tailConfig)
	if err != nil {
		log.Fatalf("Failed to tail log file: %v", err)
	}

	// Map of log buffers, keyed by thread ID or routine name
	logBuffers := make(map[string]*logBuffer)

	// Loop to read new lines from the log file
	lineNum := 0
	for line := range t.Lines {
		lineNum++
	top:
		// Parse the log entry
		entry, parserMatched, err := parseLogEntry(log, parsers, line.Text, lineNum)
		if err != nil {
			log.Fatalf("Error parsing log entry (%s)", line)
		}
		// Divide into buffers depending on granularity
		key := "DEFAULT"
		if entry.Thread != "" {
			key = entry.Thread
		} else if entry.Routine != "" {
			key = entry.Routine
		} else if entry.Process != "" {
			key = entry.Process
		} else if entry.Trace != "" {
			key = entry.Trace
		}
		log.Debugf("Process key (%s)", key)

		// Create a new buffer if necessary
		if _, ok := logBuffers[key]; !ok {
			logBuffers[key] = newLogBuffer(log, bufferSize, maxTokens-len(basePrompt))
		}

		// Buffer the log entry
		log.Debugf("Appending to buffer: (%s)", line)
		buffer := logBuffers[key]
		buffer.Append(entry)

		lineSpoofed := false
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
					entry, matched, err = parseLogEntry(log, parsers, l.Text, lineNum)
					if err != nil {
						log.Fatalf("Error parsing log entry (%s)", l)
					}

					// TODO: Have an optional "bundle" line limit to avoid packing too much context after the error
					if matched == len(parsers)-1 || matched == parserMatched && (!entry.Filtered && entry.Triggered) {
						// Matched default parser OR
						// If follow-ups match the same parser and they are triggers
						log.Debugf("Appending to buffer: (%s)", l)
						buffer := logBuffers[key]
						buffer.Append(entry)
					} else {
						// Spoof line and go back to top
						log.Debugf("Spoofing: (%s)", l)
						line = l
						lineSpoofed = true
					}
				}
			}

			// dump log context buffer
			dumpedBuffer := logBuffers[key].Dump()

			// Async call the ChatGPT API
			// TODO: We need persistance to make sure all errors are reported
			// TODO: Expose N prompts and N diagnosis per error configuration
			go func() {
				err := handler(log, fileName, outputDir, apiKey, model, entryToDiagnose, dumpedBuffer)
				if err != nil {
					log.Errorf("Handler failed: %v", err)
				}
			}()
			if lineSpoofed {
				lineSpoofed = false
				goto top
			}
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

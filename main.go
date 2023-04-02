package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"strings"
	"strconv"
	"regexp"
	"path/filepath"

	openai "github.com/sashabaranov/go-openai"
	"github.com/hpcloud/tail"
	"github.com/cenkalti/backoff/v4"
	"github.com/fatih/structs"
)

// TODO: Make error placeholder configurable
// TODO: Send which image, program and/or version is outputing the logs (if known)
const errorPlaceholder = "$ERROR"
var basePrompt = "You are ErrorDebuggingGPT. Your sole purpose in this world is to help software engineers by diagnosing software system errors and bugs that can occur in any type of computer system. The message following the first line containing \"ERROR:\" up until the end of the prompt is a computer error no more and no less. It is your job to try to diagnose and fix what went wrong. Ready?\nERROR:\n"+errorPlaceholder

func main() {
	// Parse command-line arguments
	// TODO: Monitor all logs in a directory 
	logFilePath := flag.String("logfile", "", "path to log file")
	outputDir := flag.String("outdir", "", "path to output directory")
	configFilePath := flag.String("configfile", "", "path to config file")
	flag.Parse()

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
	apiKey := os.Getenv("OPENAPI_KEY")
	if apiKey == "" {
		log.Fatal("ChatGPT API key is required")
	}

	// Read configuration
	lines, err := readLines(*configFilePath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	if len(lines) == 0 {
		log.Fatalf("Invalid config file")
	}

	// TODO: Include triggers in the parsers!
	// TODO: Move to yaml config
	var parsers []parser
	var triggers []trigger
	var triggerFlag bool
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == "PARSERS:" {
			continue
		}
		if strings.TrimSpace(line) == "TRIGGERS:" {
			triggerFlag = true
			continue
		}
		// Override base prompt
		if strings.HasPrefix(strings.TrimSpace(line), "PROMPT:") {
			basePrompt = strings.TrimPrefix(strings.TrimSpace(line), "PROMPT:")
			fmt.Printf("DEBUG: OVERRIDING PROMPT (%s)\n", basePrompt)
			continue
		}
		if !triggerFlag {
			fmt.Printf("DEBUG: APPENDING PARSER (%s)\n", line)
			parsers = append(parsers, newParser(line))
		} else {
			trigger, err := newTrigger(line)
			if err != nil {
				log.Fatalf("Invalid config file: %v", err)
			}
			fmt.Printf("DEBUG: APPENDING TRIGGER (%s)\n", line)
			triggers = append(triggers, trigger)
		}
	}
	if len(parsers) == 0 || !triggerFlag {
		log.Fatalf("Invalid config file")
	}

	// Create dir if not exists
	exists, err := exists(*outputDir)
	if err != nil {
		log.Fatalf("Failed to open output directory: %v", err)
	}	
	// TODO: If exists, check permissions
	if !exists {
		err = os.Mkdir(*outputDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}	
	}

	// Set up tail object to read log file
	tailConfig := tail.Config{
		Follow:   true,
		ReOpen:   true,
	}

	t, err := tail.TailFile(*logFilePath, tailConfig)
	if err != nil {
		log.Fatalf("Failed to tail log file: %v", err)
	}

	// Map of log buffers, keyed by thread ID or routine name
	logBuffers := make(map[string]*logBuffer)

	// Loop to read new lines from the log file
	// This will effectively never end (it doesn't handle EOF)
	lineNum := 0
	for line := range t.Lines {
		lineNum++
		top:
		// Parse the log entry
		entry, parserMatched, err := parseLogEntry(parsers, line.Text, lineNum)
		if err != nil {
			log.Fatalf("Error parsing log entry (%s)\n", line)
		}
		// Get the thread ID or routine name from the log entry
		key := "DEFAULT"
		if entry.Thread != "" {
			key = entry.Thread
		} else if entry.Routine != "" {
			key = entry.Routine
		} else if entry.Trace != "" {
			key = entry.Trace
		}
		fmt.Printf("DEBUG: PROCESS KEY (%s)\n", key)

		// Create a new buffer if necessary
		// TODO: Make buffer size configurable
		// TODO: Limit buffer in terms of the max input lenght of the API?
		if _, ok := logBuffers[key]; !ok {
			logBuffers[key] = newLogBuffer(100)
		}

		// Buffer the log entry
		fmt.Printf("DEBUG: APPENDING TO BUFFER: (%s)\n", line)
		buffer := logBuffers[key]
		buffer.Append(entry)

		lineSpoofed := false
		// Check if the log entry indicates an error
		fmt.Printf("DEBUG: SHOULD DIAGNOSE: %v\n", entry.MatchesTriggers(triggers))
		if entry.MatchesTriggers(triggers) {
			entryToDiagnose := entry
			fmt.Println("Entry to diagnose:\n", entryToDiagnose.Text)
			// Append subsequent log entries to the buffer until a new log level is detected
			// Wait for input or timeout in 5 seconds
			// TODO: Make timeout configurable
			timec := time.After(5 * time.Second)
			outer:
			for {
				select {
					case <-timec:
						fmt.Printf("Timeout!\n")
						break outer // timed out
					// Process previous entry if exist
					case l, ok := <-t.Lines:
						if !ok {
							break outer
						}
						// increment line number
						lineNum++
						// Parse lines until we hit a known log line that's not the generic one
						var matched int
						entry, matched, err = parseLogEntry(parsers, l.Text, lineNum)
						if err != nil {
							log.Fatalf("Error parsing log entry (%s)\n", l)
						}

						if matched == len(parsers)-1 || matched == parserMatched && entry.MatchesTriggers(triggers) {
							// Matched default parser OR
							// If follow-ups match the same parser and they are triggers
							fmt.Printf("DEBUG: APPENDING TO BUFFER: (%s)\n", l)
							buffer := logBuffers[key]
							buffer.Append(entry)
						} else {
							// Spoof line and go back to top
							fmt.Printf("DEBUG: SPOOFING: (%s)\n", l)
							line = l
							lineSpoofed = true
						}
				}
			}

			// logContext will contain the error as the very last thing
			dumpedBuffer := logBuffers[key].Dump()
			logContext := stringifyLogs(dumpedBuffer)

			// Async call the ChatGPT API
			// TODO: We need persistance to make sure all errors are reported
			// TODO: Expose N diagnosis per error configuration
			go func() {
				err = backoff.Retry(func () error {
					// create file and write to it
					errorLocation := *logFilePath+":"+strconv.Itoa(entryToDiagnose.LineNo)
					filename := *outputDir+"/"+safeString(errorLocation)+".diagnosing"
					f, err := os.Create(filename)
					if err != nil {
						return fmt.Errorf("error creating diagnosis file: %w", err)
					}
					fmt.Printf("Log Line: %s\n", errorLocation)
					_, err = f.WriteString(fmt.Sprintf("LOG LINE:\n%s\n\n", errorLocation))
					if err != nil {
						return fmt.Errorf("error writing to diagnosis file: %w", err)
					}
					fmt.Printf("Prompt: %s\n", basePrompt)
					_, err = f.WriteString(fmt.Sprintf("BASE PROMPT:\n%s\n\n", basePrompt))
					if err != nil {
						return fmt.Errorf("error writing to diagnosis file: %w", err)
					}
					fmt.Printf("Context: %s\n", logContext)
					_, err = f.WriteString(fmt.Sprintf("CONTEXT:\n%s\n\n", logContext))
					if err != nil {
						return fmt.Errorf("error writing to diagnosis file: %w", err)
					}
					suggestion, err := suggestion(apiKey, basePrompt, logContext)
					if err != nil {
						return fmt.Errorf("error diagnosing using the openai API: %w", err)
					}
					fmt.Println("Suggestion: \n", suggestion)
					_, err = f.WriteString(fmt.Sprintf("SUGGESTION:\n%s\n", suggestion))
					if err != nil {
						return fmt.Errorf("error writing to diagnosis file: %w", err)
					}
					err = f.Close()
					if err != nil {
						return fmt.Errorf("error closing the diagnosis file: %w", err)
					}
					fullNameNoExt := strings.TrimRight(filename, ".diagnosing")
					err = os.Rename(filename, fullNameNoExt + ".diagnosed")
					if err != nil {
						return fmt.Errorf("error renaming the diagnosis file: %w", err)
					}
					return nil
				}, backoff.WithMaxRetries(backoff.NewConstantBackOff(2*time.Second), 3))
				if err != nil {
					log.Printf("Failed to diagnose after retries: %v", err)
				}	
			}()
			if lineSpoofed {
				lineSpoofed = false
				goto top
			}
		}
	}
}

// TODO: Support parsing structured logging
type parser struct {
	regex string
	re regexp.Regexp
}

func newParser(line string) parser {
	re := regexp.MustCompile(line)
	return parser{
		regex:  line,
		re: *re,
	}
}

func (p parser) Parse(line string, lineNum int) (logEntry, error) {
	matches := p.re.FindStringSubmatch(line)
	if len(matches) == 0 {
		return logEntry{}, fmt.Errorf("parser with regex (%s) did not match line (%s)", p.regex, line)
	}
	result := make(map[string]string)
	for i, name := range p.re.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = matches[i]
				fmt.Printf("DEBUG: NAME: (%s), MATCH: (%s)\n", name, matches[i])
			}
	}
	_, ok := result["MESSAGE"]
	if !ok {
		return logEntry{}, fmt.Errorf("parser with regex (%s) did not match line (%s)", p.regex, line)
	}
	_, ok = result["LEVEL"]
	if !ok {
		return logEntry{
			Text: line,
			LineNo: lineNum,
			Message: result["MESSAGE"],
		}, nil
	}
	return logEntry{
		Text: line,
		LineNo: lineNum,
		Level: result["LEVEL"],
		Message: result["MESSAGE"],
	}, nil
}

// TODO: Composing multiple logical conditions in a single trigger
type trigger struct {
	variable string
	re regexp.Regexp
}

func newTrigger(line string) (trigger, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return trigger{}, fmt.Errorf("line is not a valid trigger (%s)", line)
	}
	variable := strings.ReplaceAll(fields[0], ":", "")
	regex := fields[1]
	re, err := regexp.Compile(regex)
	if err != nil {
		return trigger{}, fmt.Errorf("regex is not valid (%s)", regex)
	}
	return trigger{
		variable: variable,
		re: *re,
	}, nil
}

func (t trigger) Match(entry logEntry) bool {
	// Decode entry into json field map
	m := structs.Map(entry)
	fmt.Printf("DEBUG: VARIABLE (%s) MAP (%v)\n", t.variable, m)
	value, ok := m[t.variable]
	if !ok {
		fmt.Printf("DEBUG: VARIABLE NOT FOUND IN ENTRY (%s)\n", entry.Text)
		return false
	}
	castedValue, ok := value.(string)
	if !ok {
		fmt.Printf("DEBUG: COULD NOT CAST TO STRING (%v)\n", value)
		return false
	}
	fmt.Printf("DEBUG: TRYING TO MATCH REGEX (%v)\n", t.re)
	return t.re.MatchString(castedValue)
}

func stringifyLogs(logs []logEntry) string {
	result := ""
	for _, logEntry := range logs {
		result += fmt.Sprintf("%s", logEntry.Text) + "\n"
	}
	return result
}

func suggestion(key, basePrompt, errorMsg string) (string, error) {
	prompt := strings.Replace(basePrompt, errorPlaceholder, errorMsg, 1)
	client := openai.NewClient(key)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("error generating text from API: %v", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("chatGPT returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// Struct representing a single log entry (message can be a multi-line string)
type logEntry struct {
    Text      string     `structs:"TEXT"`
		LineNo    int        `structs:"LINENO"`
    Date      time.Time  `structs:"DATE"`
    Time      time.Time  `structs:"TIME"`
    Level     string     `structs:"LEVEL"`
    Thread    string     `structs:"THREAD"`
    Routine   string     `structs:"ROUTINE"`
    Trace     string     `structs:"TRACE"`
    Message   string     `structs:"MESSAGE"`
}

func (le logEntry) MatchesTriggers(triggers []trigger) bool {
	for _, trigger := range triggers {
		fmt.Printf("DEBUG: MATCHING TRIGGER: (%v)\n", trigger)
		if trigger.Match(le) {
			fmt.Printf("DEBUG: MATCHED TRIGGER: (%v)\n", trigger)
			return true
		}
	}
	return false
}

// Parse a log line into a LogEntry object
func parseLogEntry(parsers []parser, line string, lineNum int) (logEntry, int, error) {
		var entry logEntry
		var err error
		for i, parser := range parsers {
			entry, err = parser.Parse(line, lineNum)
			if err == nil {
				fmt.Printf("DEBUG: MATCHED: i (%d): REGEX (%s), LINE (%s)\n", i, parser.regex, line)
				return entry, i, nil
			}
			fmt.Printf("DEBUG: NOT MATCHED: %v\n", err)
		}
		return logEntry{}, 0, fmt.Errorf("No parser found for line (%s)", line)
}

func readLines(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    return lines, scanner.Err()
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

// TODO: Make file separator configurable
func safeString(s string) string {
	result := strings.ReplaceAll(s, " ", "-")
	result = strings.ReplaceAll(result, "/", "::")
	if len(s) > 200 {
		result = s[0:200]
	}
	return filepath.Clean(result)
}

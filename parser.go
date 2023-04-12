package main

import (
	"fmt"
	"github.com/fatih/structs"
	"go.uber.org/zap"
	"regexp"
	"time"
	"strconv"
)

// Struct representing a single log entry (message can be a multi-line string)
type logEntry struct {
	Parser    *parser
	Triggered bool
	Filtered  bool
	Text      string
	LineNo    int       `structs:"LINENO"`
	// TODO: Support date and time
	Date      time.Time
	Time      time.Time
	Filename  string    `structs:"FILENAME"`
	Level     string    `structs:"LEVEL"`
	Thread    string    `structs:"THREAD"`
	Routine   string    `structs:"ROUTINE"`
	Process   string    `structs:"PROCESS"`
	Trace     string    `structs:"TRACE"`
	Message   string    `structs:"MESSAGE"`
}

// Parse a log line into a LogEntry object
func parseLogEntry(log *zap.SugaredLogger, parsers []parser, line string, lineNum int) (logEntry, int, error) {
	var entry logEntry
	var err error
	for i, parser := range parsers {
		entry, err = parser.Parse(log, line, lineNum)
		if err == nil {
			log.Debugf("Matched: i (%d): Regex (%s), Line (%s)", i, parser.regex, line)
			if entry.Filtered {
				log.Debugf("Filtered: i (%d): Filters (%v), Line (%s)", i, parser.filters, line)
			}
			if entry.Triggered {
				log.Debugf("Triggered: i (%d): Triggers (%v), Line (%s)", i, parser.triggers, line)
			}
			return entry, i, nil
		}
		log.Debugf("Not matched: %v", err)
	}
	return logEntry{}, 0, fmt.Errorf("No parser found for line (%s)", line)
}

// TODO: Support parsing structured logging
type parser struct {
	regex    string
	re       regexp.Regexp
	triggers []trigger
	filters  []filter
}

func newParser(log *zap.SugaredLogger, regex string, filtersRegex, triggersRegex map[string]string) (parser, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return parser{}, err
	}
	var filters []filter
	for k, v := range filtersRegex {
		filter, err := newFilter(k, v)
		if err != nil {
			return parser{}, err
		}
		filters = append(filters, filter)
	}
	var triggers []trigger
	for k, v := range triggersRegex {
		trigger, err := newTrigger(k, v)
		if err != nil {
			return parser{}, err
		}
		triggers = append(triggers, trigger)
	}
	log.Debugf("New parser: (%s)", regex)
	log.Debugf("Filters: (%v)", filters)
	log.Debugf("Triggers: (%v)", triggers)
	return parser{
		regex:    regex,
		re:       *re,
		filters:  filters,
		triggers: triggers,
	}, nil
}

func (p parser) Parse(log *zap.SugaredLogger, line string, lineNum int) (logEntry, error) {
	matches := p.re.FindStringSubmatch(line)
	if len(matches) == 0 {
		log.Debugf("Parser (%s) did not match line (%s)", p.regex, line)
		return logEntry{}, fmt.Errorf("parser with regex (%s) did not match line (%s)", p.regex, line)
	}
	result := make(map[string]string)
	for i, name := range p.re.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = matches[i]
			log.Debugf("Name: (%s), Match: (%s)", name, matches[i])
		}
	}
	_, ok := result["MESSAGE"]
	if !ok {
			log.Infof("Could not match MESSAGE in line (%s)", line)
	}
	entry := logEntry{
		Parser:  &p,
		Text:    line,
		LineNo:  lineNum,
		Message: result["MESSAGE"],
	}

	lineNoStr, ok := result["LINENO"]
	if ok {
		lineNo, err := strconv.Atoi(lineNoStr)
		if err != nil {
			log.Infof("Could not cast into int (%s) in line (%s)", lineNoStr, line)
		}
		entry.LineNo = lineNo
	}

	_, ok = result["FILENAME"]
	if ok {
		entry.Trace = result["FILENAME"]
	}

	_, ok = result["LEVEL"]
	if ok {
		entry.Level = result["LEVEL"]
	}

	_, ok = result["THREAD"]
	if ok {
		entry.Thread = result["THREAD"]
	}

	_, ok = result["ROUTINE"]
	if ok {
		entry.Routine = result["ROUTINE"]
	}

	_, ok = result["PROCESS"]
	if ok {
		entry.Routine = result["PROCESS"]
	}

	_, ok = result["TRACE"]
	if ok {
		entry.Trace = result["TRACE"]
	}

	// Set Filtered
	for _, filter := range p.filters {
		log.Debugf("Matching filter: (%v)", filter)
		if filter.Match(log, entry) {
			log.Debugf("Matched filter: (%v)", filter)
			entry.Filtered = true
			break
		}
	}

	// Set Triggered
	// TODO: Make this behave like an AND
	for _, trigger := range p.triggers {
		log.Debugf("Matching trigger: (%v)", trigger)
		if trigger.Match(log, entry) {
			log.Debugf("Matched trigger: (%v)", trigger)
			entry.Triggered = true
			break
		}
	}

	return entry, nil
}

// TODO: Composing multiple logical conditions in a single trigger
type filter struct {
	variable string
	re       regexp.Regexp
}

func newFilter(variable, regex string) (filter, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return filter{}, fmt.Errorf("regex is not valid (%s)", regex)
	}
	return filter{
		variable: variable,
		re:       *re,
	}, nil
}

// TODO: Composing multiple logical conditions in a single trigger
type trigger struct {
	variable string
	re       regexp.Regexp
}

func newTrigger(variable, regex string) (trigger, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return trigger{}, fmt.Errorf("regex is not valid (%s)", regex)
	}
	return trigger{
		variable: variable,
		re:       *re,
	}, nil
}

// TODO: Deduplicate this
func (f filter) Match(log *zap.SugaredLogger, entry logEntry) bool {
	// Decode entry into json field map
	m := structs.Map(entry)
	log.Debugf("Variable (%s) map (%v)", f.variable, m)
	value, ok := m[f.variable]
	if !ok {
		log.Debugf("Variable not found in entry (%s)", entry.Text)
		return false
	}
	// TODO: Support matching on other types
	castedValue, ok := value.(string)
	if !ok {
		log.Debugf("Could not cast to string (%v)", value)
		return false
	}
	log.Debugf("Trying to match regex (%v)", f.re)
	return f.re.MatchString(castedValue)
}

func (t trigger) Match(log *zap.SugaredLogger, entry logEntry) bool {
	// Decode entry into json field map
	m := structs.Map(entry)
	log.Debugf("Variable (%s) map (%v)", t.variable, m)
	value, ok := m[t.variable]
	if !ok {
		log.Debugf("Variable not found in entry (%s)", entry.Text)
		return false
	}
	// TODO: Support matching on other types
	castedValue, ok := value.(string)
	if !ok {
		log.Debugf("Could not cast to string (%v)", value)
		return false
	}
	log.Debugf("Trying to match regex (%v)", t.re)
	return t.re.MatchString(castedValue)
}

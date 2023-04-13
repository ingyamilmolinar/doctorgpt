package main

import (
	"fmt"
	"go.uber.org/zap"
	"regexp"
)

// Struct representing a single log entry (message can be a multi-line string)
type logEntry struct {
	Parser    *parser
	Triggered bool
	Filtered  bool
	Text      string
	LineNo    int
	// TODO: Support date and time
	// TODO: Support matching on other types
	Variables map[string]string
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
	regex     string
	re        regexp.Regexp
	variables []string
	triggers  []trigger
	filters   []filter
}

func newParser(log *zap.SugaredLogger, regex string, filtersRegex, triggersRegex map[string]string) (parser, error) {
	// TODO: Get variables and save them to map
	re, err := regexp.Compile(regex)
	if err != nil {
		return parser{}, err
	}

	var variables []string
	variableSet := map[string]bool{}
	for i, variable := range re.SubexpNames() {
		if i == 0 || variable == "" {
			continue
		}
		log.Debugf("Appending variable: (%s)", variable)
		variables = append(variables, variable)
		variableSet[variable] = true
	}

	var filters []filter
	for k, v := range filtersRegex {
		// check if variable is part of variable list
		_, ok := variableSet[k]
		if !ok {
			return parser{}, fmt.Errorf("variable (%s) in filter is not a regex variable", k)
		}
		filter, err := newFilter(k, v)
		if err != nil {
			return parser{}, err
		}
		filters = append(filters, filter)
	}

	var triggers []trigger
	for k, v := range triggersRegex {
		// check if variable is part of variable list
		_, ok := variableSet[k]
		if !ok {
			return parser{}, fmt.Errorf("variable (%s) in trigger is not a regex variable", k)
		}
		trigger, err := newTrigger(k, v)
		if err != nil {
			return parser{}, err
		}
		triggers = append(triggers, trigger)
	}

	log.Debugf("New parser: (%s)", regex)
	log.Debugf("Variables: (%v)", variables)
	log.Debugf("Filters: (%v)", filters)
	log.Debugf("Triggers: (%v)", triggers)
	return parser{
		regex:     regex,
		re:        *re,
		variables: variables,
		filters:   filters,
		triggers:  triggers,
	}, nil
}

func (p parser) Parse(log *zap.SugaredLogger, line string, lineNum int) (logEntry, error) {
	matches := p.re.FindStringSubmatch(line)
	if len(matches) == 0 {
		log.Debugf("Parser (%s) did not match line (%s)", p.regex, line)
		return logEntry{}, fmt.Errorf("parser with regex (%s) did not match line (%s)", p.regex, line)
	}

	result := make(map[string]string)
	for i, variable := range p.re.SubexpNames() {
		if i == 0 || variable == "" {
			continue
		}
		result[variable] = matches[i]
		log.Debugf("Variable: (%s), Match: (%s)", variable, matches[i])
	}

	entry := logEntry{
		Parser:    &p,
		Text:      line,
		LineNo:    lineNum,
		Variables: result,
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
	log.Debugf("Variable (%s) map (%v)", f.variable, entry.Variables)
	value, ok := entry.Variables[f.variable]
	if !ok {
		log.Debugf("Variable not found in entry (%s)", entry.Text)
		return false
	}
	log.Debugf("Trying to match regex (%v)", f.re)
	return f.re.MatchString(value)
}

// TODO: Deduplicate this
func (t trigger) Match(log *zap.SugaredLogger, entry logEntry) bool {
	// Decode entry into json field map
	log.Debugf("Variable (%s) map (%v)", t.variable, entry.Variables)
	value, ok := entry.Variables[t.variable]
	if !ok {
		log.Debugf("Variable not found in entry (%s)", entry.Text)
		return false
	}
	log.Debugf("Trying to match regex (%v)", t.re)
	return t.re.MatchString(value)
}

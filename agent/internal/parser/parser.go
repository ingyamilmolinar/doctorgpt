package parser

import (
	"fmt"
	"go.uber.org/zap"
	"regexp"
	"strconv"

	"github.com/ingyamilmolinar/doctorgpt/agent/internal/config"
)

// Struct representing a single log entry (message can be a multi-line string)
type LogEntry struct {
	Parser    *Parser
	Triggered bool
	Filtered  bool
	Excluded  bool
	Text      string
	LineNo    int
	// TODO: Support date and time
	// TODO: Support matching on other types
	Variables map[string]string
}

// Parse a log line into a LogEntry object
func ParseLogEntry(log *zap.SugaredLogger, parsers []Parser, line string, lineNum int) (LogEntry, int, error) {
	var entry LogEntry
	var err error
	for i, parser := range parsers {
		entry, err = parser.Parse(log, line, lineNum)
		if err == nil {
			log.Debugf("MATCHED: i (%d): Regex (%s), Line (%s)", i, parser.Regex, line)
			if entry.Filtered {
				log.Debugf("FILTERED: i (%d): Filters (%v), Line (%s)", i, parser.Filters, line)
			} else {
				log.Debugf("NOT FILTERED: i (%d): Filters (%v), Line (%s)", i, parser.Filters, line)
			}
			if entry.Triggered {
				log.Debugf("TRIGGERED: i (%d): Triggers (%v), Line (%s)", i, parser.Triggers, line)
			} else {
				log.Debugf("NOT TRIGGERED: i (%d): Triggers (%v), Line (%s)", i, parser.Triggers, line)
			}
			if entry.Excluded {
				log.Debugf("EXCLUDED: i (%d): Excludes (%v), Line (%s)", i, parser.Excludes, line)
			} else {
				log.Debugf("NOT EXCLUDED: i (%d): Excludes (%v), Line (%s)", i, parser.Excludes, line)
			}
			return entry, i, nil
		}
		log.Debugf("Not matched: %v", err)
	}
	return LogEntry{}, 0, fmt.Errorf("No parser found for line (%s)", line)
}

// TODO: Support parsing structured logging
type Parser struct {
	Regex     string
	Re        regexp.Regexp
	Variables []string
	Triggers  []Matcher
	Filters   []Matcher
	Excludes  []Matcher
}

func NewParser(log *zap.SugaredLogger, regex string, filtersRegex, triggersRegex, excludesRegex []config.VariableMatcher) (Parser, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return Parser{}, err
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

	// We add a special LINENO variable to track the log line num
	variableSet["LINENO"] = true

	var filters []Matcher
	for _, filter := range filtersRegex {
		variable := filter.Variable
		regex := filter.Regex
		// check if variable is part of variable list
		_, ok := variableSet[variable]
		if !ok {
			return Parser{}, fmt.Errorf("variable (%s) in filter is not a regex variable", variable)
		}
		filter, err := newMatcher(log, variable, regex)
		if err != nil {
			return Parser{}, err
		}
		filters = append(filters, filter)
	}

	var triggers []Matcher
	for _, trigger := range triggersRegex {
		variable := trigger.Variable
		regex := trigger.Regex
		// check if variable is part of variable list
		_, ok := variableSet[variable]
		if !ok {
			return Parser{}, fmt.Errorf("variable (%s) in trigger is not a regex variable", variable)
		}
		trigger, err := newMatcher(log, variable, regex)
		if err != nil {
			return Parser{}, err
		}
		triggers = append(triggers, trigger)
	}

	var excludes []Matcher
	for _, exclude := range excludesRegex {
		variable := exclude.Variable
		regex := exclude.Regex
		// check if variable is part of variable list
		_, ok := variableSet[variable]
		if !ok {
			return Parser{}, fmt.Errorf("variable (%s) in exclude is not a regex variable", variable)
		}
		exclude, err := newMatcher(log, variable, regex)
		if err != nil {
			return Parser{}, err
		}
		excludes = append(excludes, exclude)
	}

	log.Debugf("New parser: (%s)", regex)
	log.Debugf("Variables: (%v)", variables)
	log.Debugf("Filters: (%v)", filters)
	log.Debugf("Triggers: (%v)", triggers)
	log.Debugf("Excludes: (%v)", excludes)
	return Parser{
		Regex:     regex,
		Re:        *re,
		Variables: variables,
		Filters:   filters,
		Triggers:  triggers,
		Excludes:  excludes,
	}, nil
}

func (p Parser) Parse(log *zap.SugaredLogger, line string, lineNum int) (LogEntry, error) {
	matches := p.Re.FindStringSubmatch(line)
	if len(matches) == 0 {
		log.Debugf("Parser (%s) did not match line (%s)", p.Regex, line)
		return LogEntry{}, fmt.Errorf("parser with regex (%s) did not match line (%s)", p.Regex, line)
	}

	result := make(map[string]string)
	for i, variable := range p.Re.SubexpNames() {
		if i == 0 || variable == "" {
			continue
		}
		result[variable] = matches[i]
		log.Debugf("Variable: (%s), Match: (%s)", variable, matches[i])
	}

	// We add a special LINENO variable to match on line num
	result["LINENO"] = strconv.Itoa(lineNum)
	log.Debugf("Variable: (%s), Match: (%s)", "LINENO", strconv.Itoa(lineNum))

	entry := LogEntry{
		Parser:    &p,
		Text:      line,
		LineNo:    lineNum,
		Variables: result,
	}

	// Set Filtered
	// TODO: Support boolean primitives
	for _, filter := range p.Filters {
		log.Debugf("Matching filter: (%v)", filter)
		if filter.Match(entry) {
			log.Debugf("Matched filter: (%v)", filter)
			entry.Filtered = true
			break
		}
	}

	// Set Triggered
	// TODO: Support boolean primitives
	for _, trigger := range p.Triggers {
		log.Debugf("Matching trigger: (%v)", trigger)
		if trigger.Match(entry) {
			log.Debugf("Matched trigger: (%v)", trigger)
			entry.Triggered = true
			break
		}
	}

	// Set excluded
	// TODO: Support boolean primitives
	for _, exclude := range p.Excludes {
		log.Debugf("Matching exclude: (%v)", exclude)
		if exclude.Match(entry) {
			log.Debugf("Matched exclude: (%v)", exclude)
			entry.Excluded = true
			break
		}
	}

	return entry, nil
}

// TODO: Composing multiple logical conditions in a single trigger
type matcher struct {
	variable string
	re       regexp.Regexp
	log      *zap.SugaredLogger
}

type Matcher interface {
	Match(entry LogEntry) bool
}

func newMatcher(log *zap.SugaredLogger, variable, regex string) (Matcher, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return matcher{}, fmt.Errorf("regex is not valid (%s)", regex)
	}
	return matcher{
		variable: variable,
		re:       *re,
		log:      log,
	}, nil
}

func (m matcher) Match(entry LogEntry) bool {
	// Decode entry into json field map
	m.log.Debugf("Variable (%s) map (%v)", m.variable, entry.Variables)
	value, ok := entry.Variables[m.variable]
	if !ok {
		m.log.Debugf("Variable not found in entry (%s)", entry.Text)
		return false
	}
	m.log.Debugf("Trying to match regex (%s)", m.re.String())
	return m.re.MatchString(value)
}

func Stringify(entries []LogEntry) string {
	var result string
	for _, entry := range entries {
		result += entry.Text + "\n"
	}
	return result
}

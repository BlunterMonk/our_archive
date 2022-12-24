package script

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	ScriptMarkerRegexFormat = `^\[([a-zA-Z0-9]+)\s-\s([a-z0-9_]+)\s-\s([_a-z0-9]+)\]$`

	// format: [subject - category - action]
	// ScriptMarkerRegexFormat = `^\[([a-zA-Z0-9]+)\s-\s([a-z0-9]+)\s-\s([a-z]+)\]$`
)

type Script struct {
	elements []ScriptElement
}

type ScriptElement struct {
	Index  int
	Name   string // name of speaker
	Mood   string // variant of facial expression that is used
	Action string // used when emoticons/special expressions are shown
	Line   string // text for line
	Lines  []string
}

func NewScriptFromFile(filename string) *Script {

	/*f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Compile the expression once, usually at init time.
	// Use raw strings to avoid having to quote the backslashes.
	var validID = regexp.MustCompile(ScriptMarkerRegexFormat)

	lines := make(map[string]ScriptElement, 0)

	var index int
	var name, mood string
	var expn string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		row := strings.Trim(scanner.Text(), " ")
		match := validID.MatchString(row)
		if match {
			// format: [name - mood - expression]
			values := validID.FindAllStringSubmatch(row, -1)
			log.Printf("Dialogue Count[%v]: %v\n", index, row)
			name = values[0][1]
			mood = values[0][2]
			expn = values[0][3]
			continue
		}

		ind := fmt.Sprint(index)
		lines[ind] = ScriptElement{
			Index: index,
			Name:  name,
			Mood:  mood,
			Expn:  expn,
			Line:  row,
		}
		index++
		expn = ""
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}*/
	e := LoadScript(filename)
	return &Script{
		elements: e,
	}
}

// returns:
// map of script
// number of dialogue found
// number of options found
// number of double options found
// number of titles found
func LoadScript(filename string) []ScriptElement {

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Compile the expression once, usually at init time.
	// Use raw strings to avoid having to quote the backslashes.
	var validID = regexp.MustCompile(ScriptMarkerRegexFormat)

	lines := make([]ScriptElement, 0)

	index := -1
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		row := strings.Trim(scanner.Text(), " ")
		match := validID.MatchString(row)
		if match {
			index++
			log.Printf("Dialogue Count[%v]: %v\n", index, row)

			values := validID.FindAllStringSubmatch(row, -1)
			name := values[0][1]
			category := values[0][2]
			action := values[0][3]

			lines = append(lines, ScriptElement{
				Index:  index,
				Name:   strings.ToLower(name),
				Mood:   strings.ToLower(category),
				Action: strings.ToLower(action),
				Lines:  make([]string, 0),
			})

		} else {
			log.Printf("Dialogue Text[%v]: %v\n", index, row)

			lines[index].Line = lines[index].Line + row
			lines[index].Lines = append(lines[index].Lines, row)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return lines
}

func (s *Script) Get(index int) ScriptElement {
	return s.elements[index]
}

func (s *Script) Elements() []ScriptElement {
	return s.elements
}

func (s *ScriptElement) ToString() string {
	return fmt.Sprintf("[%v - %v - %v]: %v", s.Name, s.Mood, s.Action, s.Line)
}

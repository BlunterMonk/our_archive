package script

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	ScriptMarkerRegexFormat = `^\[([?a-zA-Z0-9_]+)\s-\s([a-zA-Z0-9_]+)\s-\s([_a-zA-Z0-9*'\s".]+)\]$`
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

type Metadata struct {
	Actors    []ActorMetadata     `json:"actor"`
	Animation []AnimationMetadata `json:"animation"`
	Emotes    []EmoteMetadata     `json:"emote"`
}
type ActorMetadata struct {
	Name              string   `json:"name"`
	FactionName       *string  `json:"faction_name"`
	CenterX           float32  `json:"center_x"`
	CenterY           float32  `json:"center_y"`
	CenterScale       float32  `json:"center_scale"`
	EmoteOffsetHead   Position `json:"emote_offset_head"`
	EmoteOffsetBubble Position `json:"emote_offset_bubble"`
}
type AnimationMetadata struct {
	Name   string          `json:"name"`
	Speed  float32         `json:"speed"`
	Frames []FrameMetadata `json:"frames"`
}
type FrameMetadata struct {
	X      *float32 `json:"x"`
	Y      *float32 `json:"y"`
	AddX   *float32 `json:"add_x"`
	AddY   *float32 `json:"add_y"`
	Delay  *float32 `json:"delay"`
	Reset  bool     `json:"reset"`
	Center bool     `json:"center"`
}
type EmoteMetadata struct {
	Name  string  `json:"name"`
	Scale float32 `json:"scale"`
	Type  string  `json:"type"`
}
type Position struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

func NewScriptFromFile(filename string) *Script {
	e := LoadScript(filename)
	return &Script{
		elements: e,
	}
}

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
			// log.Printf("Dialogue Count[%v]: %v\n", index, row)

			values := validID.FindAllStringSubmatch(row, -1)
			name := strings.ToLower(values[0][1])
			category := strings.ToLower(values[0][2])

			action := values[0][3]
			switch name {
			case "defect":
				break
			default:
				action = strings.ToLower(action)
			}

			lines = append(lines, ScriptElement{
				Index:  index,
				Name:   name,
				Mood:   category,
				Action: action,
				Lines:  make([]string, 0),
			})

		} else {
			// log.Printf("Dialogue Text[%v]: %v\n", index, row)

			lines[index].Line = lines[index].Line + row
			lines[index].Lines = append(lines[index].Lines, row)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return lines
}

func LoadMetadata(filename string) (*Metadata, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var meta Metadata
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return nil, err
	}

	return &meta, nil
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

func (s *ScriptElement) HasDialogue() bool {
	return s.Line != "" || len(s.Lines) > 0
}

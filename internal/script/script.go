package script

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const (
	ScriptMarkerRegexFormat = `^\[([?a-zA-Z0-9_]+)\s-\s([a-zA-Z0-9_]+)\s-\s([_a-zA-Z0-9-*'\s".?!]+)\]$`
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
	Actors       map[string]ActorMetadata     `json:"actors"`
	Animations   map[string]AnimationMetadata `json:"animations"`
	Emotes       map[string]EmoteMetadata     `json:"emotes"`
	ActorOld     []ActorMetadata              `json:"actor,omitempty"`
	AnimationOld []AnimationMetadata          `json:"animation,omitempty"`
	EmoteOld     []EmoteMetadata              `json:"emote,omitempty"`
}
type ActorMetadata struct {
	Name              string   `json:"name,omitempty"`
	FactionName       *string  `json:"faction_name,omitempty"`
	CenterX           float32  `json:"center_x,omitempty"`
	CenterY           float32  `json:"center_y,omitempty"`
	CenterScale       float32  `json:"center_scale,omitempty"`
	EmoteOffsetHead   Position `json:"emote_offset_head,omitempty"`
	EmoteOffsetBubble Position `json:"emote_offset_bubble,omitempty"`
}
type AnimationMetadata struct {
	Name   string          `json:"name,omitempty"`
	Speed  float32         `json:"speed"`
	Frames []FrameMetadata `json:"frames"`
}
type FrameMetadata struct {
	X      *float32 `json:"x,omitempty"`
	Y      *float32 `json:"y,omitempty"`
	AddX   *float32 `json:"add_x,omitempty"`
	AddY   *float32 `json:"add_y,omitempty"`
	Scale  *float32 `json:"scale,omitempty"`
	Delay  *float32 `json:"delay,omitempty"`
	Reset  bool     `json:"reset,omitempty"`
	Center bool     `json:"center,omitempty"`
}
type EmoteMetadata struct {
	Name  string  `json:"name,omitempty"`
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
			if name != "defect" && category != "defect" {
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

	newFile := "./resources/settings-new.json"
	newFileErr := fmt.Errorf("old settings format detected, Arona converted it for you and saved it to: %s", newFile)
	actorOld := make([]ActorMetadata, 0)
	animOld := make([]AnimationMetadata, 0)
	emoteOld := make([]EmoteMetadata, 0)
	if len(meta.ActorOld) > 0 {
		if meta.Actors == nil {
			meta.Actors = make(map[string]ActorMetadata)
		}
		for _, v := range meta.ActorOld {
			if _, ok := meta.Actors[v.Name]; !ok {
				n := v.Name
				v.Name = ""
				meta.Actors[n] = v
			} else {
				actorOld = append(actorOld, v)
				err = errors.Wrapf(err, "duplicate actor setting: \"%s\"", v.Name)
			}
		}
		err = newFileErr
	}
	if len(meta.EmoteOld) > 0 {
		if meta.Emotes == nil {
			meta.Emotes = make(map[string]EmoteMetadata)
		}
		for _, v := range meta.EmoteOld {
			n := v.Name
			if _, ok := meta.Emotes[n]; !ok {
				v.Name = ""
				meta.Emotes[n] = v
			} else {
				emoteOld = append(emoteOld, v)
				err = errors.Wrapf(err, "duplicate emote setting: \"%s\"", v.Name)
			}
		}
		err = newFileErr
	}
	if len(meta.AnimationOld) > 0 {
		if meta.Animations == nil {
			meta.Animations = make(map[string]AnimationMetadata)
		}
		for _, v := range meta.AnimationOld {
			if _, ok := meta.Animations[v.Name]; !ok {
				n := v.Name
				v.Name = ""
				meta.Animations[n] = v
			} else {
				animOld = append(animOld, v)
				err = errors.Wrapf(err, "duplicate animation setting: \"%s\"", v.Name)
			}
		}
		err = newFileErr
	}
	if err != nil { // if we get an error here it's because there were some script cleanup to do
		newMeta := Metadata{
			Actors:       meta.Actors,
			Animations:   meta.Animations,
			Emotes:       meta.Emotes,
			ActorOld:     actorOld,
			EmoteOld:     emoteOld,
			AnimationOld: animOld,
		}

		newData, err := json.MarshalIndent(newMeta, "", "	")
		if err != nil {
			return &meta, errors.Wrap(err, "tried to save the new settings file but something happened")
		}
		err = os.WriteFile(newFile, newData, 0777)
		if err != nil {
			return &meta, errors.Wrap(err, "tried to save the new settings file but something happened")
		}
	}

	return &meta, err
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

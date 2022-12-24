package hud

import (
	"bytes"
	"strings"
	"time"
	"unicode"

	v41 "github.com/4ydx/gltext/v4.1"
	"github.com/go-gl/mathgl/mgl32"
)

type Text struct {
	tx          float32     // target X position
	ty          float32     // target Y position
	done        bool        // is done animating
	Text        []string    // total text to display
	Output      []string    // starts empty, is filled with the text that should be displayed after typewriter effect
	TextObjects []*v41.Text // screen space render objects for each line of text
	Font        *v41.Font   // font used
}

const (
	nbsp = 0xA0
	// TODO: make this a variable for when the window changes size
	WINDOW_WIDTH  = 1280
	WINDOW_HEIGHT = 720
)

var (
	COLOR_WHITE = mgl32.Vec3{1, 1, 1}
	COLOR_BLACK = mgl32.Vec3{0, 0, 0}
)

func NewText(content string, color mgl32.Vec3, font *v41.Font) *Text {

	// setup text output
	lines := strings.Split(wrapString(content, 79), "\n")
	dialogue := make([]string, 0)
	// create text objects to display
	textObjects := make([]*v41.Text, 0)
	for i := 0; i < len(lines); i++ {
		text := v41.NewText(font, 0.1, 1.0)
		text.SetColor(color)
		text.Show()

		dialogue = append(dialogue, "")
		textObjects = append(textObjects, text)
	}

	return &Text{
		Text:        lines,
		Output:      dialogue,
		TextObjects: textObjects,
	}
}

func NewSolidText(content string, color mgl32.Vec3, font *v41.Font) *Text {
	s := NewText(content, color, font)
	s.Output = s.Text
	return s
}

func (s *Text) SetText(content string) {

	lines := strings.Split(wrapString(content, 79), "\n")
	// dialogue := make([]string, 0)
	// create text objects to display
	// textObjects := make([]*v41.Text, 0)
	if len(s.TextObjects) < len(lines) {
		for i := 0; i < len(lines); i++ {
			text := v41.NewText(s.Font, 0.1, 1.0)
			text.SetColor(mgl32.Vec3{1, 1, 1})
			text.SetScale(1.0)
			text.Hide()

			s.Output = append(s.Output, "")
			s.TextObjects = append(s.TextObjects, text)
		}
	}

	s.Text = lines
}

func (s *Text) SetScale(scale float32) {
	for _, v := range s.TextObjects {
		v.SetScale(scale)
	}
}

func (s *Text) Width() float32 {

	var max float32
	for i := 0; i < len(s.Text); i++ {
		txt := s.TextObjects[i]
		txt.SetString(s.Output[i])
		t, b := txt.GetBoundingBox()
		w := b.X - t.X
		// h := b.Y - t.Y
		if w > max {
			max = w
		}
	}

	return max
}

func (s *Text) Draw(tx, ty float32) {

	wh := float32(WINDOW_HEIGHT * 0.5)
	ww := float32(WINDOW_WIDTH * 0.5)
	lineSpacing := float32(25.0)
	for i := 0; i < len(s.TextObjects); i++ {
		txt := s.TextObjects[i]
		txt.SetString(s.Output[i])
		t, b := txt.GetBoundingBox()
		w := b.X - t.X
		h := b.Y - t.Y
		x := (tx + (w * txt.Scale * 0.5)) - ww
		y := ((wh - (h * 0.5)) - (float32(i) * (h + lineSpacing) * 0.5)) - ty

		// fmt.Printf("Text Position: (%v, %v)\n", x, y)
		txt.SetPosition(mgl32.Vec2{x, y})
		txt.Draw()
	}
}

func (s *Text) Release() {
	for _, v := range s.TextObjects {
		v.Release()
	}
}

func (s *Text) Done() bool {
	return s.done
}

// func (s *Text) Complete() {
// 	s.done = false

// 	// how fast the text should display
// 	tick := time.Tick(32 * time.Millisecond)

// 	lines := s.Text
// 	output := &s.Output

// 	// chop up the string
// 	lineCount := len(lines)

// 	(*output) = lines

// 	for index := 0; index < lineCount; index++ {
// 		line := lines[index]
// 		runes := []rune(line)
// 		count := len(runes)
// 		displayed := make([]rune, 0)
// 		c := 0

// 		s.TextObjects[index].Show()

// 		for len(displayed) != count {
// 			select {
// 			case <-tick:
// 				c++
// 				break
// 			}
// 		}
// 	}

// 	s.done = true
// 	*status <- 1 // send status update to listening channel
// 	return nil
// }

// AsyncAnimate - Asynchronous function used to animate text
func (s *Text) AsyncAnimate(status *chan uint32) {
	// TODO: this is setup this way in case more animations are added
	go animateTypewriter(s, status)
}

func animateTypewriter(s *Text, status *chan uint32) error {
	s.done = false

	// how fast the text should display
	tick := time.Tick(32 * time.Millisecond)

	lines := s.Text
	output := &s.Output

	// chop up the string
	lineCount := len(lines)

	for index := 0; index < lineCount; index++ {
		line := lines[index]
		runes := []rune(line)
		count := len(runes)
		displayed := make([]rune, 0)
		c := 0

		s.TextObjects[index].Show()

		for len(displayed) != count {
			select {
			case <-tick:

				displayed = append(displayed, runes[c])
				c++

				// return string to source
				(*output)[index] = string(displayed)
				break
			}
		}
	}

	sec := time.NewTimer(time.Second)
	<-sec.C

	s.done = true
	*status <- 1 // send status update to listening channel
	return nil
}

// wrapString wraps the given string within lim width in characters.
//
// Wrapping is currently naive and only happens at white-space. A future
// version of the library will implement smarter wrapping. This means that
// pathological cases can dramatically reach past the limit, such as a very
// long word.
func wrapString(s string, lim uint) string {

	// Initialize a buffer with a slightly larger size to account for breaks
	init := make([]byte, 0, len(s))
	buf := bytes.NewBuffer(init)

	var current uint
	var wordBuf, spaceBuf bytes.Buffer
	var wordBufLen, spaceBufLen uint

	for _, char := range s {
		if char == '\n' {
			if wordBuf.Len() == 0 {
				if current+spaceBufLen > lim {
					current = 0
				} else {
					current += spaceBufLen
					spaceBuf.WriteTo(buf)
				}
				spaceBuf.Reset()
				spaceBufLen = 0
			} else {
				current += spaceBufLen + wordBufLen
				spaceBuf.WriteTo(buf)
				spaceBuf.Reset()
				spaceBufLen = 0
				wordBuf.WriteTo(buf)
				wordBuf.Reset()
				wordBufLen = 0
			}
			buf.WriteRune(char)
			current = 0
		} else if unicode.IsSpace(char) && char != nbsp {
			if spaceBuf.Len() == 0 || wordBuf.Len() > 0 {
				current += spaceBufLen + wordBufLen
				spaceBuf.WriteTo(buf)
				spaceBuf.Reset()
				spaceBufLen = 0
				wordBuf.WriteTo(buf)
				wordBuf.Reset()
				wordBufLen = 0
			}

			spaceBuf.WriteRune(char)
			spaceBufLen++
		} else {
			wordBuf.WriteRune(char)
			wordBufLen++

			if current+wordBufLen+spaceBufLen > lim && wordBufLen < lim {
				buf.WriteRune('\n')
				current = 0
				spaceBuf.Reset()
				spaceBufLen = 0
			}
		}
	}

	if wordBuf.Len() == 0 {
		if current+spaceBufLen <= lim {
			spaceBuf.WriteTo(buf)
		}
	} else {
		spaceBuf.WriteTo(buf)
		wordBuf.WriteTo(buf)
	}

	return buf.String()
}

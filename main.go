package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	v41 "github.com/4ydx/gltext/v4.1"
	"github.com/BlunterMonk/opengl/internal/hud"
	"github.com/BlunterMonk/opengl/internal/script"
	"github.com/BlunterMonk/opengl/pkg/gfx"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/ungerik/go3d/mat4"
)

var (
	wg  sync.WaitGroup
	ctx context.Context
	mtx sync.Mutex

	dialogue, reply, subjectName, mikaName *hud.Text
	dialogueDone                           bool
	dialogueIndex                          = -1
	CurrentSpeaker                         string

	Script          *script.Script
	status          = make(chan uint32)
	UniversalTicker = time.Tick(16 * time.Millisecond)

	EventQueue          = make([]eventFunc, 0)
	font                *v41.Font
	bg, fade            *hud.Sprite
	charSprite          map[string]*hud.Sprite
	Actors, Backgrounds map[string]*hud.Sprite
	Sounds              map[string]*Streamer
	Names               map[string]*hud.Text

	shuttingDown         bool
	useStrictCoreProfile = (runtime.GOOS == "darwin")

	ACTOR_LEFT  = mgl32.Vec3{-0.5, -0.65, 0.0}
	ACTOR_RIGHT = mgl32.Vec3{0.5, -0.65, 0.0}
)

const (
	CURRENT_SCRIPT = "newyear-mid"

	XCODE_SHUTDOWN_SIGNAL = 0
	XCODE_CONSUMER_FAILED = 4
	XCODE_PANIC           = 5
	XCODE_ABORT           = 6

	WINDOW_WIDTH  = 1280
	WINDOW_HEIGHT = 720
)

type eventFunc func(s *hud.Text)
type Streamer struct {
	beep.StreamSeekCloser

	data   []byte
	format beep.Format
}

func (s *Streamer) Play() {
	speaker.Init(s.format.SampleRate, s.format.SampleRate.N(time.Second/10))
	speaker.Play(s)
}
func (s *Streamer) Release() {
	s.Close()
}

func ThemeFilePath(n string) string {
	return fmt.Sprintf("./resources/bgm/Theme_%s.mp3", n)
}
func NewStreamer(filename string) *Streamer {

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	body, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	buff := ioutil.NopCloser(bytes.NewBuffer(body))
	streamer, format, err := mp3.Decode(buff)
	if err != nil {
		log.Fatal(err)
	}

	return &Streamer{
		StreamSeekCloser: streamer,
		data:             body,
		format:           format,
	}
}

// in Open GL, Y starts at the bottom

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

func main() {
	var xCode int

	window := gfx.Init()
	window.SetKeyCallback(keyCallback)
	window.SetMouseButtonCallback(mouseButtonCallback)

	// code from here
	font = gfx.MustLoadFont("NotoSans-Medium")
	font.ResizeWindow(float32(WINDOW_WIDTH), float32(WINDOW_HEIGHT))
	shaderProgram := gfx.MustInitShader()

	// start := time.Now()

	gl.ClearColor(0.4, 0.4, 0.4, 0.0)

	ft := time.Tick(time.Second)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)

	killswitch := make(chan int, 0)

	// Catch any panics
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		log.Println("app panicked!")
	// 		log.Println(r)
	// 		os.Exit(XCODE_PANIC)
	// 	}
	// }()

	go func() {
		for {
			select {
			// Shut down when we are signaled
			case <-sigc:
				log.Println("received a shutdown signal!")
				killswitch <- 0
				return
			}
		}
	}()

	counter := 0

	// setup text output
	/* script structure
	* [actor - mood - action]
	* example of background: [bg - black_screen - none]
	* exmaple of character: [mika - 03 - heart]
	 */
	Script = script.NewScriptFromFile(fmt.Sprintf("./resources/scripts/%s.txt", CURRENT_SCRIPT))
	charSprite = make(map[string]*hud.Sprite)
	Actors = make(map[string]*hud.Sprite)
	Names = make(map[string]*hud.Text)
	Sounds = make(map[string]*Streamer)
	Backgrounds = make(map[string]*hud.Sprite)
	for _, v := range Script.Elements() {
		// create names if they don't exist
		if _, ok := Names[v.Name]; !ok {
			log.Println("initializing name:", v.Name)
			Names[v.Name] = hud.NewSolidText(toTitle(v.Name), hud.COLOR_WHITE, font)
			Names[v.Name].SetScale(1)
			// mikaName2 := hud.NewSolidText("Tea Party", mgl32.Vec3{0.45, 0.69, 0.83}, font)
			// mikaName2.SetScale(0.85)
		}

		if v.Mood == "_" {
			continue
		}

		key := spriteKey(v)
		if _, ok := Actors[v.Name]; !ok {
			Actors[v.Name] = hud.NewSprite()
		}

		switch v.Name {
		case "bg":
			Backgrounds[v.Mood] = hud.NewSpriteFromFile(fmt.Sprintf("./resources/bg/%v.jpeg", v.Mood))
			Actors[v.Name].LoadTexture(key, fmt.Sprintf("./resources/bg/%s.jpeg", v.Mood))
		case "bgm":
			fmt.Println("loaded bgm:", v.Mood)
			Sounds[v.Mood] = NewStreamer(ThemeFilePath(v.Mood))
		default:
			Actors[v.Name].LoadTexture(key, fmt.Sprintf("./resources/actor/%s/%s-%s.png", v.Name, v.Name, v.Mood))
		}

		// second switch to establish starting values
		// @TODO: maybe figure out a way to put this in the script
		switch v.Name {
		case "mika":
			Actors[v.Name].SetPositionf(-1.5, -0.65, 0)
		case "seia":
			Actors[v.Name].SetPositionf(1.5, -0.65, 0)
			Actors[v.Name].SetScale(0.8)
		}
	}

	dialogue = hud.NewText(Script.Get(0).Line, hud.COLOR_WHITE, font)
	dialogue.SetScale(0.85)
	dialogue.AsyncAnimate(&status)

	// load sprites
	bg = Backgrounds["black_screen"]
	fade = hud.NewSpriteFromFile("./resources/bg/black_screen.jpeg")
	fade.SetPositionf(0, 0, 0)
	fade.SetAlpha(1)
	overlay := hud.NewSpriteFromFile("./resources/ui/text_overlay.png")
	opSingle := hud.NewSpriteFromFile("./resources/ui/text_option_single.png")

	mikaName = hud.NewSolidText("Mika", hud.COLOR_WHITE, font)
	mikaName.SetScale(1)
	mikaName2 := hud.NewSolidText("Tea Party", mgl32.Vec3{0.45, 0.69, 0.83}, font)
	mikaName2.SetScale(0.85)

	gl.BlendColor(1, 1, 1, 1)
	/*
		f, err := os.Open("./resources/bgm/Theme_52.mp3")
		if err != nil {
			log.Fatal(err)
		}

		streamer, format, err := mp3.Decode(f)
		if err != nil {
			log.Fatal(err)
		}
		defer streamer.Close()

		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

		done := make(chan bool)
		speaker.Play(beep.Seq(streamer, beep.Callback(func() {
			done <- true
		})))*/

	var delay time.Timer
F:
	for {
		if window.ShouldClose() {
			break F
		}

		gl.Clear(gl.COLOR_BUFFER_BIT)
		mtx.Lock()
		for _, v := range EventQueue {
			v(dialogue)
		}
		EventQueue = make([]eventFunc, 0)
		mtx.Unlock()

		select {
		case <-ft:
			// text.SetString(fmt.Sprint(counter))
			counter = 0

		// events must be handled on the main thread if they interact with OpenGL
		// this is a limitation on the OpenGL system where it will panic if changes are made by different threads
		case s := <-status:
			log.Printf("received a status update: %v\n", s)
			if s == 1 {
				delay = *time.NewTimer(time.Second)
			}
		case <-delay.C:
			if dialogue != nil && dialogue.Done() {
				nextDialogue(&status)
			}

		case <-UniversalTicker:
			counter++

			// enable shader
			shaderProgram.Use()
			// current := Script.Get(dialogueIndex)

			gl.Enable(gl.BLEND) //Enable blending.
			gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

			// draw image
			if bg != nil {
				DrawSprite(bg, mat4.From(&mat4.Ident), shaderProgram) // background
			}
			for name, actor := range charSprite {
				if actor != nil {
					// slightly discolor whoever isn't talking
					if name == CurrentSpeaker {
						actor.SetColorf(1, 1, 1)
					} else {
						actor.SetColorf(0.9, 0.9, 0.9)
					}

					// actor.DrawTexture(actor.GetTransform(), shaderProgram, spriteKey())
					DrawSprite(actor, actor.GetTransform(), shaderProgram)
				}
			}

			if reply != nil {
				origin := mat4.From(&mat4.Ident)
				DrawSprite(opSingle, origin, shaderProgram)
			}

			// DrawSprite(fade, fade.GetTransform(), shaderProgram)

			// Draw text
			if dialogue != nil {
				DrawSprite(overlay, mat4.From(&mat4.Ident), shaderProgram) // dialogue window
				DrawText(dialogue, 125, 565)
				if subjectName != nil {
					DrawText(subjectName, 125, 525)
					// DrawText(mikaName2, 125+subjectName.Width()+5, 525)
				}
			}
			if reply != nil {
				DrawText(reply, (WINDOW_WIDTH/2)-(reply.Width()/2)+25, 280) // @TODO: figure out bounds of text dynamically
			}

			if fade != nil {
				// DrawSprite(fade, mat4.From(&mat4.Ident), shaderProgram) // dialogue window
			}

			// end of draw loop
			window.SwapBuffers()
			glfw.PollEvents()
			break
		case xCode = <-killswitch:
			log.Println("app killswitch")
			break F
		}
	}

	shuttingDown = true
	wg.Wait()

	// clean up resources
	if dialogue != nil {
		dialogue.Release()
		dialogue = nil
	}
	if reply != nil {
		reply.Release()
		reply = nil
	}
	font.Release()
	shaderProgram.Delete()
	for _, v := range Sounds {
		v.Release()
	}
	os.Exit(xCode)
}

func keyCallback(window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {

	// When a user presses the escape key, we set the WindowShouldClose property to true,
	// which closes the application
	if key == glfw.KeyEscape && action == glfw.Press {
		window.SetShouldClose(true)
	}
}
func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
	// log.Printf("mouseButtonCallback: button(%v), action(%v)\n", button, action)

	if action == glfw.Release {
		nextDialogue(&status)
	}
}

func queueEvent(event eventFunc) {
	EventQueue = append(EventQueue, event)
}
func nextDialogue(status *chan uint32) {

	// fmt.Println("starting dialogue goroutine")
	if reply != nil {
		reply.Release()
		reply = nil
	}

	dialogueIndex++
	if len(Script.Elements()) <= dialogueIndex {
		return
		log.Println("script finished, closing app")
		os.Exit(0)
	}
	element := Script.Get(dialogueIndex)
	log.Printf("next line: %v\n", element.ToString())

	sample := element.Line
	switch element.Name {
	case "clear":
		clear()
		nextDialogue(status)
		break
	case "bg":
		bg = Backgrounds[element.Mood]
		<-time.NewTimer(time.Second).C
		nextDialogue(status)
		break
	case "bgm":
		if s, ok := Sounds[element.Mood]; ok {
			fmt.Println("playing sound:", element.Mood)
			s.Play()
		}
		break
	case "fade":
		if element.Action == "in" {
			go AsyncAnimateFadeIn(fade, status)
		} else {
			go AsyncAnimateFadeOut(fade, status)
		}
		break
	case "sensei":
		reply = hud.NewSolidText(sample, hud.COLOR_BLACK, font)
		reply.SetScale(0.85)
		delayNextDialogue(status, 2)
		break
	case "none":
		if dialogue != nil {
			dialogue.Release()
			dialogue = nil
		}
		delayNextDialogue(status, 1)
		break
	default:
		CurrentSpeaker = element.Name

		// if the current dialogue actor has a name generated then set the current displayed name to it
		if t, ok := Names[element.Name]; ok {
			subjectName = t
		} else {
			subjectName = nil
		}

		// only display dialogue if there is dialogue
		if element.Line != "" && len(element.Lines) > 0 {
			dialogue = hud.NewText(sample, hud.COLOR_WHITE, font)
			// dialogue.SetText(sample)
			dialogue.SetScale(0.85)
			dialogue.AsyncAnimate(status)
		}

		// break early when the current dialogue has no sprite associated with it
		if _, ok := Actors[element.Name]; !ok {
			break
		}

		// do not display sprites if the mood is blank
		// this indicates that the actor is off screen
		if element.Mood == "_" {
			delayNextDialogue(status, 1)
			break
		}

		charSprite[element.Name] = Actors[element.Name]

		// fmt.Println(charSprite[element.Name])
		// fmt.Println(Actors[element.Name])
		// fmt.Println("---------------------")
		err := charSprite[element.Name].SetActiveTexture(spriteKey(element))
		if err != nil {
			panic(err)
		}

		charPos := charSprite[element.Name].GetPosition()

		// @TODO: maybe find a better workaround for this
		// need to specificy async operations vs non-async on the script
		async := false
		action := element.Action
		if strings.HasSuffix(action, "_async") {
			async = true
			action = strings.TrimSuffix(action, "_async")
		}
		switch action {
		case "exit_left":
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{-2, charPos.Y(), charPos.Z()}, 0.1, status)
			break
		case "enter_left":
			charSprite[element.Name].SetPositionf(-2, charPos.Y(), charPos.Z())
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{0, charPos.Y(), charPos.Z()}, 0.1, status)
			break
		case "move_left":
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{ACTOR_LEFT.X(), charPos.Y(), charPos.Z()}, 0.05, status)
			break
		case "exit_right":
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{2, charPos.Y(), charPos.Z()}, 0.1, status)
			break
		case "enter_right":
			charSprite[element.Name].SetPositionf(2, charPos.Y(), charPos.Z())
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{0, charPos.Y(), charPos.Z()}, 0.1, status)
			break
		case "move_right":
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{ACTOR_RIGHT.X(), charPos.Y(), charPos.Z()}, 0.05, status)
			break
		case "move_center":
			go AsyncAnimateMove(charSprite[element.Name], mgl32.Vec3{0, charPos.Y(), charPos.Z()}, 0.1, status)
			break
		}
		if async {
			// if there was no action, this will always be false
			fmt.Println("performing async action")
			nextDialogue(status)
			break
		}
	}
}

func delayNextDialogue(status *chan uint32, seconds int) {
	go func() {
		<-time.NewTimer(time.Second * time.Duration(seconds)).C
		*status <- 1 // send status update to listening channel
	}()
}

//////////////////////////////////////////////

func DrawSprite(sprite *hud.Sprite, m mat4.T, shader *gfx.Program) {
	sprite.Draw(m, shader)
}

func DrawText(text *hud.Text, tx, ty float32) {
	text.Draw(tx, ty)
}

/////////////////////////////////////////////
// ANIMATIONS

func AsyncAnimateFadeOut(s *hud.Sprite, status *chan uint32) {

	s.SetAlpha(0)
	for s.GetAlpha() < 1 {
		select {
		case <-UniversalTicker:
			s.SetAlpha(s.GetAlpha() + float32(0.01))
			fmt.Println("fade out:", s.GetAlpha())
			break
		}
	}

	nextDialogue(status)
	*status <- 1 // send status update to listening channel
}

func AsyncAnimateFadeIn(s *hud.Sprite, status *chan uint32) {

	s.SetAlpha(1)
	for s.GetAlpha() > 0 {
		select {
		case <-UniversalTicker:
			s.SetAlpha(s.GetAlpha() - float32(0.01))
			fmt.Println("fade in:", s.GetAlpha())
			break
		}
	}

	nextDialogue(status)
	*status <- 1 // send status update to listening channel
}

func AsyncAnimateMove(s *hud.Sprite, targetPosition mgl32.Vec3, speed float32, status *chan uint32) {

	fmt.Println("AsyncAnimateMoveLeft, sprite position:", s.GetPosition(), "to:", targetPosition)
	// this will move the actor to a target location
	// this check uses the length of the vector to account for floating point precision issues
	for targetPosition.Sub(s.GetPosition()).Len() > 0.001 {
		select {
		case <-UniversalTicker:
			pos := s.GetPosition()
			diff := targetPosition.Sub(pos)
			direction := diff.Normalize()
			newPos := pos.Add(direction.Mul(speed))
			s.SetPositionf(newPos.X(), newPos.Y(), newPos.Z())
			// fmt.Println("position:", pos, "direction:", direction, "new position:", newPos)
			// s.Translate(s.GetPosition().X()-0.1, 0, 0)
			// fmt.Println("translated X:", s.GetPosition().X())
			break
		}
	}

	*status <- 1 // send status update to listening channel
}

/////////////////////////////////////////////
// HELPER FUNCTIONS

func spriteKey(e script.ScriptElement) string {
	return fmt.Sprintf("%v_%v", e.Name, e.Mood)
}

func clear() {
	charSprite = nil
	charSprite = make(map[string]*hud.Sprite, 0)
	subjectName = nil
	if reply != nil {
		reply.Release()
		reply = nil
	}
	if dialogue != nil {
		dialogue.Release()
		dialogue = nil
	}
}

func toTitle(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	runes := []rune(s)
	var out string
	for i := 0; i < len(runes); i++ {
		v := runes[i]

		if i == 0 || (string(runes[i-1]) == " ") {
			out += strings.ToUpper(string(v))
		} else {
			out += string(v)
		}
	}

	return out
}

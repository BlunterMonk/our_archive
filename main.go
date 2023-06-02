package main

import (
	"context"
	"fmt"
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
	"github.com/BlunterMonk/opengl/pkg/sfx"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/ungerik/go3d/mat4"
)

var (
	wg  sync.WaitGroup
	ctx context.Context
	mtx sync.Mutex

	speakerX       = float32(124)
	speakerY       = float32(515)
	dialogueX      = float32(129)
	dialogueY      = float32(573)
	speakerScale   = 1.2
	dialogueDone   bool
	dialogueIndex  = -1
	CurrentSpeaker string

	Script          *script.Script
	UniversalTicker = time.Tick(16 * time.Millisecond)

	// used to send events to main loop
	EventQueue = make([]eventFunc, 0)
	status     = make(chan uint32)

	// static assets
	font, fontBold                         *v41.Font
	bg, fade                               *hud.Sprite
	dialogue, reply, subjectName           *hud.Text
	opSingle, dialogueOverlay, dialogueBar *hud.Sprite
	// dynamic assets
	charSprite          map[string]*hud.Sprite
	Actors, Backgrounds map[string]*hud.Sprite
	Sounds              map[string]*sfx.Streamer
	Names               map[string]*hud.Text

	shuttingDown         bool
	useStrictCoreProfile = (runtime.GOOS == "darwin")
	shaderProgram        *gfx.Program

	ACTOR_LEFT  = mgl32.Vec3{-0.5, -0.65, 0.0}
	ACTOR_RIGHT = mgl32.Vec3{0.5, -0.65, 0.0}
)

const (
	CURRENT_SCRIPT = "kazusa"
	AUTO           = false

	XCODE_SHUTDOWN_SIGNAL = 0
	XCODE_CONSUMER_FAILED = 4
	XCODE_PANIC           = 5
	XCODE_ABORT           = 6

	WINDOW_WIDTH  = 1280
	WINDOW_HEIGHT = 720
)

type eventFunc func(s *hud.Text)

type animation struct {
	start *mgl32.Vec3
	end   *mgl32.Vec3
	speed float32
}

func ThemeFilePath(n string) string {
	return fmt.Sprintf("./resources/bgm/Theme_%s.mp3", n)
}

// in Open GL, Y starts at the bottom

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

func loadResources() {
	shaderProgram = gfx.MustInitShader()

	// dialogue font
	font = gfx.MustLoadFont("NotoSans-Medium")
	font.ResizeWindow(float32(WINDOW_WIDTH), float32(WINDOW_HEIGHT))
	fontBold = gfx.MustLoadFont("NotoSans-Bold")
	fontBold.ResizeWindow(float32(WINDOW_WIDTH), float32(WINDOW_HEIGHT))

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
	Sounds = make(map[string]*sfx.Streamer)
	Backgrounds = make(map[string]*hud.Sprite)
	for _, v := range Script.Elements() {
		// create names if they don't exist
		if _, ok := Names[v.Name]; !ok {
			log.Println("initializing name:", v.Name)
			Names[v.Name] = hud.NewSolidText(toTitle(v.Name), hud.COLOR_WHITE, fontBold)
			Names[v.Name].SetScale(float32(speakerScale))
			Names[v.Name].SetPositionf(speakerX, speakerY)
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
			fmt.Println("loading background:", v.Name)
			Backgrounds[v.Mood] = hud.NewSpriteFromFile(fmt.Sprintf("./resources/bg/%v.jpeg", v.Mood))
			Actors[v.Name].LoadTexture(key, fmt.Sprintf("./resources/bg/%s.jpeg", v.Mood))
		case "bgm":
			fmt.Println("loading bgm:", v.Mood)
			Sounds[v.Mood] = sfx.NewStreamer(ThemeFilePath(v.Mood))
		default:
			fmt.Println("loading actor:", v.Name)
			Actors[v.Name].LoadTexture(key, fmt.Sprintf("./resources/actor/%s/%s-%s.png", v.Name, v.Name, v.Mood))
			Actors[v.Name].SetPositionf(0, 0, 0)
		}

		// second switch to establish starting values
		// @TODO: maybe figure out a way to put this in the script
		switch v.Name {
		case "mika":
			Actors[v.Name].SetPositionf(-1.5, -0.65, 0)
		case "seia":
			Actors[v.Name].SetPositionf(1.5, -0.65, 0)
			Actors[v.Name].SetScale(0.8)
		case "kazusa":
			Actors[v.Name].SetPositionf(-0.15, -0.9, 0)
		}
	}

	metadata, err := script.LoadMetadata(fmt.Sprintf("./resources/scripts/%s.json", CURRENT_SCRIPT))
	if err != nil {
		panic(err)
	}

	for _, actor := range metadata.Actors {
		if a, ok := Actors[actor.Name]; ok {
			fmt.Println("setting center:", actor.Name)
			a.SetCenter(actor.CenterX, actor.CenterY, actor.CenterScale)
		}
	}

	// load the remaining static assets needed for all scripts
	Sounds["next"] = sfx.NewStreamer("./resources/audio/chat.mp3")

	// fade overlay
	fade = hud.NewSpriteFromFile("./resources/bg/black_screen.jpeg")
	fade.SetPositionf(0, 0, 0)
	fade.SetAlpha(0)
	// dialogue text
	// dialogue = hud.NewText(Script.Get(0).Line, hud.COLOR_WHITE, font)
	// dialogue.SetScale(0.85)
	// dialogue.AsyncAnimate(&status)

	opSingle = hud.NewSpriteFromFile("./resources/ui/text_option_single.png")
	dialogueOverlay = hud.NewSpriteFromFile("./resources/ui/dialogue_bg.png")
	dialogueBar = hud.NewSpriteFromFile("./resources/ui/dialogue_bar.png")
}

func releaseResources() {

	sfx.Close()

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
}

func main() {
	var xCode int

	window := gfx.Init()
	window.SetKeyCallback(keyCallback)
	window.SetMouseButtonCallback(mouseButtonCallback)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)

	ft := time.Tick(time.Second)
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

	loadResources()

	// load system sounds
	sfx.Init(Sounds["next"])

	// load sprites
	screenshotOverlay := hud.NewSpriteFromFile("./resources/temp/kayoko_overlay.png")
	screenshotOverlay.SetAlpha(0.5)
	screenshotOverlay.SetPositionf(0, 0, -1)

	gl.ClearColor(0.4, 0.4, 0.4, 0.0)
	gl.BlendColor(1, 1, 1, 1)

	var delay time.Timer
	counter := 0

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
			switch s {
			case 0:
				break F
			case 1:
				delay = *time.NewTimer(time.Second)
			case 2:
				nextDialogue(&status)
			}
		case <-delay.C:
			if AUTO {
				if dialogue != nil && dialogue.Done() {
					if s, ok := Sounds["next"]; ok {
						s.Play()
					}
					nextDialogue(&status)
				}
			}

		case <-UniversalTicker:
			counter++

			// enable shader
			shaderProgram.Use()

			gl.Enable(gl.BLEND) //Enable blending.
			gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

			// draw image

			drawBackgrounds()
			drawActors()
			drawUI()

			if reply != nil {
				DrawSprite(opSingle, mat4.From(&mat4.Ident), shaderProgram)
			}

			// re-enable blending to resolve alpha issue
			shaderProgram.Use()
			gl.Enable(gl.BLEND) //Enable blending.
			gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

			// DrawSprite(screenshotOverlay, mat4.From(&mat4.Ident), shaderProgram)

			if fade != nil {
				DrawSprite(fade, mat4.From(&mat4.Ident), shaderProgram) // dialogue window
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

	fmt.Println("shutting down...")
	shuttingDown = true
	wg.Wait()

	releaseResources()
	os.Exit(xCode)
}

func drawBackgrounds() {
	if bg != nil {
		DrawSprite(bg, mat4.From(&mat4.Ident), shaderProgram) // background
	}
}
func drawActors() {
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
}
func drawUI() {
	// Draw text
	if dialogue != nil {
		DrawSprite(dialogueOverlay, mat4.From(&mat4.Ident), shaderProgram) // dialogue window
		DrawSprite(dialogueBar, mat4.From(&mat4.Ident), shaderProgram)     // dialogue bar overlay
		DrawText(dialogue, dialogueX, dialogueY)                           // actual text
	}
	if subjectName != nil {
		DrawText(subjectName, speakerX, speakerY) // speaker's name
	}
	if reply != nil {
		DrawText(reply, (WINDOW_WIDTH/2)-(reply.Width()/2)+25, 280) // @TODO: figure out bounds of text dynamically
	}
}

func keyCallback(window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {

	if action != glfw.Press {
		return
	}

	// When a user presses the escape key, we set the WindowShouldClose property to true,
	// which closes the application
	if key == glfw.KeyEscape && action == glfw.Press {
		window.SetShouldClose(true)
	}

	// activeChar := Actors["kayoko"]
	if key == glfw.KeyLeft {
		// moveActor(activeChar, -0.01, 0)
		// dialogueX -= 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyRight {
		// moveActor(activeChar, 0.01, 0)
		// dialogueX += 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyUp {
		// moveActor(activeChar, 0, 0.01)
		// dialogueY -= 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyDown {
		// dialogueY += 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyS {
		// scaleActor(activeChar, -0.01)
		// subjectName.SetScale(subjectName.GetScale() + 0.1)
		// fmt.Println("scale:", subjectName.GetScale())
		// dialogue.SetSpacing(dialogue.GetSpacing() + 0.1)
	}
	if key == glfw.KeyD {
		// scaleActor(activeChar, 0.01)
		// subjectName.SetScale(subjectName.GetScale() - 0.1)
		// fmt.Println("scale:", subjectName.GetScale())
		// dialogue.SetSpacing(dialogue.GetSpacing() - 0.1)
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
		// *status <- 0
		return
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
			nextDialogue(status)
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
		} else if element.Action == "_" {
			// if there is no dialogue and no actions, consider this just setting up the sprites
			// usually in a transition or something, so continue
			nextDialogue(status)
			break
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

		// convert action into predefined parameters
		anim, async := getActionParameters(element.Action, charSprite[element.Name].GetPosition())

		// move the actor if the action set the position
		if anim != nil {
			applyAnimation(status, anim, charSprite[element.Name], async)
		}
	}
}

func getActionParameters(action string, charPos mgl32.Vec3) (*animation, bool) {
	var async bool
	var startingPosition *mgl32.Vec3
	var targetPosition *mgl32.Vec3
	var transitionSpeed = float32(0.1)

	// @TODO: maybe find a better workaround for this
	// need to specificy async operations vs non-async on the script
	if strings.HasSuffix(action, "_async") {
		async = true
		action = strings.TrimSuffix(action, "_async")
	}

	switch action {
	case "enter_left":
		targetPosition = &mgl32.Vec3{0, charPos.Y(), charPos.Z()}
		startingPosition = &mgl32.Vec3{-2, charPos.Y(), charPos.Z()}
		break
	case "enter_right":
		targetPosition = &mgl32.Vec3{0, charPos.Y(), charPos.Z()}
		startingPosition = &mgl32.Vec3{2, charPos.Y(), charPos.Z()}
		break
	case "exit_left":
		targetPosition = &mgl32.Vec3{-2, charPos.Y(), charPos.Z()}
		break
	case "exit_right":
		targetPosition = &mgl32.Vec3{2, charPos.Y(), charPos.Z()}
		break
	case "move_left":
		transitionSpeed = 0.05
		targetPosition = &mgl32.Vec3{ACTOR_LEFT.X(), charPos.Y(), charPos.Z()}
		break
	case "move_right":
		targetPosition = &mgl32.Vec3{ACTOR_RIGHT.X(), charPos.Y(), charPos.Z()}
		break
	case "move_center":
		targetPosition = &mgl32.Vec3{0, charPos.Y(), charPos.Z()}
		break
	default:
		return nil, false
	}

	return &animation{
		start: startingPosition,
		end:   targetPosition,
		speed: transitionSpeed,
	}, async
}

func applyAnimation(status *chan uint32, anim *animation, actor *hud.Sprite, async bool) uint32 {
	if anim.start != nil {
		actor.SetPosition(*anim.start)
	}

	if anim.end != nil {
		if async {
			fmt.Println("performing async action")
			go AsyncAnimateMove(actor, *anim.end, anim.speed, status)
		} else {
			actor.SetPosition(*anim.end)
			return 2
		}
	}

	return 1
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
			s.SetAlpha(s.GetAlpha() + float32(0.025))
			fmt.Println("fade out:", s.GetAlpha())
			break
		}
	}

	*status <- 2 // send status update to listening channel
}

func AsyncAnimateFadeIn(s *hud.Sprite, status *chan uint32) {

	s.SetAlpha(1)
	for s.GetAlpha() > 0 {
		select {
		case <-UniversalTicker:
			s.SetAlpha(s.GetAlpha() - float32(0.025))
			fmt.Println("fade in:", s.GetAlpha())
			break
		}
	}

	*status <- 2 // send status update to listening channel
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

func moveActor(actor *hud.Sprite, x, y float32) {
	pos := actor.GetPosition()
	actor.SetPositionf(pos.X()+x, pos.Y()+y, pos.Z())
	fmt.Println("new postion:", actor.GetPosition())
}
func scaleActor(actor *hud.Sprite, s float32) {
	actor.SetScale(actor.GetScale() + s)
	fmt.Println("new scale:", actor.GetScale())
}

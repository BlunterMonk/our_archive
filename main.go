package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	v41 "github.com/4ydx/gltext/v4.1"
	"github.com/BlunterMonk/our_archive/internal/hud"
	"github.com/BlunterMonk/our_archive/internal/script"
	"github.com/BlunterMonk/our_archive/pkg/gfx"
	"github.com/BlunterMonk/our_archive/pkg/sfx"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/pkg/errors"
)

var (
	wg  sync.WaitGroup
	ctx context.Context
	mtx sync.Mutex

	LANDSCAPE_VIEW = View{
		speakerX:     float32(124),
		speakerY:     float32(515),
		dialogueX:    float32(129),
		dialogueY:    float32(573),
		WindowWidth:  1280,
		WindowHeight: 720,
	}
	PORTRAIT_VIEW = View{
		speakerX:     float32(0),
		speakerY:     float32(0),
		dialogueX:    float32(0),
		dialogueY:    float32(0),
		WindowWidth:  720,
		WindowHeight: 1280,
	}
	speakerScale     = 1.2
	dialogueDone     bool
	dialogueIndex    = -1
	CurrentSpeaker   string
	CurrentFontSize  float32
	CurrentBgmVolume = float64(-2)
	CurrentSfxVolume = float64(1)
	DefaultFontSize  = 0.85

	Script          *script.Script
	UniversalTicker = time.Tick(16 * time.Millisecond)

	// used to send events to main loop
	EventQueue = make([]eventFunc, 0)
	status     = make(chan uint32)

	// static assets
	font, fontBold                                *v41.Font
	bg, fade                                      *hud.Sprite
	spriteAutoOn, spriteAutoOff, spriteMenuButton *hud.Sprite
	dialogue, subjectName                         *hud.Text
	reply                                         []*hud.Text
	opSingle, dialogueOverlay, dialogueBar        *hud.Sprite
	opDoubleA                                     *hud.Sprite
	opDoubleB                                     *hud.Sprite
	// emoteBalloon *hud.Sprite
	// dynamic assets
	charSprite      map[string]*Actor
	Actors          map[string]*Actor
	ActorAnimations map[string]script.AnimationMetadata
	Emotes          map[string]*hud.AnimatedSprite
	Backgrounds     map[string]*hud.Sprite
	Sounds          map[string]*sfx.Streamer
	Names, Factions map[string]*hud.Text

	currentBGM           *sfx.Streamer
	shuttingDown         bool
	useStrictCoreProfile = (runtime.GOOS == "darwin")
	shaderProgram        *gfx.Program

	ACTOR_LEFT  = hud.Vec3{-0.5, -0.65, 0.0}
	ACTOR_RIGHT = hud.Vec3{0.5, -0.65, 0.0}
	AUTO        = false
	DEBUG       = false
	DEBUG_TEXT  *hud.Text
)

const (
	XCODE_SHUTDOWN_SIGNAL = 0
	XCODE_CONSUMER_FAILED = 4
	XCODE_PANIC           = 5
	XCODE_ABORT           = 6
)

type eventFunc func(s *hud.Text)
type View struct {
	speakerX     float32
	speakerY     float32
	dialogueX    float32
	dialogueY    float32
	WindowWidth  int
	WindowHeight int
}

// in Open GL, Y starts at the bottom

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

func main() {

	var scriptName string
	if len(os.Args) > 3 {
		scriptName = os.Args[3]
	} else {
		scriptName = "test"
	}

	runGame(LANDSCAPE_VIEW, scriptName)
}

func loadResources(view View, scriptName string) []error {
	missing := make([]error, 0)

	// init resource containers
	charSprite = make(map[string]*Actor)
	Actors = make(map[string]*Actor)
	Emotes = make(map[string]*hud.AnimatedSprite)
	Names = make(map[string]*hud.Text)
	Factions = make(map[string]*hud.Text)
	Sounds = make(map[string]*sfx.Streamer)
	Backgrounds = make(map[string]*hud.Sprite)

	shaderProgram = gfx.MustInitShader()

	// dialogue font
	font = gfx.MustLoadFont("NotoSans-Regular")
	font.ResizeWindow(float32(view.WindowWidth), float32(view.WindowHeight))
	fontBold = gfx.MustLoadFont("NotoSans-Bold")
	fontBold.ResizeWindow(float32(view.WindowWidth), float32(view.WindowHeight))
	// static and reusable UI elements
	opSingle, _ = hud.NewSpriteFromFile("./resources/ui/text_option_single.png")
	opDoubleA, _ = hud.NewSpriteFromFile("./resources/ui/text_option_a.png")
	opDoubleB, _ = hud.NewSpriteFromFile("./resources/ui/text_option_b.png")
	dialogueOverlay, _ = hud.NewSpriteFromFile("./resources/ui/dialogue_bg.png")
	dialogueBar, _ = hud.NewSpriteFromFile("./resources/ui/dialogue_bar.png")
	spriteAutoOn, _ = hud.NewSpriteFromFile("./resources/ui/auto_on.png")
	spriteAutoOff, _ = hud.NewSpriteFromFile("./resources/ui/auto_off.png")
	spriteMenuButton, _ = hud.NewSpriteFromFile("./resources/ui/menu.png")
	// emoteBalloon = hud.NewSpriteFromFile("./resources/ui/balloon.png")
	// emoteBalloon.SetScale(0.085)

	// fade overlay
	fade, _ = hud.NewSpriteFromFile("./resources/bg/black_screen.jpeg")
	fade.SetPositionf(0, 0, 0)
	fade.SetAlpha(0)
	// static sound for advancing to the next dialogue
	// Sounds["next"] = sfx.NewStreamer("./resources/audio/chat.mp3")

	// setup text output
	/* script structure
	* [actor - mood - action]
	* example of background: [bg - black_screen - none]
	* exmaple of character: [mika - 03 - heart]
	 */
	Script = script.NewScriptFromFile(fmt.Sprintf("./resources/scripts/%s.txt", scriptName))
	metadata, err := script.LoadMetadata("./resources/settings.json")
	if err != nil {
		panic(err)
	}

	Names["all"] = hud.NewSolidText("All", hud.COLOR_WHITE, fontBold)
	Names["all"].SetScale(float32(speakerScale))
	Names["all"].SetPositionf(view.speakerX, view.speakerY)
	for _, v := range Script.Elements() {
		switch v.Name {
		case "bg":
			log.Println("loading background:", v.Name)
			Backgrounds[v.Mood], err = hud.NewSpriteFromFile(fmt.Sprintf("./resources/bg/%v.jpeg", v.Mood))
			if err != nil {
				missing = append(missing, errors.Wrap(err, "failed to load background"))
			}
			continue
		case "bgm":
			log.Println("loading bgm:", v.Mood)
			Sounds[v.Mood], err = sfx.NewStreamer(fmt.Sprintf("./resources/bgm/%s.mp3", v.Mood))
			if err != nil {
				missing = append(missing, errors.Wrap(err, "failed to load bgm"))
			}
			continue
		case "sfx":
			log.Println("loading sfx:", v.Mood)
			Sounds[v.Mood], err = sfx.NewStreamer(fmt.Sprintf("./resources/sfx/%s.mp3", v.Mood))
			if err != nil {
				missing = append(missing, errors.Wrap(err, "failed to load sfx"))
			}
			continue
		case "all":
			if v.Action != "emote" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
			continue
		case "defect", "none", "clear", "_", "font":
			continue
		}

		// create names if they don't exist
		if _, ok := Names[v.Name]; !ok {
			log.Println("initializing name:", v.Name)
			Names[v.Name] = hud.NewSolidText(toTitle(v.Name), hud.COLOR_WHITE, fontBold)
			Names[v.Name].SetScale(float32(speakerScale))
			Names[v.Name].SetPositionf(view.speakerX, view.speakerY)
		}
		if _, ok := Factions[v.Name]; !ok {
			// create faction text
			for _, actor := range metadata.Actors {
				if actor.FactionName == nil || *actor.FactionName == "" {
					continue
				}
				if actor.Name == v.Name {
					log.Println("creating faction name for:", v.Name, *actor.FactionName)
					name := strings.ToLower(v.Name)
					Factions[name] = hud.NewSolidText(*actor.FactionName, mgl32.Vec3{0.49, 0.81, 1}, fontBold)
					Factions[name].SetScale(0.8)
					break
				}
			}
		}

		// if it's not a system asset it's an actor
		key := spriteKey(v)

		// check to see if the action is an emote
		switch v.Action {
		case "emote": // load the emote if it isn't already
			if _, ok := Emotes[v.Mood]; !ok {
				Emotes[v.Mood] = hud.NewAnimatedSpriteFromFile(fmt.Sprintf("./resources/emote/%s.gif", v.Mood))
				emoteSfx := fmt.Sprintf("sfx_%s", v.Mood)
				if _, ok := Sounds[emoteSfx]; !ok {
					Sounds[emoteSfx], err = sfx.NewStreamer(fmt.Sprintf("./resources/sfx/%s.mp3", emoteSfx))
					missing = append(missing, err)
				}
			}
			break
		default: // if it's not an emote, then load the texture onto the actor as an expression
			// if there's an actor sprite load the actor if it doesn't exist
			if v.Mood != "_" {
				log.Println("loading actor:", v.Name, "with expression:", v.Mood, "and action:", v.Action)
				if _, ok := Actors[v.Name]; !ok {
					Actors[v.Name], err = NewActor(v.Name)
					missing = append(missing, err)
				}

				err = Actors[v.Name].LoadTexture(key, fmt.Sprintf("./resources/actor/%s/%s-%s.png", v.Name, v.Name, v.Mood))
				if err != nil {
					missing = append(missing, errors.Wrap(err, "failed to load actor sprite"))
				}
				Actors[v.Name].SetPositionf(0, 0, 0)
			}

			if v.Action != "_" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
		}
	}

	// fmt.Println(Emotes)
	for _, actor := range metadata.Actors {
		if a, ok := Actors[actor.Name]; ok {
			a.SetCenter(actor.CenterX, actor.CenterY, actor.CenterScale)
			a.SetPositionf(actor.CenterX, actor.CenterY, 0)
			a.SetScale(actor.CenterScale)

			if actor.FactionName != nil && *actor.FactionName != "" {
				a.FactionName = *actor.FactionName
			}

			// add emote data to actor
			for _, emote := range metadata.Emotes {
				// fmt.Println("laoding metadata for emote:", emote.Name)
				if _, ok := Emotes[emote.Name]; ok {
					// fmt.Println("type:", emote.Type)
					switch emote.Type {
					case "head":
						a.AddEmoteData(emote.Name, hud.Vec3{actor.EmoteOffsetHead.X, actor.EmoteOffsetHead.Y, 0})
					case "bubble":
						a.AddEmoteData(emote.Name, hud.Vec3{actor.EmoteOffsetBubble.X, actor.EmoteOffsetBubble.Y, 0})
					}
				}
			}

			// p := hud.Vec3{-0.24000002, 0.4199995, 0}
			// diff := hud.Vec3{0.26000002, -1.0799996, 0}
			// fmt.Println("diff:", p.Sub(Actors["kayoko"].GetPosition()))
			// emoteBalloon.SetPosition(Actors["kayoko"].GetPosition().Sub(diff))
			// fmt.Println("pos:", emoteBalloon.GetPosition())
		}
	}
	for _, emote := range metadata.Emotes {
		if a, ok := Emotes[emote.Name]; ok {
			a.SetScale(emote.Scale)
		}
	}
	ActorAnimations = make(map[string]script.AnimationMetadata)
	for _, anim := range metadata.Animation {
		if _, ok := ActorAnimations[anim.Name]; ok {
			log.Println("duplicate animation:", anim.Name)
		}
		ActorAnimations[anim.Name] = anim
	}

	// display errors
	if len(missing) > 0 {
		var texts []string
		for _, v := range missing {
			if v == nil {
				continue
			}
			texts = append(texts, v.Error())
		}
		if len(texts) > 0 {
			text := fmt.Sprintf("Script Errors:\n%s", strings.Join(texts, "\n"))
			DEBUG_TEXT = hud.NewSolidText(text, mgl32.Vec3{0.9, 0.9, 0.9}, font)
		}
	}

	return missing
}

func verifyAnimation(name string, metadata *script.Metadata) error {
	for _, anim := range metadata.Animation {
		// fmt.Println("comparing animation:", anim.Name, name)
		if anim.Name == name {
			// fmt.Println("found animation:", anim.Name)
			return nil
		}
	}

	return fmt.Errorf("animation with name \"%s\" found in script, but not in settings.json", name)
}

func releaseResources() {

	sfx.Close()

	// clean up resources
	if dialogue != nil {
		dialogue.Release()
		dialogue = nil
	}
	releaseReplies()
	font.Release()
	shaderProgram.Delete()
	for _, v := range Sounds {
		v.Release()
	}
}

func runGame(CurrentViewConfig View, scriptName string) {
	var xCode int

	window := gfx.Init(CurrentViewConfig.WindowWidth, CurrentViewConfig.WindowHeight)
	window.SetKeyCallback(keyCallback)
	window.SetMouseButtonCallback(mouseButtonCallback)
	sfx.Init()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)

	ft := time.Tick(time.Second)
	killswitch := make(chan int, 0)

	// Catch any panics
	// go func() {
	// 	if r := recover(); r != nil {
	// 		log.Println("app panicked!")
	// 		log.Println(r)
	// os.Exit(XCODE_PANIC)
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

	loadResources(CurrentViewConfig, scriptName)

	// load system sounds

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
			counter = 0
		// events must be handled on the main thread if they interact with OpenGL
		// this is a limitation on the OpenGL system where it will panic if changes are made by different threads
		case s := <-status:
			// log.Printf("received a status update: %v\n", s)
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
					// if s, ok := Sounds["next"]; ok {
					// 	s.Play()
					// }
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
			drawUI(CurrentViewConfig)
			drawText(CurrentViewConfig)

			// re-enable blending to resolve alpha issue
			shaderProgram.Use()
			gl.Enable(gl.BLEND) //Enable blending.
			gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

			drawOverlays()

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
		DrawSprite(bg, hud.NewMat4(), shaderProgram) // background
	}
}
func drawActors() {
	keys := make([]string, 0, len(charSprite))
	for k := range charSprite {
		// don't add the current speaker to the list so we can draw them last
		if k != CurrentSpeaker {
			keys = append(keys, k)
		}
	}
	// sort all the actors other than the one speaking
	sort.Strings(keys)
	keys = append(keys, CurrentSpeaker)
	for _, name := range keys {
		actor := charSprite[name]
		if actor != nil {
			// slightly discolor whoever isn't talking
			if CurrentSpeaker == "all" || name == CurrentSpeaker {
				actor.SetColorf(1, 1, 1)
				if DEBUG {
					actor.SetAlpha(0.7)
				} else {
					actor.SetAlpha(1)
				}
			} else {
				actor.SetColorf(0.7, 0.7, 0.7)
			}

			// draw the actor
			actor.Draw(shaderProgram)
		}
	}
}
func drawUI(view View) {
	// Draw text
	if dialogue != nil {
		DrawSprite(dialogueOverlay, hud.NewMat4(), shaderProgram) // dialogue window
		DrawSprite(dialogueBar, hud.NewMat4(), shaderProgram)     // dialogue bar overlay
	}
	if len(reply) == 1 {
		DrawSprite(opSingle, hud.NewMat4(), shaderProgram)
	} else if len(reply) == 2 {
		DrawSprite(opDoubleA, hud.NewMat4(), shaderProgram)
		DrawSprite(opDoubleB, hud.NewMat4(), shaderProgram)
	}
	if AUTO {
		DrawSprite(spriteAutoOn, hud.NewMat4(), shaderProgram)
	} else {
		DrawSprite(spriteAutoOff, hud.NewMat4(), shaderProgram)
	}
	DrawSprite(spriteMenuButton, hud.NewMat4(), shaderProgram)
	// emoteBalloon.Draw(emoteBalloon.GetTransform(), shaderProgram)
}
func drawText(view View) {
	// Draw text
	if dialogue != nil {
		DrawText(view, dialogue, view.dialogueX, view.dialogueY) // actual text
		if subjectName != nil {
			DrawText(view, subjectName, view.speakerX, view.speakerY) // speaker's name
			if factionName, ok := Factions[strings.ToLower(CurrentSpeaker)]; ok {
				DrawText(view, factionName, view.speakerX+subjectName.Width()+10, view.speakerY+2) // speaker's name
			}
		}
	}
	if len(reply) == 1 {
		DrawText(view, reply[0], (float32(view.WindowWidth)/2)-(reply[0].Width()/2)+25, 285)
	} else if len(reply) == 2 {
		DrawText(view, reply[0], (float32(view.WindowWidth)/2)-(reply[0].Width()/2)+25, 240)
		DrawText(view, reply[1], (float32(view.WindowWidth)/2)-(reply[0].Width()/2)+25, 330)
	}
	if DEBUG {
		if s, ok := charSprite[CurrentSpeaker]; ok {
			fmt.Println("printing actor position")
			p := s.GetPosition()
			pos := hud.NewSolidText(fmt.Sprintf("position: (%f, %f)\nscale: (%f)", p.X(), p.Y(), s.GetScale()), hud.COLOR_WHITE, font)
			pos.SetScale(2)
			DrawText(view, pos, 0, 0)
		}
	}
	if DEBUG_TEXT != nil {
		DrawText(view, DEBUG_TEXT, 0, 0)
	}
}
func drawOverlays() {
	if fade != nil {
		DrawSprite(fade, hud.NewMat4(), shaderProgram) // dialogue window
	}
}

func keyCallback(window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {

	if action != glfw.Press && action != glfw.Repeat {
		return
	}

	// When a user presses the escape key, we set the WindowShouldClose property to true,
	// which closes the application
	if key == glfw.KeyEscape && action == glfw.Press {
		window.SetShouldClose(true)
	}
	if key == glfw.KeyTab {
		AUTO = !AUTO
	}
	if key == glfw.KeyD {
		DEBUG = !DEBUG
		log.Println("debug toggled")
	}
	if key == glfw.KeyEqual {
		CurrentBgmVolume += 0.5
		if currentBGM != nil {
			currentBGM.SetVolume(CurrentBgmVolume)
		}
		// fmt.Println("bgm volume:", CurrentBgmVolume)
	}
	if key == glfw.KeyMinus {
		CurrentBgmVolume -= 0.5
		if currentBGM != nil {
			currentBGM.SetVolume(CurrentBgmVolume)
		}
		// fmt.Println("bgm volume:", CurrentBgmVolume)
	}

	s, ok := charSprite[CurrentSpeaker]
	if !ok {
		return
	}

	activeChar := s.Sprite
	if key == glfw.KeyLeft {
		moveActor(activeChar, -0.01, 0)
		// dialogueX -= 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyRight {
		moveActor(activeChar, 0.01, 0)
		// dialogueX += 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyUp {
		moveActor(activeChar, 0, 0.01)
		// dialogueY -= 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyDown {
		moveActor(activeChar, 0, -0.01)
		// dialogueY += 1
		// fmt.Printf("speaker (%d,%d)\n", dialogueX, dialogueY)
	}
	if key == glfw.KeyS {
		scaleActor(activeChar, -0.01)
		// subjectName.SetScale(subjectName.GetScale() + 0.1)
		// fmt.Println("scale:", subjectName.GetScale())
		// dialogue.SetSpacing(dialogue.GetSpacing() + 0.1)
	}
	if key == glfw.KeyB {
		scaleActor(activeChar, 0.01)
		// subjectName.SetScale(subjectName.GetScale() - 0.1)
		// fmt.Println("scale:", subjectName.GetScale())
		// dialogue.SetSpacing(dialogue.GetSpacing() - 0.1)
	}

	// p := emoteBalloon.GetPosition()
	// fmt.Println("diff:", Actors["akira"].GetPosition().Sub(p))
	// emoteBalloon.SetPosition(Actors["kayoko"].GetPosition().Sub(diff))
	// fmt.Println("pos:", emoteBalloon.GetPosition())
}
func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
	// log.Printf("mouseButtonCallback: button(%v), action(%v)\n", button, action)
	cursorX, cursorY := glfw.GetCurrentContext().GetCursorPos()
	// fmt.Printf("cursor pos: (%f %f)\n", cursorX, cursorY)

	if action == glfw.Release {
		autoRect := image.Rectangle{Min: image.Point{X: 1020, Y: 20}, Max: image.Point{X: 1130, Y: 60}}
		menuRect := image.Rectangle{Min: image.Point{X: 1150, Y: 20}, Max: image.Point{X: 1260, Y: 60}}
		if inside(autoRect, int(cursorX), int(cursorY)) {
			AUTO = !AUTO
		} else if inside(menuRect, int(cursorX), int(cursorY)) {
			fmt.Println("resetting scene")
			dialogueIndex = 0
			clear()
			loadResources(LANDSCAPE_VIEW, "test")
		} else {
			nextDialogue(&status)
		}
	}
}
func inside(rect image.Rectangle, x, y int) bool {
	return x >= rect.Min.X && x <= rect.Max.X && y >= rect.Min.Y && y <= rect.Max.Y
}

func queueEvent(event eventFunc) {
	EventQueue = append(EventQueue, event)
}
func nextDialogue(status *chan uint32) {

	// fmt.Println("starting dialogue goroutine")
	releaseReplies()

	dialogueIndex++
	if len(Script.Elements()) <= dialogueIndex {
		// *status <- 0
		return
	}

	element := Script.Get(dialogueIndex)
	log.Printf("next line: %v\n", element.ToString())

	elementText := element.Line
	switch element.Name {
	case "defect":
		name := strings.ToLower(element.Mood)
		if _, ok := Factions[name]; ok {
			Factions[name] = hud.NewSolidText(elementText, mgl32.Vec3{0.49, 0.81, 1}, fontBold)
			Factions[name].SetScale(0.8)
		}
		nextDialogue(status)
		break
	// case "emote":
	// 	// @TODO: shortcut
	// 	copy := element
	// 	copy.Action = "emote"
	// 	copy.Name = element.Action
	// 	prepareActor(status, copy)
	// 	break
	case "clear":
		clear()
		nextDialogue(status)
		break
	case "bg":
		bg = Backgrounds[element.Mood]
		<-time.NewTimer(time.Second).C
		nextDialogue(status)
		break
	case "sfx":
		if s, ok := Sounds[element.Mood]; ok {
			fmt.Println("playing sfx:", element.Mood)
			s.Play(CurrentSfxVolume)
			nextDialogue(status)
		}
		break
	case "bgm":
		if s, ok := Sounds[element.Mood]; ok {
			fmt.Println("playing bgm:", element.Mood)
			if currentBGM != nil {
				currentBGM.Close()
			}
			s.PlayOnRepeat(CurrentBgmVolume)
			currentBGM = s
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
	case "font":
		switch element.Mood {
		case "size":
			switch element.Action {
			case "reset":
				CurrentFontSize = float32(DefaultFontSize)
			default:
				f, err := strconv.ParseFloat(element.Action, 64)
				if err == nil {
					CurrentFontSize = float32(f)
				}
			}
		}
		nextDialogue(status)
		break
	case "sensei":
		// 2E4152
		for _, v := range element.Lines {
			reply = append(reply, hud.NewSolidText(v, mgl32.Vec3{0.18, 0.255, 0.322}, font))
		}
		break
	case "none":
		releaseDialogue()
		break
	default:
		prepareActor(status, element)
		break
	}
}

func prepareActor(status *chan uint32, element script.ScriptElement) {
	CurrentSpeaker = element.Name

	// if the current dialogue actor has a name generated then set the current displayed name to it
	if t, ok := Names[element.Name]; ok {
		subjectName = t
	} else {
		subjectName = nil
	}

	// only display dialogue if there is dialogue
	if element.Line != "" && len(element.Lines) > 0 {
		dialogue = hud.NewText(element.Line, hud.COLOR_WHITE, font)
		dialogue.SetScale(CurrentFontSize)
		dialogue.AsyncAnimate(status)
	}

	// if we're affecting all actors, apply the animations and exit
	if element.Name == "all" {
		emoteData, emoteSfx := elementToEmoteData(element)
		if emoteData != nil {
			for _, actor := range charSprite {
				actor.AnimateEmote(element.Mood, emoteData, func() {
					// nextDialogue(status)
					fmt.Println("done animating emote")
				})
			}
		}
		if emoteSfx != nil {
			emoteSfx.Play(CurrentSfxVolume)
		}

		// if there's no dialogue but there's an emote, delay the advance
		if !element.HasDialogue() {
			releaseDialogue()
			if emoteData != nil {
				delayNextDialogue(status, emoteData.GetDuration())
			}
		}
		return
	}

	// break early when the current dialogue has no sprite associated with it
	if _, ok := Actors[element.Name]; !ok {
		return
	}

	// do not display sprites if the mood is blank
	// this indicates that the actor is off screen
	if element.Mood == "_" {
		return
	}

	charSprite[element.Name] = Actors[element.Name]

	// convert action into predefined parameters
	prepareActorAnimation(status, &element, charSprite[element.Name], !element.HasDialogue())

	// fmt.Println(charSprite[element.Name])
	// fmt.Println(Actors[element.Name])
	// fmt.Println("---------------------")
	err := charSprite[element.Name].SetActiveTexture(spriteKey(element))
	if err != nil {
		panic(err)
	}

	if !element.HasDialogue() {
		nextDialogue(status)
	}
}

func delayNextDialogue(status *chan uint32, duration time.Duration) {
	go func() {
		<-time.NewTimer(duration).C
		*status <- 2 // send status update to listening channel
	}()
}

// get the emote data from the script element
func elementToEmoteData(element script.ScriptElement) (*hud.AnimatedSprite, *sfx.Streamer) {
	if element.Action != "emote" {
		return nil, nil
	}

	if emoteData, ok := Emotes[element.Mood]; ok {
		fmt.Println("emote found")
		if s, ok := Sounds[fmt.Sprintf("sfx_%s", element.Mood)]; ok {
			fmt.Println("sfx found")
			return emoteData, s
		}
		return emoteData, nil
	}

	fmt.Println("not found")
	return nil, nil
}

func prepareActorAnimation(status *chan uint32, element *script.ScriptElement, actor *Actor, autoNextDialogue bool) {
	switch element.Action {
	case "emote":
		if _, ok := Emotes[element.Mood]; ok {
			if emoteData, ok := Emotes[element.Mood]; ok {
				actor.AnimateEmote(element.Mood, emoteData, func() {
					// nextDialogue(status)
					fmt.Println("done animating emote")
				})
				if s, ok := Sounds[fmt.Sprintf("sfx_%s", element.Mood)]; ok {
					fmt.Println("playing sfx:", fmt.Sprintf("sfx_%s", element.Mood))
					s.Play(CurrentSfxVolume)
					if autoNextDialogue {
						delayNextDialogue(status, emoteData.GetDuration())
					}
				}
			} else {
				fmt.Println("trying to animate with no active animation")
			}
		} else {
			log.Fatalf("no emote found with name: %s", element.Mood)
		}
		return // return here because if it's an emote, we don't want to change the actor sprite
	default: // if the action isn't a special case like emotes, then it's probably a sprite animation
		// move the actor if the action set the position
		if anim, ok := ActorAnimations[element.Action]; ok {
			fmt.Println("starting animation:", anim.Name)
			go AsyncAnimateActor(actor, anim, status)
		}
		break
	}
}

//////////////////////////////////////////////

func DrawSprite(sprite *hud.Sprite, m hud.Mat4, shader *gfx.Program) {
	sprite.Draw(m, shader)
}

func DrawText(view View, text *hud.Text, tx, ty float32) {
	text.Draw(float32(view.WindowWidth), float32(view.WindowHeight), tx, ty)
}

/////////////////////////////////////////////
// ANIMATIONS

func AsyncAnimateFadeOut(s *hud.Sprite, status *chan uint32) {

	s.SetAlpha(0)
	for s.GetAlpha() < 1 {
		select {
		case <-UniversalTicker:
			s.SetAlpha(s.GetAlpha() + float32(0.025))
			// fmt.Println("fade out:", s.GetAlpha())
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
			// fmt.Println("fade in:", s.GetAlpha())
			break
		}
	}

	*status <- 2 // send status update to listening channel
}

func AsyncAnimateActor(s *Actor, anim script.AnimationMetadata, status *chan uint32) {
	originalPosition := s.GetPosition()
	fmt.Println("original position:", originalPosition)
	speed := anim.Speed

	for _, frame := range anim.Frames {
		startingPosition := s.GetPosition()
		var targetPosition hud.Vec3
		if frame.Reset {
			targetPosition = originalPosition
		} else if frame.Center {
			targetPosition = s.GetCenter()
		} else {
			targetPosition = startingPosition
			if frame.X != nil {
				targetPosition[0] = *frame.X
			}
			if frame.AddX != nil {
				targetPosition[0] += *frame.AddX
			}
			if frame.Y != nil {
				targetPosition[1] = *frame.Y
			}
			if frame.AddY != nil {
				targetPosition[1] += *frame.AddY
			}
		}

		// fmt.Println("animation frame:", ind, "playing animation:", anim.Name, ", position:", s.GetPosition(), "to:", targetPosition)

		// this will move the actor to a target location
		// this check uses the length of the vector to account for floating point precision issues

		for targetPosition.Sub(s.GetPosition()).Len() > 0.01 {
			select {
			case <-UniversalTicker:
				pos := s.GetPosition()
				diff := targetPosition.Sub(startingPosition)
				// direction := diff.Normalize()
				move := diff.Mul(speed)
				newPos := pos.Add(move)
				s.SetPositionf(newPos.X(), newPos.Y(), newPos.Z())
				// fmt.Println("position:", pos, "new position:", newPos, "diff:", targetPosition.Sub(pos), "move:", move)
				// s.Translate(s.GetPosition().X()-0.1, 0, 0)
				// fmt.Println("translated X:", s.GetPosition().X())
				break
			}
		}

		if frame.Delay != nil {
			// fmt.Println("delaying frame by:", *frame.Delay)
			time.Sleep(time.Second * time.Duration(*frame.Delay))
		}
	}

	*status <- 1 // send status update to listening channel
}

func AsyncAnimateMove(s *hud.Sprite, targetPosition hud.Vec3, speed float32, status *chan uint32) {

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
	charSprite = make(map[string]*Actor, 0)
	subjectName = nil
	releaseReplies()
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

func releaseReplies() {
	for _, v := range reply {
		v.Release()
		v = nil
	}
	reply = make([]*hud.Text, 0)
}

func releaseDialogue() {
	if dialogue != nil {
		dialogue.Release()
		dialogue = nil
	}
}

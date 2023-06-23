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
	"github.com/BlunterMonk/our_archive/internal/loop"
	"github.com/BlunterMonk/our_archive/internal/script"
	"github.com/BlunterMonk/our_archive/pkg/gfx"
	"github.com/BlunterMonk/our_archive/pkg/sfx"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
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
	CurrentBG        string
	CurrentSpeaker   string
	CurrentFontSize  float32
	CurrentBgmVolume = float64(-2)
	MaxBgmVolume     = float64(0)
	MinBgmVolume     = float64(-10)
	CurrentSfxVolume = float64(1)
	DefaultFontSize  = 0.85

	Script *script.Script
	// Metadata        *script.Metadata
	FRAME_DURATION  = 16 * time.Millisecond
	UniversalTicker = time.Tick(FRAME_DURATION)

	// used to send events to main loop
	EventQueue   = make([]eventFunc, 0)
	EventChannel = make(chan loadEvent)
	DebugChannel = make(chan string)
	status       = make(chan uint32)

	// static assets
	fontRegular           = "regular"
	fontBold              = "bold"
	dialogue, subjectName *hud.Text
	reply                 []*hud.Text
	fade                  *hud.Sprite
	spriteReplySingle     = "text_option_single"
	spriteReplyDoubleA    = "text_option_a"
	spriteReplyDoubleB    = "text_option_b"
	spriteDialogueOverlay = "dialogue_bg"
	spriteDialogueBar     = "dialogue_bar"
	spriteAutoOn          = "auto_on"
	spriteAutoOff         = "auto_off"
	spriteMenuButton      = "menu"
	spriteEmoteBalloon    = "balloon"

	// dynamic assets
	Fonts           map[string]*v41.Font
	charSprite      map[string]*Actor
	Actors          map[string]*Actor
	ActorAnimations map[string]script.AnimationMetadata
	Backgrounds     map[string]*hud.Sprite
	Emotes          map[string]*hud.AnimatedSprite
	Factions        map[string]*hud.Text
	Names           map[string]*hud.Text
	Sounds          map[string]*sfx.Streamer
	Sprites         map[string]*hud.Sprite

	currentBGM           *sfx.Streamer
	shuttingDown         bool
	useStrictCoreProfile = (runtime.GOOS == "darwin")
	shaderProgram        *gfx.Program

	ACTOR_LEFT  = hud.Vec3{-0.5, -0.65, 0.0}
	ACTOR_RIGHT = hud.Vec3{0.5, -0.65, 0.0}
	AUTO        = false
	DEBUG       = false
	DEBUG_TEXT  string
	LOADING     = false
)

const (
	XCODE_SHUTDOWN_SIGNAL = 0
	XCODE_CONSUMER_FAILED = 4
	XCODE_PANIC           = 5
	XCODE_ABORT           = 6
)

type eventFunc func(s *hud.Text)
type loadEvent struct {
	Category string
	Key      string
	Object   string
}
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

	loadGame(LANDSCAPE_VIEW, scriptName)
	runGame(LANDSCAPE_VIEW, scriptName)
}

func loadScriptFilenames() []string {
	files, err := os.ReadDir("./resources/scripts/")
	if err != nil {
		panic(err)
	}

	filenames := make([]string, 0)
	for _, v := range files {
		if v.IsDir() {
			continue
		}

		filenames = append(filenames, v.Name())
	}

	return filenames
}

func loadResource(load loadEvent) error {

	var err error
	category := load.Category
	key := load.Key
	objectName := load.Object

	log.Printf("loading %s: %s", category, key)

	switch category {
	case "font":
		Fonts[objectName] = gfx.MustLoadFont(key)
		Fonts[objectName].ResizeWindow(float32(LANDSCAPE_VIEW.WindowWidth), float32(LANDSCAPE_VIEW.WindowHeight))
	case "bg":
		if _, ok := Backgrounds[key]; !ok {
			Backgrounds[key], err = hud.NewSpriteFromFile(fmt.Sprintf("./resources/%s/%s.jpeg", category, key))
		}
	case "bgm":
		if _, ok := Sounds[key]; !ok {
			Sounds[key], err = sfx.NewStreamer(fmt.Sprintf("./resources/%s/%s.mp3", category, key))
		}
	case "sfx":
		if _, ok := Sounds[key]; !ok {
			Sounds[key], err = sfx.NewStreamer(fmt.Sprintf("./resources/%s/%s.mp3", category, key))
		}
	case "emote": // load the emote if it isn't already
		if _, ok := Emotes[key]; !ok {
			Emotes[key] = hud.NewAnimatedSpriteFromFile(fmt.Sprintf("./resources/%s/%s.gif", category, key))

			// load sfx for emote
			if _, ok := Sounds[key]; !ok {
				Sounds[key], err = sfx.NewStreamer(fmt.Sprintf("./resources/sfx/%s.mp3", key))
			}
		}
		break
	case "name":
		if _, ok := Names[key]; !ok {
			Names[key] = hud.NewSolidText(toTitle(key), hud.COLOR_WHITE, Fonts["bold"])
			Names[key].SetScale(float32(speakerScale))
		}
	case "faction":
		// create names if they don't exist
		if _, ok := Factions[key]; !ok {
			name := strings.ToLower(load.Object)
			Factions[name] = hud.NewSolidText(key, mgl32.Vec3{0.49, 0.81, 1}, Fonts["bold"])
			Factions[name].SetScale(0.8)
		}
	case "actor":
		if _, ok := Actors[objectName]; !ok {
			// need to create the actor in the same thread as the sprites
			// for whatever reason, when the sprite and actor are created separately
			// open gl thinks they are on different threads and crashes
			Actors[objectName] = NewActor(objectName)
		}

		// fmt.Printf("loading %s texture for %s\n", key, objectName)
		err = Actors[objectName].LoadTexture(key, fmt.Sprintf("./resources/%s/%s/%s.png", category, objectName, key))
		break
	case "sprite":
		Sprites[key], err = hud.NewSpriteFromFile(fmt.Sprintf("./resources/%s/%s.png", objectName, key))
		if key == spriteEmoteBalloon {
			Sprites[key].SetScale(0.085)
		}
		break
	case "overlay":
		fade = hud.NewSprite()
		err = fade.LoadTexture("black", "./resources/bg/black_screen.jpeg")
		err = fade.LoadTexture("white", "./resources/bg/white_screen.jpeg")
		fade.SetPositionf(0, 0, 0)
		fade.SetAlpha(0)
		break
	}

	return err
}

func queueResources(view View, scriptName string) ([]loadEvent, *script.Metadata, []error) {
	var err error

	loads := make([]loadEvent, 0)
	missing := make([]error, 0)

	releaseResources()

	// init resource containers
	loads = append(loads, loadEvent{Object: "regular", Key: "NotoSans-Regular", Category: "font"})
	loads = append(loads, loadEvent{Object: "bold", Key: "NotoSans-Bold", Category: "font"})
	loads = append(loads, loadEvent{Object: "ui", Key: "text_option_single", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "text_option_a", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "text_option_b", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "dialogue_bg", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "dialogue_bar", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "auto_on", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "auto_off", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "menu", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "ui", Key: "balloon", Category: "sprite"})
	loads = append(loads, loadEvent{Object: "", Key: "", Category: "overlay"})

	fmt.Println("system resources loaded")

	Script = script.NewScriptFromFile(fmt.Sprintf("./resources/scripts/%s.txt", scriptName))
	metadata, err := script.LoadMetadata("./resources/settings.json")
	if err != nil {
		missing = append(missing, err)
	}

	loads = append(loads, loadEvent{Key: "all", Category: "name"})
	for _, v := range Script.Elements() {
		switch v.Name {
		case "all":
			if v.Action != "emote" && v.Action != "_" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
			continue
		case "defect", "none", "clear", "_", "font", "fade":
			continue
		case "bgm":
			switch v.Mood {
			case "pause", "resume", "fade", "_":
				continue
			case "play":
				loads = append(loads, loadEvent{Key: v.Action, Category: "bgm"})
			default:
				loads = append(loads, loadEvent{Key: v.Mood, Category: "bgm"})
			}
			continue
		case "bg", "sfx", "emote": //, "name", "faction", "sprite":
			loads = append(loads, loadEvent{Key: v.Mood, Category: v.Name})
			continue
		}

		// anything else is regarded as an actor

		// create names if they don't exist
		loads = append(loads, loadEvent{Key: v.Name, Category: "name"})
		// create faction text
		for _, actor := range metadata.Actors {
			if actor.FactionName == nil || *actor.FactionName == "" {
				continue
			}
			if actor.Name == v.Name {
				loads = append(loads, loadEvent{Object: v.Name, Key: *actor.FactionName, Category: "faction"})
				break
			}
		}

		// check to see if the action is an emote
		switch v.Mood {
		case "fade", "full", "silhouette", "rename", "defect":
			break
		case "emote": // load the emote if it isn't already
			loads = append(loads, loadEvent{Key: v.Action, Category: "emote"})
			break
		case "animation", "_":
			if v.Action != "_" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
			break
		default: // if it's not an emote, then load the texture onto the actor as an expression
			loads = append(loads, loadEvent{Object: v.Name, Key: spriteKey(v), Category: "actor"})
		}
	}

	filtered := make([]error, 0)
	for _, v := range missing {
		if v != nil {
			filtered = append(filtered, v)
		}
	}

	return loads, metadata, filtered
}

func applyMetadata(metadata *script.Metadata) {
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
				switch emote.Type {
				case "head":
					a.AddEmoteData(emote.Name, hud.Vec3{actor.EmoteOffsetHead.X, actor.EmoteOffsetHead.Y, 0})
				case "bubble":
					a.AddEmoteData(emote.Name, hud.Vec3{actor.EmoteOffsetBubble.X, actor.EmoteOffsetBubble.Y, 0})
				}
			}
		}
	}
	for _, emote := range metadata.Emotes {
		if a, ok := Emotes[emote.Name]; ok {
			a.SetScale(emote.Scale)
		}
	}
	for _, anim := range metadata.Animation {
		if _, ok := ActorAnimations[anim.Name]; ok {
			log.Println("duplicate animation:", anim.Name)
		}
		ActorAnimations[anim.Name] = anim
	}
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
	for _, v := range Fonts {
		v.Release()
	}
	if shaderProgram != nil {
		shaderProgram.Delete()
	}
	for _, v := range Sounds {
		v.Release()
	}

	charSprite = make(map[string]*Actor)
	Actors = make(map[string]*Actor)
	ActorAnimations = make(map[string]script.AnimationMetadata)
	Backgrounds = make(map[string]*hud.Sprite)
	Emotes = make(map[string]*hud.AnimatedSprite)
	Factions = make(map[string]*hud.Text)
	Names = make(map[string]*hud.Text)
	Sounds = make(map[string]*sfx.Streamer)
	Sprites = make(map[string]*hud.Sprite)
	Fonts = make(map[string]*v41.Font)
}

func loadGame(view View, scriptName string) {
	if LOADING {
		return
	}

	EventChannel = make(chan loadEvent)
	LOADING = true
	loadEvents, metadata, missing := queueResources(view, scriptName)
	fmt.Println("events to load:", len(loadEvents))
	go func() {
		missingText := errorsToString(missing)
		if missingText != "" {
			DebugChannel <- missingText
		}

		for _, v := range loadEvents {
			// fmt.Println("queuing event:", v)
			EventChannel <- v
		}
		log.Println("applying metadata")
		applyMetadata(metadata)
		log.Println("finishing load")
		close(EventChannel)
	}()
}

var spinner *hud.Animation

func runGame(CurrentViewConfig View, scriptName string) {
	var xCode int

	window := gfx.Init(CurrentViewConfig.WindowWidth, CurrentViewConfig.WindowHeight)
	window.SetKeyCallback(keyCallback)
	window.SetMouseButtonCallback(mouseButtonCallback)
	sfx.Init()
	hud.Init()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)

	ft := time.Tick(time.Second)
	killswitch := make(chan int, 0)
	shaderProgram = gfx.MustInitShader()

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

	// mustLoadSystemResources(CurrentViewConfig)

	// load system sounds
	logo, _ := hud.NewSpriteFromFile("./resources/ui/splash.jpeg")
	spinner = hud.NewAnimation("spinner", hud.NewAnimatedSpriteFromFile("./resources/ui/spinrona.gif"))
	spinner.SetPositionf(0, -0.45, 0)
	spinner.SetScale(0.3)
	spinner.AnimateForever()

	gl.ClearColor(0.4, 0.4, 0.4, 0.0)
	gl.BlendColor(1, 1, 1, 1)

	var delay time.Timer
	var debugText *hud.Text
	var debugString string
	loadErrors := make([]error, 0)
	counter := 0
F:
	for {
		if window.ShouldClose() {
			break F
		}

		gl.Clear(gl.COLOR_BUFFER_BIT)
		// mtx.Lock()
		// for _, v := range EventQueue {
		// 	v(dialogue)
		// }
		// EventQueue = make([]eventFunc, 0)
		// loadResource(<-EventChannel)
		// mtx.Unlock()

		select {
		case <-ft:
			counter = 0

		case d, ok := <-DebugChannel:
			fmt.Println("debug channel")
			if ok {
				if debugString != d {
					debugText = nil
				}
				debugString = d
			}
			break

		// case le, ok := <-EventChannel:
		// 	// fmt.Println("event channel")
		// 	if ok {
		// 		// if !LOADING {
		// 		// 	LOADING = true
		// 		// }
		// 		fmt.Println("loading:", le)
		// 		loadResource(le)
		// 	} else if LOADING {
		// 		LOADING = false
		// 		fmt.Println("load complete")
		// 		for i := range EventChannel {
		// 			fmt.Printf("flushing %v\n", i)
		// 		}
		// 	}

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
					// if s, ok := Sounds["next"]; ok {
					// 	s.Play()
					// }
					nextDialogue(&status)
				}
			}

		case <-UniversalTicker:
			// fmt.Println("ticker channel:", counter)
			counter++

			// enable shader
			shaderProgram.Use()

			gl.Enable(gl.BLEND) //Enable blending.
			gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

			if debugString != "" && debugText == nil {
				if _, ok2 := Fonts[fontRegular]; ok2 {
					debugText = hud.NewSolidText(debugString, mgl32.Vec3{0.9, 0.9, 0.9}, Fonts[fontRegular])
				}
			}

			if LOADING {
				DrawSprite(logo, hud.NewMat4(), shaderProgram) // dialogue window
				spinner.Draw(spinner.GetPosition(), shaderProgram)
				if debugText != nil {
					DrawText(CurrentViewConfig, debugText, 0, 0)
				}
				if counter > 1 {
					le, ok := <-EventChannel
					if ok {
						// fmt.Println("loading:", le)
						err := loadResource(le)
						if err != nil {
							debugString = fmt.Sprintf("%s\n%s", debugString, err.Error())
							loadErrors = append(loadErrors, err)
						}
					} else if LOADING {
						LOADING = false
						// fmt.Println("load complete")
						for i := range EventChannel {
							fmt.Printf("flushing %v\n", i)
						}
					}
				}

				// end of draw loop
				window.SwapBuffers()
				glfw.PollEvents()
				break
			}

			// draw image

			drawBackgrounds()
			drawActors()
			drawUI(CurrentViewConfig)
			drawText(CurrentViewConfig)
			if debugText != nil {
				DrawText(CurrentViewConfig, debugText, 0, 0)
			}

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
	if bg, ok := Backgrounds[CurrentBG]; ok {
		DrawSprite(bg, hud.NewMat4(), shaderProgram) // background
	}
}
func drawActors() {
	// if sprite, ok := charSprite["akira"]; ok {
	// 	// DrawSprite(sprite.Sprite, hud.NewMat4(), shaderProgram)
	// 	sprite.Draw(shaderProgram)
	// }
	// return
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
		if actor, ok := charSprite[name]; ok {
			// don't recolor actors being faded by animations
			if !actor.Faded {
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
			}

			// draw the actor
			actor.Draw(shaderProgram)
		}
	}
}
func drawUI(view View) {
	// Draw text
	if dialogue != nil {
		DrawSprite(Sprites[spriteDialogueOverlay], hud.NewMat4(), shaderProgram) // dialogue window
		DrawSprite(Sprites[spriteDialogueBar], hud.NewMat4(), shaderProgram)     // dialogue bar overlay
	}
	if len(reply) == 1 {
		DrawSprite(Sprites[spriteReplySingle], hud.NewMat4(), shaderProgram)
	} else if len(reply) == 2 {
		DrawSprite(Sprites[spriteReplyDoubleA], hud.NewMat4(), shaderProgram)
		DrawSprite(Sprites[spriteReplyDoubleB], hud.NewMat4(), shaderProgram)
	}
	if AUTO {
		DrawSprite(Sprites[spriteAutoOn], hud.NewMat4(), shaderProgram)
	} else {
		DrawSprite(Sprites[spriteAutoOff], hud.NewMat4(), shaderProgram)
	}
	DrawSprite(Sprites[spriteMenuButton], hud.NewMat4(), shaderProgram)
	if DEBUG {
		Sprites[spriteEmoteBalloon].Draw(Sprites[spriteEmoteBalloon].GetTransform(), shaderProgram)
	}
}
func drawText(view View) {
	// Draw text
	if dialogue != nil {
		DrawText(view, dialogue, view.dialogueX, view.dialogueY) // actual text
		if subjectName != nil {
			DrawText(view, subjectName, view.speakerX, view.speakerY) // speaker's name
			if factionName, ok := Factions[strings.ToLower(CurrentSpeaker)]; ok && factionName != nil {
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
			p := s.GetPosition()
			var text string
			if sprite, ok := charSprite[CurrentSpeaker]; ok {
				emoteOffset := sprite.GetPosition().Sub(Sprites[spriteEmoteBalloon].GetPosition())
				text = fmt.Sprintf("position: (%f, %f)\nscale: (%f)\nemote offset from %s: (%f, %f)\nVolume: (%f)", p.X(), p.Y(), s.GetScale(), CurrentSpeaker, emoteOffset.X(), emoteOffset.Y(), CurrentBgmVolume)
			} else {
				text = fmt.Sprintf("position: (%f, %f)\nscale: (%f)\nVolume: (%f)", p.X(), p.Y(), s.GetScale(), CurrentBgmVolume)
			}
			pos := hud.NewSolidText(text, hud.COLOR_WHITE, Fonts[fontRegular])
			pos.SetScale(2)
			DrawText(view, pos, 0, 0)
		}
	}
}
func drawOverlays() {
	if fade != nil {
		DrawSprite(fade, hud.NewMat4(), shaderProgram) // dialogue window
	}
}

func keyCallback(window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if LOADING {
		return
	}

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
		fmt.Println("bgm volume:", CurrentBgmVolume)
	}
	if key == glfw.KeyMinus {
		CurrentBgmVolume -= 0.5
		if currentBGM != nil {
			currentBGM.SetVolume(CurrentBgmVolume)
		}
		fmt.Println("bgm volume:", CurrentBgmVolume)
	}

	s, ok := charSprite[CurrentSpeaker]
	if !ok {
		return
	}

	activeChar := s.Sprite
	if key == glfw.KeyLeft {
		moveActor(activeChar, -0.01, 0)
	}
	if key == glfw.KeyRight {
		moveActor(activeChar, 0.01, 0)
	}
	if key == glfw.KeyUp {
		moveActor(activeChar, 0, 0.01)
	}
	if key == glfw.KeyDown {
		moveActor(activeChar, 0, -0.01)
	}
	if key == glfw.KeyS {
		scaleActor(activeChar, -0.01)
	}
	if key == glfw.KeyB {
		scaleActor(activeChar, 0.01)
	}

	// move emote balloon
	if key == glfw.KeyJ {
		moveActor(Sprites[spriteEmoteBalloon], -0.01, 0)
	}
	if key == glfw.KeyL {
		moveActor(Sprites[spriteEmoteBalloon], 0.01, 0)
	}
	if key == glfw.KeyI {
		moveActor(Sprites[spriteEmoteBalloon], 0, 0.01)
	}
	if key == glfw.KeyK {
		moveActor(Sprites[spriteEmoteBalloon], 0, -0.01)
	}
}
func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
	if LOADING {
		return
	}

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
			loadGame(LANDSCAPE_VIEW, "test")
		} else {
			delayNextDialogue(&status, time.Millisecond)
		}
	}
}

func queueEvent(event eventFunc) {
	EventQueue = append(EventQueue, event)
}
func nextDialogue(status *chan uint32) {
	if LOADING {
		return
	}

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
			Factions[name] = hud.NewSolidText(elementText, mgl32.Vec3{0.49, 0.81, 1}, Fonts["bold"])
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
		CurrentBG = element.Mood
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
		prepareBgm(element, status)
		nextDialogue(status)
		break
	case "fade":
		if element.Mood == "white" {
			fade.SetActiveTexture("white")
		} else {
			fade.SetActiveTexture("black")
		}

		// fade the overlay and continue the script after
		AsyncSpriteAlpha(fade, status, element.Action != "in", func() {
			*status <- 2
		})
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
			reply = append(reply, hud.NewSolidText(v, mgl32.Vec3{0.18, 0.255, 0.322}, Fonts[fontRegular]))
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
		dialogue = hud.NewText(element.Line, hud.COLOR_WHITE, Fonts[fontRegular])
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
	shouldChangeSprite := prepareActorAnimation(status, &element, charSprite[element.Name], !element.HasDialogue())

	if shouldChangeSprite {
		// if sprite, ok := Sprites[spriteKey(element)]; ok {
		err := charSprite[element.Name].SetActiveTexture(spriteKey(element))
		if err != nil {
			fmt.Println("error loading sprite: ", err.Error())
			DebugChannel <- fmt.Sprintf("actor (%s) is missing sprite (%s)", element.Name, element.Mood)
		}
		// } else {
		// 	log.Println("sprite not found:", spriteKey(element))
		// }
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
		if s, ok := Sounds[element.Mood]; ok {
			fmt.Println("sfx found")
			return emoteData, s
		}
		return emoteData, nil
	}

	fmt.Println("not found")
	return nil, nil
}

func prepareActorAnimation(status *chan uint32, element *script.ScriptElement, actor *Actor, autoNextDialogue bool) bool {

	action := element.Mood
	actionName := element.Action
	if element.Action == "emote" { // LEGACY
		action = "emote"
		actionName = element.Mood
	}

	switch action {
	case "emote":
		if _, ok := Emotes[actionName]; ok {
			if emoteData, ok := Emotes[actionName]; ok {
				actor.AnimateEmote(actionName, emoteData, func() {
					// nextDialogue(status)
					fmt.Println("done animating emote")
				})

				if s, ok := Sounds[actionName]; ok {
					fmt.Println("playing sfx:", actionName)
					s.Play(CurrentSfxVolume)
					if autoNextDialogue {
						delayNextDialogue(status, emoteData.GetDuration())
					}
				}
			} else {
				fmt.Println("trying to animate with no active animation")
			}
		} else {
			log.Fatalf("no emote found with name: %s", actionName)
		}
		return false // return here because if it's an emote, we don't want to change the actor sprite
	case "silhouette":
		actor.Faded = true
		actor.SetColorf(0, 0, 0)
		return false
	case "full":
		actor.Faded = false
		actor.SetColorf(1, 1, 1)
		return false
	case "fade":
		if actionName == "in" {
			fmt.Println("actor fade in")
			AsyncActorColor(actor, status, false)
		} else {
			fmt.Println("actor fade out")
			AsyncActorColor(actor, status, true)
		}
		return false
	case "defect":
		if _, ok := Factions[actor.name]; ok {
			if actionName == "_" {
				Factions[actor.name] = nil // remove faction
			} else {
				Factions[actor.name] = hud.NewSolidText(actionName, mgl32.Vec3{0.49, 0.81, 1}, Fonts["bold"])
				Factions[actor.name].SetScale(0.8)
			}
		}
		return false
	case "rename":
		Names[actor.name] = hud.NewSolidText(toTitle(actionName), hud.COLOR_WHITE, Fonts["bold"])
		Names[actor.name].SetScale(float32(speakerScale))
		return false
	default: // if the action isn't a special case like emotes, then it's probably a sprite animation
		// move the actor if the action set the position
		if anim, ok := ActorAnimations[actionName]; ok {
			// fmt.Println("starting animation:", anim.Name)
			go AsyncAnimateActor(actor, anim, status)
		}
		if action == "animation" {
			return false
		}
		break
	}

	return true
}

func prepareBgm(element script.ScriptElement, status *chan uint32) {
	bgmAction := element.Mood
	bgmActionName := element.Action
	if element.Mood == "play" {
		bgmAction = element.Action
	}

	switch bgmAction {
	case "resume":
		if currentBGM != nil {
			currentBGM.Resume()
		}
		break
	case "pause":
		if currentBGM != nil {
			currentBGM.Pause()
		}
		break
	case "fade":
		if currentBGM != nil {
			AsyncStreamerVolume(currentBGM, bgmActionName == "in")
		}
		break
	default:
		if s, ok := Sounds[bgmAction]; ok {
			fmt.Println("playing bgm:", bgmAction)
			if currentBGM != nil {
				currentBGM.Close()
			}
			s.PlayOnRepeat(CurrentBgmVolume)
			currentBGM = s
		}
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

func AsyncActorColor(s *Actor, status *chan uint32, toBlack bool) {

	s.Faded = true
	var fadeFunc func(f float64) int
	if toBlack {
		fadeFunc = func(f float64) int {
			c := s.GetColor()
			v := c.X() - float32(0.025)
			if v <= 0 {
				s.SetColorf(0, 0, 0)
				return -1
			}
			s.SetColorf(v, v, v)
			return 0
		}
	} else {
		fadeFunc = func(f float64) int {
			c := s.GetColor()
			v := c.X() + float32(0.025)
			if v >= 1 {
				s.SetColorf(1, 1, 1)
				return -1
			}
			s.SetColorf(v, v, v)
			return 0
		}
	}

	loop := loop.New(FRAME_DURATION, fadeFunc, func() {
		log.Println("finished actor fade")
		s.Faded = false
	})
	loop.Start()
}

func AsyncStreamerVolume(s *sfx.Streamer, in bool) {
	go func() {
		if in {
			fmt.Println("fading bgm in")
			s.SetVolume(MinBgmVolume)
			for s.GetVolume() < CurrentBgmVolume {
				select {
				case <-UniversalTicker:
					f := s.GetVolume() + 0.5
					fmt.Println("increasing volume:", f)
					if f > CurrentBgmVolume {
						f = CurrentBgmVolume
					}
					s.SetVolume(f)
				}
			}
		} else {
			s.SetVolume(CurrentBgmVolume)
			for s.GetVolume() > MinBgmVolume {
				select {
				case <-UniversalTicker:
					f := s.GetVolume() - 0.5
					if f < MinBgmVolume {
						f = MinBgmVolume
					}
					s.SetVolume(f)
				}
			}
		}

		fmt.Println("finished bgm fade")
	}()
}

func AsyncSpriteAlpha(s *hud.Sprite, status *chan uint32, in bool, done func()) {

	var fadeFunc func(f float64) int
	if in {
		s.SetAlpha(0)
		fadeFunc = func(f float64) int {
			v := s.GetAlpha() + float32(0.025)
			if v >= 1 {
				s.SetAlpha(1)
				return -1
			}
			s.SetAlpha(v)
			return 0
		}
	} else {
		s.SetAlpha(1)
		fadeFunc = func(f float64) int {
			v := s.GetAlpha() - float32(0.025)
			if v <= 0 {
				s.SetAlpha(0)
				return -1
			}
			s.SetAlpha(v)
			return 0
		}
	}

	loop := loop.New(FRAME_DURATION, fadeFunc, func() {
		log.Println("finished sprite fade")
		if done != nil {
			done()
		}
	})
	loop.Start()
}

func AsyncAnimateActor(s *Actor, anim script.AnimationMetadata, status *chan uint32) {
	originalPosition := s.GetPosition()
	// fmt.Println("original position:", originalPosition)
	speed := anim.Speed

	ticker := time.Tick(FRAME_DURATION)
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
			case <-ticker:
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

/////////////////////////////////////////////
// HELPER FUNCTIONS

func spriteKey(e script.ScriptElement) string {
	return fmt.Sprintf("%v-%v", e.Name, e.Mood)
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

//////////////////////////////////////////////////////
// HELPER

func errorsToString(errors []error) string {
	if len(errors) == 0 {
		return ""
	}

	var texts []string
	for _, v := range errors {
		if v == nil {
			continue
		}
		texts = append(texts, v.Error())
	}

	return fmt.Sprintf("Script Errors:\n%s", strings.Join(texts, "\n"))
}

func inside(rect image.Rectangle, x, y int) bool {
	return x >= rect.Min.X && x <= rect.Max.X && y >= rect.Min.Y && y <= rect.Max.Y
}

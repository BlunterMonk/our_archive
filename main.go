package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/alecthomas/kingpin"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	fontRegular           = "regular"
	fontBold              = "bold"
	spriteReplySingle     = "text_option_single"
	spriteReplyDoubleA    = "text_option_a"
	spriteReplyDoubleB    = "text_option_b"
	spriteDialogueOverlay = "dialogue_bg"
	spriteDialogueBar     = "dialogue_bar"
	spriteAutoOn          = "auto_on"
	spriteAutoOff         = "auto_off"
	spriteMenuButton      = "menu"
	spriteEmoteBalloon    = "balloon"
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
	// PORTRAIT_VIEW = View{
	// 	speakerX:     float32(0),
	// 	speakerY:     float32(0),
	// 	dialogueX:    float32(0),
	// 	dialogueY:    float32(0),
	// 	WindowWidth:  720,
	// 	WindowHeight: 1280,
	// }
	PORTRAIT_VIEW = View{
		speakerX:     float32(124),
		speakerY:     float32(515),
		dialogueX:    float32(450),
		dialogueY:    float32(573),
		WindowWidth:  1280,
		WindowHeight: 720,
	}
	CURRENT_VIEW     = LANDSCAPE_VIEW
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
	FPS              int

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
	dialogue, subjectName *hud.Text
	reply                 []*Reply
	fade                  *hud.Sprite

	// dynamic assets
	Fonts           map[string]*v41.Font
	charSprite      map[string]*Actor
	Actors          map[string]*Actor
	Clones          map[string]string
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

	ACTOR_LEFT           = hud.Vec3{-0.5, -0.65, 0.0}
	ACTOR_RIGHT          = hud.Vec3{0.5, -0.65, 0.0}
	AUTO                 = false
	DEBUG                = false
	DEBUG_TEXT           string
	LOADING              = false
	WAITING_CONFIRMATION = false

	program        = kingpin.New("our_archive", "our_archive")
	flagScriptName = program.Flag("script", "name of the script to run").Short('s').String()
	// flagLogLevel = program.Flag("log", "log level").String()

	// HUD Rects
	rectReplySingle  = image.Rect(200, 265, 1080, 340)
	rectReplyDoubleA = image.Rect(200, 220, 1080, 290)
	rectReplyDoubleB = image.Rect(200, 315, 1080, 380)
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
type Reply struct {
	*hud.Text
	Position hud.Vec2
	Active   bool
	Sprite   *hud.Sprite
	start    *hud.Animation
	end      *hud.Animation
}

func (r *Reply) IsAnimating() bool {
	return r.start.IsAnimating() || r.end.IsAnimating()
}

// in Open GL, Y starts at the bottom

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

var scriptName string

// go run ./... -ldflags "-H windowsgui" "rabu"
func main() {

	_, err := program.Parse(os.Args[1:])
	if err != nil {
		_, err := program.Parse(os.Args[3:])
		if err != nil {
			panic(err)
		}
	}

	if flagScriptName != nil && *flagScriptName != "" {
		scriptName = *flagScriptName
	} else {
		scriptName = "test"
	}

	loadGame(CURRENT_VIEW, scriptName)
	runGame(CURRENT_VIEW, scriptName)
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
		if _, ok := Fonts[objectName]; !ok {
			Fonts[objectName] = gfx.MustLoadFont(key)
			Fonts[objectName].ResizeWindow(float32(LANDSCAPE_VIEW.WindowWidth), float32(LANDSCAPE_VIEW.WindowHeight))
		}
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
			s, err := sfx.NewStreamer(fmt.Sprintf("./resources/%s/%s.mp3", category, key))
			if err != nil {
				return err
			}
			Sounds[key] = s
		}
	case "emote": // load the emote if it isn't already
		if _, ok := Emotes[key]; !ok {
			Emotes[key] = hud.NewAnimatedSpriteFromFile(fmt.Sprintf("./resources/%s/%s.gif", category, key))

			// load sfx for emote
			if _, ok := Sounds[key]; !ok {
				Sounds[key], err = sfx.NewStreamer(fmt.Sprintf("./resources/sfx/%s.mp3", key))
			}
		}
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
		originalName := objectName
		if k, ok := Clones[objectName]; ok {
			// fmt.Println("replacing cloned name", objectName, k)
			originalName = k
		}

		if _, ok := Actors[objectName]; !ok {
			// need to create the actor in the same thread as the sprites
			// for whatever reason, when the sprite and actor are created separately
			// open gl thinks they are on different threads and crashes
			Actors[objectName] = NewActor(objectName)
		}

		// fmt.Printf("loading %s texture for %s\n", key, objectName)
		err = Actors[objectName].LoadTexture(key, fmt.Sprintf("./resources/%s/%s/%s-%s.png", category, originalName, originalName, key))
	case "sprite":
		Sprites[key], err = hud.NewSpriteFromFile(fmt.Sprintf("./resources/%s/%s.png", objectName, key))
		if key == spriteEmoteBalloon {
			Sprites[key].SetScale(0.085)
		}
	case "overlay":
		fade = hud.NewSprite()
		err = fade.LoadTexture("black", "./resources/bg/black_screen.jpeg")
		if err != nil {
			return err
		}
		err = fade.LoadTexture("white", "./resources/bg/white_screen.jpeg")
		fade.SetPositionf(0, 0, 0)
		fade.SetAlpha(0)
	}

	return err
}

func queueResources(view View, scriptName string) ([]loadEvent, *script.Metadata, []error) {
	var err error

	loads := make([]loadEvent, 0)
	missing := make([]error, 0)

	releaseResources()

	// init resource containers
	loads = append(loads, loadEvent{Object: "regular", Key: "NotoSansJP-Regular", Category: "font"})
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
	loads = append(loads, loadEvent{Object: "sfx", Key: "touch", Category: "sfx"})

	fmt.Println("system resources loaded")

	Script = script.NewScriptFromFile(fmt.Sprintf("./resources/scripts/%s.txt", scriptName))
	metadata, err := script.LoadMetadata("./resources/settings.json")
	if err != nil {
		missing = append(missing, err)
	}

	// search for clones first so we can populate the cache
	for _, v := range Script.Elements {
		if v.Name == "clone" {
			Clones[v.Action] = v.Mood
		}
	}

	loads = append(loads, loadEvent{Key: "all", Category: "name"})
	for _, v := range Script.Elements {
		switch v.Name {
		case "all":
			if v.Action != "emote" && v.Action != "_" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
			continue
		case "clone", "defect", "delay", "none", "clear", "_", "font", "fade":
			// special action tags don't need to be loaded
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
		if actor, ok := metadata.Actors[v.Name]; ok {
			if actor.FactionName != nil && *actor.FactionName != "" {
				loads = append(loads, loadEvent{Object: v.Name, Key: *actor.FactionName, Category: "faction"})
			}
		}

		// check to see if the action is an emote
		switch v.Mood {
		case "fade", "full", "silhouette", "rename", "defect":
		case "emote": // load the emote if it isn't already
			loads = append(loads, loadEvent{Key: v.Action, Category: "emote"})
		case "animation", "_":
			if v.Action != "_" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
		default: // if it's not an emote, then load the texture onto the actor as an expression
			loads = append(loads, loadEvent{Object: v.Name, Key: v.Mood, Category: "actor"})
			if v.Action != "_" {
				missing = append(missing, verifyAnimation(v.Action, metadata))
			}
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
	for name, actor := range metadata.Actors {
		if a, ok := Actors[name]; ok {
			a.SetCenter(actor.CenterX, actor.CenterY, actor.CenterScale)
			a.SetPositionf(actor.CenterX, actor.CenterY, 0)
			a.SetScale(actor.CenterScale)

			if actor.FactionName != nil && *actor.FactionName != "" {
				a.FactionName = *actor.FactionName
			}

			// add emote data to actor
			for emoteName, emote := range metadata.Emotes {
				switch emote.Type {
				case "head":
					a.AddEmoteData(emoteName, hud.Vec3{actor.EmoteOffsetHead.X, actor.EmoteOffsetHead.Y, 0})
				case "bubble":
					a.AddEmoteData(emoteName, hud.Vec3{actor.EmoteOffsetBubble.X, actor.EmoteOffsetBubble.Y, 0})
				}
			}
		}
	}
	for name, emote := range metadata.Emotes {
		if a, ok := Emotes[name]; ok {
			a.SetScale(emote.Scale)
		}
	}
	ActorAnimations = metadata.Animations
}

func verifyAnimation(name string, metadata *script.Metadata) error {
	if _, ok := metadata.Animations[name]; ok {
		return nil
	}

	return fmt.Errorf("animation with name \"%s\" found in script, but not in settings.json", name)
}

func releaseResources() {

	// clean up resources
	releaseDialogue()
	releaseReplies()
	for _, v := range Sounds {
		if v != nil {
			v.Release()
		}
	}

	charSprite = make(map[string]*Actor)
	Actors = make(map[string]*Actor)
	Clones = make(map[string]string)
	ActorAnimations = make(map[string]script.AnimationMetadata)
	Backgrounds = make(map[string]*hud.Sprite)
	Emotes = make(map[string]*hud.AnimatedSprite)
	Factions = make(map[string]*hud.Text)
	Names = make(map[string]*hud.Text)
	Sounds = make(map[string]*sfx.Streamer)
	Sprites = make(map[string]*hud.Sprite)
	if Fonts == nil {
		Fonts = make(map[string]*v41.Font)
	}
	dialogueIndex = -1
}

func shutdown() {
	for _, v := range Fonts {
		v.Release()
	}
	if shaderProgram != nil {
		shaderProgram.Delete()
	}

	releaseResources()
	sfx.Close()
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

var replyStart, replyEnd *hud.Animation

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
	killswitch := make(chan int)
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

	// load loading screen first
	logo, _ := hud.NewSpriteFromFile("./resources/ui/splash.jpeg")
	spinner := hud.NewAnimation("spinner", hud.NewAnimatedSpriteFromFile("./resources/ui/spinrona.gif"))
	spinner.SetPositionf(0, -0.45, 0)
	spinner.SetScale(0.3)
	spinner.AnimateForever()

	// replyStart = hud.NewAnimation("reply_start", hud.NewAnimatedSpriteFromFile("./resources/ui/reply_start.gif"))
	// replyStart.SetPositionf(0, 0, 0)
	// replyStart.SetScale(1)
	// replyStart.AnimateForever()

	// replyEnd = hud.NewAnimation("reply_end", hud.NewAnimatedSpriteFromFile("./resources/ui/reply_end.gif"))
	// replyEnd.SetPositionf(0, 0, 0)
	// replyEnd.SetScale(1)
	// replyEnd.AnimateForever()

	gl.ClearColor(0.4, 0.4, 0.4, 0.0)
	gl.BlendColor(1, 1, 1, 1)

	// get 2D projection matrix for the aspect ratio
	var screenProjMatrix hud.Mat4 = hud.ProjMatrix(float32(CurrentViewConfig.WindowWidth), float32(CurrentViewConfig.WindowHeight))

	var delay time.Timer
	var debugText *hud.Text
	var debugString string
	var counter int
	loadErrors := make([]error, 0)
F:
	for {
		if window.ShouldClose() {
			break F
		}

		gl.Clear(gl.COLOR_BUFFER_BIT)

		select {
		case <-ft:
			FPS = counter
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
			case 3:
				WAITING_CONFIRMATION = false
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

			// if replyEnd != nil {
			// 	replyEnd.Draw(replyEnd.GetPosition(), shaderProgram)
			// }
			if LOADING {
				DrawSprite(logo, hud.NewMat4(), shaderProgram) // dialogue window
				spinner.Draw(screenProjMatrix, spinner.GetPosition(), shaderProgram)
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
			drawActors(screenProjMatrix)
			drawUI(CurrentViewConfig, screenProjMatrix)
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

	shutdown()
	os.Exit(xCode)
}

func drawBackgrounds() {
	if bg, ok := Backgrounds[CurrentBG]; ok {
		DrawSprite(bg, hud.NewMat4(), shaderProgram) // background
	}
}
func drawActors(proj hud.Mat4) {
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
			if !actor.Faded && !actor.Silhouette {
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
			actor.Draw(shaderProgram, proj)
		}
	}
}
func drawUI(view View, proj hud.Mat4) {
	// Draw text
	if dialogue != nil {
		DrawSprite(Sprites[spriteDialogueOverlay], hud.NewMat4(), shaderProgram) // dialogue window
		DrawSprite(Sprites[spriteDialogueBar], hud.NewMat4(), shaderProgram)     // dialogue bar overlay
	}

	// draw all reply buttons
	for _, v := range reply {
		if !v.Active {
			continue
		}
		if v.start.IsAnimating() {
			v.start.Draw(proj, v.start.GetPosition(), shaderProgram)
		} else if v.end.IsAnimating() {
			v.end.Draw(proj, v.end.GetPosition(), shaderProgram)
		} else {
			DrawSprite(v.Sprite, hud.NewMat4(), shaderProgram)
		}
	}

	if AUTO {
		DrawSprite(Sprites[spriteAutoOn], hud.NewMat4(), shaderProgram)
	} else {
		DrawSprite(Sprites[spriteAutoOff], hud.NewMat4(), shaderProgram)
	}
	DrawSprite(Sprites[spriteMenuButton], hud.NewMat4(), shaderProgram)
	if DEBUG {
		Sprites[spriteEmoteBalloon].Draw(Sprites[spriteEmoteBalloon].GetTransform(proj), shaderProgram)
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
	// draw reply text
	for _, v := range reply {
		if v.Active {
			DrawText(view, v.Text, v.Position.X(), v.Position.Y())
		}
	}
	if DEBUG {
		var text []string
		text = append(text, fmt.Sprintf("FPS: %d", FPS))
		text = append(text, fmt.Sprintf("Volume: (%f)", CurrentBgmVolume))
		if sprite, ok := charSprite[CurrentSpeaker]; ok {
			p := sprite.GetPosition()
			emoteOffset := p.Sub(Sprites[spriteEmoteBalloon].GetPosition())
			text = append(text, fmt.Sprintf("position: (%f, %f)", p.X(), p.Y()))
			text = append(text, fmt.Sprintf("scale: (%f)", sprite.GetScale()))
			text = append(text, fmt.Sprintf("emote: (%f, %f)", emoteOffset.X(), emoteOffset.Y()))
		}
		pos := hud.NewSolidText(strings.Join(text, "\n"), hud.COLOR_WHITE, Fonts[fontRegular])
		pos.SetScale(2)
		DrawText(view, pos, 0, 0)
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

	// activeChar := replyEnd.GetSprite()
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
	fmt.Printf("cursor pos: (%f %f)\n", cursorX, cursorY)
	if action == glfw.Release {
		if len(reply) > 0 {
			if len(reply) == 1 {
				fmt.Println("checking single reply bounds:", rectReplySingle)
				if inside(rectReplySingle, int(cursorX), int(cursorY)) {
					fmt.Println("reply hit")
					if s, ok := Sounds["touch"]; ok {
						s.Play(CurrentSfxVolume)
					}

					reply[0].end.Animate(func() {
						fmt.Println("reply animation ended")
						delayConfirmation(&status, time.Second)
						reply[0].Active = false
					})
					return
				} else {
					fmt.Println("option button not hit:", cursorX, cursorY)
				}
			} else if len(reply) == 2 {
				fmt.Println("checking double reply bounds")
				if inside(rectReplyDoubleA, int(cursorX), int(cursorY)) {
					fmt.Println("reply a hit")
					if s, ok := Sounds["touch"]; ok {
						s.Play(CurrentSfxVolume)
					}
					reply[0].end.Animate(func() {
						delayConfirmation(&status, time.Second)
						reply[0].Active = false
					})
					return
				} else if inside(rectReplyDoubleB, int(cursorX), int(cursorY)) {
					fmt.Println("reply b hit")
					if s, ok := Sounds["touch"]; ok {
						s.Play(CurrentSfxVolume)
					}
					reply[1].end.Animate(func() {
						delayConfirmation(&status, time.Second)
						reply[1].Active = false
					})
					return
				}
			}
			return
		}

		autoRect := image.Rectangle{Min: image.Point{X: 1020, Y: 20}, Max: image.Point{X: 1130, Y: 60}}
		menuRect := image.Rectangle{Min: image.Point{X: 1150, Y: 20}, Max: image.Point{X: 1260, Y: 60}}
		if inside(autoRect, int(cursorX), int(cursorY)) {
			AUTO = !AUTO
		} else if inside(menuRect, int(cursorX), int(cursorY)) {
			fmt.Println("resetting scene")
			dialogueIndex = 0
			clear()
			loadGame(LANDSCAPE_VIEW, scriptName)
		} else {
			sendConfirmation(&status)
		}
	}
}

func queueEvent(event eventFunc) {
	EventQueue = append(EventQueue, event)
}
func nextDialogue(status *chan uint32) {
	if LOADING || WAITING_CONFIRMATION {
		fmt.Println("waiting confirmation:", WAITING_CONFIRMATION)
		return
	}

	// fmt.Println("starting dialogue goroutine")
	releaseReplies()

	dialogueIndex++
	if len(Script.Elements) <= dialogueIndex {
		// *status <- 0
		return
	}

	element := Script.Get(dialogueIndex)
	log.Printf("next line: %v\n", element.ToString())

	elementText := element.Line
	switch element.Name {
	case "delay":
		f, err := strconv.ParseFloat(element.Action, 64)
		if err != nil {
			f = 0.5
		}

		fmt.Println("delaying by:", f)
		// <-time.NewTimer(time.Duration(f) * time.Second).C
		// fmt.Println("delay ended")
		// nextDialogue(status)
		delayNextDialogue(status, time.Duration(f)*time.Second)
	case "defect":
		name := strings.ToLower(element.Mood)
		if _, ok := Factions[name]; ok {
			Factions[name] = hud.NewSolidText(elementText, mgl32.Vec3{0.49, 0.81, 1}, Fonts["bold"])
			Factions[name].SetScale(0.8)
		}
		nextDialogue(status)
	case "clear":
		clear()
		nextDialogue(status)
	case "bg":
		CurrentBG = element.Mood
		nextDialogue(status)
	case "sfx":
		if s, ok := Sounds[element.Mood]; ok {
			fmt.Println("playing sfx:", element.Mood)
			s.Play(CurrentSfxVolume)
			nextDialogue(status)
		}
	case "bgm":
		prepareBgm(element, status)
		nextDialogue(status)
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
	case "sensei":
		// 2E4152
		for i, v := range element.Lines {
			reply = append(reply, createReply(v, i, len(element.Lines)))
		}
		WAITING_CONFIRMATION = true
	case "none":
		releaseDialogue()
		nextDialogue(status)
	default:
		prepareActor(status, element)
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
		dialogue = hud.NewText(element.Lines, hud.COLOR_WHITE, Fonts[fontRegular])
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
	if element.Mood != "_" && element.Mood != "animation" {
		charSprite[element.Name] = Actors[element.Name]
	}

	// convert action into predefined parameters
	shouldChangeSprite, isAnimated := prepareActorAnimation(status, &element, Actors[element.Name], !element.HasDialogue())

	if shouldChangeSprite {
		err := charSprite[element.Name].SetActiveTexture(element.Mood)
		if err != nil {
			fmt.Println("error loading sprite: ", err.Error())
			DebugChannel <- fmt.Sprintf("actor (%s) is missing sprite (%s)", element.Name, element.Mood)
		}

		charSprite[element.Name].Silhouette = false
	}

	if !element.HasDialogue() && !isAnimated {
		nextDialogue(status)
	}
}

func delayNextDialogue(status *chan uint32, duration time.Duration) {
	go func() {
		// WAITING_CONFIRMATION = true
		<-time.NewTimer(duration).C
		*status <- 2 // send status update to listening channel
	}()
}

func sendConfirmation(status *chan uint32) {
	go func() {
		*status <- 3 // send status update to listening channel
	}()
}

func delayConfirmation(status *chan uint32, duration time.Duration) {
	go func() {
		<-time.NewTimer(duration).C
		*status <- 3 // send status update to listening channel
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

func prepareActorAnimation(status *chan uint32, element *script.ScriptElement, actor *Actor, autoNextDialogue bool) (bool, bool) {

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
		return false, true // return here because if it's an emote, we don't want to change the actor sprite
	case "full":
		actor.Faded = false
		actor.SetColorf(1, 1, 1)
		return false, false
	case "fade":
		fmt.Println("actor fade")
		AsyncActorColor(actor, status, actionName != "in", func() {
			delayNextDialogue(status, time.Millisecond)
		})
		return false, true
	case "defect":
		if _, ok := Factions[actor.name]; ok {
			if actionName == "_" {
				Factions[actor.name] = nil // remove faction
			} else {
				Factions[actor.name] = hud.NewSolidText(actionName, mgl32.Vec3{0.49, 0.81, 1}, Fonts["bold"])
				Factions[actor.name].SetScale(0.8)
			}
		}
		return false, false
	case "rename":
		Names[actor.name] = hud.NewSolidText(toTitle(actionName), hud.COLOR_WHITE, Fonts["bold"])
		Names[actor.name].SetScale(float32(speakerScale))
		return false, false
	default: // if the action isn't a special case like emotes, then it's probably a sprite animation
		// move the actor if the action set the position
		if anim, ok := ActorAnimations[actionName]; ok {
			// fmt.Println("starting animation:", anim.Name)
			WAITING_CONFIRMATION = anim.Speed < 1
			go AsyncAnimateActor(actor, anim, status, func() {
				// if anim.Speed == 1 {
				// nextDialogue(status)
				// } else {
				// sendConfirmation(status)
				// }
				fmt.Println("============= ANIMATION ENDED")
				WAITING_CONFIRMATION = false
				if anim.Speed >= 1 {
					sendConfirmation(status)
				}
			})
		}
		if action == "animation" || action == "_" {
			return false, true
		}
		if action == "silhouette" {
			actor.Silhouette = true
			actor.SetColorf(0, 0, 0)
			return false, false
		}
	}

	return true, false
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
	case "pause":
		if currentBGM != nil {
			currentBGM.Pause()
		}
	case "fade":
		if currentBGM != nil {
			AsyncStreamerVolume(currentBGM, bgmActionName == "in")
		}
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

func AsyncActorColor(s *Actor, status *chan uint32, toBlack bool, done func()) {

	s.Faded = true
	var fadeFunc func(f float64) int
	if toBlack {
		s.SetColorf(1, 1, 1)
		fadeFunc = func(f float64) int {
			c := s.GetColor()
			v := c.X() - float32(0.015)
			if v <= 0 {
				s.SetColorf(0, 0, 0)
				return -1
			}
			s.SetColorf(v, v, v)
			fmt.Println("fading actor:", v)
			return 0
		}
	} else {
		s.SetColorf(0, 0, 0)
		fadeFunc = func(f float64) int {
			c := s.GetColor()
			v := c.X() + float32(0.015)
			if v >= 1 {
				s.SetColorf(1, 1, 1)
				s.Faded = false
				return -1
			}
			s.SetColorf(v, v, v)
			fmt.Println("fading actor:", v)
			return 0
		}
	}

	loop := loop.New(FRAME_DURATION, fadeFunc, func() {
		log.Println("finished actor fade")
		if done != nil {
			done()
		}
	})
	loop.Start()
}

func AsyncStreamerVolume(s *sfx.Streamer, in bool) {
	var fadeFunc func(f float64) int
	if in {
		fmt.Println("fading bgm in")
		s.SetVolume(MinBgmVolume)
		fadeFunc = func(f float64) int {
			volume := s.GetVolume() + 0.5
			if volume > CurrentBgmVolume {
				s.SetVolume(CurrentBgmVolume)
				return -1
			}
			s.SetVolume(volume)
			return 0
		}
	} else {
		s.SetVolume(CurrentBgmVolume)
		fadeFunc = func(f float64) int {
			volume := s.GetVolume() - 0.5
			if volume < MinBgmVolume {
				s.SetVolume(MinBgmVolume)
				return -1
			}
			s.SetVolume(volume)
			return 0
		}
	}

	loop := loop.New(FRAME_DURATION, fadeFunc, func() {
		log.Println("finished bgm fade")
		// if done != nil {
		// 	done()
		// }
	})
	loop.Start()
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

func frameToTargetPosition(frame script.FrameMetadata, center, startingPosition, originalPosition hud.Vec3, originalScale float32) (hud.Vec3, float32) {
	var targetPosition hud.Vec3
	targetScale := originalScale
	if frame.Reset {
		targetPosition = originalPosition
	} else if frame.Center {
		targetPosition = center
		targetScale = center.Z()
	} else {
		targetPosition = startingPosition
		if frame.Scale != nil {
			targetScale = *frame.Scale
		}
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

	return targetPosition, targetScale
}

func AsyncAnimateActor(s *Actor, anim script.AnimationMetadata, status *chan uint32, done func()) {
	originalPosition := s.GetPosition()
	originalScale := s.GetScale()
	// fmt.Println("original position:", originalPosition)
	speed := anim.Speed

	if speed == 1 {
		targetPosition, targetScale := frameToTargetPosition(anim.Frames[0], s.GetCenter(), originalPosition, originalPosition, originalScale)
		s.SetPosition(targetPosition)
		s.SetScale(targetScale)
		if done != nil {
			done()
		}
		return
	}

	ticker := time.Tick(FRAME_DURATION)
	for _, frame := range anim.Frames {
		startingPosition := s.GetPosition()
		targetPosition, targetScale := frameToTargetPosition(frame, s.GetCenter(), startingPosition, originalPosition, originalScale)

		// fmt.Println("animation frame:", ind, "playing animation:", anim.Name, ", position:", s.GetPosition(), "to:", targetPosition)

		// this will move the actor to a target location
		// this check uses the length of the vector to account for floating point precision issues

		s.SetScale(targetScale)

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
			}
		}

		if frame.Delay != nil {
			// fmt.Println("delaying frame by:", *frame.Delay)
			time.Sleep(time.Second * time.Duration(*frame.Delay))
		}
	}

	// loop := loop.New(FRAME_DURATION, fadeFunc, func() {
	// 	log.Println("finished actor fade")
	if done != nil {
		done()
	}
	// })
	// loop.Start()
}

func AsyncAnimateReply(s *hud.Sprite, status *chan uint32, done func()) {
	// var originalScale float32 = 1.0
	var targetScale float32 = 1.25
	var speed float32 = 0.01

	ticker := time.Tick(FRAME_DURATION)
	for s.GetScale() < targetScale {
		select {
		case <-ticker:
			fmt.Println("reply scale:", s.GetScale())
			s.SetScale(s.GetScale() + speed)
		}
	}

	if done != nil {
		done()
	}
}

/////////////////////////////////////////////
// HELPER FUNCTIONS

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
	reply = make([]*Reply, 0)
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

func LoadImageFromFile(imgPath string) image.Image {
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		panic(fmt.Errorf("file does not exist, aborting script: %v", imgPath))
	}
	imageFile, err := os.Open(imgPath)
	if err != nil {
		panic(err)
	}
	defer imageFile.Close()
	img, _, err := image.Decode(imageFile)
	if err != nil {
		panic(err)
	}
	return img
}

func SaveImage(rgba image.Image, filename string) error {
	// Ensure the directories exist before writing the file
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}

	out, err := os.Create(filename)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	png.Encode(out, rgba)
	return nil
}

func createReply(text string, index, total int) *Reply {

	var textPosition hud.Vec2
	var yOffset float32
	var sprite *hud.Sprite

	txtObj := hud.NewSolidText(text, mgl32.Vec3{0.18, 0.255, 0.322}, Fonts[fontRegular])
	switch total {
	case 1:
		sprite = Sprites[spriteReplySingle]
		yOffset = 0.15
		textPosition = hud.Vec2{(float32(1280/2) - (txtObj.Width() / 2) + 25), 285}
	case 2:
		if index == 0 {
			sprite = Sprites[spriteReplyDoubleA]
			yOffset = 0.27
			textPosition = hud.Vec2{(float32(1280/2) - (txtObj.Width() / 2) + 25), 240}
		} else {
			sprite = Sprites[spriteReplyDoubleB]
			yOffset = 0.03
			textPosition = hud.Vec2{(float32(1280/2) - (txtObj.Width() / 2) + 25), 330}
		}
	}

	startAnim := hud.NewAnimation("reply_start", hud.NewAnimatedSpriteFromFile("./resources/ui/reply_start_v3.gif"))
	startAnim.SetPositionf(0, yOffset, 0)
	startAnim.Animate(func() {})

	endAnim := hud.NewAnimation("reply_end", hud.NewAnimatedSpriteFromFile("./resources/ui/reply_end_v3.gif"))
	endAnim.SetPositionf(0, yOffset, 0)

	return &Reply{
		Position: textPosition,
		Active:   true,
		Sprite:   sprite,
		Text:     txtObj,
		start:    startAnim,
		end:      endAnim,
	}
}

func FileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

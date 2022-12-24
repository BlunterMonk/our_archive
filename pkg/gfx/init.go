package gfx

import (
	"fmt"
	"log"
	"os"

	"github.com/4ydx/gltext"
	v41 "github.com/4ydx/gltext/v4.1"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"golang.org/x/image/math/fixed"
)

const (
	WINDOW_WIDTH  = 1280
	WINDOW_HEIGHT = 720
)

func Init() *glfw.Window {
	window := initGlfw()
	initGL()

	return window
}
func Shutdown() {
	glfw.Terminate()
}

func initGL() uint32 {
	if err := gl.Init(); err != nil {
		panic(err)
	}
	version := gl.GoStr(gl.GetString(gl.VERSION))
	log.Println("OpenGL version", version)

	prog := gl.CreateProgram()
	gl.LinkProgram(prog)
	return prog
}

func initGlfw() *glfw.Window {
	if err := glfw.Init(); err != nil {
		panic(err)
	}

	// glfw.WindowHint(glfw.Decorated, glfw.False)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4) // OR 2
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(WINDOW_WIDTH, WINDOW_HEIGHT, "VN Test", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()
	// window.SetKeyCallback(keyCallback)
	// window.SetMouseButtonCallback(mouseButtonCallback)
	// window.SetPos(0, 0)
	return window
}

func MustInitShader() *Program {

	// the linked shader program determines how the data will be rendered
	vertShader, err := NewShaderFromFile("shaders/basic.vert", gl.VERTEX_SHADER)
	if err != nil {
		panic(err)
	}

	fragShader, err := NewShaderFromFile("shaders/basic.frag", gl.FRAGMENT_SHADER)
	if err != nil {
		panic(err)
	}

	shaderProgram, err := NewProgram(vertShader, fragShader)
	if err != nil {
		panic(err)
	}

	return shaderProgram
}

func MustLoadFont(fontName string) *v41.Font {
	var font *v41.Font
	var config *gltext.FontConfig

	config, err := gltext.LoadTruetypeFontConfig("fontconfigs", fontName)
	if err == nil {
		font, err = v41.NewFont(config)
		if err != nil {
			panic(err)
		}
		fmt.Println("Font loaded from disk...")
	} else {
		fd, err := os.Open(fmt.Sprintf("font/%v.ttf", fontName))
		if err != nil {
			panic(err)
		}
		defer fd.Close()

		// Japanese character ranges
		// http://www.rikai.com/library/kanjitables/kanji_codes.unicode.shtml
		runeRanges := make(gltext.RuneRanges, 0)
		runeRanges = append(runeRanges, gltext.RuneRange{Low: 32, High: 128})
		runeRanges = append(runeRanges, gltext.RuneRange{Low: 0x3000, High: 0x3030})
		runeRanges = append(runeRanges, gltext.RuneRange{Low: 0x3040, High: 0x309f})
		runeRanges = append(runeRanges, gltext.RuneRange{Low: 0x30a0, High: 0x30ff})
		runeRanges = append(runeRanges, gltext.RuneRange{Low: 0x4e00, High: 0x9faf})
		runeRanges = append(runeRanges, gltext.RuneRange{Low: 0xff00, High: 0xffef})

		scale := fixed.Int26_6(32)
		runesPerRow := fixed.Int26_6(128)
		config, err = gltext.NewTruetypeFontConfig(fd, scale, runeRanges, runesPerRow, 5)
		if err != nil {
			panic(err)
		}
		err = config.Save("fontconfigs", fontName)
		if err != nil {
			panic(err)
		}
		font, err = v41.NewFont(config)
		if err != nil {
			panic(err)
		}
	}

	return font
}

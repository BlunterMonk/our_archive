package hud

import (
	"fmt"
	"image/gif"
	"time"

	"github.com/BlunterMonk/opengl/pkg/gfx"
	"github.com/go-gl/gl/v4.1-core/gl"
)

// an animation is a type of sprite build from a gif
// the sprite will hold all frames of the gif as textures keyed by index
type AnimatedSprite struct {
	*Sprite       // the sprite holding all frames.
	delay   []int // The successive delay times, one per frame, in 100ths of a second.
	// LoopCount controls the number of times an animation will be
	// restarted during display.
	// A LoopCount of 0 means to loop forever.
	// A LoopCount of -1 means to show each frame only once.
	// Otherwise, the animation is looped LoopCount+1 times.
	loopCount int
	// Disposal is the successive disposal methods, one per frame. For
	// backwards compatibility, a nil Disposal is valid to pass to EncodeAll,
	// and implies that each frame's disposal method is 0 (no disposal
	// specified).
	disposal []byte
}

func NewAnimatedSpriteFromFile(filename string) *AnimatedSprite {
	if SpriteVAO == 0 {
		SpriteVAO = gfx.CreateVAO(squareVerts, squareInds)
	}

	templateGif := loadGif(filename)
	textures := make(map[string]*gfx.Texture, 0)
	for ind, img := range templateGif.Image {
		t, err := gfx.NewTexture(img, gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
		if err != nil {
			panic(err.Error())
		}

		textures[fmt.Sprint(ind)] = t
	}

	fmt.Println("created animation with textures:", len(textures))
	fmt.Println(textures)
	fmt.Println("----------------------------")

	return &AnimatedSprite{
		Sprite: &Sprite{
			vbo:           SpriteVAO,
			activeTexture: "0",
			textures:      textures,
			ULOC:          "ourTexture0",
			overlayColor:  Vec3{1, 1, 1},
			alpha:         1,
			scale:         1,
		},
		loopCount: templateGif.LoopCount,
		delay:     append(make([]int, 0), templateGif.Delay...),
		disposal:  append(make([]byte, 0), templateGif.Disposal...),
	}
}

func (a *AnimatedSprite) GetFirstDelay() time.Duration {
	return (time.Duration(a.delay[0]) * time.Millisecond * 10)
}

func (a *AnimatedSprite) GetLoopCount() int {
	return a.loopCount
}

func (a *AnimatedSprite) GetNextFrame(currentFrame int) (int, time.Duration) {
	index := currentFrame + 1
	if index >= len(a.textures) {
		index = 0
	}

	nextDelay := a.delay[index]
	return index, (time.Duration(nextDelay) * time.Millisecond * 10)
}

func (a *AnimatedSprite) DrawFrame(frame int, position Vec3, shader *gfx.Program) {
	transform := CalculateTransform(a.Width(), a.Height(), a.scale, position.ToV3())

	// draw any previous frames if the disposal is none
	for i := 0; i < frame; i++ {
		texture := a.textures[fmt.Sprint(i)]
		disposal := a.disposal[i]
		// delay := a.delay[i]

		if disposal == gif.DisposalNone {
			a.drawTexture(transform, shader, texture)
		}

		// fmt.Println("disposal method:", disposal)
	}

	// draw the current texture
	current := a.textures[fmt.Sprint(frame)]
	a.drawTexture(transform, shader, current)
}

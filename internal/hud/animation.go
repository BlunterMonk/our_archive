package hud

import (
	"github.com/BlunterMonk/our_archive/internal/loop"
	"github.com/BlunterMonk/our_archive/pkg/gfx"
)

type Animation struct {
	name         string
	data         *AnimatedSprite
	isAnimating  bool
	currentFrame int
	loopCount    int
	looper       *loop.GameLoop
}

func NewAnimation(name string, gif *AnimatedSprite) *Animation {
	return &Animation{
		name:      name,
		data:      gif,
		loopCount: 1,
	}
}

func (a *Animation) Animate(onFinish func()) {
	if a.isAnimating {
		return
	}

	a.isAnimating = true
	remainingLoops := a.loopCount
	// log.Println("starting animation with loops:", remainingLoops)
	a.looper = loop.New(a.data.GetFirstDelay(), func(f float64) int {
		frame, delay := a.data.GetNextFrame(a.currentFrame)
		a.currentFrame = frame
		// log.Printf("animation frame: %d, loop: %d", a.currentFrame, remainingLoops)
		if frame == 0 {
			remainingLoops--
			if remainingLoops == 0 {
				return -1
			}
		}
		return int(delay)
	}, func() {
		// log.Println("animation ended")
		a.isAnimating = false
		onFinish()
	})
	a.looper.Start()
}

func (a *Animation) AnimateForever() {
	if a.isAnimating {
		return
	}

	a.isAnimating = true
	a.looper = loop.New(a.data.GetFirstDelay(), func(f float64) int {
		frame, delay := a.data.GetNextFrame(a.currentFrame)
		a.currentFrame = frame
		return int(delay)
	}, func() {})
	a.looper.Start()
}

func (a *Animation) Stop() {
	if a.looper != nil {
		a.isAnimating = false
		a.looper.Stop()
	}
}

func (a *Animation) IsAnimating() bool {
	return a.isAnimating
}

func (a *Animation) Draw(position Vec3, shader *gfx.Program) {
	a.data.DrawFrame(a.currentFrame, position, shader)
}

func (a *Animation) GetName() string {
	return a.name
}

func (a *Animation) SetScale(n float32) {
	a.data.SetScale(n)
}
func (a *Animation) GetScale() float32 {
	return a.data.GetScale()
}

func (a *Animation) SetPositionf(x, y, z float32) {
	a.data.SetPositionf(x, y, z)
}
func (a *Animation) SetPosition(p Vec3) {
	a.data.SetPosition(p)
}
func (a *Animation) GetPosition() Vec3 {
	return a.data.GetPosition()
}

func (a *Animation) GetSprite() *Sprite {
	return a.data.Sprite
}

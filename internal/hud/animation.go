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
	loopTL := loop.New(a.data.GetFirstDelay(), func(f float64) int {
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
	loopTL.Start()
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

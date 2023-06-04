package main

import (
	"fmt"

	"github.com/BlunterMonk/opengl/internal/hud"
	"github.com/BlunterMonk/opengl/pkg/gfx"
)

type Actor struct {
	*hud.Sprite

	name               string
	emoteBalloonOffset hud.Vec3
	emoteAnimation     *hud.Animation
}
type animation struct {
	start     *hud.Vec3
	end       *hud.Vec3
	speed     float32
	emoteName string
}

func NewActor(name string) *Actor {
	return &Actor{
		name:   name,
		Sprite: hud.NewSpriteFromFile(fmt.Sprintf("./resources/actor/%s/%s-00.png", name, name)),
	}
}

func (a *Actor) AnimateEmote(name string) {
	if a.emoteAnimation != nil || (a.emoteAnimation != nil && a.emoteAnimation.IsAnimating()) {
		return
	}

	var ok bool
	var emoteData *hud.AnimatedSprite
	if emoteData, ok = Emotes[name]; !ok {
		fmt.Println("trying to animate with no active animation")
		return
	}

	a.emoteAnimation = hud.NewAnimation(emoteData)
	a.emoteAnimation.Animate(func() {
		a.emoteAnimation = nil
		// log.Println("emote finished for:", a.name)
	})
}

func (a *Actor) Draw(shader *gfx.Program) {
	a.Sprite.Draw(a.GetTransform(), shader)
	// draw the emote if it's active
	a.DrawEmoteIfActive(shaderProgram)
}

func (a *Actor) DrawEmoteIfActive(shader *gfx.Program) {
	if a.emoteAnimation == nil || !a.emoteAnimation.IsAnimating() {
		return
	}
	a.emoteAnimation.Draw(a.GetPosition().Sub(a.emoteBalloonOffset), shader)
}

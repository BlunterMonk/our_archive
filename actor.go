package main

import (
	"fmt"

	"github.com/BlunterMonk/opengl/internal/hud"
	"github.com/BlunterMonk/opengl/pkg/gfx"
)

type Actor struct {
	*hud.Sprite

	name           string
	FactionName    string
	emoteOffsets   map[string]hud.Vec3
	emoteAnimation *hud.Animation
}
type animation struct {
	start     *hud.Vec3
	end       *hud.Vec3
	speed     float32
	emoteName string
}

func NewActor(name string) *Actor {
	return &Actor{
		name:         name,
		Sprite:       hud.NewSpriteFromFile(fmt.Sprintf("./resources/actor/%s/%s-00.png", name, name)),
		emoteOffsets: make(map[string]hud.Vec3),
	}
}

func (a *Actor) AddEmoteData(name string, offset hud.Vec3) {
	fmt.Println("adding emote data for:", name, offset)
	a.emoteOffsets[name] = offset
}

func (a *Actor) AnimateEmote(name string, emoteData *hud.AnimatedSprite) {
	if a.emoteAnimation != nil || (a.emoteAnimation != nil && a.emoteAnimation.IsAnimating()) {
		return
	}

	a.emoteAnimation = hud.NewAnimation(name, emoteData)
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

	fmt.Println(a.emoteAnimation.GetName())
	fmt.Println(a.emoteOffsets)
	// decide offset based on emote type
	offset := a.emoteOffsets[a.emoteAnimation.GetName()]
	a.emoteAnimation.Draw(a.GetPosition().Sub(offset), shader)
}

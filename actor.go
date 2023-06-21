package main

import (
	"fmt"

	"github.com/BlunterMonk/our_archive/internal/hud"
	"github.com/BlunterMonk/our_archive/pkg/gfx"
)

type Actor struct {
	*hud.Sprite

	name           string
	FactionName    string
	emoteOffsets   map[string]hud.Vec3
	emoteAnimation *hud.Animation
	centerPosition hud.Vec3
	centerScale    float32
}
type animation struct {
	start     *hud.Vec3
	end       *hud.Vec3
	speed     float32
	emoteName string
}

func NewActor(name string) (*Actor, error) {
	sprite, err := hud.NewSpriteFromFile(fmt.Sprintf("./resources/actor/%s/%s-00.png", name, name))
	return &Actor{
		name:         name,
		Sprite:       sprite,
		emoteOffsets: make(map[string]hud.Vec3),
	}, err
}

func (a *Actor) AddEmoteData(name string, offset hud.Vec3) {
	// fmt.Println("adding emote data for:", name, offset)
	a.emoteOffsets[name] = offset
}

func (a *Actor) AnimateEmote(name string, emoteData *hud.AnimatedSprite, callback func()) {
	if a.emoteAnimation != nil || (a.emoteAnimation != nil && a.emoteAnimation.IsAnimating()) {
		return
	}

	a.emoteAnimation = hud.NewAnimation(name, emoteData)
	a.emoteAnimation.Animate(func() {
		a.emoteAnimation = nil
		if callback != nil {
			callback()
		}
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

	// fmt.Println(a.emoteAnimation.GetName())
	// fmt.Println(a.emoteOffsets)
	// decide offset based on emote type
	offset := a.emoteOffsets[a.emoteAnimation.GetName()]
	a.emoteAnimation.Draw(a.GetPosition().Sub(offset), shader)
}

func (s *Actor) SetCenter(x, y, scale float32) {
	s.centerPosition = hud.Vec3{x, y, 0}
	s.centerScale = scale
}
func (a *Actor) GetCenter() hud.Vec3 {
	return a.centerPosition
}

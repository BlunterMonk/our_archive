package main

import (
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

	Faded      bool
	Silhouette bool
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
		Sprite:       hud.NewSprite(),
		emoteOffsets: make(map[string]hud.Vec3),
	}
}

func (a *Actor) GetTextures() map[string]*gfx.Texture {
	return a.Sprite.GetTextures()
}

func (a *Actor) AddEmoteData(name string, offset hud.Vec3) {
	// fmt.Println("adding emote data for:", name, offset)
	a.emoteOffsets[name] = offset
}

func (a *Actor) AddSpriteData(mood string, texture *gfx.Texture) {
	a.Sprite.AddTexture(mood, texture)
}

func (a *Actor) SetTexture(key string, texture *gfx.Texture) error {
	a.Sprite.AddTexture(key, texture)
	return a.Sprite.SetActiveTexture(key)
}

func (a *Actor) SetActiveTexture(key string) error {
	return a.Sprite.SetActiveTexture(key)
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

func (a *Actor) Draw(shader *gfx.Program, proj hud.Mat4) {
	a.Sprite.Draw(a.GetTransform(proj), shader)
	// draw the emote if it's active
	a.DrawEmoteIfActive(shaderProgram, proj)
}

func (a *Actor) DrawEmoteIfActive(shader *gfx.Program, proj hud.Mat4) {
	if a.emoteAnimation == nil || !a.emoteAnimation.IsAnimating() {
		return
	}

	// fmt.Println(a.emoteAnimation.GetName())
	// fmt.Println(a.emoteOffsets)
	// decide offset based on emote type
	offset := a.emoteOffsets[a.emoteAnimation.GetName()]
	a.emoteAnimation.Draw(proj, a.GetPosition().Sub(offset), shader)
}

func (s *Actor) SetCenter(x, y, scale float32) {
	s.centerPosition = hud.Vec3{x, y, scale}
	s.centerScale = scale
}
func (a *Actor) GetCenter() hud.Vec3 {
	return a.centerPosition
}

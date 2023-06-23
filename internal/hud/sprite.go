package hud

import (
	"fmt"
	"unsafe"

	"github.com/BlunterMonk/our_archive/pkg/gfx"
	"github.com/go-gl/gl/v4.1-core/gl"
)

var (
	SpriteVAO   = uint32(0)
	squareVerts = []float32{
		// top left
		-1, 1, 0.0, // position
		1.0, 0.0, 0.0, // Color
		1.0, 0.0, // texture coordinates
		// top right
		1, 1, 0.0,
		0.0, 1.0, 0.0,
		0.0, 0.0,
		// bottom right
		1, -1, 0.0,
		0.0, 0.0, 1.0,
		0.0, 1.0,
		// bottom left
		-1, -1, 0.0,
		1.0, 1.0, 1.0,
		1.0, 1.0,
	}
	squareInds = []uint32{
		// rectangle
		0, 1, 2, // top triangle
		0, 2, 3, // bottom triangle
	}
)

type Sprite struct {
	vbo           uint32 // pointer to vertex buffer
	activeTexture string // texture object
	textures      map[string]*gfx.Texture
	ULOC          string
	alpha         float32
	scale         float32
	overlayColor  Vec3
	position      Vec3
}

func NewSprite() *Sprite {
	return &Sprite{
		vbo:          SpriteVAO,
		textures:     make(map[string]*gfx.Texture, 0),
		ULOC:         "ourTexture0",
		overlayColor: Vec3{1, 1, 1},
		alpha:        1,
		scale:        1,
	}
}

func NewSpriteFromFile(filename string) (*Sprite, error) {

	t, err := gfx.NewTextureFromFile(filename, gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	if err != nil {
		fmt.Println(err.Error())
		t, _ = gfx.NewTextureFromFile("./resources/ui/missing.png", gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	}

	textures := make(map[string]*gfx.Texture, 0)
	textures["default"] = t
	return &Sprite{
		vbo:           SpriteVAO,
		activeTexture: "default",
		textures:      textures,
		ULOC:          "ourTexture0",
		overlayColor:  Vec3{1, 1, 1},
		alpha:         1,
		scale:         1,
	}, err
}

func (s *Sprite) AddTexture(key string, texture *gfx.Texture) {
	s.textures[key] = texture
}

func (s *Sprite) LoadTexture(key, filename string) error {

	if _, ok := s.textures[key]; ok {
		// avoid loading textures that already exist
		return nil
	}

	t, err := gfx.NewTextureFromFile(filename, gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	if err != nil {
		fmt.Println(err.Error())
		t, _ = gfx.NewTextureFromFile("./resources/ui/missing.png", gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	}

	if s.activeTexture == "" {
		s.activeTexture = key
	}

	s.textures[key] = t
	return err
}

func (s *Sprite) getActiveTexture() *gfx.Texture {
	return s.textures[s.activeTexture]
}

func (s *Sprite) Width() float32 {
	return float32(s.getActiveTexture().Width())
}

func (s *Sprite) Height() float32 {
	return float32(s.getActiveTexture().Height())
}

func (s *Sprite) SetColorf(r, g, b float32) {
	s.overlayColor = Vec3{r, g, b}
}
func (s *Sprite) GetColor() Vec3 {
	return s.overlayColor
}

func (s *Sprite) GetAlpha() float32 {
	return s.alpha
}

func (s *Sprite) SetAlpha(a float32) {
	s.alpha = a
}

func (s *Sprite) SetScale(n float32) {
	s.scale = n
}
func (s *Sprite) GetScale() float32 {
	return s.scale
}

func (s *Sprite) SetPositionf(x, y, z float32) {
	s.position = Vec3{x, y, z}
}

func (s *Sprite) SetPosition(p Vec3) {
	s.position = p
}

func (s *Sprite) GetPosition() Vec3 {
	return s.position
}

func (s *Sprite) SetActiveTexture(key string) error {
	if _, ok := s.textures[key]; !ok {
		return fmt.Errorf("texture doesn't exist: %s", key)
	}

	s.activeTexture = key
	return nil
}

func (s *Sprite) Draw(m Mat4, shader *gfx.Program) {
	s.drawTexture(m, shader, s.getActiveTexture())
}

func (s *Sprite) DrawTexture(m Mat4, shader *gfx.Program, key string) {
	s.drawTexture(m, shader, s.textures[key])
}

func (s *Sprite) drawTexture(m Mat4, shader *gfx.Program, texture *gfx.Texture) {
	if shader == nil || texture == nil {
		panic("cannot draw nil objects")
	}

	vbo := s.vbo
	ul := s.ULOC

	// set texture0 to uniform0 in the fragment shader
	texture.Bind(gl.TEXTURE0)
	texture.SetUniform(shader.GetUniformLocation(ul))

	// mat := []float32{
	// 	1, 0, 0, 0,
	// 	0, 1, 0, 0,
	// 	0, 0, 1, 0,
	// 	0, 0, 0, 1,
	// }
	mat := m.Slice()
	gl.UniformMatrix4fv(shader.GetUniformLocation("prjMatrix"), 1, false, &mat[0])
	gl.Uniform4f(shader.GetUniformLocation("overlayColor"), s.overlayColor.X(), s.overlayColor.Y(), s.overlayColor.Z(), s.alpha)

	// draw vertices
	gl.BindVertexArray(vbo)
	gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, unsafe.Pointer(nil))
	gl.BindVertexArray(0)

	texture.UnBind()
}

func (s *Sprite) GetTransform() Mat4 {
	return CalculateTransform(s.Width(), s.Height(), s.scale, s.position.ToV3())
}

func (s *Sprite) Translate(x, y, z float32) {
	s.position = Vec3{s.position.X() + x, s.position.Y() + y, s.position.Z() + z}
}

package hud

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/BlunterMonk/opengl/pkg/gfx"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/ungerik/go3d/mat4"
	v2 "github.com/ungerik/go3d/vec2"
	v3 "github.com/ungerik/go3d/vec3"
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

type vec2 v2.T

func (v *vec2) X() float32 { return v[0] }
func (v *vec2) Y() float32 { return v[1] }

type vec3 v3.T

func (v *vec3) X() float32 { return v[0] }
func (v *vec3) Y() float32 { return v[1] }
func (v *vec3) Z() float32 { return v[2] }

type Sprite struct {
	vbo          uint32       // pointer to vertex buffer
	texture      *gfx.Texture // texture object
	textures     map[string]*gfx.Texture
	ULOC         string
	alpha        float32
	scale        float32
	overlayColor vec3
	position     vec3
}

func NewSprite() *Sprite {
	return &Sprite{
		vbo:          SpriteVAO,
		textures:     map[string]*gfx.Texture{},
		ULOC:         "ourTexture0",
		overlayColor: vec3{1, 1, 1},
		alpha:        1,
		scale:        1,
	}
}

func NewSpriteFromFile(filename string) *Sprite {

	if SpriteVAO == 0 {
		SpriteVAO = gfx.CreateVAO(squareVerts, squareInds)
	}

	t, err := gfx.NewTextureFromFile(filename, gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	if err != nil {
		panic(err.Error())
	}

	return &Sprite{
		vbo:          SpriteVAO,
		texture:      t,
		textures:     map[string]*gfx.Texture{},
		ULOC:         "ourTexture0",
		overlayColor: vec3{1, 1, 1},
		alpha:        1,
		scale:        1,
	}
}

func (s *Sprite) LoadTexture(key, filename string) {

	if _, ok := s.textures[key]; ok {
		// avoid loading textures that already exist
		return
	}

	if SpriteVAO == 0 {
		SpriteVAO = gfx.CreateVAO(squareVerts, squareInds)
	}

	t, err := gfx.NewTextureFromFile(filename, gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	if err != nil {
		panic(err.Error())
	}

	s.textures[key] = t
}

func (s *Sprite) Width() float32 {
	return float32(s.texture.Width())
}

func (s *Sprite) Height() float32 {
	return float32(s.texture.Height())
}

func (s *Sprite) SetColorf(r, g, b float32) {
	s.overlayColor = vec3{r, g, b}
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

func (s *Sprite) SetPositionf(x, y, z float32) {
	s.position = vec3{x, y, z}
}

func (s *Sprite) GetPosition() mgl32.Vec3 {
	return mgl32.Vec3{s.position.X(), s.position.Y(), s.position.Z()}
}

func (s *Sprite) SetActiveTexture(key string) error {
	if _, ok := s.textures[key]; !ok {
		return fmt.Errorf("texture doesn't exist: %s", key)
	}

	s.texture = s.textures[key]
	return nil
}

func (s *Sprite) Draw(m mat4.T, shader *gfx.Program) {
	s.drawTexture(m, shader, s.texture)
}

func (s *Sprite) DrawTexture(m mat4.T, shader *gfx.Program, key string) {
	s.drawTexture(m, shader, s.textures[key])
}

func (s *Sprite) drawTexture(m mat4.T, shader *gfx.Program, texture *gfx.Texture) {
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

func (s *Sprite) GetTransform() mat4.T {
	// convert parameters into a transform
	out := mat4.From(&mat4.Ident)

	// get dimensions
	w := float64(s.Width())
	h := float64(s.Height())
	// determine longer side
	w = math.Min(w, h) / math.Max(w, h)

	// identity matrix to build other matrices from
	m := mat4.From(&mat4.Ident)

	// get 2D projection matrix for the aspect ratio
	prj := mat4.From(&mat4.Ident)
	prj = *prj.ScaleVec3(&v3.T{1, float32(16) / float32(9), 1})

	// scale
	sc := mat4.From(&mat4.Ident)
	sc = *sc.ScaleVec3(&v3.T{float32(w) * s.scale, s.scale, s.scale})

	// apply scale
	out = *m.AssignMul(&out, &sc)
	// apply projection to 2D
	out = *m.AssignMul(&out, &prj)
	// screen space translations need to be normalized
	out = *m.Translate(&v3.T{s.position.X(), s.position.Y(), s.position.Z()})
	// open gl is column major, with translation in the right-most values,
	// however this math library seems to be row majow, doing a transpose here puts the values in the correct order
	out = *out.Transpose()

	return out
}

func (s *Sprite) Translate(x, y, z float32) {
	s.position = vec3{s.position.X() + x, s.position.Y() + y, s.position.Z() + z}
}

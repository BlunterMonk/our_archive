package hud

import (
	"fmt"
	"image/gif"
	"log"
	"math"
	"os"

	"github.com/BlunterMonk/our_archive/pkg/gfx"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/ungerik/go3d/mat4"
	v2 "github.com/ungerik/go3d/vec2"
	v3 "github.com/ungerik/go3d/vec3"
)

type Vec2 v2.T

func (v *Vec2) X() float32 { return v[0] }
func (v *Vec2) Y() float32 { return v[1] }

type Vec3 v3.T

func (v *Vec3) X() float32 { return v[0] }
func (v *Vec3) Y() float32 { return v[1] }
func (v *Vec3) Z() float32 { return v[2] }
func (v *Vec3) ToV3() v3.T { return v3.T{v.X(), v.Y(), v.Z()} }
func (v Vec3) Add(p Vec3) Vec3 {
	return Vec3(v3.Add((*v3.T)(&v), (*v3.T)(&p)))
}
func (v Vec3) Sub(p Vec3) Vec3 {
	return Vec3(v3.Sub((*v3.T)(&v), (*v3.T)(&p)))
}
func (v Vec3) Mul(c float32) Vec3 {
	return Vec3{v[0] * c, v[1] * c, v[2] * c}
}
func (v Vec3) Normalize() Vec3 {
	l := 1.0 / v.Len()
	return Vec3{v[0] * l, v[1] * l, v[2] * l}
}
func (v Vec3) Len() float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

type Mat4 mat4.T

func (mat *Mat4) T() *mat4.T {
	return (*mat4.T)(mat)
}
func (mat *Mat4) Print() {
	fmt.Printf("[%v\n%v\n%v\n%v]\n", mat[0], mat[1], mat[2], mat[3])
}
func (mat *Mat4) Slice() []float32 {
	return (*mat4.T)(mat).Slice()
}
func (mat *Mat4) SetTranslation(position Vec3) *Mat4 {
	m := (*mat4.T)(mat).SetTranslation((*v3.T)(&position))
	return (*Mat4)(m)
}
func (mat *Mat4) Translate(position Vec3) *Mat4 {
	m := (*mat4.T)(mat).Translate((*v3.T)(&position))
	return (*Mat4)(m)
}
func (mat *Mat4) Transpose() *Mat4 {
	m := (*mat4.T)(mat).Transpose()
	return (*Mat4)(m)
}

func ProjMatrix(windowWidth, windowHeight float32) Mat4 {
	prj := mat4.From(&mat4.Ident)
	prj = *prj.ScaleVec3(&v3.T{1, windowWidth / windowHeight, 1})
	return (Mat4)(prj)
}

// func (mat *Mat4) Scale(scale float32) *Mat4 {
// 	// scale
// 	sc := mat4.From(&mat4.Ident)
// 	sc = *sc.ScaleVec3(&v3.T{float32(w) * scale, scale, scale})
// 	return *mat.AssignMul(mat, &sc)
// }

func NewMat4() Mat4 {
	return (Mat4)(mat4.From(&mat4.Ident))
}

func CalculateTransform(prj Mat4, width, height, scale float32, position v3.T) Mat4 {
	// convert parameters into a transform
	out := mat4.From(&mat4.Ident)

	// get dimensions
	w := float64(width)
	h := float64(height)
	// determine longer side
	r := math.Min(w, h) / math.Max(w, h)

	// identity matrix to build other matrices from
	m := mat4.From(&mat4.Ident)

	// scale
	sc := mat4.From(&mat4.Ident)
	if w > h {
		sc = *sc.ScaleVec3(&v3.T{scale, float32(r) * scale, scale})
	} else if h > w {
		sc = *sc.ScaleVec3(&v3.T{float32(r) * scale, scale, scale})
	} else {
		sc = *sc.ScaleVec3(&v3.T{float32(r) * scale, float32(r) * scale, scale})
	}

	// apply scale
	out = *m.AssignMul(&out, &sc)
	// apply projection to 2D
	out = *m.AssignMul(&out, prj.T())
	// screen space translations need to be normalized
	out = *m.Translate(&position)
	// open gl is column major, with translation in the right-most values,
	// however this math library seems to be row majow, doing a transpose here puts the values in the correct order
	out = *out.Transpose()
	return (Mat4)(out)
}

func loadGif(filename string) *gif.GIF {

	templateFile, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	templateImg, err := gif.DecodeAll(templateFile)
	if err != nil {
		log.Fatal(err)
	}

	return templateImg
}

func Init() {
	if SpriteVAO == 0 {
		SpriteVAO = gfx.CreateVAO(squareVerts, squareInds)
	}
}

func LoadTexture(filename string) (*gfx.Texture, error) {
	t, err := gfx.NewTextureFromFile(filename, gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	if err != nil {
		fmt.Println(err.Error())
		t, _ = gfx.NewTextureFromFile("./resources/ui/missing.png", gl.CLAMP_TO_EDGE, gl.CLAMP_TO_EDGE)
	}

	return t, err
}

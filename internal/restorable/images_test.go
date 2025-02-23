// Copyright 2017 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package restorable_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/affine"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/v2/internal/restorable"
	etesting "github.com/hajimehoshi/ebiten/v2/internal/testing"
	"github.com/hajimehoshi/ebiten/v2/internal/ui"
)

func TestMain(m *testing.M) {
	restorable.EnableRestoringForTesting()
	etesting.MainWithRunLoop(m)
}

func pixelsToColor(p *restorable.Pixels, i, j int) color.RGBA {
	r, g, b, a := p.At(i, j)
	return color.RGBA{r, g, b, a}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// sameColors compares c1 and c2 and returns a boolean value indicating
// if the two colors are (almost) same.
//
// Pixels read from GPU might include errors (#492), and
// sameColors considers such errors as delta.
func sameColors(c1, c2 color.RGBA, delta int) bool {
	return abs(int(c1.R)-int(c2.R)) <= delta &&
		abs(int(c1.G)-int(c2.G)) <= delta &&
		abs(int(c1.B)-int(c2.B)) <= delta &&
		abs(int(c1.A)-int(c2.A)) <= delta
}

func TestRestore(t *testing.T) {
	img0 := restorable.NewImage(1, 1, restorable.ImageTypeRegular)
	defer img0.Dispose()

	clr0 := color.RGBA{0x00, 0x00, 0x00, 0xff}
	img0.ReplacePixels([]byte{clr0.R, clr0.G, clr0.B, clr0.A}, nil, 0, 0, 1, 1)
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	want := clr0
	got := pixelsToColor(img0.BasePixelsForTesting(), 0, 0)
	if !sameColors(got, want, 1) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRestoreWithoutDraw(t *testing.T) {
	img0 := restorable.NewImage(1024, 1024, restorable.ImageTypeRegular)
	defer img0.Dispose()

	// If there is no drawing command on img0, img0 is cleared when restored.

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}

	for j := 0; j < 1024; j++ {
		for i := 0; i < 1024; i++ {
			want := color.RGBA{0x00, 0x00, 0x00, 0x00}
			got := pixelsToColor(img0.BasePixelsForTesting(), i, j)
			if !sameColors(got, want, 0) {
				t.Errorf("got %v, want %v", got, want)
			}
		}
	}
}

func quadVertices(srcImage *restorable.Image, sw, sh, x, y int) []float32 {
	dx0 := float32(x)
	dy0 := float32(y)
	dx1 := float32(x + sw)
	dy1 := float32(y + sh)
	sx0 := float32(0)
	sy0 := float32(0)
	sx1 := float32(sw)
	sy1 := float32(sh)
	iswf := float32(1)
	ishf := float32(1)
	if srcImage != nil {
		isw, ish := srcImage.InternalSize()
		iswf, ishf = float32(isw), float32(ish)
	}
	return []float32{
		dx0, dy0, sx0 / iswf, sy0 / ishf, 1, 1, 1, 1,
		dx1, dy0, sx1 / iswf, sy0 / ishf, 1, 1, 1, 1,
		dx0, dy1, sx0 / iswf, sy1 / ishf, 1, 1, 1, 1,
		dx1, dy1, sx1 / iswf, sy1 / ishf, 1, 1, 1, 1,
	}
}

func TestRestoreChain(t *testing.T) {
	const num = 10
	imgs := []*restorable.Image{}
	for i := 0; i < num; i++ {
		img := restorable.NewImage(1, 1, restorable.ImageTypeRegular)
		imgs = append(imgs, img)
	}
	defer func() {
		for _, img := range imgs {
			img.Dispose()
		}
	}()
	clr := color.RGBA{0x00, 0x00, 0x00, 0xff}
	imgs[0].ReplacePixels([]byte{clr.R, clr.G, clr.B, clr.A}, nil, 0, 0, 1, 1)
	for i := 0; i < num-1; i++ {
		vs := quadVertices(imgs[i], 1, 1, 0, 0)
		is := graphics.QuadIndices()
		dr := graphicsdriver.Region{
			X:      0,
			Y:      0,
			Width:  1,
			Height: 1,
		}
		imgs[i+1].DrawTriangles([graphics.ShaderImageNum]*restorable.Image{imgs[i]}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	}
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	want := clr
	for i, img := range imgs {
		got := pixelsToColor(img.BasePixelsForTesting(), 0, 0)
		if !sameColors(got, want, 1) {
			t.Errorf("%d: got %v, want %v", i, got, want)
		}
	}
}

func TestRestoreChain2(t *testing.T) {
	const (
		num = 10
		w   = 1
		h   = 1
	)
	imgs := []*restorable.Image{}
	for i := 0; i < num; i++ {
		img := restorable.NewImage(w, h, restorable.ImageTypeRegular)
		imgs = append(imgs, img)
	}
	defer func() {
		for _, img := range imgs {
			img.Dispose()
		}
	}()

	clr0 := color.RGBA{0xff, 0x00, 0x00, 0xff}
	imgs[0].ReplacePixels([]byte{clr0.R, clr0.G, clr0.B, clr0.A}, nil, 0, 0, w, h)
	clr7 := color.RGBA{0x00, 0xff, 0x00, 0xff}
	imgs[7].ReplacePixels([]byte{clr7.R, clr7.G, clr7.B, clr7.A}, nil, 0, 0, w, h)
	clr8 := color.RGBA{0x00, 0x00, 0xff, 0xff}
	imgs[8].ReplacePixels([]byte{clr8.R, clr8.G, clr8.B, clr8.A}, nil, 0, 0, w, h)

	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	imgs[8].DrawTriangles([graphics.ShaderImageNum]*restorable.Image{imgs[7]}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(imgs[7], w, h, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	imgs[9].DrawTriangles([graphics.ShaderImageNum]*restorable.Image{imgs[8]}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(imgs[8], w, h, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	for i := 0; i < 7; i++ {
		imgs[i+1].DrawTriangles([graphics.ShaderImageNum]*restorable.Image{imgs[i]}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(imgs[i], w, h, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	}

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	for i, img := range imgs {
		want := clr0
		if i == 8 || i == 9 {
			want = clr7
		}
		got := pixelsToColor(img.BasePixelsForTesting(), 0, 0)
		if !sameColors(got, want, 1) {
			t.Errorf("%d: got %v, want %v", i, got, want)
		}
	}
}

func TestRestoreOverrideSource(t *testing.T) {
	const (
		w = 1
		h = 1
	)
	img0 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img1 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img2 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img3 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	defer func() {
		img3.Dispose()
		img2.Dispose()
		img1.Dispose()
		img0.Dispose()
	}()
	clr0 := color.RGBA{0x00, 0x00, 0x00, 0xff}
	clr1 := color.RGBA{0x00, 0x00, 0x01, 0xff}
	img1.ReplacePixels([]byte{clr0.R, clr0.G, clr0.B, clr0.A}, nil, 0, 0, w, h)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	img2.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img1}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img1, w, h, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img3.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img2}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img2, w, h, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img0.ReplacePixels([]byte{clr1.R, clr1.G, clr1.B, clr1.A}, nil, 0, 0, w, h)
	img1.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img0}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img0, w, h, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name string
		want color.RGBA
		got  color.RGBA
	}{
		{
			"0",
			clr1,
			pixelsToColor(img0.BasePixelsForTesting(), 0, 0),
		},
		{
			"1",
			clr1,
			pixelsToColor(img1.BasePixelsForTesting(), 0, 0),
		},
		{
			"2",
			clr0,
			pixelsToColor(img2.BasePixelsForTesting(), 0, 0),
		},
		{
			"3",
			clr0,
			pixelsToColor(img3.BasePixelsForTesting(), 0, 0),
		},
	}
	for _, c := range testCases {
		if !sameColors(c.got, c.want, 1) {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestRestoreComplexGraph(t *testing.T) {
	const (
		w = 4
		h = 1
	)
	// 0 -> 3
	// 1 -> 3
	// 1 -> 4
	// 2 -> 4
	// 2 -> 7
	// 3 -> 5
	// 3 -> 6
	// 3 -> 7
	// 4 -> 6
	base := image.NewRGBA(image.Rect(0, 0, w, h))
	base.Pix[0] = 0xff
	base.Pix[1] = 0xff
	base.Pix[2] = 0xff
	base.Pix[3] = 0xff
	img0 := newImageFromImage(base)
	img1 := newImageFromImage(base)
	img2 := newImageFromImage(base)
	img3 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img4 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img5 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img6 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img7 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	defer func() {
		img7.Dispose()
		img6.Dispose()
		img5.Dispose()
		img4.Dispose()
		img3.Dispose()
		img2.Dispose()
		img1.Dispose()
		img0.Dispose()
	}()
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	var offsets [graphics.ShaderImageNum - 1][2]float32
	vs := quadVertices(img0, w, h, 0, 0)
	img3.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img0}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img1, w, h, 1, 0)
	img3.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img1}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img1, w, h, 1, 0)
	img4.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img1}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img2, w, h, 2, 0)
	img4.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img2}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img3, w, h, 0, 0)
	img5.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img3}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img3, w, h, 0, 0)
	img6.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img3}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img4, w, h, 1, 0)
	img6.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img4}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img2, w, h, 0, 0)
	img7.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img2}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	vs = quadVertices(img3, w, h, 2, 0)
	img7.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img3}, offsets, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name  string
		out   string
		image *restorable.Image
	}{
		{
			"0",
			"*---",
			img0,
		},
		{
			"1",
			"*---",
			img1,
		},
		{
			"2",
			"*---",
			img2,
		},
		{
			"3",
			"**--",
			img3,
		},
		{
			"4",
			"-**-",
			img4,
		},
		{
			"5",
			"**--",
			img5,
		},
		{
			"6",
			"****",
			img6,
		},
		{
			"7",
			"*-**",
			img7,
		},
	}
	for _, c := range testCases {
		for i := 0; i < 4; i++ {
			want := color.RGBA{}
			if c.out[i] == '*' {
				want = color.RGBA{0xff, 0xff, 0xff, 0xff}
			}
			got := pixelsToColor(c.image.BasePixelsForTesting(), i, 0)
			if !sameColors(got, want, 1) {
				t.Errorf("%s[%d]: got %v, want %v", c.name, i, got, want)
			}
		}
	}
}

func newImageFromImage(rgba *image.RGBA) *restorable.Image {
	s := rgba.Bounds().Size()
	img := restorable.NewImage(s.X, s.Y, restorable.ImageTypeRegular)
	img.ReplacePixels(rgba.Pix, nil, 0, 0, s.X, s.Y)
	return img
}

func TestRestoreRecursive(t *testing.T) {
	const (
		w = 4
		h = 1
	)
	base := image.NewRGBA(image.Rect(0, 0, w, h))
	base.Pix[0] = 0xff
	base.Pix[1] = 0xff
	base.Pix[2] = 0xff
	base.Pix[3] = 0xff

	img0 := newImageFromImage(base)
	img1 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	defer func() {
		img1.Dispose()
		img0.Dispose()
	}()
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	img1.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img0}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img0, w, h, 1, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img0.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img1}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img1, w, h, 1, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name  string
		out   string
		image *restorable.Image
	}{
		{
			"0",
			"*-*-",
			img0,
		},
		{
			"1",
			"-*--",
			img1,
		},
	}
	for _, c := range testCases {
		for i := 0; i < 4; i++ {
			want := color.RGBA{}
			if c.out[i] == '*' {
				want = color.RGBA{0xff, 0xff, 0xff, 0xff}
			}
			got := pixelsToColor(c.image.BasePixelsForTesting(), i, 0)
			if !sameColors(got, want, 1) {
				t.Errorf("%s[%d]: got %v, want %v", c.name, i, got, want)
			}
		}
	}
}

func TestReplacePixels(t *testing.T) {
	img := restorable.NewImage(17, 31, restorable.ImageTypeRegular)
	defer img.Dispose()

	pix := make([]byte, 4*4*4)
	for i := range pix {
		pix[i] = 0xff
	}
	img.ReplacePixels(pix, nil, 5, 7, 4, 4)
	// Check the region (5, 7)-(9, 11). Outside state is indeterministic.
	for j := 7; j < 11; j++ {
		for i := 5; i < 9; i++ {
			r, g, b, a, err := img.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := color.RGBA{0xff, 0xff, 0xff, 0xff}
			if got != want {
				t.Errorf("img.At(%d, %d): got: %v, want: %v", i, j, got, want)
			}
		}
	}
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	for j := 7; j < 11; j++ {
		for i := 5; i < 9; i++ {
			r, g, b, a, err := img.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := color.RGBA{0xff, 0xff, 0xff, 0xff}
			if got != want {
				t.Errorf("img.At(%d, %d): got: %v, want: %v", i, j, got, want)
			}
		}
	}
}

func TestDrawTrianglesAndReplacePixels(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 1, 1))
	base.Pix[0] = 0xff
	base.Pix[1] = 0
	base.Pix[2] = 0
	base.Pix[3] = 0xff
	img0 := newImageFromImage(base)
	defer img0.Dispose()
	img1 := restorable.NewImage(2, 1, restorable.ImageTypeRegular)
	defer img1.Dispose()

	vs := quadVertices(img0, 1, 1, 0, 0)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  2,
		Height: 1,
	}
	img1.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img0}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img1.ReplacePixels([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, nil, 0, 0, 2, 1)

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	r, g, b, a, err := img1.At(ui.GraphicsDriverForTesting(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := color.RGBA{r, g, b, a}
	want := color.RGBA{0xff, 0xff, 0xff, 0xff}
	if !sameColors(got, want, 1) {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestDispose(t *testing.T) {
	base0 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img0 := newImageFromImage(base0)
	defer img0.Dispose()

	base1 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img1 := newImageFromImage(base1)

	base2 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	base2.Pix[0] = 0xff
	base2.Pix[1] = 0xff
	base2.Pix[2] = 0xff
	base2.Pix[3] = 0xff
	img2 := newImageFromImage(base2)
	defer img2.Dispose()

	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  1,
		Height: 1,
	}
	img1.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img2}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img2, 1, 1, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img0.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img1}, [graphics.ShaderImageNum - 1][2]float32{}, quadVertices(img1, 1, 1, 0, 0), is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img1.Dispose()

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	r, g, b, a, err := img0.At(ui.GraphicsDriverForTesting(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := color.RGBA{r, g, b, a}
	want := color.RGBA{0xff, 0xff, 0xff, 0xff}
	if !sameColors(got, want, 1) {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestReplacePixelsPart(t *testing.T) {
	pix := make([]uint8, 4*2*2)
	for i := range pix {
		pix[i] = 0xff
	}

	img := restorable.NewImage(4, 4, restorable.ImageTypeRegular)
	// This doesn't make the image stale. Its base pixels are available.
	img.ReplacePixels(pix, nil, 1, 1, 2, 2)

	cases := []struct {
		i    int
		j    int
		want color.RGBA
	}{
		{
			i:    0,
			j:    0,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    3,
			j:    0,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    0,
			j:    1,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    1,
			j:    1,
			want: color.RGBA{0xff, 0xff, 0xff, 0xff},
		},
		{
			i:    3,
			j:    1,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    0,
			j:    2,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    2,
			j:    2,
			want: color.RGBA{0xff, 0xff, 0xff, 0xff},
		},
		{
			i:    3,
			j:    2,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    0,
			j:    3,
			want: color.RGBA{0, 0, 0, 0},
		},
		{
			i:    3,
			j:    3,
			want: color.RGBA{0, 0, 0, 0},
		},
	}
	for _, c := range cases {
		got := pixelsToColor(img.BasePixelsForTesting(), c.i, c.j)
		want := c.want
		if got != want {
			t.Errorf("base pixel (%d, %d): got %v, want %v", c.i, c.j, got, want)
		}
	}
}

func TestReplacePixelsOnly(t *testing.T) {
	const w, h = 128, 128
	img0 := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	defer img0.Dispose()
	img1 := restorable.NewImage(1, 1, restorable.ImageTypeRegular)
	defer img1.Dispose()

	for i := 0; i < w*h; i += 5 {
		img0.ReplacePixels([]byte{1, 2, 3, 4}, nil, i%w, i/w, 1, 1)
	}

	vs := quadVertices(img0, 1, 1, 0, 0)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  1,
		Height: 1,
	}
	img1.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{img0}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	img0.ReplacePixels([]byte{5, 6, 7, 8}, nil, 0, 0, 1, 1)

	// BasePixelsForTesting is available without GPU accessing.
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			idx := j*w + i
			var want color.RGBA
			switch {
			case idx == 0:
				want = color.RGBA{5, 6, 7, 8}
			case idx%5 == 0:
				want = color.RGBA{1, 2, 3, 4}
			}
			got := pixelsToColor(img0.BasePixelsForTesting(), i, j)
			if !sameColors(got, want, 0) {
				t.Errorf("got %v, want %v", got, want)
			}
		}
	}

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	want := color.RGBA{1, 2, 3, 4}
	got := pixelsToColor(img1.BasePixelsForTesting(), 0, 0)
	if !sameColors(got, want, 0) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TODO: How about volatile/screen images?

// Issue #793
func TestReadPixelsFromVolatileImage(t *testing.T) {
	const w, h = 16, 16
	dst := restorable.NewImage(w, h, restorable.ImageTypeVolatile)
	src := restorable.NewImage(w, h, restorable.ImageTypeRegular)

	// First, make sure that dst has pixels
	dst.ReplacePixels(make([]byte, 4*w*h), nil, 0, 0, w, h)

	// Second, draw src to dst. If the implementation is correct, dst becomes stale.
	pix := make([]byte, 4*w*h)
	for i := range pix {
		pix[i] = 0xff
	}
	src.ReplacePixels(pix, nil, 0, 0, w, h)
	vs := quadVertices(src, 1, 1, 0, 0)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeCopy, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)

	// Read the pixels. If the implementation is correct, dst tries to read its pixels from GPU due to being
	// stale.
	want := byte(0xff)
	got, _, _, _, err := dst.At(ui.GraphicsDriverForTesting(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestAllowReplacePixelsAfterDrawTriangles(t *testing.T) {
	const w, h = 16, 16
	src := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	dst := restorable.NewImage(w, h, restorable.ImageTypeRegular)

	vs := quadVertices(src, w, h, 0, 0)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	dst.ReplacePixels(make([]byte, 4*w*h), nil, 0, 0, w, h)
	// ReplacePixels for a whole image doesn't panic.
}

func TestDisallowReplacePixelsForPartAfterDrawTriangles(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ReplacePixels for a part after DrawTriangles must panic but not")
		}
	}()

	const w, h = 16, 16
	src := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	dst := restorable.NewImage(w, h, restorable.ImageTypeRegular)

	vs := quadVertices(src, w, h, 0, 0)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	dst.ReplacePixels(make([]byte, 4), nil, 0, 0, 1, 1)
}

func TestExtend(t *testing.T) {
	pixAt := func(i, j int) byte {
		return byte(17*i + 13*j + 0x40)
	}

	const w, h = 16, 16
	orig := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	pix := make([]byte, 4*w*h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			idx := j*w + i
			v := pixAt(i, j)
			pix[4*idx] = v
			pix[4*idx+1] = v
			pix[4*idx+2] = v
			pix[4*idx+3] = v
		}
	}

	orig.ReplacePixels(pix, nil, 0, 0, w, h)
	extended := orig.Extend(w*2, h*2) // After this, orig is already disposed.

	for j := 0; j < h*2; j++ {
		for i := 0; i < w*2; i++ {
			got, _, _, _, err := extended.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			want := byte(0)
			if i < w && j < h {
				want = pixAt(i, j)
			}
			if got != want {
				t.Errorf("extended.At(%d, %d): got: %v, want: %v", i, j, got, want)
			}
		}
	}
}

func TestClearPixels(t *testing.T) {
	const w, h = 16, 16
	img := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	img.ReplacePixels(make([]byte, 4*4*4), nil, 0, 0, 4, 4)
	img.ReplacePixels(make([]byte, 4*4*4), nil, 4, 0, 4, 4)
	img.ClearPixels(0, 0, 4, 4)
	img.ClearPixels(4, 0, 4, 4)

	// After clearing, the regions will be available again.
	img.ReplacePixels(make([]byte, 4*8*4), nil, 0, 0, 8, 4)
}

func TestMutateSlices(t *testing.T) {
	const w, h = 16, 16
	dst := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	src := restorable.NewImage(w, h, restorable.ImageTypeRegular)
	pix := make([]byte, 4*w*h)
	for i := 0; i < w*h; i++ {
		pix[4*i] = byte(i)
		pix[4*i+1] = byte(i)
		pix[4*i+2] = byte(i)
		pix[4*i+3] = 0xff
	}
	src.ReplacePixels(pix, nil, 0, 0, w, h)

	vs := quadVertices(src, w, h, 0, 0)
	is := make([]uint16, len(graphics.QuadIndices()))
	copy(is, graphics.QuadIndices())
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*restorable.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	for i := range vs {
		vs[i] = 0
	}
	for i := range is {
		is[i] = 0
	}
	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}

	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			r, g, b, a, err := src.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			want := color.RGBA{r, g, b, a}
			r, g, b, a, err = dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			if !sameColors(got, want, 1) {
				t.Errorf("(%d, %d): got %v, want %v", i, j, got, want)
			}
		}
	}
}

func TestOverlappedPixels(t *testing.T) {
	dst := restorable.NewImage(3, 3, restorable.ImageTypeRegular)

	pix0 := make([]byte, 4*2*2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			idx := 4 * (j*2 + i)
			pix0[idx] = 0xff
			pix0[idx+1] = 0
			pix0[idx+2] = 0
			pix0[idx+3] = 0xff
		}
	}
	dst.ReplacePixels(pix0, nil, 0, 0, 2, 2)

	pix1 := make([]byte, 4*2*2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			idx := 4 * (j*2 + i)
			pix1[idx] = 0
			pix1[idx+1] = 0xff
			pix1[idx+2] = 0
			pix1[idx+3] = 0xff
		}
	}
	dst.ReplacePixels(pix1, nil, 1, 1, 2, 2)

	wantColors := []color.RGBA{
		{0xff, 0, 0, 0xff},
		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},

		{0xff, 0, 0, 0xff},
		{0, 0xff, 0, 0xff},
		{0, 0xff, 0, 0xff},

		{0, 0, 0, 0},
		{0, 0xff, 0, 0xff},
		{0, 0xff, 0, 0xff},
	}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}

	dst.ReplacePixels(nil, nil, 1, 0, 2, 2)

	wantColors = []color.RGBA{
		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},
		{0, 0, 0, 0},

		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},
		{0, 0, 0, 0},

		{0, 0, 0, 0},
		{0, 0xff, 0, 0xff},
		{0, 0xff, 0, 0xff},
	}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}

	pix2 := make([]byte, 4*2*2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			idx := 4 * (j*2 + i)
			pix2[idx] = 0
			pix2[idx+1] = 0
			pix2[idx+2] = 0xff
			pix2[idx+3] = 0xff
		}
	}
	dst.ReplacePixels(pix2, nil, 1, 1, 2, 2)

	wantColors = []color.RGBA{
		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},
		{0, 0, 0, 0},

		{0xff, 0, 0, 0xff},
		{0, 0, 0xff, 0xff},
		{0, 0, 0xff, 0xff},

		{0, 0, 0, 0},
		{0, 0, 0xff, 0xff},
		{0, 0, 0xff, 0xff},
	}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}

	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}
}

func TestReplacePixelsWithMask(t *testing.T) {
	dst := restorable.NewImage(3, 3, restorable.ImageTypeRegular)

	pix0 := make([]byte, 4*2*2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			idx := 4 * (j*2 + i)
			pix0[idx] = 0xff
			pix0[idx+1] = 0
			pix0[idx+2] = 0
			pix0[idx+3] = 0xff
		}
	}
	dst.ReplacePixels(pix0, nil, 0, 0, 2, 2)

	pix1 := make([]byte, 4*2*2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			idx := 4 * (j*2 + i)
			pix1[idx] = 0
			pix1[idx+1] = 0xff
			pix1[idx+2] = 0
			pix1[idx+3] = 0xff
		}
	}

	mask1 := []byte{0b00001110}
	dst.ReplacePixels(pix1, mask1, 1, 1, 2, 2)

	wantColors := []color.RGBA{
		{0xff, 0, 0, 0xff},
		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},

		{0xff, 0, 0, 0xff},
		{0xff, 0, 0, 0xff},
		{0, 0xff, 0, 0xff},

		{0, 0, 0, 0},
		{0, 0xff, 0, 0xff},
		{0, 0xff, 0, 0xff},
	}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}

	dst.ReplacePixels(nil, nil, 1, 0, 2, 2)

	wantColors = []color.RGBA{
		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},
		{0, 0, 0, 0},

		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},
		{0, 0, 0, 0},

		{0, 0, 0, 0},
		{0, 0xff, 0, 0xff},
		{0, 0xff, 0, 0xff},
	}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}

	// Update the same region with pix1/mask1.
	pix2 := make([]byte, 4*2*2)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			idx := 4 * (j*2 + i)
			pix2[idx] = 0
			pix2[idx+1] = 0
			pix2[idx+2] = 0xff
			pix2[idx+3] = 0xff
		}
	}
	mask2 := []byte{0b00000111}
	dst.ReplacePixels(pix2, mask2, 1, 1, 2, 2)

	wantColors = []color.RGBA{
		{0xff, 0, 0, 0xff},
		{0, 0, 0, 0},
		{0, 0, 0, 0},

		{0xff, 0, 0, 0xff},
		{0, 0, 0xff, 0xff},
		{0, 0, 0xff, 0xff},

		{0, 0, 0, 0},
		{0, 0, 0xff, 0xff},
		{0, 0xff, 0, 0xff},
	}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}

	if err := restorable.ResolveStaleImages(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}
	if err := restorable.RestoreIfNeeded(ui.GraphicsDriverForTesting()); err != nil {
		t.Fatal(err)
	}

	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r, g, b, a, err := dst.At(ui.GraphicsDriverForTesting(), i, j)
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBA{r, g, b, a}
			want := wantColors[3*j+i]
			if got != want {
				t.Errorf("color at (%d, %d): got %v, want: %v", i, j, got, want)
			}
		}
	}
}

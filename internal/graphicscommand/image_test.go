// Copyright 2018 The Ebiten Authors
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

package graphicscommand_test

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/affine"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicscommand"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	etesting "github.com/hajimehoshi/ebiten/v2/internal/testing"
	"github.com/hajimehoshi/ebiten/v2/internal/ui"
)

func TestMain(m *testing.M) {
	etesting.MainWithRunLoop(m)
}

func quadVertices(srcImage *graphicscommand.Image, w, h float32) []float32 {
	sw, sh := srcImage.InternalSize()
	swf, shf := float32(sw), float32(sh)
	return []float32{
		0, 0, 0, 0, 1, 1, 1, 1,
		w, 0, w / swf, 0, 1, 1, 1, 1,
		0, w, 0, h / shf, 1, 1, 1, 1,
		w, h, w / swf, h / shf, 1, 1, 1, 1,
	}
}

func TestClear(t *testing.T) {
	const w, h = 1024, 1024
	src := graphicscommand.NewImage(w/2, h/2, false)
	dst := graphicscommand.NewImage(w, h, false)

	vs := quadVertices(src, w/2, h/2)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*graphicscommand.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeClear, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)

	pix := make([]byte, 4*w*h)
	if err := dst.ReadPixels(ui.GraphicsDriverForTesting(), pix); err != nil {
		t.Fatal(err)
	}
	for j := 0; j < h/2; j++ {
		for i := 0; i < w/2; i++ {
			idx := 4 * (i + w*j)
			got := color.RGBA{pix[idx], pix[idx+1], pix[idx+2], pix[idx+3]}
			want := color.RGBA{}
			if got != want {
				t.Errorf("dst.At(%d, %d) after DrawTriangles: got %v, want: %v", i, j, got, want)
			}
		}
	}
}

func TestReplacePixelsPartAfterDrawTriangles(t *testing.T) {
	const w, h = 32, 32
	clr := graphicscommand.NewImage(w, h, false)
	src := graphicscommand.NewImage(w/2, h/2, false)
	dst := graphicscommand.NewImage(w, h, false)
	vs := quadVertices(src, w/2, h/2)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*graphicscommand.Image{clr}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeClear, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	dst.DrawTriangles([graphics.ShaderImageNum]*graphicscommand.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)
	dst.ReplacePixels(make([]byte, 4), nil, 0, 0, 1, 1)

	// TODO: Check the result.
}

func TestReplacePixelsWithMask(t *testing.T) {
	const w, h = 4, 3
	src := graphicscommand.NewImage(w, h, false)
	dst := graphicscommand.NewImage(w, h, false)

	vs := quadVertices(src, w, h)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*graphicscommand.Image{src}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeClear, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)

	pix0 := make([]byte, 4*w*h)
	for i := range pix0 {
		pix0[i] = 0x40
	}
	dst.ReplacePixels(pix0, nil, 0, 0, w, h)

	pix1 := make([]byte, 4*w*h)
	for i := range pix1 {
		pix1[i] = 0x80
	}
	mask1 := []byte{0b11110110, 0b00000110}
	dst.ReplacePixels(pix1, mask1, 0, 0, w, h)

	readPix := make([]byte, 4*w*h)
	if err := dst.ReadPixels(ui.GraphicsDriverForTesting(), readPix); err != nil {
		t.Fatal(err)
	}
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			idx := 4 * (i + w*j)
			got := color.RGBA{readPix[idx], readPix[idx+1], readPix[idx+2], readPix[idx+3]}
			var want color.RGBA
			if (i != 0 && i != w-1) || (j != 0 && j != h-1) {
				want = color.RGBA{0x80, 0x80, 0x80, 0x80}
			} else {
				want = color.RGBA{0x40, 0x40, 0x40, 0x40}
			}
			if got != want {
				t.Errorf("dst.At(%d, %d) after ReplacePixels: got %v, want: %v", i, j, got, want)
			}
		}
	}
}

func TestShader(t *testing.T) {
	const w, h = 16, 16
	clr := graphicscommand.NewImage(w, h, false)
	dst := graphicscommand.NewImage(w, h, false)
	vs := quadVertices(clr, w, h)
	is := graphics.QuadIndices()
	dr := graphicsdriver.Region{
		X:      0,
		Y:      0,
		Width:  w,
		Height: h,
	}
	dst.DrawTriangles([graphics.ShaderImageNum]*graphicscommand.Image{clr}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeClear, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, nil, nil, false)

	g := ui.GraphicsDriverForTesting()
	s := graphicscommand.NewShader(etesting.ShaderProgramFill(0xff, 0, 0, 0xff))
	dst.DrawTriangles([graphics.ShaderImageNum]*graphicscommand.Image{}, [graphics.ShaderImageNum - 1][2]float32{}, vs, is, affine.ColorMIdentity{}, graphicsdriver.CompositeModeSourceOver, graphicsdriver.FilterNearest, graphicsdriver.AddressUnsafe, dr, graphicsdriver.Region{}, s, nil, false)

	pix := make([]byte, 4*w*h)
	if err := dst.ReadPixels(g, pix); err != nil {
		t.Fatal(err)
	}
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			idx := 4 * (i + w*j)
			got := color.RGBA{pix[idx], pix[idx+1], pix[idx+2], pix[idx+3]}
			want := color.RGBA{0xff, 0, 0, 0xff}
			if got != want {
				t.Errorf("dst.At(%d, %d) after DrawTriangles: got %v, want: %v", i, j, got, want)
			}
		}
	}
}

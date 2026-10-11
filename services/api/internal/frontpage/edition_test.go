package frontpage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestNormalizeLayout(t *testing.T) {
	allowed := map[string]bool{"event:a": true, "event:b": true, "event:c": true, "event:d": true, "event:e": true}
	fixed := map[string]string{"event:d": SlotLead, "event:e": SlotMore}
	blocks := normalizeLayout([]storedBlock{
		{Kind: KindEvent, ID: "a", Size: SizeLead, Image: true},
		{Kind: KindEvent, ID: "a", Size: SizeFeature},
		{Kind: KindEvent, ID: "x", Size: SizeFeature},
		{Kind: KindEvent, ID: "b", Size: SizeLead, Image: true},
		{Kind: KindEvent, ID: "c", Size: SizeShort, Image: true},
		{Kind: KindEvent, ID: "d", Size: SizeBrief},
	}, allowed, fixed)
	if blocks[0].ID != "d" || blocks[0].Size != SizeLead {
		t.Fatalf("fixed lead must lead: %+v", blocks)
	}
	leads, seen := 0, map[string]bool{}
	for _, block := range blocks {
		if seen[block.ID] {
			t.Fatalf("duplicate story %s", block.ID)
		}
		seen[block.ID] = true
		if block.Size == SizeLead {
			leads++
		}
		if block.ID == "x" {
			t.Fatal("unknown story kept")
		}
		if block.ID == "c" && block.Image {
			t.Fatal("short blocks must not get images")
		}
		if block.ID == "e" && block.Size != SizeShort {
			t.Fatalf("fixed more story must be short or brief, got %s", block.Size)
		}
	}
	if leads != 1 || !seen["e"] {
		t.Fatalf("want one lead and every fixed story: %+v", blocks)
	}
}

func TestNewsprintMakesSmallGrayJPEG(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2000, 1500))
	for y := 0; y < 1500; y++ {
		for x := 0; x < 2000; x++ {
			source.Set(x, y, color.RGBA{R: uint8(x), G: 40, B: uint8(y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		t.Fatal(err)
	}
	data, width, height, err := newsprint(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if width != illustrationWidth || height != 1500*illustrationWidth/2000 {
		t.Fatalf("size = %dx%d", width, height)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, gray := decoded.(*image.Gray); !gray {
		t.Fatalf("want grayscale JPEG, got %T", decoded)
	}
}

func TestIllustrationKey(t *testing.T) {
	if key, ok := IllustrationKey("3166cd78-a11b-47f4-a44f-96eef3e53b56.jpg"); !ok || key != "illustrations/3166cd78-a11b-47f4-a44f-96eef3e53b56.jpg" {
		t.Fatalf("valid name rejected: %q", key)
	}
	for _, name := range []string{"../secret.jpg", "x.jpg", "3166cd78-a11b-47f4-a44f-96eef3e53b56.png"} {
		if _, ok := IllustrationKey(name); ok {
			t.Fatalf("%q accepted", name)
		}
	}
}

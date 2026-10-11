package frontpage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"log/slog"
	"regexp"
)

// illustrationName matches the stored illustration file names served publicly.
var illustrationName = regexp.MustCompile(`^[0-9a-f-]{36}\.jpg$`)

// IllustrationKey returns the media key for a public illustration file name.
func IllustrationKey(name string) (string, bool) {
	if !illustrationName.MatchString(name) {
		return "", false
	}
	return "illustrations/" + name, true
}

const (
	illustrationWidth   = 960
	illustrationQuality = 72
	illustrationCaption = "චිත්‍රය: AI නිර්මාණයකි — සැබෑ ඡායාරූපයක් නොවේ"
)

// illustrate draws the edition's illustrations one at a time. A story keeps its illustration
// across editions; a new one is drawn only when the story has none.
func (service *Service) illustrate(ctx context.Context, blocks []storedBlock, stories map[string]Story) {
	if service.desk.Illustrator == nil || service.desk.Media == nil || service.desk.ImageModel == "" {
		return
	}
	for _, block := range blocks {
		if !block.Image || block.ReportID == "" {
			continue
		}
		var existing, prompt, alt string
		err := service.store.pool.QueryRow(ctx, `
			SELECT COALESCE((SELECT image_key FROM story_reports other
			                 WHERE other.image_key IS NOT NULL AND (other.event_id::text = $2 OR other.article_id::text = $2)
			                 ORDER BY other.created_at DESC LIMIT 1), ''),
			       illustration_prompt, headline
			FROM story_reports WHERE id = $1`, block.ReportID, block.ID).Scan(&existing, &prompt, &alt)
		if err != nil || prompt == "" && existing == "" {
			continue
		}
		if existing != "" {
			_, _ = service.store.pool.Exec(ctx, `
				UPDATE story_reports report SET image_key = source.image_key, image_alt = source.image_alt,
				       image_width = source.image_width, image_height = source.image_height
				FROM story_reports source WHERE report.id = $1 AND source.image_key = $2`, block.ReportID, existing)
			continue
		}
		data, err := service.desk.Illustrator.GenerateImage(ctx, service.desk.ImageModel, illustrationStyle+prompt, imageTimeout)
		if err != nil {
			slog.Warn("front page illustration not drawn", "story", block.ID, "error", err)
			continue
		}
		encoded, width, height, err := newsprint(data)
		if err != nil {
			slog.Warn("front page illustration unreadable", "story", block.ID, "error", err)
			continue
		}
		key := fmt.Sprintf("illustrations/%s.jpg", block.ReportID)
		if err := service.desk.Media.Put(ctx, key, "image/jpeg", encoded); err != nil {
			slog.Warn("front page illustration not stored", "story", block.ID, "error", err)
			continue
		}
		if _, err := service.store.pool.Exec(ctx, `
			UPDATE story_reports SET image_key = $2, image_alt = $3, image_width = $4, image_height = $5 WHERE id = $1`,
			block.ReportID, key, truncateRunes(alt, 200), width, height); err != nil {
			slog.Warn("front page illustration not recorded", "story", block.ID, "error", err)
		}
	}
}

// newsprint converts an image to a grayscale JPEG at most illustrationWidth wide, averaging
// the pixels each output pixel covers.
func newsprint(data []byte) ([]byte, int, int, error) {
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width == 0 || height == 0 {
		return nil, 0, 0, fmt.Errorf("empty image")
	}
	outWidth, outHeight := width, height
	if width > illustrationWidth {
		outWidth, outHeight = illustrationWidth, height*illustrationWidth/width
	}
	output := image.NewGray(image.Rect(0, 0, outWidth, outHeight))
	for y := 0; y < outHeight; y++ {
		y0, y1 := y*height/outHeight, max((y+1)*height/outHeight, y*height/outHeight+1)
		for x := 0; x < outWidth; x++ {
			x0, x1 := x*width/outWidth, max((x+1)*width/outWidth, x*width/outWidth+1)
			var sum, count uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					gray := color.GrayModel.Convert(source.At(bounds.Min.X+sx, bounds.Min.Y+sy)).(color.Gray)
					sum += uint64(gray.Y)
					count++
				}
			}
			output.SetGray(x, y, color.Gray{Y: uint8(sum / count)})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, output, &jpeg.Options{Quality: illustrationQuality}); err != nil {
		return nil, 0, 0, err
	}
	return buffer.Bytes(), outWidth, outHeight, nil
}

func imageFor(key, alt *string, width, height *int) *StoryImage {
	if key == nil || *key == "" {
		return nil
	}
	item := &StoryImage{URL: "/api/v1/" + *key, Caption: illustrationCaption, Kind: "illustration"}
	if alt != nil {
		item.Alt = *alt
	}
	if width != nil && height != nil {
		item.Width, item.Height = *width, *height
	}
	return item
}

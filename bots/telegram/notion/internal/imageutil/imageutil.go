package imageutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"

	"golang.org/x/image/draw"
)

func DownloadToTempFile(ctx context.Context, srcURL string, maxBytes int64) (path string, size int64, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", 0, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", 0, "", fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.CreateTemp("", "tg-img-*")
	if err != nil {
		return "", 0, "", err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(f.Name())
		}
		_ = f.Close()
	}()

	limited := io.LimitReader(resp.Body, maxBytes)
	n, err := io.Copy(f, limited)
	if err != nil {
		return "", 0, "", err
	}
	if n >= maxBytes {
		return "", 0, "", errors.New("image too large to download")
	}

	_, _ = f.Seek(0, 0)
	head := make([]byte, 512)
	m, _ := io.ReadFull(f, head)
	contentType = http.DetectContentType(head[:m])

	return f.Name(), n, contentType, nil
}

func CompressToJPEGUnder(srcPath string, maxBytes int64, minQuality int) (outPath string, outSize int64, err error) {
	in, err := os.Open(srcPath)
	if err != nil {
		return "", 0, err
	}
	defer in.Close()

	img, _, err := image.Decode(in)
	if err != nil {
		return "", 0, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return "", 0, errors.New("invalid image dimensions")
	}

	quality := 85
	scale := 1.0

	for i := 0; i < 12; i++ {
		var cur image.Image = img
		if scale < 0.999 {
			nw := int(float64(width) * scale)
			nh := int(float64(height) * scale)
			if nw < 320 {
				nw = 320
			}
			if nh < 320 {
				nh = 320
			}
			dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
			draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
			cur = dst
		}

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, cur, &jpeg.Options{Quality: quality}); err != nil {
			return "", 0, fmt.Errorf("encode jpeg: %w", err)
		}
		if int64(buf.Len()) <= maxBytes {
			f, err := os.CreateTemp("", "tg-img-out-*.jpg")
			if err != nil {
				return "", 0, err
			}
			defer func() {
				_ = f.Close()
				if err != nil {
					_ = os.Remove(f.Name())
				}
			}()
			if _, err := io.Copy(f, bytes.NewReader(buf.Bytes())); err != nil {
				return "", 0, err
			}
			return f.Name(), int64(buf.Len()), nil
		}

		if quality > minQuality {
			quality -= 10
			continue
		}
		scale *= 0.85
		quality = 85
	}

	return "", 0, errors.New("unable to compress image under size limit")
}

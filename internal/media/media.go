package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"

	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
)

var (
	ErrTooLarge        = errors.New("ukuran file melebihi batas")
	ErrUnsupportedType = errors.New("format file harus JPG, PNG, atau WebP")
	ErrUnknownPreset   = errors.New("preset media tidak dikenal")
)

type Variant struct {
	Width  int    `json:"w"`
	Height int    `json:"h"`
	File   string `json:"file"`
}

type Service struct {
	dir string
	log *slog.Logger
}

func NewService(uploadDir string, log *slog.Logger) *Service {
	return &Service{dir: uploadDir, log: log}
}

func (s *Service) PublicDir() string  { return filepath.Join(s.dir, "public") }
func (s *Service) PrivateDir() string { return filepath.Join(s.dir, "private") }

func (s *Service) root(private bool) string {
	if private {
		return s.PrivateDir()
	}
	return s.PublicDir()
}

// Save validates, re-encodes, and stores every variant of an uploaded image, then records it in
// the media table through q. Files are written before the caller commits; if the transaction is
// rolled back they become orphans that the sweeper removes, so an old image is never lost.
func (s *Service) Save(ctx context.Context, q *sqlc.Queries, r io.Reader, presetName, alt string) (sqlc.Medium, error) {
	p, ok := Presets[presetName]
	if !ok {
		return sqlc.Medium{}, ErrUnknownPreset
	}
	data, err := io.ReadAll(io.LimitReader(r, p.MaxBytes+1))
	if err != nil {
		return sqlc.Medium{}, err
	}
	if int64(len(data)) > p.MaxBytes {
		return sqlc.Medium{}, fmt.Errorf("%w (maks %d MB)", ErrTooLarge, p.MaxBytes>>20)
	}
	img, err := decode(data)
	if err != nil {
		return sqlc.Medium{}, err
	}
	img = cropToRatio(img, p.CropW, p.CropH)

	id, err := randomID()
	if err != nil {
		return sqlc.Medium{}, err
	}
	key := id[0:2] + "/" + id[2:4] + "/" + id
	root := s.root(p.Private)
	if err := os.MkdirAll(filepath.Join(root, id[0:2], id[2:4]), 0o750); err != nil {
		return sqlc.Medium{}, err
	}

	var variants []Variant
	var total int64
	var written []string
	cleanup := func() {
		for _, f := range written {
			_ = os.Remove(f)
		}
	}
	srcW := img.Bounds().Dx()
	seen := map[int]bool{}
	for _, w := range p.Widths {
		if w > srcW {
			w = srcW
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		out := resize(img, w)
		file := fmt.Sprintf("%s-%d.%s", key, w, p.Format.Ext())
		path := filepath.Join(root, filepath.FromSlash(file))
		n, err := encodeTo(path, out, p)
		if err != nil {
			cleanup()
			return sqlc.Medium{}, err
		}
		written = append(written, path)
		total += n
		variants = append(variants, Variant{Width: out.Bounds().Dx(), Height: out.Bounds().Dy(), File: file})
	}
	vjson, _ := json.Marshal(variants)
	largest := variants[len(variants)-1]
	m, err := q.CreateMedia(ctx, sqlc.CreateMediaParams{
		Preset:     p.Name,
		StorageKey: key,
		Width:      int32(largest.Width),
		Height:     int32(largest.Height),
		Bytes:      total,
		Variants:   vjson,
		AltText:    strings.TrimSpace(alt),
		IsPrivate:  p.Private,
	})
	if err != nil {
		cleanup()
		return sqlc.Medium{}, err
	}
	return m, nil
}

// Delete removes the media row through q and returns the files to remove once the caller's
// transaction commits. Deleting files earlier would leave a broken image on rollback.
func (s *Service) Delete(ctx context.Context, q *sqlc.Queries, id int64) ([]string, error) {
	m, err := q.DeleteMedia(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.Files(m), nil
}

func (s *Service) Files(m sqlc.Medium) []string {
	root := s.root(m.IsPrivate)
	var files []string
	for _, v := range Variants(m) {
		files = append(files, filepath.Join(root, filepath.FromSlash(v.File)))
	}
	return files
}

// Purge deletes files returned by Delete. Call it only after the transaction has committed.
func (s *Service) Purge(files []string) {
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("media purge failed", "file", f, "err", err)
		}
	}
}

func Variants(m sqlc.Medium) []Variant {
	var v []Variant
	_ = json.Unmarshal(m.Variants, &v)
	return v
}

func decode(data []byte) (image.Image, error) {
	ct := http.DetectContentType(data)
	var (
		img image.Image
		err error
	)
	switch ct {
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
		if err == nil {
			img = applyOrientation(img, jpegOrientation(data))
		}
	case "image/png":
		img, err = png.Decode(bytes.NewReader(data))
	case "image/webp":
		img, err = webp.Decode(bytes.NewReader(data), webp.Options{AutoRotate: true})
	default:
		return nil, ErrUnsupportedType
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedType, err)
	}
	return img, nil
}

func cropToRatio(img image.Image, rw, rh int) image.Image {
	if rw == 0 || rh == 0 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	tw, th := w, w*rh/rw
	if th > h {
		th = h
		tw = h * rw / rh
	}
	x0 := b.Min.X + (w-tw)/2
	y0 := b.Min.Y + (h-th)/2
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.Draw(dst, dst.Bounds(), img, image.Point{X: x0, Y: y0}, draw.Src)
	return dst
}

func resize(img image.Image, width int) image.Image {
	b := img.Bounds()
	if width >= b.Dx() {
		dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	height := int(float64(b.Dy()) * float64(width) / float64(b.Dx()))
	if height < 1 {
		height = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

func encodeTo(path string, img image.Image, p Preset) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return 0, err
	}
	switch p.Format {
	case FormatJPEG:
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: p.Quality})
	case FormatPNG:
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(f, img)
	default:
		err = webp.Encode(f, img, webp.Options{Quality: p.Quality, Lossless: p.Lossless, Method: 4})
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
		return 0, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

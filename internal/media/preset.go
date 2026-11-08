package media

type Format string

const (
	FormatWebP Format = "webp"
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
)

type Preset struct {
	Name     string
	Widths   []int
	Format   Format
	Quality  int
	Lossless bool
	Private  bool
	MaxBytes int64
	// CropRatio forces a center crop to width:height before resizing (0 = keep the source ratio).
	CropW, CropH int
}

const (
	contentMaxBytes = 10 << 20
	proofMaxBytes   = 5 << 20
)

var Presets = map[string]Preset{
	"hero":          {Name: "hero", Widths: []int{640, 1024, 1600}, Format: FormatWebP, Quality: 75, MaxBytes: contentMaxBytes},
	"hero_mobile":   {Name: "hero_mobile", Widths: []int{480, 900}, Format: FormatWebP, Quality: 75, MaxBytes: contentMaxBytes},
	"page_cover":    {Name: "page_cover", Widths: []int{640, 1200}, Format: FormatWebP, Quality: 75, MaxBytes: contentMaxBytes},
	"logo":          {Name: "logo", Widths: []int{160, 320}, Format: FormatWebP, Quality: 90, MaxBytes: contentMaxBytes},
	"qris":          {Name: "qris", Widths: []int{720}, Format: FormatWebP, Lossless: true, MaxBytes: contentMaxBytes},
	"payment_proof": {Name: "payment_proof", Widths: []int{1280}, Format: FormatWebP, Quality: 70, Private: true, MaxBytes: proofMaxBytes},
	"og_image":      {Name: "og_image", Widths: []int{1200}, Format: FormatJPEG, Quality: 85, MaxBytes: contentMaxBytes, CropW: 1200, CropH: 630},
	"favicon":       {Name: "favicon", Widths: []int{32, 180, 192, 512}, Format: FormatPNG, MaxBytes: contentMaxBytes, CropW: 1, CropH: 1},
}

func (f Format) Ext() string {
	switch f {
	case FormatJPEG:
		return "jpg"
	case FormatPNG:
		return "png"
	default:
		return "webp"
	}
}

func (f Format) ContentType() string {
	switch f {
	case FormatJPEG:
		return "image/jpeg"
	case FormatPNG:
		return "image/png"
	default:
		return "image/webp"
	}
}

package settings

type Kind string

const (
	KindString Kind = "string"
	KindText   Kind = "text"
	KindURL    Kind = "url"
	KindBool   Kind = "bool"
	KindInt    Kind = "int"
	KindMedia  Kind = "media"
	KindZone   Kind = "zone"
)

type Group struct {
	Slug  string
	Label string
}

var Groups = []Group{
	{"identity", "Identitas"},
	{"contact", "Kontak"},
	{"social", "Media sosial"},
	{"payment", "Pembayaran"},
	{"session-rules", "Aturan sesi"},
	{"broadcast", "Broadcast"},
	{"seo", "SEO"},
	{"display", "Tampilan"},
}

type Def struct {
	Key     string
	Group   string
	Label   string
	Help    string
	Kind    Kind
	Default any
	Preset  string
	Min     int64
	Max     int64
}

const (
	AppName            = "app_name"
	SiteURL            = "site_url"
	Tagline            = "tagline"
	LogoLightMediaID   = "logo_light_media_id"
	LogoDarkMediaID    = "logo_dark_media_id"
	FaviconMediaID     = "favicon_media_id"
	ContactWhatsApp    = "contact_whatsapp"
	ContactEmail       = "contact_email"
	ContactAddress     = "contact_address"
	ContactMapsURL     = "contact_maps_url"
	SocialTikTok       = "social_tiktok"
	SocialInstagram    = "social_instagram"
	SocialAyo          = "social_ayo"
	SocialYouTube      = "social_youtube"
	SocialFacebook     = "social_facebook"
	SocialX            = "social_x"
	BankName           = "bank_name"
	BankAccountNumber  = "bank_account_number"
	BankAccountName    = "bank_account_name"
	QRISMediaID        = "qris_media_id"
	UniqueCodeDefault  = "unique_code_default"
	PaymentDeadlineHrs = "payment_deadline_hours"
	RefundDeadlineDays = "refund_deadline_days"
	CapacityPerCourt   = "capacity_per_court"
	PointRule          = "point_rule"
	TimeZone           = "time_zone"
	AutoCloseHours     = "auto_close_hours"
	ProofRetentionMos  = "proof_retention_months"
	BroadcastTemplate  = "broadcast_template"
	SEOTitlePattern    = "seo_title_pattern"
	SEODescription     = "seo_description"
	SEOKeywords        = "seo_keywords"
	SEOOGMediaID       = "seo_og_media_id"
	SEOGoogleVerify    = "seo_google_verification"
	SEOAllowIndexing   = "seo_allow_indexing"
	LeaderboardPublic  = "leaderboard_public"
	HomeUpcomingCount  = "home_upcoming_count"
)

const DefaultBroadcastTemplate = `🏸 *MABAR BADMINTON {app_name}* 🏸

📅 {date}
⏰ {time_range}
📍 {venue}
{address}
🗺️ {maps_url}

🎾 Lapangan: {court_count}
👥 Kuota: {capacity} orang
💵 HTM: {price}
🎯 Poin: {point_rule}
📶 Level: {level}
🪶 Shuttlecock: {shuttlecock}

*Daftar peserta* ({registered_count}/{capacity}):
{participants}
{waitlist}
💰 = sudah lunas

*Pembayaran*
{payment_info}

*Catatan*
{notes}

Daftar dan bayar lewat link:
{session_url}

{social}`

var Registry = []Def{
	{Key: AppName, Group: "identity", Label: "Nama aplikasi", Kind: KindString, Default: "MabarBadmin"},
	{Key: SiteURL, Group: "identity", Label: "URL situs", Kind: KindURL, Default: "https://mabarbadmin.web.id", Help: "Dipakai di teks broadcast. Canonical URL tetap dari BASE_URL server."},
	{Key: Tagline, Group: "identity", Label: "Slogan", Kind: KindString, Default: "Main bareng, bayar gampang, rekap otomatis."},
	{Key: LogoLightMediaID, Group: "identity", Label: "Logo (latar terang)", Kind: KindMedia, Preset: "logo"},
	{Key: LogoDarkMediaID, Group: "identity", Label: "Logo (latar gelap)", Kind: KindMedia, Preset: "logo"},
	{Key: FaviconMediaID, Group: "identity", Label: "Favicon", Kind: KindMedia, Preset: "favicon", Help: "Gambar persegi minimal 512 x 512 piksel."},

	{Key: ContactWhatsApp, Group: "contact", Label: "No WA admin", Kind: KindString, Help: "Format 0812-xxxx-xxxx atau 628xxx."},
	{Key: ContactEmail, Group: "contact", Label: "Email", Kind: KindString},
	{Key: ContactAddress, Group: "contact", Label: "Alamat", Kind: KindText},
	{Key: ContactMapsURL, Group: "contact", Label: "Link peta", Kind: KindURL},

	{Key: SocialTikTok, Group: "social", Label: "TikTok", Kind: KindURL},
	{Key: SocialInstagram, Group: "social", Label: "Instagram", Kind: KindURL},
	{Key: SocialAyo, Group: "social", Label: "Ayo", Kind: KindURL},
	{Key: SocialYouTube, Group: "social", Label: "YouTube", Kind: KindURL},
	{Key: SocialFacebook, Group: "social", Label: "Facebook", Kind: KindURL},
	{Key: SocialX, Group: "social", Label: "X", Kind: KindURL},

	{Key: BankName, Group: "payment", Label: "Nama bank", Kind: KindString},
	{Key: BankAccountNumber, Group: "payment", Label: "No rekening", Kind: KindString},
	{Key: BankAccountName, Group: "payment", Label: "Atas nama", Kind: KindString},
	{Key: QRISMediaID, Group: "payment", Label: "Gambar QRIS statis", Kind: KindMedia, Preset: "qris"},
	{Key: UniqueCodeDefault, Group: "payment", Label: "Kode unik aktif secara default", Kind: KindBool, Default: true},

	{Key: PaymentDeadlineHrs, Group: "session-rules", Label: "Batas bayar (jam sebelum mulai)", Kind: KindInt, Default: int64(0), Min: 0, Max: 720, Help: "0 = fleksibel sampai sesi dimulai."},
	{Key: RefundDeadlineDays, Group: "session-rules", Label: "Batas refund (H-n pukul 00.00)", Kind: KindInt, Default: int64(1), Min: 0, Max: 30, Help: "1 = H-1 pukul 00.00."},
	{Key: CapacityPerCourt, Group: "session-rules", Label: "Kuota per lapangan", Kind: KindInt, Default: int64(4), Min: 2, Max: 20, Help: "Sistem gantian, default 4 orang."},
	{Key: PointRule, Group: "session-rules", Label: "Aturan poin (info sesi)", Kind: KindString, Default: "15x2"},
	{Key: TimeZone, Group: "session-rules", Label: "Zona waktu", Kind: KindZone, Default: "Asia/Jakarta"},
	{Key: AutoCloseHours, Group: "session-rules", Label: "Tutup otomatis (jam setelah selesai)", Kind: KindInt, Default: int64(24), Min: 1, Max: 720},
	{Key: ProofRetentionMos, Group: "session-rules", Label: "Retensi bukti bayar (bulan)", Kind: KindInt, Default: int64(12), Min: 1, Max: 120},

	{Key: BroadcastTemplate, Group: "broadcast", Label: "Template teks broadcast WA", Kind: KindText, Default: DefaultBroadcastTemplate,
		Help: "Placeholder: {app_name} {title} {date} {time_range} {venue} {address} {maps_url} {court_count} {capacity} {registered_count} {price} {point_rule} {level} {shuttlecock} {participants} {waitlist} {payment_info} {notes} {session_url} {social}"},

	{Key: SEOTitlePattern, Group: "seo", Label: "Pola judul", Kind: KindString, Default: "{title} | {site}", Help: "Placeholder: {title} dan {site}."},
	{Key: SEODescription, Group: "seo", Label: "Deskripsi default", Kind: KindText, Default: "Jadwal mabar badminton, daftar dan bayar lewat satu link, rekap peserta otomatis."},
	{Key: SEOKeywords, Group: "seo", Label: "Keyword default", Kind: KindString, Default: "mabar badminton, main bareng badminton, jadwal badminton"},
	{Key: SEOOGMediaID, Group: "seo", Label: "Gambar OG default", Kind: KindMedia, Preset: "og_image"},
	{Key: SEOGoogleVerify, Group: "seo", Label: "Kode verifikasi Google Search Console", Kind: KindString},
	{Key: SEOAllowIndexing, Group: "seo", Label: "Izinkan mesin pencari mengindeks situs", Kind: KindBool, Default: true},

	{Key: LeaderboardPublic, Group: "display", Label: "Tampilkan peringkat publik", Kind: KindBool, Default: true},
	{Key: HomeUpcomingCount, Group: "display", Label: "Jumlah jadwal di beranda", Kind: KindInt, Default: int64(6), Min: 1, Max: 30},
}

func Lookup(key string) (Def, bool) {
	for _, d := range Registry {
		if d.Key == key {
			return d, true
		}
	}
	return Def{}, false
}

func GroupDefs(group string) []Def {
	var out []Def
	for _, d := range Registry {
		if d.Group == group {
			out = append(out, d)
		}
	}
	return out
}

func GroupLabel(slug string) (string, bool) {
	for _, g := range Groups {
		if g.Slug == slug {
			return g.Label, true
		}
	}
	return "", false
}

package content

import (
	"context"

	"github.com/suwahadi/mabarbadmin/internal/database"
	"github.com/suwahadi/mabarbadmin/internal/database/sqlc"
)

type DefaultPage struct {
	Slug, Title, Body string
}

var DefaultPages = []DefaultPage{
	{"about", "Tentang Kami", "Kami komunitas mabar (main bareng) badminton yang terbuka untuk semua level.\n\nSetiap sesi diumumkan di grup WhatsApp dan halaman jadwal. Daftar, bayar, dan cek status slot cukup lewat satu link."},
	{"contact", "Kontak", "Ada pertanyaan soal jadwal, pembayaran, atau refund? Hubungi admin lewat WhatsApp."},
	{"terms", "Syarat dan Ketentuan", "1. Satu nomor WA hanya bisa mendaftar sekali per sesi.\n2. Pembayaran diterima sampai sesi dimulai. Saat kuota penuh, peserta yang sudah lunas diprioritaskan.\n3. Pembatalan sebelum batas refund (default H-1 pukul 00.00) mendapat refund penuh. Setelah itu pembayaran hangus, tetapi slot boleh dioper ke orang lain.\n4. Peserta waiting list yang sudah lunas tetapi tidak mendapat slot akan di-refund.\n5. Aturan poin dan level di halaman sesi bersifat informasi."},
	{"privacy", "Kebijakan Privasi", "Kami menyimpan nama, nomor WhatsApp, level, dan bukti bayar untuk keperluan pendaftaran dan pembukuan.\n\n- Nomor WA disamarkan di halaman publik.\n- Bukti bayar dan rekening refund hanya bisa dilihat admin dan pemiliknya.\n- Bukti bayar dihapus otomatis setelah masa retensi, status pembayaran tetap tersimpan."},
	{"faq", "FAQ", "Pertanyaan yang sering ditanyakan seputar mabar."},
}

var DefaultFAQ = [][2]string{
	{"Bagaimana cara ikut mabar?", "Buka link sesi dari grup WA, klik **Ikut Mabar**, lalu bayar dan unggah bukti bayar."},
	{"Kapan batas bayar?", "Pembayaran diterima sampai sesi dimulai. Jika kuota penuh, peserta waiting list yang sudah lunas otomatis menggantikan peserta yang belum lunas."},
	{"Bagaimana jika saya batal?", "Batal sebelum batas refund (H-1 pukul 00.00) mendapat refund penuh. Setelahnya pembayaran hangus, tapi kamu bisa **oper slot** ke teman."},
	{"Apa itu kode unik?", "Tambahan Rp 1 sampai Rp 499 pada nominal bayar agar admin mudah mencocokkan transferanmu."},
}

// IsDefaultPage reports whether slug belongs to a built-in page. These pages back fixed links and
// features (the FAQ accordion is tied to /faq), so they cannot be deleted or renamed.
func IsDefaultPage(slug string) bool {
	for _, p := range DefaultPages {
		if p.Slug == slug {
			return true
		}
	}
	return false
}

// SeedPages inserts the built-in pages that do not exist yet; existing pages are never overwritten.
func SeedPages(ctx context.Context, q *sqlc.Queries) error {
	for i, p := range DefaultPages {
		_, err := q.GetPageBySlug(ctx, p.Slug)
		if err == nil {
			continue
		}
		if !database.IsNotFound(err) {
			return err
		}
		if err := q.InsertPageIfMissing(ctx, sqlc.InsertPageIfMissingParams{Slug: p.Slug, Title: p.Title, BodyMarkdown: p.Body, SortOrder: int32(i + 1)}); err != nil {
			return err
		}
		if p.Slug == "faq" {
			for j, f := range DefaultFAQ {
				if _, err := q.CreateFAQ(ctx, sqlc.CreateFAQParams{Question: f[0], AnswerMarkdown: f[1], SortOrder: int32(j + 1), IsPublished: true}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

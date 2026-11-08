# AGENTS.md

Panduan untuk semua kontributor, manusia maupun AI coding tool, yang bekerja di repository MabarBadmin. Spesifikasi produk lengkap ada di [PRD.md](PRD.md). Jika dokumen ini dan PRD berbeda, PRD yang berlaku.

## Ringkasan proyek

Aplikasi web untuk satu komunitas mabar badminton: sesi, pendaftaran member dan tamu, pembayaran manual dengan bukti, waiting list dengan prioritas lunas, cancel dan refund, oper slot, absensi, rotasi lapangan, keuangan, dan konten publik (beranda, halaman statis, FAQ, hero slider, pengumuman).

- Domain: `mabarbadmin.web.id`
- Satu binary Go (SSR) di depan PostgreSQL, dideploy ke satu VPS dengan systemd dan Caddy.

## Stack

| Lapisan | Pilihan |
| --- | --- |
| Bahasa | Go, versi di-pin di `go.mod` (`go` dan `toolchain`) |
| Router | `net/http` ServeMux bawaan (pola method + path) |
| Template | `html/template` + HTMX + Alpine.js (`@alpinejs/csp`) |
| Database | PostgreSQL + `pgx/v5` |
| Query | `sqlc` (hasil generate di `internal/database/sqlc`, jangan diedit) |
| Migrasi | `goose`, file SQL di-embed |
| Password | argon2id (`golang.org/x/crypto`) |
| CSS dan JS | Tailwind CSS v4 CLI + Bun, hanya di lokal atau CI |
| Gambar | `golang.org/x/image/draw` + `github.com/gen2brain/webp` (tetap `CGO_ENABLED=0`) |
| Markdown | `goldmark` + `bluemonday` |

## Struktur proyek

```text
cmd/mabar/main.go          # subcommand: serve, migrate, seed, create-admin
internal/
  config/                  # env: DATABASE_URL, LISTEN_ADDR, UPLOAD_DIR, BASE_URL, SESSION_SECRET
  database/                # pool, migrasi (embed di database/migrations), queries/*.sql, sqlc/ (generate)
  domain/                  # aturan slot, waiting list, refund, rotasi (murni, tanpa HTTP dan DB)
  service/                 # transaksi DB yang menjalankan aturan domain (row lock, log aktivitas)
  auth/                    # argon2id, token sesi dan tiket
  handler/                 # HTTP handler publik, member, admin
  middleware/              # auth, csrf, logging, recover, security headers
  locale/                  # FormatRupiah, FormatDateTime, nama hari dan bulan
  scheduler/               # tutup sesi, penyapu media, retensi bukti bayar
  storage/                 # sajikan bukti bayar privat
  broadcast/               # generator teks WA
  media/                   # media.Save, varian WebP, hapus file, penyapu yatim
  settings/                # registry kunci, default, cache
  seo/                     # meta, OG, JSON-LD, sitemap, robots
  content/                 # halaman statis, FAQ, hero, pengumuman
web/
  templates/               # layout, pages, partials
  static/                  # hasil build (tidak di-commit kecuali icons.svg)
  embed.go
frontend/                  # sumber Tailwind dan JS, dibuild Bun
deploy/                    # contoh systemd, Caddyfile, skrip backup
.githooks/commit-msg
scripts/check-commit-messages.sh
Makefile
```

## Perintah umum

```sh
make dev         # jalankan server lokal
make test        # go test ./...
make lint        # gofmt -l, go vet, staticcheck
make sqlc        # sqlc generate
make assets      # build CSS dan JS lewat Bun
make release     # build lengkap ke dist/mabar
```

Binary: `mabar serve`, `mabar migrate up|down|status`, `mabar seed [--demo]`, `mabar create-admin`.

Integration test butuh PostgreSQL: `TEST_DATABASE_URL=postgres://... go test -p 1 ./...`. Test tersebut menghapus dan membuat ulang skema `public`, jadi selalu pakai database khusus test.

## Catatan implementasi

- Build Go selalu dengan tag `nodynamic` (sudah di `Makefile`). Tanpa tag ini `gen2brain/webp` memuat libwebp sistem lewat purego sehingga binary tidak statis.
- Aset hasil build (`web/static/css`, `web/static/js`, `manifest.json`) tidak di-commit. Jalankan `make assets` sebelum `go build`; tanpa itu halaman tampil tanpa gaya.
- Alpine memakai build CSP. Di build ini `this` pada method yang dipanggil lewat `@click` tidak selalu reaktif dan ekspresi seperti `!flag` tidak didukung. Komponen Alpine hanya untuk state sederhana (`toggle`, `slider`); interaksi yang cukup mengubah DOM (salin teks, tutup pengumuman, konfirmasi form lewat `data-confirm`) memakai listener biasa di `frontend/app.js`.
- Jangan ada skrip atau handler inline (`onclick`, `onchange`) di template; CSP memblokirnya. Gunakan atribut `data-*` dan listener di `app.js`.
- Atribut `imagesrcset` di-escape `html/template` sebagai satu URL, jadi tag preload LCP dirakit di Go (`preloadTags`).

## Aturan arsitektur

- Logika bisnis hanya di `internal/domain`, diuji dengan unit test tanpa HTTP (cakupan minimal 80%). Handler hanya memanggil domain dan merender template.
- Setiap perpindahan slot berjalan dalam satu transaksi dengan row lock pada baris sesi (`SELECT ... FOR UPDATE`).
- Setiap perubahan status dicatat di `activity_logs` beserta pelakunya.
- Status peserta (`registrations.status`) dan status pembayaran (`registrations.payment_status`) selalu terpisah.
- Uang disimpan sebagai `bigint` rupiah. Waktu sebagai `timestamptz` (UTC); konversi zona waktu hanya di layer tampilan.
- Parameter yang bisa berubah diatur lewat tabel `settings` dan dibaca lewat `settings.Get`. Environment variable hanya untuk infrastruktur dan rahasia.
- Semua upload gambar wajib lewat `media.Save(ctx, file, preset)`. Handler tidak boleh menulis file ke disk secara langsung. File lama dihapus setelah commit transaksi, bukan sebelumnya.
- Semua route mengembalikan HTML. Request HTMX (`HX-Request`) mendapat partial, request biasa mendapat halaman penuh.
- Route `/{slug}` untuk halaman statis didaftarkan paling akhir. Daftar slug terlarang dipegang di satu konstanta Go.
- Tanpa skrip pihak ketiga di halaman publik. CSP ketat (`script-src 'self'`).

## Bahasa dan penamaan

- Teks yang dilihat pengguna: Bahasa Indonesia, dikumpulkan di template.
- Kode, URL, database, enum, API, dan pesan commit: Bahasa Inggris.
- URL dan slug kebab-case, query params dan form field snake_case, tabel jamak snake_case, enum snake_case.
- Format tampilan: `Rp 85.000`, `Senin, 9 November 2026 21:00 WIB`, no WA `0812-3456-7890` (disimpan `628...`).

## Gaya kode dan komentar

- Kode harus jelas dari nama, tipe, dan struktur.
- Dilarang komentar yang mengulang isi kode, pembatas section dekoratif, banner file, dan TODO tanpa referensi issue (format `TODO(#123)`).
- Komentar hanya untuk alasan yang tidak terlihat dari kode: konkurensi, batasan keamanan, keanehan provider, invariant, workaround.
- Pertahankan directive tool (`//go:embed`, `//go:build`, `//nolint` beralasan), notice lisensi (ikon Lucide), dan penanda kode hasil generate.
- Jangan menghapus komentar dengan regex global.
- `gofmt`, `go vet`, dan `staticcheck` wajib lolos.

## Git policy

- Branch kerja: `main` saja. Jangan membuat branch baru tanpa izin.
- **Dilarang** menambahkan trailer co-author atau atribusi AI pada commit, termasuk `Co-authored-by` yang menyebut Claude, Anthropic, ChatGPT, OpenAI, Copilot, Gemini, Cursor, atau AI tool lain, serta footer seperti "Generated with ...". Ini berlaku juga untuk deskripsi pull request.
- Jika coding tool menambahkan atribusi secara default, matikan di konfigurasinya (Claude Code: `"includeCoAuthoredBy": false` di `.claude/settings.json`).
- Aktifkan hook dengan `git config core.hooksPath .githooks`.
- Gunakan identitas Git milik pemilik repository. Periksa `git config user.name` dan `git config user.email` sebelum commit. Jangan mengubah konfigurasi Git global.
- Pesan commit singkat, deskriptif, berbahasa Inggris, misal `Add waitlist promotion after payment confirmation`.
- Periksa `git diff --staged` sebelum setiap commit.
- **Minta izin pemilik repository sebelum commit dan push.** Jangan amend, rebase, force-push, menulis ulang history, membuat release, atau menyentuh branch terlindungi tanpa izin eksplisit.
- Jangan pernah mengklaim commit atau push berhasil kecuali memang berhasil.

## Keputusan implementasi (hasil tanya jawab, 8 Nov 2026)

Keputusan ini melengkapi PRD untuk hal yang belum diatur secara eksplisit.

| Topik | Keputusan |
| --- | --- |
| Versi Go | Go 1.25.x, di-pin di `go.mod` (`go` dan `toolchain`), agar `http.CrossOriginProtection` tersedia |
| CI | Belum ada untuk saat ini. Pemeriksaan lewat `Makefile` dan git hook lokal. `scripts/check-commit-messages.sh` tetap disediakan |
| Batas refund | Awal hari H-1 di zona waktu sesi. Sesi Senin 9 Nov 21:00 WIB berarti batas refund Minggu 8 Nov 00:00 WIB |
| Cancel waiting list | Peserta waiting list yang sudah lunas selalu mendapat refund kapan pun cancel, karena tidak pernah memegang slot |
| Sesi dibatalkan admin | Semua pembayaran `paid` (terdaftar maupun waiting list) otomatis menjadi `refund_pending` |
| Oper slot | Sistem membuat link tiket baru untuk pengganti dan menampilkannya ke peserta lama untuk dikirim lewat WA. Jika no WA pengganti adalah member, slot juga muncul di akunnya. No WA yang sudah terdaftar di sesi yang sama ditolak |
| Tutup sesi | Manual oleh admin setelah mengisi biaya. Scheduler menutup otomatis sesi yang belum ditutup 24 jam setelah `ends_at` |
| Kode unik | Acak 1 sampai 499, unik per sesi, ditetapkan saat mendaftar sehingga `amount_due` tetap |
| Rotasi | Lapangan diisi 4 pemain per lapangan sesuai prioritas. Sisa pemain duduk dan mendapat prioritas di ronde berikutnya. Lapangan dibiarkan kosong bila pemain kurang dari 4 |
| `create-admin` | Prompt interaktif (nama, no WA, password tersembunyi). Tersedia flag `--name`, `--phone`, dan password dari stdin untuk skrip |
| Teks broadcast | Format default disusun sesuai PRD (header, detail sesi, list bernomor dengan emoji uang untuk yang lunas, info bayar, aturan, sosial media, link sesi), bisa diubah admin di Pengaturan > Broadcast |
| Kode sqlc | Hasil generate di-commit |
| Data demo | `mabar seed --demo` mengisi venue, template, sesi, dan peserta contoh. `mabar seed` biasa hanya mengisi pengaturan dan halaman bawaan |
| Verifikasi | Unit test domain, integration test dengan PostgreSQL lokal, menjalankan server, dan pengecekan alur utama di browser headless termasuk tampilan mobile |
| Identitas Git | `suwahadi <suwahadi@users.noreply.github.com>`, diset di konfigurasi lokal repository |

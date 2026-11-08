# PRD Aplikasi Mabar Badminton

Nov 8, 2026 · @Ambon

## 1. Ringkasan produk

Aplikasi web untuk satu komunitas mabar badminton yang menggantikan absensi, pembayaran, dan rekap manual di grup WhatsApp. Admin membuat sesi sekali, member mendaftar dan bayar lewat satu link, dan sistem menjalankan aturan H-2, waiting list, refund, rotasi lapangan, serta laporan keuangan.

### Masalah saat ini

- Admin mengetik ulang template broadcast (hari, lokasi, jam, lapangan, HTM, poin, level, map, rekening) untuk setiap sesi.
- Member menulis nama sendiri di list grup atau DM admin, sehingga list rawan slot kosong, nama ganda, dan edit saling menimpa.
- Bukti transfer dikirim lewat chat pribadi, lalu admin menandai emoji uang secara manual. Pembayaran mudah terlewat dan tidak ada rekap.
- Aturan bayar maksimal H-2, prioritas yang sudah bayar, dan cancel hari H hangus ditegakkan manual dan tidak konsisten.
- Tidak ada data siapa yang paling sering ikut, siapa yang sering no-show, dan berapa untung atau rugi per sesi.

### Tujuan

1. Waktu admin menyiapkan satu sesi turun dari belasan menit menjadi di bawah 2 menit.
2. Seluruh pembayaran tercatat dengan status jelas, tanpa rekap manual.
3. Aturan prioritas lunas, waiting list, dan cancel dijalankan sistem secara konsisten.
4. Admin punya data kehadiran, peringkat member, dan keuangan per sesi dan per bulan.

### Metrik sukses (3 bulan setelah rilis)

| Metrik | Target |
| --- | --- |
| Sesi yang dibuat lewat aplikasi | 100% |
| Pendaftaran lewat link (bukan ketik di grup) | 90% atau lebih |
| Peserta sudah lunas saat sesi dimulai | 95% atau lebih |
| Waktu admin membuat sesi dari template | kurang dari 2 menit |
| Selisih kas vs laporan aplikasi per bulan | Rp 0 |

## 2. Lingkup dan peran

MVP mencakup satu komunitas dengan satu admin. WA gateway, OTP, dan payment gateway masuk fase 2.

### Aktor

| Peran | Cara masuk | Hak utama |
| --- | --- | --- |
| Admin | No WA + password, akun dibuat lewat CLI seed | Kelola sesi, venue, template, konfirmasi bayar, refund, absensi, rotasi, laporan, member |
| Member | Daftar sendiri, no WA + password | Join sesi, upload bukti bayar, cancel, oper slot, lihat riwayat dan peringkat |
| Tamu | Tanpa akun, isi nama + no WA + level | Join sesi, upload bukti bayar lewat link tiket, cancel dan oper slot lewat link tiket |

Tamu mendapat **link tiket** unik (token acak) setelah join. Link ini satu-satunya akses tamu ke slotnya, jadi ditampilkan jelas dan bisa disalin. Jika no WA tamu sudah terdaftar sebagai member, sistem meminta tamu login.

### Masuk MVP

- Venue tersimpan (nama, alamat, link map) dan template sesi berulang.
- Pembuatan sesi, halaman sesi publik, dan teks broadcast WA siap salin.
- Pendaftaran member dan tamu, kuota, waiting list.
- Pembayaran QRIS statis dan transfer bank dengan upload bukti, kode unik opsional, konfirmasi admin.
- Prioritas lunas otomatis, cancel, refund manual, oper slot.
- Absensi hadir dan no-show, rotasi lapangan berdasarkan level.
- Dashboard admin: statistik, peringkat member, laporan keuangan.
- Beranda dengan bar pengumuman, hero slider, dan event terdekat; halaman statis dan FAQ; pengaturan global; helper media WebP; SEO dasar.

### Fase 2

- Kirim broadcast dan reminder otomatis lewat WA gateway (Fonnte, Wablas, atau sejenis).
- Login dan reset password lewat WA OTP.
- QRIS dinamis atau payment gateway dengan verifikasi otomatis.
- Saldo atau kredit member.

### Di luar lingkup

- Multi-komunitas atau multi-admin.
- Aplikasi mobile native (cukup web responsif, bisa dipasang sebagai PWA di fase 2).
- Pemesanan lapangan langsung ke venue.

## 3. Skenario pengguna

Delapan skenario berikut menggantikan seluruh alur manual di grup WA, dari pembuatan jadwal sampai rekap keuangan.

### S1. Admin membuat sesi

1. Admin memilih template (misal "Senin malam GBA") atau membuat sesi baru.
2. Form terisi otomatis: venue, jam, jumlah lapangan, kuota, HTM, aturan poin, level, shuttlecock, catatan aturan. Admin cukup memilih tanggal.
3. Admin menyimpan sebagai draf atau langsung publikasi.
4. Sistem membuat halaman sesi publik di `/sessions/{slug}` dan teks broadcast WA siap salin dengan tombol Salin.

Kuota default = jumlah lapangan x 4 (4 orang per lapangan, sistem gantian), bisa diubah per sesi. Teks broadcast mengikuti format grup lama: header, detail sesi, list peserta bernomor dengan tanda bayar, info pembayaran, catatan aturan, dan link sosial media. List di teks selalu diambil dari data terkini, jadi admin bisa menyalin ulang kapan saja.

### S2. Member atau tamu mendaftar

1. Pengguna membuka link sesi dari grup WA.
2. Member yang sudah login klik **Ikut Mabar**. Tamu mengisi nama, no WA, dan level.
3. Jika kuota masih ada, status menjadi Terdaftar. Jika penuh, status menjadi Waiting list.
4. Sistem menampilkan instruksi bayar: nominal (dengan kode unik bila aktif), QRIS statis, dan rekening bank.

Satu no WA hanya bisa mendaftar sekali per sesi.

### S3. Pembayaran

1. Pengguna membayar lewat QRIS statis atau transfer bank.
2. Pengguna upload bukti (JPG, PNG, atau WebP, maks 5 MB) dan memilih metode.
3. Status pembayaran menjadi Menunggu konfirmasi.
4. Admin membuka antrean konfirmasi, mencocokkan nominal, lalu klik Konfirmasi atau Tolak dengan alasan.
5. Setelah dikonfirmasi, tanda lunas muncul di halaman sesi dan di teks broadcast.

Kode unik (opsional per sesi) menambah 1 sampai 499 rupiah ke HTM, misal Rp 60.017, agar admin mudah mencocokkan dengan mutasi rekening.

### S4. Prioritas lunas (batas bayar fleksibel)

1. Tidak ada batas bayar tetap. Pembayaran diterima sampai sesi dimulai di hari H.
2. Selama kuota belum penuh, peserta yang belum lunas tetap memegang slot.
3. Saat kuota penuh dan ada peserta waiting list yang lunas, sistem langsung menukar: peserta waiting list naik ke slot, dan peserta Terdaftar yang belum lunas dengan waktu daftar paling akhir turun ke waiting list.
4. Pertukaran berulang sampai tidak ada lagi peserta waiting list yang lunas atau tidak ada lagi peserta Terdaftar yang belum lunas.

Pemicu pertukaran adalah konfirmasi bayar oleh admin, bukan scheduler. Peserta yang tergeser langsung melihat statusnya berubah di halaman sesi atau tiket. Admin tetap bisa mengisi batas bayar per sesi bila suatu saat dibutuhkan; jika kosong, aturan fleksibel di atas yang berlaku.

### S5. Cancel dan refund

1. Peserta klik Batalkan.
2. Jika cancel sebelum batas refund (default H-1 pukul 00.00 WIB) dan sudah lunas, sistem meminta rekening tujuan refund lalu membuat antrean refund.
3. Jika cancel setelah batas refund, pembayaran hangus. Sistem menyarankan oper slot sebagai alternatif.
4. Slot yang kosong diisi otomatis oleh waiting list teratas yang sudah lunas, lalu yang belum lunas.

### S6. Oper slot

1. Peserta klik Oper Slot lalu mengisi nama dan no WA pengganti.
2. Slot langsung berpindah beserta status pembayarannya. Peserta lama keluar dengan status Dioper.
3. Urusan uang antara keduanya di luar aplikasi.

Oper slot tersedia kapan saja sampai sesi dimulai, tanpa persetujuan admin.

### S7. Refund waiting list

Saat sesi selesai atau ditutup, peserta waiting list yang sudah lunas tetapi tidak mendapat slot otomatis masuk antrean refund. Admin transfer di luar aplikasi, lalu menandai Sudah di-refund.

### S8. Hari H

1. Admin membuka mode Hari H: daftar peserta final, checklist hadir, dan tombol tandai no-show.
2. Admin membuat rotasi lapangan. Sistem membagi pemain per lapangan dengan pasangan seimbang menurut level.
3. Setelah ronde selesai, admin klik Ronde Berikutnya. Sistem memprioritaskan pemain yang paling lama duduk.
4. Setelah sesi, admin mengisi biaya sewa lapangan dan shuttlecock, lalu menutup sesi.

## 4. Aturan bisnis dan status

Status peserta dan status pembayaran disimpan terpisah, karena peserta waiting list boleh sudah lunas dan peserta terdaftar boleh belum lunas.

```text
registered <-> waitlisted   (kuota, prioritas lunas, slot kosong)
registered -> cancelled | transferred | attended | no_show
waitlisted -> cancelled | transferred
```

Hanya Terdaftar yang memegang slot. Perpindahan antara Terdaftar dan Waiting list terjadi lewat kuota, prioritas lunas, atau slot yang kosong.

### Status peserta (`registration.status`)

| Kode | Label UI | Arti |
| --- | --- | --- |
| `registered` | Terdaftar | Memegang slot |
| `waitlisted` | Waiting list | Antre, tanpa slot |
| `cancelled` | Batal | Keluar atas kemauan sendiri |
| `transferred` | Dioper | Slot diberikan ke orang lain |
| `attended` | Hadir | Ditandai hadir di hari H |
| `no_show` | Tidak datang | Terdaftar tetapi tidak hadir |

### Status pembayaran (`payment.status`)

| Kode | Label UI | Arti |
| --- | --- | --- |
| `unpaid` | Belum bayar | Belum ada bukti |
| `pending` | Menunggu konfirmasi | Bukti sudah diunggah |
| `paid` | Lunas | Dikonfirmasi admin |
| `rejected` | Ditolak | Bukti tidak valid, bisa upload ulang |
| `refund_pending` | Menunggu refund | Berhak refund, belum ditransfer |
| `refunded` | Sudah di-refund | Admin sudah transfer |
| `forfeited` | Hangus | Cancel setelah batas refund |

### Aturan

1. **Kuota.** Jumlah peserta `registered` tidak boleh melebihi kuota. Admin bisa menambah kuota kapan saja, dan slot baru diisi dari waiting list.
2. **Urutan waiting list.** Yang sudah lunas di depan, diurutkan waktu konfirmasi bayar. Yang belum lunas di belakang, diurutkan waktu daftar.
3. **Batas bayar.** Fleksibel sampai hari H. Saat kuota penuh, peserta waiting list yang lunas langsung menggantikan peserta Terdaftar yang belum lunas dan paling akhir mendaftar (S4).
4. **Batas refund.** Default H-1 pukul 00.00 WIB. Cancel sebelum batas = refund penuh. Cancel setelahnya = hangus.
5. **Oper slot.** Bebas sampai jam mulai sesi. Status pembayaran ikut berpindah.
6. **Satu no WA per sesi.** Dicek dengan unique index.
7. **Tutup sesi.** Saat sesi ditutup, waiting list yang lunas otomatis masuk `refund_pending`, peserta `registered` yang tidak ditandai menjadi `no_show`.
8. **Konsistensi.** Setiap perpindahan slot dijalankan dalam satu transaksi database dengan row lock pada sesi, agar dua pendaftar tidak merebut slot terakhir bersamaan.
9. **Audit.** Setiap perubahan status dicatat di `activity_logs` beserta pelaku (admin, member, tamu, atau sistem).

## 5. Kebutuhan fungsional

Sepuluh modul membentuk MVP: frontend publik, akun member, dashboard admin, manajemen sesi, hari H, keuangan, pengaturan global, halaman statis, hero slider, dan pengumuman.

### 5.1 Frontend publik

- **Bar pengumuman.** Satu baris teks singkat di atas header, dengan link opsional dan tombol tutup (5.10).
- **Hero slider.** Slide gambar dengan judul, subjudul, dan tombol aksi yang diatur admin (5.9).
- **Event Terdekat.** Kartu tepat di bawah hero: tanggal, jam, venue, sisa slot (misal 8 dari 20), HTM, level, dan tombol Ikut Mabar. Di bawahnya daftar jadwal mendatang.
- **Halaman sesi.** Detail lengkap, tombol peta, daftar peserta bernomor dengan badge Lunas, waiting list, instruksi bayar, dan aturan sesi.
- **Halaman tiket.** Untuk tamu: status slot, upload bukti, cancel, oper slot.
- **Peringkat publik.** 10 member teratas bulan ini berdasarkan jumlah kehadiran. Bisa dimatikan admin.
- **Halaman statis.** Tentang Kami, Kontak, Syarat dan Ketentuan, Kebijakan Privasi, dan FAQ (5.8).
- **Footer.** Logo, slogan, kontak, alamat, link media sosial, dan link halaman statis, semuanya dari pengaturan global.

### 5.2 Akun member

- Daftar dan login dengan no WA (format dinormalisasi ke 628xxx) dan password.
- Profil: nama tampilan, level, rekening refund default.
- Riwayat: sesi yang diikuti, status bayar, total hadir, peringkat bulan ini.

### 5.3 Dashboard admin

Dashboard dibuka dengan ringkasan yang langsung bisa ditindaklanjuti.

- **Kartu statistik.** Sesi bulan ini, total peserta hadir, pemasukan bersih bulan ini, rata-rata keterisian slot.
- **Butuh tindakan.** Jumlah bukti bayar menunggu konfirmasi, refund tertunda, dan peserta belum lunas di sesi terdekat. Setiap item menautkan ke halamannya.
- **Sesi terdekat.** Kartu dengan bar keterisian slot dan bar pelunasan.
- **Peringkat member.** Tabel peringkat dengan filter periode (bulan ini, 3 bulan, tahun ini, semua). Kolom: peringkat, nama, jumlah hadir, jumlah no-show, persentase kehadiran, terakhir main. Tiga teratas diberi ikon medali.
- **Grafik.** Peserta hadir per sesi (8 sesi terakhir) dan pemasukan vs biaya per bulan (6 bulan terakhir).

Peringkat dihitung dari status `attended`. No-show tidak menambah poin dan ditampilkan sebagai kolom terpisah.

### 5.4 Manajemen sesi

- CRUD venue, template sesi, dan sesi.
- Daftar peserta per sesi dengan aksi: konfirmasi atau tolak bayar, pindahkan ke waiting list, hapus, tambah peserta manual (untuk yang masih DM).
- Tombol Salin Teks Broadcast dan Salin Link Sesi.
- Antrean konfirmasi bayar lintas sesi, dengan pratinjau bukti, nominal, dan kode unik.
- Antrean refund dengan rekening tujuan dan tombol Tandai Sudah Di-refund.

### 5.5 Hari H dan rotasi

- Checklist hadir dan no-show yang cepat ditekan di ponsel.
- Pembuat rotasi: input jumlah lapangan aktif, sistem membuat pasangan 2 lawan 2 dengan total level berimbang.
- Bobot level: Pemula 1, Menengah 2, Mahir 3. Admin bisa mengoreksi level peserta.
- Pemilihan pemain per ronde: prioritas jumlah main paling sedikit, lalu waktu duduk paling lama, lalu hindari pasangan yang sama dengan ronde sebelumnya.
- Tampilan papan rotasi besar yang bisa diproyeksikan atau dibagikan sebagai link.

### 5.6 Keuangan

- Per sesi: pemasukan (pembayaran lunas), refund, biaya sewa lapangan, biaya shuttlecock, biaya lain, dan saldo bersih.
- Rekap bulanan dengan ekspor CSV.
- Pembayaran hangus tetap dihitung sebagai pemasukan.

### 5.7 Pengaturan global

Semua parameter yang bisa berubah diatur dari panel admin, bukan di kode, env, atau seed database. Environment variable hanya untuk infrastruktur dan rahasia: `DATABASE_URL`, `LISTEN_ADDR`, `UPLOAD_DIR`, `BASE_URL`, `SESSION_SECRET`.

| Grup | Isi |
| --- | --- |
| Identitas | Nama aplikasi (seed: MabarBadmin), slogan, logo terang, logo gelap, favicon |
| Kontak | No WA admin (tombol chat), email, alamat, link peta |
| Media sosial | TikTok, Instagram, Ayo, YouTube, Facebook, X (kosong = disembunyikan) |
| Pembayaran | Nama bank, no rekening, atas nama, gambar QRIS statis, kode unik aktif secara default |
| Aturan sesi | Batas bayar opsional (default fleksibel sampai hari H), batas refund (default H-1 00.00 WIB), kuota per lapangan (default 4, sistem gantian), aturan poin sebagai info sesi (default 15x2), zona waktu |
| Broadcast | Template teks broadcast WA dengan placeholder |
| SEO | Pola judul, deskripsi default, keyword default, gambar OG default, kode verifikasi Google Search Console, izin indeks mesin pencari |
| Tampilan | Peringkat publik on/off, jumlah jadwal di beranda |

- Disimpan sebagai pasangan kunci dan nilai di tabel `settings`, dengan daftar kunci, tipe, dan nilai default didefinisikan di kode Go. Seeder awal (perintah mabar seed) mengisi nama aplikasi MabarBadmin, URL situs https://mabarbadmin.web.id, nilai default lain, serta lima halaman statis bawaan. Seeder idempoten: hanya mengisi kunci atau halaman yang belum ada, sehingga aman dijalankan ulang tanpa menimpa isi yang sudah diubah admin.
- Dibaca lewat helper `settings.Get` yang memakai cache di memori. Cache dibuang setiap kali admin menyimpan, sehingga perubahan langsung tampil tanpa restart.
- Template menerima objek `Site` (nama, logo, kontak, sosial, SEO) di setiap render, sehingga tidak ada teks identitas yang ditulis langsung di template.
- Favicon yang diunggah diolah otomatis menjadi ukuran 32, 180 (apple-touch-icon), 192, dan 512 piksel serta `site.webmanifest`.

### 5.8 Halaman statis

- Halaman bawaan: `about`, `contact`, `terms`, `privacy`, `faq`. Admin bisa menambah halaman lain. Semua halaman diakses langsung di root domain, misal mabarbadmin.web.id/about atau mabarbadmin.web.id/cara-bayar, tanpa prefix seperti /p/ atau /pages/. Slug yang bentrok dengan route sistem ditolak saat disimpan.
- Field: judul, slug, isi (Markdown dengan pratinjau), gambar sampul opsional, status terbit, tampil di footer, urutan, dan field SEO (judul meta, deskripsi, keyword, gambar OG).
- Markdown dirender dengan `goldmark` lalu disanitasi `bluemonday`, sehingga isi aman dari skrip.
- Halaman Kontak otomatis menampilkan tombol WA, email, alamat, dan peta dari pengaturan global, di atas isi halaman.
- FAQ dikelola sebagai daftar tanya jawab yang bisa diurutkan, dirender sebagai accordion dan JSON-LD `FAQPage`.

### 5.9 Hero slider

- Field per slide: gambar desktop, gambar mobile opsional, judul, subjudul, label tombol, URL tombol, urutan, aktif, tanggal mulai dan berakhir tayang.
- Maksimal 5 slide aktif. Jika tidak ada slide, hero diganti blok teks dengan nama aplikasi dan slogan.
- Ukuran rekomendasi 1600 x 900 (desktop) dan 900 x 1200 (mobile), ditampilkan sebagai petunjuk di form upload.
- Slide pertama dirender di server sebagai gambar LCP. Slide berikutnya dimuat lazy dan hanya bergeser setelah halaman selesai dimuat. Autoplay berhenti saat disentuh dan mati untuk pengguna dengan `prefers-reduced-motion`.

### 5.10 Pengumuman

- Field: teks singkat (maks 160 karakter), link opsional, gaya (info, penting), tanggal mulai dan berakhir, aktif.
- Tampil sebagai bar di atas header semua halaman publik. Jika ada lebih dari satu yang aktif, yang terbaru yang tampil.
- Tombol tutup menyimpan ID pengumuman di cookie, sehingga bar tidak muncul lagi sampai ada pengumuman baru.

## 6. UI, ikon, dan lokalisasi

Semua yang dilihat pengguna berbahasa Indonesia, sedangkan semua yang ada di URL, kode, database, dan API berbahasa Inggris.

### Prinsip desain

- **Mobile-first.** Mayoritas pengguna membuka dari link WA di ponsel. Target lebar minimum 360 px.
- **Bersih.** Latar netral, satu warna aksen (hijau lapangan), kartu dengan sudut membulat dan border tipis, tanpa gradien berlebihan.
- **Navigasi admin.** Sidebar di desktop (lebar 1024 px ke atas), bottom navigation 5 item di ponsel: Dasbor, Sesi, Pembayaran, Member, Lainnya.
- **Mode gelap** mengikuti pengaturan perangkat, dengan token warna Tailwind di `:root` dan `.dark`.
- **Interaksi HTMX.** Aksi seperti konfirmasi bayar dan checklist hadir memperbarui baris tanpa reload penuh. Alpine.js untuk modal, dropdown, dan tombol salin.
- **Aksesibilitas.** Target sentuh minimal 44 px, kontras teks WCAG AA, label pada setiap input, fokus terlihat.

### Ikon SVG

Ikon dirender sebagai SVG inline dari satu sprite `icons.svg` yang di-embed (`<use href="#icon-...">`), sehingga bisa diwarnai lewat `currentColor` tanpa request tambahan. Ikon umum diambil dari Lucide (lisensi ISC, simpan notice lisensinya). Ikon khas badminton dibuat sendiri dengan gaya garis yang sama (stroke 2, grid 24).

| Konteks | Ikon |
| --- | --- |
| Jadwal dan tanggal | `calendar-days` |
| Jam | `clock` |
| Venue dan peta | `map-pin` |
| Lapangan | `court` (custom, garis lapangan badminton) |
| Shuttlecock | `shuttlecock` (custom) |
| Raket dan rotasi | `racket` (custom) |
| Level | `signal` (1 sampai 3 bar) |
| Peserta | `users` |
| Waiting list | `hourglass` |
| Lunas | `badge-check` |
| Menunggu konfirmasi | `clock-alert` |
| Bukti bayar | `receipt` |
| QRIS | `qr-code` |
| Transfer bank | `landmark` |
| Refund | `undo-2` |
| Oper slot | `repeat` |
| Pengumuman | `megaphone` |
| Peringkat | `trophy`, medali custom emas, perak, perunggu |
| Salin teks | `copy` |
| Keuangan | `wallet` |

Emoji uang dari format lama tetap dipakai **hanya** di teks broadcast WA, karena WA tidak bisa menampilkan SVG.

### Lokalisasi

| Hal | Format | Contoh |
| --- | --- | --- |
| Mata uang | `Rp` + spasi + titik ribuan, tanpa desimal | Rp 85.000 |
| Tanggal dan jam | Hari, tanggal bulan tahun jam:menit zona | Senin, 9 November 2026 21:00 WIB |
| Rentang jam | jam mulai sampai jam selesai, durasi | 21:00–24:00 WIB (3 jam) |
| Tanggal singkat | dd/mm/yyyy | 09/11/2026 |
| No WA | Disimpan `628xxx`, ditampilkan `0812-xxxx-xxxx` | 0812-3456-7890 |

- Zona waktu dari pengaturan, default `Asia/Jakarta` (WIB). Label WITA atau WIT mengikuti zona yang dipilih.
- Database menyimpan waktu sebagai `timestamptz` (UTC). Konversi ke zona lokal hanya di layer tampilan.
- Go tidak punya locale Indonesia bawaan, jadi dibuat paket `internal/locale` dengan nama hari dan bulan, `FormatRupiah`, dan `FormatDateTime`, dipasang sebagai template func.
- Import `time/tzdata` agar binary tidak bergantung pada tzdata di server.

### Konvensi nama (English)

- URL dan slug: kebab-case, misal `/sessions/monday-night-gba-2026-11-09`, `/admin/payments/pending`.
- Query params dan form fields: snake\_case, misal `?period=this_month`, `payment_method`.
- Tabel dan kolom database: snake\_case jamak untuk tabel, misal `registrations.payment_status`.
- Nilai enum: snake\_case English (`no_show`, `refund_pending`), diterjemahkan ke label Indonesia di template.

## 7. Gambar, performa, dan SEO

Semua gambar melewati satu helper yang mengompres ke WebP dan membersihkan file lama, agar beranda mencapai skor Lighthouse mobile 90+ dan server bebas file sampah.

### 7.1 Helper media

Semua upload gambar wajib lewat `media.Save(ctx, file, preset)` di paket `internal/media`. Handler tidak boleh menulis file ke disk secara langsung, dan aturan ini dicek saat code review.

1. Validasi ukuran mentah (maks 10 MB untuk konten, 5 MB untuk bukti bayar) dan tipe dari isi file, bukan dari ekstensi. Diterima: JPG, PNG, WebP.
2. Decode, putar sesuai orientasi EXIF, lalu buang semua metadata.
3. Resize per varian dengan `golang.org/x/image/draw` (CatmullRom), menjaga rasio dan tidak pernah memperbesar.
4. Encode ke WebP lossy dengan `github.com/gen2brain/webp`, yang menjalankan libwebp lewat WebAssembly, sehingga build tetap `CGO_ENABLED=0`.
5. Simpan dengan nama acak `ab/cd/<id>-<lebar>.webp` dan catat di tabel `media` (preset, ukuran, dimensi, daftar varian).

| Preset | Varian lebar (px) | Format dan kualitas |
| --- | --- | --- |
| `hero` | 640, 1024, 1600 | WebP 75 |
| `hero_mobile` | 480, 900 | WebP 75 |
| `page_cover` | 640, 1200 | WebP 75 |
| `logo` | 160, 320 | WebP 90 |
| `payment_proof` | 1280 | WebP 70, folder privat |
| `og_image` | 1200 x 630 (crop tengah) | JPEG 85 |
| `favicon` | 32, 180, 192, 512 | PNG |

Dua pengecualian dari WebP disengaja. Gambar OG memakai JPEG karena pratinjau link di WhatsApp, kanal utama komunitas, tidak konsisten menampilkan WebP. Favicon dan apple-touch-icon memakai PNG karena itu yang didukung browser dan iOS.

### 7.2 Pembersihan file

- Setiap tabel pemilik gambar menyimpan `media_id` (foreign key), bukan path file. Contoh: `hero_slides.image_media_id`, `pages.cover_media_id`, `payment_proofs.media_id`, dan kunci pengaturan `logo_media_id`.
- **Ganti gambar.** Dalam satu transaksi, pemilik menunjuk media baru dan baris media lama dihapus. File lama dihapus dari disk **setelah** commit berhasil, sehingga rollback tidak pernah meninggalkan gambar rusak.
- **Hapus data.** Baris pemilik dan baris media dihapus dalam satu transaksi, lalu semua varian file dihapus setelah commit.
- **Penyapu yatim.** Scheduler mingguan menghapus file di disk yang tidak punya baris `media`, dan baris `media` yang tidak dipakai pemilik mana pun, keduanya hanya jika berumur lebih dari 24 jam agar upload yang sedang berjalan tidak ikut terhapus. Jumlah file dan byte yang dibebaskan dicatat di log.
- **Retensi bukti bayar.** Default 12 bulan setelah sesi ditutup, bisa diatur di pengaturan. Setelah itu bukti dihapus, status pembayaran tetap tersimpan.
- Pengaturan menampilkan total pemakaian penyimpanan per preset.
- Unit test wajib: mengganti logo tidak menyisakan file lama, menghapus slide menghapus semua variannya, dan rollback transaksi tidak menghapus file.

### 7.3 Target performa

| Metrik (mobile) | Target |
| --- | --- |
| Lighthouse Performance | 90 atau lebih |
| Lighthouse SEO dan Accessibility | 90 atau lebih |
| LCP | 2,5 detik atau kurang |
| CLS | 0,1 atau kurang |
| INP | 200 ms atau kurang |
| TTFB | 200 ms atau kurang |

Target berlaku untuk beranda, halaman sesi, dan halaman statis.

- **Gambar LCP.** Slide pertama dirender sebagai `<picture>` dengan `srcset`, `sizes`, `width`, `height`, dan `fetchpriority="high"`, tanpa `loading="lazy"`, plus `<link rel="preload" as="image" imagesrcset=...>` di head.
- **Gambar lain.** `loading="lazy"` dan `decoding="async"`, dengan `aspect-ratio` agar tidak menggeser layout.
- **Font.** Memakai system font stack, tanpa web font.
- **CSS dan JS.** Satu file CSS Tailwind di bawah 25 KB gzip. HTMX dan Alpine dimuat `defer`. Aset bernama hash dengan cache `immutable` 1 tahun.
- **Tanpa skrip pihak ketiga** di halaman publik. Peta berupa tombol link, bukan iframe.
- **Server.** Data beranda dan pengaturan di-cache di memori. Caddy memberi kompresi zstd atau gzip dan HTTP/2 serta HTTP/3.
- **Bar pengumuman** dirender di server, bukan disisipkan JavaScript, agar tidak memicu CLS.
- **Verifikasi.** Lighthouse CI dijalankan di pipeline CI terhadap 3 halaman tersebut, dan build gagal jika skor di bawah 90. Node atau Chrome hanya dipakai di CI, tidak di server.

### 7.4 SEO dasar

- `<html lang="id">`, satu `h1` per halaman, dan HTML semantik.
- Judul memakai pola dari pengaturan, misal `{judul halaman} | {nama aplikasi}`. Meta description, meta keywords, dan canonical URL dari `BASE_URL`.
- Meta keywords tetap diisi sesuai permintaan, walaupun Google tidak memakainya untuk peringkat.
- Open Graph (`og:title`, `og:description`, `og:image`, `og:url`, `og:type`, `og:site_name`, `og:locale=id_ID`) dan Twitter Card `summary_large_image`.
- Urutan nilai SEO: field SEO milik entitas, lalu nilai otomatis (sesi: judul dari hari, tanggal, dan venue), lalu default dari pengaturan.
- JSON-LD: `Organization` dan `WebSite` di beranda, `SportsEvent` per sesi (tanggal dengan zona waktu, lokasi, harga IDR, ketersediaan slot), `FAQPage` di FAQ, `BreadcrumbList` di halaman statis.
- `sitemap.xml` dinamis berisi beranda, halaman statis yang terbit, dan sesi mendatang yang terbit, dengan `lastmod`.
- `robots.txt` dinamis memblokir `/admin`, `/me`, `/tickets`, `/login`, `/register` dan mencantumkan sitemap. Jika indeks dimatikan di pengaturan, seluruh situs mendapat `Disallow: /` dan `noindex`.
- Field alt teks wajib diisi saat upload hero dan gambar sampul.
- Slug yang diubah otomatis tercatat di `slug_redirects` dan dialihkan dengan 301.
- Meta verifikasi Google Search Console diisi dari pengaturan.
- Fase 2: gambar OG otomatis per sesi berisi tanggal, jam, dan venue.

## 8. Arsitektur dan database

Satu binary Go melayani HTML (SSR), aset statis yang di-embed, upload bukti, dan scheduler, di depan satu database PostgreSQL.

### Stack

| Lapisan | Pilihan | Alasan |
| --- | --- | --- |
| Bahasa | Go stabil terbaru, versi di-pin di `go.mod` dan `toolchain` | Satu binary, tanpa runtime |
| Router | `net/http` ServeMux bawaan (pola method + path) | Tanpa dependensi framework |
| Template | `html/template` + HTMX + Alpine.js | SSR, auto-escape, interaktif ringan |
| Database | PostgreSQL + `pgx/v5` | Driver native, pool bawaan |
| Query | `sqlc` | Query SQL type-safe, kode hasil generate |
| Migrasi | `goose` dengan file SQL di-embed | Migrasi jalan saat binary start atau via subcommand |
| Password | argon2id (`golang.org/x/crypto`) | Standar saat ini |
| CSRF | `http.CrossOriginProtection` + token pada form | Proteksi bawaan Go |
| CSS dan JS | Tailwind CSS v4 CLI + Bun (hanya lokal) | Build sekali, hasil di-embed |
| Konfigurasi | Environment variable | Cocok untuk systemd |

### Struktur proyek

```text
mabar/
  cmd/mabar/main.go          # serve, migrate, seed, create-admin
  internal/
    config/
    database/                # pool, migrations (embed)
    database/queries/        # *.sql untuk sqlc
    database/sqlc/           # hasil generate, jangan diedit
    domain/                  # aturan slot, waiting list, refund, rotasi
    handler/                 # HTTP handler publik dan admin
    middleware/              # auth, csrf, logging, recover
    locale/                  # FormatRupiah, FormatDateTime
    scheduler/               # job tutup sesi dan penyapu media
    storage/                 # simpan dan sajikan bukti bayar
    broadcast/               # generator teks WA
    media/                   # kompres WebP, varian, hapus file, penyapu yatim
    settings/                # registry kunci, default, cache
    seo/                     # meta, OG, JSON-LD, sitemap, robots
    content/                 # halaman statis, FAQ, hero, pengumuman
  web/
    templates/               # layout, pages, partials
    static/                  # css, js, icons.svg, img (hasil build)
    embed.go
  frontend/                  # sumber Tailwind dan JS, dibuild Bun
  migrations/
  .githooks/commit-msg
  Makefile
```

Logika aturan bisnis ada di `internal/domain` dan diuji dengan unit test tanpa HTTP. Handler hanya memanggil domain dan merender template.

### Skema database

Uang disimpan sebagai `bigint` rupiah. Waktu sebagai `timestamptz`. Primary key `bigint generated always as identity`, kecuali token yang memakai hash.

| Tabel | Kolom utama |
| --- | --- |
| `users` | `phone` (unique), `name`, `password_hash`, `role` (admin, member), `level`, `refund_bank_name`, `refund_account_number`, `refund_account_name`, `is_active` |
| `auth_sessions` | `token_hash`, `user_id`, `expires_at`, `user_agent` |
| `settings` | `key` (primary key), `value` (jsonb), `updated_at`; daftar kunci, tipe, dan default didefinisikan di kode |
| `media` | `preset`, `storage_key`, `width`, `height`, `bytes`, `variants` (jsonb), `alt_text`, `is_private`, `created_at` |
| `venues` | `name`, `address`, `maps_url` |
| `session_templates` | `name`, `venue_id`, `weekday`, `start_time`, `end_time`, `court_count`, `capacity`, `price`, `point_rule`, `level_label`, `shuttlecock_included`, `notes` |
| `sessions` | `slug` (unique), `template_id`, `venue_id`, `starts_at`, `ends_at`, `court_count`, `capacity`, `price`, `unique_code_enabled`, `payment_deadline_at` (nullable), `refund_deadline_at`, `status` (draft, published, closed, cancelled), `point_rule`, `level_label`, `notes`, `court_cost`, `shuttlecock_cost`, `other_cost`, `meta_title`, `meta_description` |
| `registrations` | `session_id`, `user_id` (null untuk tamu), `display_name`, `phone`, `level`, `status`, `payment_status`, `amount_due`, `unique_code`, `ticket_token_hash`, `registered_at`, `paid_at`, `transferred_to_id`, `cancelled_at`; unique (`session_id`, `phone`) |
| `payment_proofs` | `registration_id`, `method` (qris, bank\_transfer), `media_id`, `uploaded_at`, `reviewed_at`, `review_note` |
| `refunds` | `registration_id`, `amount`, `bank_name`, `account_number`, `account_name`, `status` (pending, completed), `completed_at` |
| `rotation_rounds` | `session_id`, `round_number`, `created_at` |
| `rotation_matches` | `round_id`, `court_number`, `team_a_player1_id`, `team_a_player2_id`, `team_b_player1_id`, `team_b_player2_id` |
| `pages` | `slug` (unique), `title`, `body_markdown`, `cover_media_id`, `is_published`, `show_in_footer`, `sort_order`, `meta_title`, `meta_description`, `meta_keywords`, `og_media_id`, `updated_at` |
| `faq_items` | `question`, `answer_markdown`, `sort_order`, `is_published` |
| `hero_slides` | `image_media_id`, `mobile_image_media_id`, `title`, `subtitle`, `cta_label`, `cta_url`, `sort_order`, `is_active`, `starts_at`, `ends_at` |
| `announcements` | `message` (maks 160), `link_url`, `style` (info, important), `starts_at`, `ends_at`, `is_active` |
| `slug_redirects` | `entity_type`, `old_slug`, `new_slug`, `created_at` |
| `activity_logs` | `actor_type`, `actor_id`, `entity_type`, `entity_id`, `action`, `data` (jsonb), `created_at` |

Semua kolom `*_media_id` memakai `ON DELETE RESTRICT`, sehingga baris media hanya bisa dihapus lewat helper media yang juga menghapus filenya.

Peringkat dihitung dengan query agregat `count(*) filter (where status = 'attended')` per `user_id` dalam rentang periode, dengan index pada (`user_id`, `status`). Tamu tidak masuk peringkat. Jika tamu kemudian mendaftar dengan no WA yang sama, riwayatnya digabung ke akun member.

## 9. Route dan endpoint

Semua route mengembalikan HTML. Request dari HTMX (header `HX-Request`) mendapat partial, request biasa mendapat halaman penuh, sehingga setiap URL tetap bisa dibuka langsung.

### Publik

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/` | Beranda: event terdekat, jadwal, pengumuman |
| GET | `/sessions` | Daftar sesi mendatang |
| GET | `/sessions/{slug}` | Detail sesi dan daftar peserta |
| POST | `/sessions/{slug}/registrations` | Join (member atau tamu) |
| GET | `/tickets/{token}` | Halaman tiket tamu |
| POST | `/tickets/{token}/payment-proofs` | Upload bukti bayar tamu |
| POST | `/tickets/{token}/cancel` | Cancel tamu |
| POST | `/tickets/{token}/transfer` | Oper slot tamu |
| GET | `/announcements` | Arsip pengumuman |
| GET | `/leaderboard?period=this_month` | Peringkat publik |
| GET, POST | `/login`, `/register` | Autentikasi |
| POST | `/logout` | Keluar |

### Member

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/me` | Profil dan statistik pribadi |
| GET | `/me/registrations` | Riwayat sesi |
| POST | `/me/registrations/{id}/payment-proofs` | Upload bukti |
| POST | `/me/registrations/{id}/cancel` | Cancel |
| POST | `/me/registrations/{id}/transfer` | Oper slot |
| POST | `/me/profile` | Ubah profil dan rekening refund |

### Admin

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/admin` | Dashboard |
| GET | `/admin/leaderboard?period=last_3_months` | Peringkat lengkap |
| GET, POST | `/admin/venues` | Kelola venue |
| GET, POST | `/admin/templates` | Kelola template sesi |
| POST | `/admin/templates/{id}/sessions` | Buat sesi dari template |
| GET, POST | `/admin/sessions`, `/admin/sessions/{id}` | Kelola sesi |
| GET | `/admin/sessions/{id}/broadcast` | Teks broadcast siap salin |
| POST | `/admin/sessions/{id}/registrations` | Tambah peserta manual |
| POST | `/admin/registrations/{id}/move` | Pindah ke slot atau waiting list |
| GET | `/admin/payments?status=pending` | Antrean konfirmasi |
| POST | `/admin/payment-proofs/{id}/approve` | Konfirmasi bayar |
| POST | `/admin/payment-proofs/{id}/reject` | Tolak bayar |
| GET | `/admin/refunds?status=pending` | Antrean refund |
| POST | `/admin/refunds/{id}/complete` | Tandai sudah di-refund |
| GET | `/admin/sessions/{id}/game-day` | Mode hari H |
| POST | `/admin/registrations/{id}/attendance` | Hadir atau no-show |
| POST | `/admin/sessions/{id}/rotation-rounds` | Buat ronde rotasi berikutnya |
| GET | `/sessions/{slug}/rotation` | Papan rotasi (publik, hanya baca) |
| POST | `/admin/sessions/{id}/close` | Tutup sesi |
| POST | `/admin/sessions/{id}/costs` | Isi biaya sesi |
| GET | `/admin/finance?month=2026-11` | Laporan bulanan |
| GET | `/admin/finance/export.csv?month=2026-11` | Ekspor CSV |
| GET, POST | `/admin/announcements` | Kelola pengumuman |
| GET, POST | `/admin/members` | Kelola member, reset password |
| GET, POST | `/admin/settings` | Pengaturan |
| GET | `/admin/uploads/{id}` | Lihat bukti bayar (khusus admin) |

### Konten, SEO, dan media

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/about`, `/contact`, `/terms`, `/privacy`, `/faq` | Halaman statis bawaan hasil seeder, dilayani oleh route /{slug} yang sama |
| GET | `/{slug}` | Halaman statis apa pun di root domain, didaftarkan paling akhir agar route sistem selalu menang |
| GET | `/sitemap.xml`, `/robots.txt`, `/site.webmanifest` | Dibuat dinamis dari data dan pengaturan |
| GET | `/media/{key}` | Gambar publik, cache `immutable` 1 tahun |
| GET, POST | `/admin/pages`, `/admin/pages/{id}` | Kelola halaman statis |
| POST | `/admin/pages/{id}/preview` | Pratinjau Markdown (HTMX) |
| GET, POST | `/admin/faq` | Kelola FAQ |
| POST | `/admin/faq/reorder` | Ubah urutan FAQ |
| GET, POST | `/admin/hero-slides`, `/admin/hero-slides/{id}` | Kelola hero slider |
| POST | `/admin/hero-slides/reorder` | Ubah urutan slide |
| GET, POST | `/admin/settings/{group}` | Pengaturan per grup: `identity`, `contact`, `social`, `payment`, `session-rules`, `broadcast`, `seo`, `display` |
| GET | `/admin/settings/storage` | Pemakaian penyimpanan |

Slug halaman statis tidak boleh memakai kata yang dipakai route sistem: `admin`, `me`, `sessions`, `tickets`, `login`, `register`, `logout`, `leaderboard`, `announcements`, `media`, `static`, `healthz`, `sitemap.xml`, `robots.txt`, `site.webmanifest`. Daftar ini dipegang di satu konstanta Go yang juga dipakai validasi form.

### Sistem

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/healthz` | Cek hidup untuk monitoring |
| GET | `/static/*` | Aset embed, cache 1 tahun dengan nama file ber-hash |

## 10. Build dan deploy

Semua build terjadi di mesin lokal atau CI. Server hanya menerima satu binary Linux, tanpa Node, Bun, atau toolchain Go.

### Build lokal

1. `bun install` memasang Tailwind CLI, HTMX, dan Alpine.js dari `frontend/package.json` dengan versi di-pin dan `bun.lock` di-commit.
2. `bunx @tailwindcss/cli -i frontend/app.css -o web/static/css/app.css --minify` membangun CSS.
3. `bun build frontend/app.js --outdir web/static/js --minify` membundel HTMX, Alpine, dan skrip kecil seperti tombol salin.
4. Skrip kecil menambahkan hash ke nama file aset dan menulis `manifest.json` untuk template.
5. `sqlc generate` bila ada perubahan query.
6. `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always)" -o dist/mabar ./cmd/mabar`.

Semua langkah dibungkus `make release`. Hasil build di `web/static` tidak di-commit, kecuali `icons.svg` yang merupakan sumber.

### Deploy ke VPS

1. Upload `dist/mabar` ke `/opt/mabar/releases/<versi>/` lewat `scp` atau `rsync`.
2. Arahkan symlink `/opt/mabar/current` ke rilis baru.
3. `sudo systemctl restart mabar`. Migrasi berjalan otomatis saat start, atau manual dengan `mabar migrate up`.
4. Rollback dengan mengarahkan symlink ke rilis sebelumnya lalu restart. Migrasi dibuat aditif agar rollback binary tetap aman.

Instalasi pertama di server: `mabar migrate up`, lalu `mabar seed` untuk nama aplikasi MabarBadmin dan halaman bawaan, lalu `mabar create-admin`. Isi `BASE_URL=https://mabarbadmin.web.id` di `/etc/mabar/env`, dan arahkan DNS A record `mabarbadmin.web.id` serta `www` ke IP VPS sebelum Caddy dijalankan.

### systemd

```ini
[Unit]
Description=Mabar Badminton
After=network-online.target postgresql.service

[Service]
User=mabar
EnvironmentFile=/etc/mabar/env
ExecStart=/opt/mabar/current/mabar serve
Restart=on-failure
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/mabar
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Variabel di `/etc/mabar/env` (mode 600): `DATABASE_URL`, `LISTEN_ADDR=127.0.0.1:8080`, `UPLOAD_DIR=/var/lib/mabar/uploads`, `BASE_URL`, `SESSION_SECRET`.

### Caddy

```text
www.mabarbadmin.web.id {
    redir https://mabarbadmin.web.id{uri} permanent
}

mabarbadmin.web.id {
    encode zstd gzip
    request_body {
        max_size 6MB
    }
    reverse_proxy 127.0.0.1:8080
}
```

Caddy mengurus HTTPS otomatis. Nginx tetap bisa dipakai bila sudah ada di server, dengan Certbot untuk sertifikat.

### Backup

- `pg_dump -Fc` harian lewat systemd timer, simpan 14 hari lokal dan salin ke penyimpanan luar server.
- Folder upload bukti bayar di-`rsync` harian ke lokasi yang sama.
- Uji restore minimal sekali sebulan.

## 11. Aturan coding dan git

Aturan ini berlaku untuk semua kontributor, manusia maupun AI coding tool, dan ditegakkan oleh hook lokal serta CI.

### Gaya kode dan komentar

- Kode harus jelas dari nama, tipe, dan struktur.
- Dilarang komentar yang mengulang isi kode, pembatas section dekoratif, banner file, dan TODO tanpa referensi issue (format `TODO(#123)`).
- Komentar hanya untuk alasan yang tidak terlihat dari kode: konkurensi, batasan keamanan, keanehan provider, invariant, workaround.
- Pertahankan directive tool (`//go:embed`, `//go:build`, `//nolint` beralasan), notice lisensi (misal ikon Lucide), dan penanda kode hasil generate (`// Code generated ... DO NOT EDIT.` dari sqlc).
- Jangan menghapus komentar dengan regex global. Tinjau setiap penghapusan sesuai konteksnya.
- Wajib `gofmt`, `go vet`, dan `staticcheck` lolos sebelum merge.
- Nama dalam kode, URL, database, dan pesan commit berbahasa Inggris. Teks yang tampil ke pengguna berbahasa Indonesia dan dikumpulkan di template.

Contoh komentar yang boleh:

```go
// Lock the session row so two concurrent joins cannot both take the last slot.
session, err := q.GetSessionForUpdate(ctx, sessionID)
```

### Git policy

- Jangan menambahkan trailer co-author atau atribusi AI pada commit. Termasuk baris `Co-authored-by` yang menyebut Claude, Anthropic, ChatGPT, OpenAI, Copilot, Gemini, Cursor, atau AI tool lain, serta footer seperti "Generated with ...".
- Jika coding tool menambahkan atribusi secara default, matikan di konfigurasi tool tersebut. Contoh: Claude Code dengan `"includeCoAuthoredBy": false` di `.claude/settings.json`.
- Aktifkan hook dengan `git config core.hooksPath .githooks`. CI menjalankan pemeriksaan yang sama pada setiap commit di pull request.
- Gunakan identitas Git milik pemilik repository. Periksa dengan `git config user.name` dan `git config user.email` sebelum commit. Jangan mengubah konfigurasi Git global.
- Pesan commit singkat, deskriptif, berbahasa Inggris, misal `Add waitlist promotion after payment deadline`.
- Jangan amend, rebase, force-push, menulis ulang history, push, membuat release, atau menyentuh branch terlindungi tanpa izin eksplisit.
- Periksa staged diff (`git diff --staged`) sebelum setiap commit.
- Jangan pernah mengklaim commit atau push berhasil kecuali memang berhasil.

### `.githooks/commit-msg`

```sh
#!/bin/sh
set -eu

pattern='(co-authored-by:.*(claude|anthropic|chatgpt|openai|copilot|gemini|cursor|codeium|windsurf|devin))|(generated (with|by).*(claude|chatgpt|copilot|gemini|cursor|ai\b))'

if grep -Eiq "$pattern" "$1"; then
    echo "commit-msg: AI attribution or co-author trailers are not allowed." >&2
    exit 1
fi
```

### Pemeriksaan CI

```sh
#!/bin/sh
set -eu
base="$1"
head="$2"
status=0
for sha in $(git rev-list "$base..$head"); do
    git log -1 --format=%B "$sha" > /tmp/msg
    if ! sh .githooks/commit-msg /tmp/msg; then
        echo "Rejected commit: $sha" >&2
        status=1
    fi
done
exit "$status"
```

Skrip CI disimpan di `scripts/check-commit-messages.sh` dan dipanggil dengan base dan head pull request.

## 12. Non-fungsional, roadmap, dan risiko

Aplikasi harus cepat dibuka dari link WA di jaringan seluler, aman untuk data no WA dan rekening, dan murah dijalankan di satu VPS kecil.

### Kebutuhan non-fungsional

| Aspek | Target |
| --- | --- |
| Performa | Lighthouse mobile 90 atau lebih, LCP 2,5 detik atau kurang, total aset CSS + JS di bawah 60 KB gzip (detail di 7.3) |
| Kapasitas | 300 pengguna aktif dan 50 pendaftar bersamaan pada sesi populer di VPS 1 vCPU, 1 GB RAM |
| Ketersediaan | 99% per bulan, restart otomatis via systemd |
| Keamanan login | Rate limit 5 percobaan per 15 menit per no WA dan IP, cookie `HttpOnly`, `Secure`, `SameSite=Lax` |
| Upload | Hanya JPG, PNG, WebP; cek MIME dari isi file; maks 5 MB; bukti bayar disimpan di folder privat; semua gambar dikonversi ke WebP tanpa metadata lewat helper media (7.1) |
| Privasi | No WA disamarkan di halaman publik (0812-xxxx-7890); bukti bayar dan rekening hanya terlihat oleh admin dan pemiliknya |
| Token tamu | 32 byte acak, disimpan sebagai hash SHA-256 |
| Header | CSP ketat (`script-src 'self'`), `X-Content-Type-Options`, `Referrer-Policy` |
| Observability | Log terstruktur `log/slog` JSON ke journald, endpoint `/healthz` |
| Pengujian | Unit test untuk `internal/domain` (slot, waiting list, H-2, refund, rotasi) dengan cakupan minimal 80%; integration test dengan PostgreSQL di CI |

Alpine.js versi standar memakai evaluasi ekspresi yang butuh `unsafe-eval`. Gunakan build CSP Alpine (`@alpinejs/csp`) agar CSP tetap ketat.

### Roadmap

1. **Fase 0, fondasi.** Struktur proyek, migrasi, auth, layout admin dan publik, sprite ikon, paket locale, registry pengaturan global, helper media dan penyapu yatim, pipeline build dan deploy, Lighthouse CI, git hook.
2. **Fase 1a, inti.** Venue, template, sesi, pendaftaran member dan tamu, teks broadcast, pembayaran dan konfirmasi.
3. **Fase 1b, aturan.** Prioritas lunas otomatis, waiting list, cancel, refund, oper slot, tutup sesi.
4. **Fase 1c, hari H dan data.** Absensi, rotasi, keuangan, dashboard, peringkat, pengumuman, hero slider, halaman statis, FAQ, beranda event terdekat, SEO (meta, OG, JSON-LD, sitemap, robots).
5. **Fase 2.** WA gateway untuk broadcast dan reminder, WA OTP, payment gateway atau QRIS dinamis, saldo member, PWA.

Rilis ke komunitas dilakukan setelah Fase 1b, dengan satu sesi uji coba berjalan paralel dengan cara lama.

### Risiko

| Risiko | Dampak | Mitigasi |
| --- | --- | --- |
| Member tetap mendaftar lewat grup | Data tidak lengkap | Admin tambah peserta manual, teks broadcast selalu menyertakan link |
| Bukti transfer palsu | Kas kurang | Admin wajib cek mutasi, kode unik mempermudah pencocokan |
| Lupa password tanpa OTP | Member tidak bisa masuk | Admin reset dengan password sementara yang wajib diganti |
| Dobel daftar dengan nama berbeda | Slot terbuang | Unique no WA per sesi |
| Level self-assess tidak akurat | Rotasi tidak seimbang | Admin bisa koreksi level |
| Server atau database rusak | Data hilang | Backup harian di luar server, uji restore bulanan |

### Keputusan

| Topik | Keputusan |
| --- | --- |
| Kuota per lapangan | 4 orang, sistem gantian |
| Batas bayar | Fleksibel sampai hari H, mengikuti kuota slot dengan prioritas yang sudah lunas |
| Aturan poin | 15x2 hanya info sesi di halaman sesi dan teks broadcast, tanpa pencatatan skor |
| Domain | mabarbadmin.web.id |
| Nama aplikasi awal | MabarBadmin, diisi lewat seeder |
| Peringkat | Publik, dengan tombol tampil atau sembunyi di panel admin |
| URL halaman statis | Langsung di root domain, misal /about, tanpa /p/ |
| Batas refund | Tetap H-1 pukul 00.00 WIB; cancel setelahnya hangus kecuali dioper |

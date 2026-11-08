# Panduan Deploy ke VPS

Panduan ini memasang MabarBadmin di satu VPS Linux dengan PostgreSQL, systemd, dan Caddy (HTTPS otomatis). Semua build dilakukan di mesin lokal; server hanya menerima satu binary tanpa Go, Node, atau Bun.

Contoh memakai **Ubuntu 24.04** dan domain **mabarbadmin.web.id**. Sesuaikan bila berbeda.

## Ringkasan

| Item | Lokasi di server |
| --- | --- |
| Binary per rilis | `/opt/mabar/releases/<versi>/mabar` |
| Rilis aktif (symlink) | `/opt/mabar/current` |
| Konfigurasi rahasia | `/etc/mabar/env` (root, mode 600) |
| Upload gambar dan bukti bayar | `/var/lib/mabar/uploads` |
| Backup | `/var/backups/mabar` |
| Service | `mabar.service`, `mabar-backup.timer` |
| Aplikasi mendengarkan | `127.0.0.1:8080` (hanya diakses lewat Caddy) |

## 1. Persiapan

**VPS:** minimal 1 vCPU, 1 GB RAM, 20 GB disk, akses SSH dengan user ber-`sudo`.

**DNS:** buat dua A record yang menunjuk ke IP VPS, sebelum Caddy dijalankan, agar sertifikat HTTPS bisa diterbitkan.

| Nama | Tipe | Nilai |
| --- | --- | --- |
| `mabarbadmin.web.id` | A | IP VPS |
| `www.mabarbadmin.web.id` | A | IP VPS |

Cek dari komputer lokal: `dig +short mabarbadmin.web.id` harus mengembalikan IP VPS.

**Mesin lokal:** Go (versi di `go.mod`), Bun, `staticcheck`, `make`, `git`, `rsync`.

## 2. Siapkan server (sekali saja)

Login ke VPS, lalu jalankan:

```sh
sudo apt update && sudo apt upgrade -y
sudo apt install -y postgresql rsync ufw debian-keyring debian-archive-keyring apt-transport-https curl
```

### Firewall

```sh
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 443/udp   # HTTP/3
sudo ufw enable
```

Port 8080 sengaja tidak dibuka; aplikasi hanya mendengarkan di `127.0.0.1`.

### User dan folder

```sh
sudo useradd --system --home /var/lib/mabar --shell /usr/sbin/nologin mabar
sudo mkdir -p /opt/mabar/releases /var/lib/mabar/uploads /var/backups/mabar /etc/mabar
sudo chown -R mabar:mabar /var/lib/mabar /var/backups/mabar
sudo chmod 750 /var/lib/mabar /var/backups/mabar
```

### PostgreSQL

Buat password database yang kuat, lalu database dan user-nya:

```sh
DB_PASS="$(openssl rand -hex 24)"; echo "$DB_PASS"   # simpan nilai ini
sudo -u postgres psql -c "CREATE USER mabar WITH PASSWORD '$DB_PASS';"
sudo -u postgres psql -c "CREATE DATABASE mabar OWNER mabar;"
```

PostgreSQL bawaan Ubuntu hanya mendengarkan di `localhost`, jadi tidak perlu diubah.

### File konfigurasi `/etc/mabar/env`

```sh
SECRET="$(openssl rand -hex 32)"
sudo tee /etc/mabar/env > /dev/null <<EOF
DATABASE_URL=postgres://mabar:${DB_PASS}@127.0.0.1:5432/mabar?sslmode=disable
LISTEN_ADDR=127.0.0.1:8080
UPLOAD_DIR=/var/lib/mabar/uploads
BASE_URL=https://mabarbadmin.web.id
SESSION_SECRET=${SECRET}
EOF
sudo chmod 600 /etc/mabar/env
sudo chown root:root /etc/mabar/env
```

- `BASE_URL` wajib diawali `https://` di produksi; cookie login otomatis memakai flag `Secure`.
- Jangan pernah mengganti `SESSION_SECRET` tanpa alasan. Mengubahnya membuat semua formulir yang sedang terbuka gagal dan harus dimuat ulang.
- Semua pengaturan lain (nama aplikasi, rekening, QRIS, aturan sesi, SEO) diatur dari panel admin, bukan dari file ini.

### Caddy

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install -y caddy
```

Konfigurasi Caddy dipasang di langkah 4 bersama berkas deploy lainnya.

## 3. Build rilis di mesin lokal

Dari folder repository:

```sh
git pull
make release
```

`make release` menjalankan build CSS/JS (Bun), `gofmt`, `go vet`, `staticcheck`, unit test, lalu membuat binary Linux statis di `dist/mabar` (dengan `CGO_ENABLED=0` dan tag `nodynamic`). Versi diambil dari `git describe`, misalnya `9941261` atau tag `v1.0.0`.

Cek hasilnya:

```sh
file dist/mabar          # harus: statically linked
./dist/mabar version     # hanya bisa dijalankan bila mesin lokal juga Linux amd64
```

Bila VPS memakai ARM (misalnya Ampere), ganti `GOARCH=amd64` menjadi `GOARCH=arm64` di target `release` pada `Makefile`.

## 4. Instalasi pertama

### Unggah binary dan berkas deploy

Dari mesin lokal (ganti `user@vps` dan versi):

```sh
VERSION=$(git describe --tags --always)
ssh user@vps "sudo mkdir -p /opt/mabar/releases/$VERSION && sudo chown \$USER /opt/mabar/releases/$VERSION"
rsync -avz dist/mabar user@vps:/opt/mabar/releases/$VERSION/mabar
rsync -avz deploy/ user@vps:/tmp/mabar-deploy/
```

### Pasang di server

```sh
VERSION=<versi-yang-diunggah>
sudo chown -R root:root /opt/mabar/releases/$VERSION
sudo chmod 755 /opt/mabar/releases/$VERSION/mabar
sudo ln -sfn /opt/mabar/releases/$VERSION /opt/mabar/current

sudo install -m 644 /tmp/mabar-deploy/mabar.service /etc/systemd/system/mabar.service
sudo install -m 644 /tmp/mabar-deploy/mabar-backup.service /etc/systemd/system/mabar-backup.service
sudo install -m 644 /tmp/mabar-deploy/mabar-backup.timer /etc/systemd/system/mabar-backup.timer
sudo install -m 755 /tmp/mabar-deploy/backup.sh /opt/mabar/backup.sh
sudo install -m 644 /tmp/mabar-deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl daemon-reload
```

Bila domain berbeda, ubah nama domain di `/etc/caddy/Caddyfile` sebelum melanjutkan.

### Migrasi, seed, dan akun admin

`/etc/mabar/env` hanya bisa dibaca root, jadi perintah CLI dijalankan lewat `systemd-run` yang membaca file itu lalu menjalankan binary sebagai user `mabar`:

```sh
alias mabar='sudo systemd-run --quiet --pty --wait --collect --uid=mabar -p EnvironmentFile=/etc/mabar/env /opt/mabar/current/mabar'

mabar migrate up
mabar seed            # nama aplikasi MabarBadmin, pengaturan default, 5 halaman bawaan + FAQ
mabar create-admin    # isi nama, no WA, dan password (tidak tampil saat diketik)
```

Jangan jalankan `mabar seed --demo` di produksi; perintah itu mengisi data contoh.

`seed` aman dijalankan ulang kapan saja; ia hanya mengisi pengaturan atau halaman yang belum ada dan tidak menimpa perubahan admin.

### Nyalakan layanan

```sh
sudo systemctl enable --now mabar
sudo systemctl enable --now mabar-backup.timer
sudo systemctl reload caddy
```

## 5. Verifikasi

```sh
systemctl status mabar --no-pager
curl -s http://127.0.0.1:8080/healthz          # ok <versi>
curl -sI https://mabarbadmin.web.id | head -5   # HTTP/2 200
curl -sI https://www.mabarbadmin.web.id | grep -i location   # dialihkan ke domain utama
journalctl -u mabar -n 50 --no-pager
```

Lalu dari browser:

1. Buka `https://mabarbadmin.web.id/login`, masuk dengan akun admin.
2. **Pengaturan → Identitas:** unggah logo dan favicon, isi slogan.
3. **Pengaturan → Kontak, Media sosial, Pembayaran:** isi no WA admin, rekening bank, unggah gambar QRIS.
4. **Pengaturan → SEO:** isi kode verifikasi Google Search Console bila ada, lalu daftarkan `https://mabarbadmin.web.id/sitemap.xml` di Search Console.
5. **Sesi → Venue**, lalu **Template sesi**, lalu buat satu sesi uji coba dan coba alur daftar sampai bayar dari ponsel.

## 6. Update ke versi baru

Di mesin lokal:

```sh
git pull
make release
VERSION=$(git describe --tags --always)
ssh user@vps "sudo mkdir -p /opt/mabar/releases/$VERSION && sudo chown \$USER /opt/mabar/releases/$VERSION"
rsync -avz dist/mabar user@vps:/opt/mabar/releases/$VERSION/mabar
```

Di server:

```sh
VERSION=<versi-baru>
sudo chown -R root:root /opt/mabar/releases/$VERSION
sudo ln -sfn /opt/mabar/releases/$VERSION /opt/mabar/current
sudo systemctl restart mabar
curl -s http://127.0.0.1:8080/healthz
```

Migrasi database berjalan otomatis saat service start. Bila sebuah rilis mengubah `deploy/` (unit systemd atau Caddyfile), salin ulang berkas tersebut seperti di langkah 4 lalu `sudo systemctl daemon-reload`.

Simpan beberapa rilis terakhir di `/opt/mabar/releases` untuk rollback; rilis lama bisa dihapus manual.

## 7. Rollback

```sh
ls /opt/mabar/releases
sudo ln -sfn /opt/mabar/releases/<versi-sebelumnya> /opt/mabar/current
sudo systemctl restart mabar
```

Migrasi dibuat aditif, sehingga binary lama tetap berjalan di skema baru. Jangan menjalankan `mabar migrate down` di produksi kecuali benar-benar paham dampaknya; perintah itu bisa menghapus tabel beserta datanya.

## 8. Backup dan restore

`mabar-backup.timer` berjalan setiap hari sekitar pukul 03.30 waktu server:

- `pg_dump -Fc` ke `/var/backups/mabar/db/` (disimpan 14 hari),
- salinan folder upload ke `/var/backups/mabar/uploads/`.

Jalankan sekali secara manual untuk memastikan berfungsi:

```sh
sudo systemctl start mabar-backup.service
ls -lh /var/backups/mabar/db/
journalctl -u mabar-backup -n 20 --no-pager
```

**Salin ke luar server.** Backup yang hanya ada di VPS yang sama tidak melindungi dari VPS rusak. Contoh menarik backup dari komputer lain setiap hari:

```sh
rsync -az --delete user@vps:/var/backups/mabar/ ~/backup-mabar/
```

Atau gunakan `rclone` ke penyimpanan objek (S3, Backblaze B2, Google Drive).

**Uji restore minimal sebulan sekali** ke database terpisah:

```sh
sudo -u postgres createdb -O mabar mabar_restore_test
sudo -u postgres pg_restore --no-owner --role=mabar -d mabar_restore_test /var/backups/mabar/db/<file>.dump
sudo -u postgres psql -d mabar_restore_test -c "select count(*) from registrations;"
sudo -u postgres dropdb mabar_restore_test
```

**Restore sungguhan** (misalnya setelah server rusak): pasang server baru dengan langkah 2 dan 4 tanpa `seed` dan `create-admin`, hentikan service, restore dump ke database `mabar` yang kosong, salin isi `uploads/` ke `/var/lib/mabar/uploads/` (`chown -R mabar:mabar`), lalu nyalakan service.

## 9. Operasional harian

| Kebutuhan | Perintah |
| --- | --- |
| Lihat log aplikasi | `journalctl -u mabar -f` |
| Restart aplikasi | `sudo systemctl restart mabar` |
| Status migrasi | `mabar migrate status` (alias dari langkah 4) |
| Tambah admin lain | `mabar create-admin` |
| Pemakaian disk upload | Panel admin → Pengaturan → Penyimpanan, atau `du -sh /var/lib/mabar/uploads` |
| Log Caddy | `journalctl -u caddy -f` |

Tugas terjadwal di dalam aplikasi berjalan otomatis: tutup sesi 24 jam setelah selesai (bila admin lupa), penegakan batas bayar bila diisi, hapus bukti bayar setelah masa retensi (default 12 bulan), dan penyapu file gambar yatim setiap minggu.

## 10. Pemecahan masalah

| Gejala | Penyebab umum | Solusi |
| --- | --- | --- |
| `mabar.service` gagal start, log `DATABASE_URL is required` atau `SESSION_SECRET must be at least 32 characters` | `/etc/mabar/env` kosong atau salah nama variabel | Periksa isi file, lalu `sudo systemctl restart mabar` |
| Log `ping database: ... password authentication failed` | Password di `DATABASE_URL` tidak cocok | `sudo -u postgres psql -c "ALTER USER mabar WITH PASSWORD '...';"` dan samakan di env |
| Caddy gagal mendapatkan sertifikat | DNS belum mengarah ke VPS, atau port 80/443 tertutup | Cek `dig`, `ufw status`, lalu `sudo systemctl reload caddy` |
| Halaman tampil tanpa gaya | Binary dibuild tanpa `make assets` | Build ulang dengan `make release` |
| Upload gagal `permission denied` | Folder upload bukan milik `mabar` | `sudo chown -R mabar:mabar /var/lib/mabar` |
| Upload gagal untuk file besar | Batas Caddy `request_body` | Batas aplikasi 10 MB untuk gambar konten, 5 MB untuk bukti bayar; Caddyfile mengizinkan 12 MB |
| Login gagal terus, pesan "Terlalu banyak percobaan" | Rate limit 5 kali per 15 menit | Tunggu 15 menit, atau restart service untuk mengosongkan penghitung |
| Member lupa password | Belum ada OTP (fase 2) | Admin → Member → Reset password, kirim password sementara lewat WA |

## Checklist keamanan

- [ ] `/etc/mabar/env` bermode 600 milik root.
- [ ] Port 8080 dan 5432 tidak terbuka ke internet (`sudo ss -tlnp`).
- [ ] Login SSH memakai kunci, bukan password (`PasswordAuthentication no`).
- [ ] Update keamanan otomatis aktif: `sudo apt install unattended-upgrades`.
- [ ] Backup tersalin ke luar server dan restore sudah pernah diuji.

# Rancangan GoStarter: Generator Dinamis dan Kontrak Upstream

Status: rancangan untuk review, belum merupakan implementasi atau schema yang sudah didukung aplikasi.

Tanggal: 7 Oktober 2026.

Roadmap eksekusi: [Development roadmap](development-roadmap.md). Urutan tahap, task ID, dan gate implementasi mengikuti roadmap tersebut; dokumen ini menjelaskan kontrak dan batas produk.

## 1. Tujuan dan keputusan yang sudah disepakati

GoStarter menghasilkan proyek Go dari upstream dengan pilihan ekstensi yang dinamis. Hasil proyek mengikuti struktur direktori dan standar arsitektur upstream.

Keputusan yang sudah disepakati:

1. Upstream menentukan struktur direktori dan arsitektur hasil proyek.
2. Upstream menyediakan implementasi ekstensi feature/usecase, seperti Auth dan User, serta pkg/library/utils.
3. GoStarter memiliki dan merender template arsitektur serta infrastructure sendiri. Struktur direktori, pola wiring, dan perilaku hasilnya mengikuti standar upstream.
4. GoStarter dapat menambah, mengubah, dan menghapus file hasil proyek, termasuk import, dependency injection, registrasi transport, dan konfigurasi.
5. Dependensi ekstensi ditambahkan otomatis sampai seluruh dependensi transitif terpenuhi.
6. Dependensi tidak dapat dihapus selama masih dibutuhkan oleh ekstensi aktif atau core.
7. Katalog ekstensi non-default berasal dari manifest upstream, sehingga menambahkan ekstensi yang mengikuti kontrak tidak membutuhkan boolean atau kartu UI baru dalam kode GoStarter. Core, infrastructure, dan konvensi default ditentukan profil internal aplikasi.
8. Perubahan upstream diusahakan cukup dengan satu manifest. Tidak mewajibkan pemindahan kode, penambahan placeholder, template `.tmpl`, atau refactor arsitektur khusus generator.
9. Template arsitektur/infrastructure, adapter transformasi, fixture generator, dan pengujian hasil generate dikelola di repository GoStarter. Template aplikasi merupakan mekanisme utama penyusunan arsitektur dan infrastructure, bukan hanya fallback saat source upstream tidak cukup.
10. ZIP hasil generate dan log validasi besar tidak disimpan. ZIP dibuat saat diminta dan dikirim langsung ke pengguna; validasi hanya menyimpan status serta ringkasan diagnostik yang dibatasi ukurannya.
11. REST-only dihapus dari cakupan saat ini karena belum tersedia di upstream. Dukungan REST-only direncanakan melalui branch upstream khusus di kemudian hari, bukan melalui konversi paksa dari upstream dual-protocol.

Bagian selanjutnya merupakan usulan pelaksanaan keputusan tersebut dan perlu direview sebelum implementasi.

## 2. Ruang lingkup awal

### Termasuk

- Satu upstream aktif dengan riwayat snapshot yang tervalidasi.
- Katalog ekstensi dinamis, pemilihan ekstensi, resolusi dependensi, dan penjelasan alasan dependensi terkunci.
- Penyusunan proyek untuk protokol yang secara eksplisit didukung snapshot upstream.
- Template arsitektur dan infrastructure milik GoStarter yang kompatibel dengan standar snapshot upstream.
- Penyesuaian metadata proyek, file, module/import, DI, infrastructure, transport, konfigurasi, dan dependency Go.
- Preview dan ZIP dari rencana proyek yang sama.
- Sinkronisasi upstream dengan validasi kompatibilitas dan aktivasi snapshot.
- Pengujian hasil generate terhadap standar upstream.

### Belum masuk tahap awal

- Editor kode manual di browser dan penyimpanan proyek hasil edit pengguna.
- Memperbarui atau merge ulang proyek yang sudah diunduh dan dimodifikasi pengguna.
- Penggabungan ekstensi dari beberapa repository atau marketplace.
- Pilihan arsitektur lain yang tidak disediakan kontrak upstream.
- Plugin executable atau script hook arbitrer yang dijalankan saat generate.
- REST-only dan pemilihan branch upstream khusus REST-only; keduanya merupakan pengembangan berikutnya setelah branch tersebut tersedia.

Kemampuan edit/tambah/hapus pada tahap awal berarti transformasi otomatis oleh generator. Editor manual dapat menjadi fitur terpisah bila diperlukan.

## 3. Pembagian tanggung jawab

| Area | Upstream menyiapkan | GoStarter mengerjakan |
| --- | --- | --- |
| Struktur dan arsitektur | Layout, aturan penempatan, pola hubungan layer | Mengikuti layout dan memvalidasi hasil |
| Core | Source sebagai acuan standar core | Menentukan default profil, membentuk core melalui template, dan menyertakan package wajib |
| Feature/usecase | Kode domain, constructor, handler, repository, migrasi | Memilih, menempatkan, dan merangkai feature |
| Pkg/library | Kode package, dependency, opsi konfigurasi | Menyertakan package yang dibutuhkan |
| Infrastructure | Standar implementasi, interface, provider, dan lifecycle yang harus diikuti | Memiliki template infrastructure dan menghasilkan implementasi/config/wiring yang diperlukan |
| Dependency injection | Pola DI serta kontrak provider/constructor ekstensi | Merender deklarasi DI melalui template aplikasi dan menghasilkan wiring |
| Transport | Handler/registrasi ekstensi serta protokol yang didukung branch aktif | Merender bootstrap server dan registrasi melalui template aplikasi yang kompatibel |
| Konfigurasi | Deklarasi key, tipe, default, dan kebutuhan ekstensi | Menggabungkan konfigurasi tanpa benturan |
| Dependency Go | Kebutuhan module dan aturan versi yang diuji | Menyusun go.mod/go.sum yang konsisten |
| Katalog UI | ID, kind, title, description, path, requires | Merender kelompok Features/Packages, kompatibilitas profil, pencarian, pilihan, dan lock |
| Kualitas | Tes ekstensi yang sudah tersedia | Menyiapkan fixture generator, matriks kompatibilitas, dan validasi hasil |

GoStarter adalah pemilik template arsitektur dan infrastructure. Kode upstream menjadi acuan standar, sedangkan implementasi feature/usecase dan pkg tetap diambil dari upstream. Manifest menghubungkan kebutuhan ekstensi dengan template aplikasi yang kompatibel. Jenis infrastructure baru memerlukan template/dukungan engine yang sesuai di GoStarter; entri manifest saja tidak menciptakan implementasi baru.

## 4. Artefak yang perlu dipersiapkan di upstream

Target perubahan upstream adalah menambahkan satu file kontrak di root repository:

```text
gostarter.yaml
```

Manifest mereferensikan kode asli di tempatnya. Tidak ada kewajiban mengubah `bootstrap/`, `infrastructure/`, `internal/`, atau `pkg/` menjadi template khusus generator.

Isi manifest minimum:

1. Manifest utama beserta versi kontrak.
2. Satu daftar ekstensi non-default dengan `kind: feature` atau `kind: pkg`.
3. ID, judul, deskripsi singkat, serta path source untuk setiap ekstensi.
4. Daftar `requires` yang hanya merujuk ekstensi non-default lain.

Core, struktur arsitektur, infrastructure, integrasi global, kompatibilitas, dan file default disimpan pada profil internal GoStarter.

Informasi yang bisa dibaca dari source, seperti module path dan deklarasi versi Go, tidak diduplikasi dalam manifest. Konvensi provider, handler, dan lifecycle disimpan pada profil GoStarter; manifest v1 tidak menyediakan metadata integrasi arbitrer.

Yang dipersiapkan di repository GoStarter: schema/validator manifest, template arsitektur dan infrastructure, adapter integrasi, operasi AST, fixture core/User/Auth, tes hasil generate, dan panduan manifest. Penambahan ekstensi bisnis tetap berupa kode ekstensi dan entri manifest di upstream.

Contoh pengelompokan template di GoStarter, dengan lokasi final ditentukan saat implementasi:

```text
templates/
  feature-modular-clean/
    bootstrap/
    infrastructure/
    transport/
    configuration/
```

Template tersebut mengikuti layout upstream saat menghasilkan file. Pemilihan profil/versi template ditentukan GoStarter dan diverifikasi terhadap source snapshot; manifest ekstensi tidak memuat standar arsitektur atau isi template. Versi kumpulan template aplikasi dicatat bersama snapshot upstream pada setiap rencana proyek.

Metadata generator dan workflow upstream tidak otomatis disalin ke proyek hasil generate. Path ekstensi dalam manifest dan ownership pada profil aplikasi bersama-sama menentukan file produk.

Jika template aplikasi belum mendukung kebutuhan ekstensi atau standar upstream tertentu, laporkan batas kompatibilitasnya dahulu dan lengkapi dukungan di GoStarter. Perubahan source upstream hanya menjadi pengecualian yang diidentifikasi secara spesifik; tidak menjadi syarat umum adopsi generator.

## 5. Kontrak manifest v1 — hanya non-default

Kontrak awal sudah dibuat pada `gostarter.yaml` di working tree upstream. Definisi normatif dan hasil inventory tersedia pada [kontrak v1](contracts/upstream-manifest-v1.md); bentuk data divalidasi menurut [schema JSON](contracts/gostarter.schema.json). Parser produksi tetap bagian roadmap.

Root hanya berisi `schema_version` dan satu daftar `extensions`. Feature/usecase dan pkg dibedakan melalui `kind`, tanpa dua katalog terpisah.

```yaml
schema_version: 1
extensions:
  - id: user
    kind: feature
    title: User Management
    description: Create, find, update, and delete user profiles.
    path: internal/user
    requires: [cryptoutil, validator]
  - id: auth
    kind: feature
    title: Authentication
    description: Login, logout, and refresh token rotation for users.
    path: internal/auth
    requires: [user, cryptoutil, validator]
  - id: cryptoutil
    kind: pkg
    title: Cryptographic Utilities
    description: Bcrypt password hashing, random tokens, and SHA-256 helpers.
    path: pkg/cryptoutil
  - id: validator
    kind: pkg
    title: Struct Validation
    description: Shared struct validation using go-playground validation tags.
    path: pkg/validator
```

Contoh merupakan subset tertutup dependensi dari katalog nyata. File upstream juga mendeklarasikan HTTP Client, Idempotency, Pagination, Pointer, String Utilities, dan Time Utilities.

### Batas kontrak

| Manifest upstream | Profil/template internal GoStarter |
| --- | --- |
| ID unik dan stabil | Daftar core default dan dependensinya |
| Kind `feature` atau `pkg` | Struktur/arsitektur dan template infrastructure |
| Title dan description singkat | Protokol dan kompatibilitas versi Go yang dibuktikan lewat tes |
| Path source ekstensi | Konvensi provider, handler, lifecycle, dan konfigurasi |
| Requires antar-ekstensi non-default | Ownership generated code, migrasi, dan source pendamping |

Field wajib per entri adalah `id`, `kind`, `title`, `description`, dan `path`; `requires` opsional dengan default daftar kosong. Judul maksimal 80 karakter, deskripsi maksimal 160 karakter, keduanya satu baris plain text tanpa whitespace di awal/akhir.

Tidak ada blok `core`, `architecture`, `infrastructure`, protokol, template, atau titik integrasi global pada manifest upstream. Konvensi default tidak diduplikasi per ekstensi. Field metadata integrasi tambahan memerlukan revisi kontrak yang eksplisit jika kelak dibutuhkan; jangan menebak pola yang belum didukung.

Daftar default awal di profil aplikasi meliputi Health serta Logger, Contextutil, Errorutil, Routine, Transaction, dan JWT Utils. JWT Utils tetap default karena middleware upstream mengimpornya; contoh diskusi lama yang menempatkannya sebagai ekstensi opsional tidak dipakai pada kontrak awal ini.

Validasi schema saja tidak cukup: periksa keberadaan path, unik ID/path, dependency hilang/siklus, import lintas-ekstensi, serta referensi SQL/proto. File pendamping di luar `path`, termasuk migrasi User/Auth dan OpenAPI gabungan, ditangani mapping/konvensi profil yang terdokumentasi dalam kontrak v1.

Menambah ekstensi yang mengikuti konvensi yang didukung tidak membutuhkan perubahan katalog GoStarter. Jenis integrasi atau file pendamping yang belum dikenal tetap memerlukan dukungan profil aplikasi sebelum ekstensi dinyatakan kompatibel. Komponen upstream yang tidak terdaftar sebagai default maupun ekstensi harus dilaporkan, tidak otomatis dianggap core.

## 6. Resolusi dependensi dan perilaku pilihan

Generator menyimpan dua himpunan yang berbeda:

- `requested_extensions`: pilihan eksplisit pengguna.
- `resolved_extensions`: pilihan pengguna dan seluruh dependensi transitif non-default.

Core/default dihitung dan ditampilkan terpisah dari profil aplikasi; komposisi proyek adalah gabungan default tersebut dan `resolved_extensions`. ID core tidak dimasukkan ke `requires` manifest.

Setiap ekstensi hasil resolusi memiliki informasi `selected_by_user`, `required_by`, dan `locked`. Satu ekstensi dapat dipilih pengguna sekaligus dibutuhkan ekstensi lain.

Aturan:

1. Core dan dependensi wajibnya dari profil aplikasi selalu aktif serta terkunci.
2. Memilih ekstensi otomatis menambahkan seluruh dependensi transitif.
3. Tombol hapus dinonaktifkan jika ekstensi masih dibutuhkan komponen aktif lain.
4. Dependensi bersama tidak dilepas selama masih ada pemakainya.
5. Menghapus ekstensi utama menghitung ulang seluruh graph.
6. Dependensi otomatis yang tidak lagi dibutuhkan ikut dilepas.
7. Ekstensi yang sebelumnya dipilih secara eksplisit tetap aktif setelah pemakainya dihapus.
8. Frontend tidak menjadi sumber kebenaran; backend selalu menghitung ulang resolusi.
9. ID tidak dikenal, dependensi hilang, konflik, atau siklus menghasilkan error yang menjelaskan jalur penyebabnya.

Contoh urutan:

| Tindakan | Hasil |
| --- | --- |
| Pilih Auth | Auth aktif; User, Cryptoutil, dan Validator ditambahkan; JWT Utils/Transaction sudah disediakan default profil |
| Coba hapus User | Ditolak/dikunci karena Auth membutuhkan User |
| Hapus Auth | Dependensi otomatis yang tidak lagi dibutuhkan dilepas |
| Pilih User, lalu Auth, lalu hapus Auth | User tetap aktif karena merupakan pilihan eksplisit |
| Hapus Auth sementara core membutuhkan PostgreSQL dan Transaction | PostgreSQL dan Transaction tetap aktif serta terkunci |

Untuk perubahan konfigurasi melalui API, pengguna mengirim daftar pilihan akhir. Menghilangkan sebuah dependency dari daftar tidak menonaktifkannya jika masih dibutuhkan; resolver menambahkannya kembali. Mengirim permintaan menonaktifkan dependency secara eksplisit, jika operasi tersebut kelak disediakan, harus ditolak.

## 7. Rencana proyek sebagai sumber hasil generate

Urutan proses yang diusulkan:

```text
Snapshot upstream tervalidasi
          +
Metadata proyek dan pilihan eksplisit pengguna
          ↓
Validasi input dan kompatibilitas
          ↓
Resolusi core, ekstensi, dan seluruh dependensi
          ↓
Rencana file, DI, infrastructure, transport, config, dan module
          ↓
Materialisasi dan transformasi pada salinan terisolasi
          ↓
Validasi hasil
          ↓
Preview dan ZIP
```

Rencana proyek mencatat:

- Snapshot ID dan full commit SHA upstream.
- Versi schema, architecture revision, versi generator, serta versi kumpulan template/adapter aplikasi.
- Konfigurasi proyek yang sudah dinormalisasi.
- Pilihan eksplisit dan ekstensi hasil resolusi beserta alasannya.
- File yang ditambahkan, disertakan, diubah, dan dikeluarkan.
- Keputusan transport, infrastructure, DI, module, dan konfigurasi.
- Fingerprint rencana dan diagnostik validasi.

Preview dan unduhan menggunakan rencana serta snapshot yang sama. Jika upstream tersinkronisasi di tengah sesi, ZIP tidak boleh diam-diam memakai template berbeda dari preview. Rencana yang kedaluwarsa harus menghasilkan pesan eksplisit untuk menghitung ulang preview.

ZIP dibuat ulang saat download dari rencana, versi generator/template/adapter, dan snapshot yang sama, lalu dikirim langsung ke response tanpa penyimpanan arsip hasil. Semua versi tersebut harus tetap tersedia selama plan berlaku; jika tidak, plan dinyatakan kedaluwarsa dan pengguna menghitung ulang preview. Transformasi harus deterministik untuk isi file. Validasi yang diwajibkan diselesaikan sebelum streaming dimulai; kegagalan saat streaming membatalkan transfer dan dicatat sebagai ringkasan error.

## 8. Metode tambah, ubah, dan hapus file

### Kepemilikan file

Setiap file hasil proyek harus memiliki pemilik yang jelas:

- **Core:** file/package dasar yang selalu disertakan dan file dasar yang dihasilkan template aplikasi.
- **Ekstensi:** file milik satu ekstensi; ikut masuk jika pemiliknya terpilih atau diperlukan.
- **Komposisi dan infrastructure:** file yang dirender dari template GoStarter sesuai standar upstream dan rencana ekstensi akhir.
- **Shared:** file bersama yang dimodelkan sebagai komponen/dependensi eksplisit, sehingga tidak terhapus hanya karena satu pemakai dihapus.

File di luar daftar yang dikenal tidak otomatis disalin. Dua pemilik yang menghasilkan path sama ditolak kecuali aturan penggabungannya ditentukan dalam kontrak.

Path bootstrap/infrastructure yang dimiliki template aplikasi tidak sekaligus disalin dari upstream. Pemetaan ownership menentukan bagian yang dihasilkan aplikasi dan bagian yang diambil dari source ekstensi, sehingga tidak ada overwrite implisit.

### Teknik transformasi

| Kebutuhan | Metode |
| --- | --- |
| Entrypoint, provider list, registrasi server | Render template arsitektur milik GoStarter menggunakan kontrak manifest dan ekstensi hasil resolusi |
| Infrastructure, koneksi resource, lifecycle | Render template infrastructure milik GoStarter sesuai standar upstream dan kebutuhan ekstensi |
| Mengubah import atau deklarasi Go terstruktur | Parsing AST dan pemeriksaan bentuk kode yang diharapkan |
| Module path, versi, metadata config | Edit format terstruktur berdasarkan nilai source asli dan metadata manifest; tidak mewajibkan placeholder baru di upstream |
| Menggabungkan env/config | Merge berdasarkan deklarasi key dan tipe |
| Mengeluarkan ekstensi | Tidak materialisasikan file miliknya; render ulang komposisi/infrastructure dan sesuaikan referensi terstruktur yang diperlukan |
| File biner | Salin tanpa transformasi teks |

Transformasi tidak boleh gagal diam-diam. Jika titik integrasi, simbol, atau bentuk kode yang diperlukan berubah, generator menghentikan proses dengan diagnostik yang menyebut snapshot, ekstensi, dan file terkait.

Untuk Wire, v1 perlu menetapkan strategi yang konsisten: render deklarasi provider/injector dari template aplikasi sesuai pola upstream, lalu jalankan tahap code generation yang terkendali sebelum validasi build. File hasil Wire harus mengikuti pilihan ekstensi dan tidak boleh sekadar memakai `wire_gen.go` upstream yang memuat semua feature. Versi tool dan tahap pengerjaannya harus menjadi bagian rencana implementasi; build/test berat dijalankan pada worker validasi, bukan diasumsikan selesai di HTTP handler.

Cache asli upstream tidak dimodifikasi. Setiap rencana menggunakan salinan/materialisasi tersendiri.

## 9. Kontrak transport dan infrastructure

### Transport

- `dual`: gRPC dan HTTP REST gateway aktif mengikuti standar upstream saat ini. REST gateway dalam mode dual tetap tersedia; yang dihapus adalah pilihan REST-only.
- `grpc`: dapat ditawarkan jika profil branch aktif dan template GoStarter mendukung serta lulus validasi; registrasi dan lifecycle harus konsisten.

REST-only tidak ditawarkan di UI, tidak dicantumkan dalam katalog protokol aktif, dan tidak menjadi fixture atau target implementasi tahap ini. API menolak `protocol=rest`, termasuk melalui jalur CLI/API lama, dengan pesan bahwa upstream aktif belum mendukung REST-only. Jangan mengubahnya diam-diam menjadi dual.

Dukungan REST-only berikutnya berasal dari branch upstream khusus yang direncanakan pengguna. Nama branch belum ditetapkan. Saat branch tersedia, manifest dan snapshot branch tersebut akan divalidasi terhadap kumpulan template aplikasi yang sesuai. GoStarter tidak membuat REST-only dengan menonaktifkan gRPC pada branch dual, dan tidak berpindah branch otomatis untuk memenuhi request yang belum didukung.

Jika kombinasi ekstensi/profil belum mendukung protokol terpilih berdasarkan pemeriksaan aplikasi, UI menampilkan alasannya dan backend menolak konfigurasi. Mengubah string protokol atau menghapus pemanggilan server saja tidak memenuhi kontrak.

### Infrastructure

- Kebutuhan infrastructure default dipetakan melalui profil integrasi aplikasi; manifest v1 hanya mencatat dependency antar-feature/pkg non-default.
- Generator merender implementasi infrastructure, config, provider, dan lifecycle dari template milik GoStarter yang mengikuti standar upstream.
- Infrastructure yang dibutuhkan otomatis ditambahkan serta terkunci.
- Kebutuhan berbagi koneksi harus disatukan; beberapa feature tidak menghasilkan beberapa instance database tanpa alasan.
- Profil awal mengikuti kebutuhan upstream saat ini, termasuk PostgreSQL bila tetap menjadi bagian core.
- PostgreSQL tetap dapat memakai Transaction sebagai dependensi wajib. Routine juga harus menjadi kebutuhan core jika bootstrap masih memakainya.

Istilah proyek minimal berarti hanya core dan dependensi wajibnya; bukan proyek tanpa dependency sama sekali.

## 10. Perubahan UI dan API

### UI generator

- Katalog sepenuhnya berasal dari API manifest aktif.
- Search dan pengelompokan Features/Packages berdasarkan `kind`, dengan `title` dan `description` dari manifest.
- Tanda berbeda untuk pilihan pengguna, dependency otomatis, dan core.
- Dependency menampilkan ikon lock dan alasan, misalnya `Dibutuhkan oleh Auth → User`.
- Jumlah pilihan eksplisit dan dependency otomatis ditampilkan terpisah.
- Informasi protokol/Go version yang tidak kompatibel muncul sebelum generate.
- Pilihan REST-only dihapus; API dan CLI menolak permintaan REST-only pada upstream aktif saat ini.
- Preview menampilkan hasil akhir setelah seluruh transformasi.
- Ringkasan hasil mencakup snapshot template dan daftar ekstensi efektif.
- Default admin diterapkan saat inisialisasi form; tidak menimpa perubahan yang sedang dilakukan pengguna.

### API yang diusulkan

| Endpoint | Peran |
| --- | --- |
| `GET /api/v1/info` | Metadata, default efektif, snapshot, katalog, kompatibilitas |
| `POST /api/v1/resolve` | Validasi pilihan dan menghasilkan rencana/resolusi |
| `POST /api/v1/explore` | Preview berdasarkan rencana |
| `POST /api/v1/generate` | ZIP berdasarkan rencana |
| `GET /starter.zip` | Jalur CLI yang memakai resolver dan pipeline yang sama |

Contoh permintaan resolusi:

```json
{
  "snapshot_id": "opaque-snapshot-id",
  "module": "github.com/acme/payment-service",
  "service_name": "payment-service",
  "project_version": "0.1.0",
  "protocol": "dual",
  "extensions": ["auth", "httpclient"]
}
```

Contoh bagian respons:

```json
{
  "plan_id": "opaque-plan-id",
  "requested_extensions": ["auth", "httpclient"],
  "resolved_extensions": [
    {"id": "auth", "selected_by_user": true, "locked": false, "required_by": []},
    {"id": "user", "selected_by_user": false, "locked": true, "required_by": ["auth"]}
  ]
}
```

Contoh respons dipersingkat; hasil nyata memuat seluruh dependency. Backend menyimpan atau merekonstruksi rencana dari data kanonik. Hasil resolusi kiriman client tidak dipercaya sebagai pengganti perhitungan backend.

API boolean lama dapat dipertahankan sementara sebagai adapter ke ID ekstensi. Perlu aturan eksplisit jika boolean dan `extensions` dikirim bersamaan; usulan v1: request ambigu ditolak. Parameter CLI/Base64 diseragamkan antara UI, handler, tes, dan dokumentasi.

## 11. Sinkronisasi dan aktivasi upstream

Usulan: sinkronisasi menghasilkan kandidat snapshot, bukan langsung mengganti template aktif.

1. Resolve branch/tag menjadi full commit SHA.
2. Unduh arsip berdasarkan SHA tersebut agar metadata dan isi berasal dari commit yang sama.
3. Baca serta validasi schema, graph dependensi, file, dan kontrak integrasi.
4. Jalankan fixture wajib milik GoStarter terhadap source snapshot tersebut pada worker validasi.
5. Catat hasil format/build/test yang diwajibkan.
6. Aktifkan kandidat secara atomik setelah pemeriksaan wajib lulus.
7. Jika gagal, pertahankan snapshot aktif terakhir dan tampilkan diagnostik kandidat.

Admin menampilkan snapshot aktif, kandidat, status pemeriksaan, error yang dapat ditindaklanjuti, dan pilihan rollback ke snapshot valid yang masih disimpan.

Kebijakan jumlah snapshot, masa berlaku rencana, dan durasi validasi perlu dikonfigurasi. Snapshot yang masih dipakai rencana aktif tidak dihapus sebelum rencana berakhir.

Pengaturan interval sinkronisasi admin harus benar-benar memperbarui jadwal worker. Perubahan repository/branch menjalankan validasi kandidat baru, bukan memakai cache lama yang tidak memiliki identitas sumber.

## 12. Validasi dan kriteria penerimaan

Validasi dibagi menjadi pemeriksaan cepat pada setiap request dan pemeriksaan kompilasi/integrasi pada worker atau CI. Proyek tidak boleh diberi status berhasil divalidasi jika tahap yang diwajibkan belum selesai.

### Pemeriksaan cepat

- Metadata proyek, module path, nama layanan, port, dan versi valid.
- Graph dependensi lengkap, tanpa siklus, konflik, atau ID tidak dikenal.
- Semua output path dan kepemilikan file valid.
- Semua path manifest serta simbol/template/adapter yang diwajibkan profil aplikasi tersedia dan kompatibel.
- Kompatibilitas Go dan protokol terpenuhi.
- AST hasil transformasi dapat diparse dan diformat.

### Matriks build dan integrasi

Minimal mencakup:

1. Core minimal.
2. Setiap ekstensi secara individual beserta dependency transitifnya.
3. Seluruh ekstensi yang dapat dipakai bersama.
4. Auth + User pada dual dan gRPC, jika didukung profil upstream dan template aplikasi; uji penolakan request REST-only.
5. Kombinasi dependency bersama dan pelepasan ekstensi utama.
6. Setiap versi Go yang ditawarkan sebagai kompatibel.
7. Skenario perubahan kontrak dan kegagalan upstream.

Menguji seluruh kombinasi ekstensi dapat terlalu besar. Kombinasi per dependency cluster dan area interaksi berisiko harus ditambahkan ke matriks; setiap rencana tetap memerlukan pemeriksaan konsistensi. Keberhasilan matriks tidak boleh diklaim sebagai bukti semua kombinasi runtime sudah teruji.

### Daftar penerimaan yang bisa direview

- [ ] Menambah ekstensi sesuai kontrak di upstream membuatnya muncul di katalog tanpa perubahan kode GoStarter.
- [ ] Upstream yang mengikuti pola yang didukung dapat diintegrasikan dengan penambahan satu manifest, tanpa refactor direktori atau konversi source menjadi `.tmpl`.
- [ ] Source upstream dan cache aslinya tetap utuh; transformasi hanya dilakukan pada hasil proyek.
- [ ] Auth otomatis menambahkan seluruh dependency dan UI tidak mengizinkan penghapusannya selama dibutuhkan.
- [ ] Pemanggilan API langsung menghasilkan aturan dependency yang sama dengan UI.
- [ ] Menghapus feature tidak meninggalkan import, provider, handler, atau referensi config yang menggantung.
- [ ] Dependency bersama dan pilihan eksplisit dipertahankan sesuai aturan.
- [ ] Core minimal berhasil dikompilasi dengan template nyata, bukan hanya mock.
- [ ] REST-only tidak muncul pada UI/katalog aktif dan request REST-only ditolak oleh seluruh jalur API/CLI saat ini.
- [ ] Template arsitektur dan infrastructure berada di GoStarter; file hasil render mengikuti struktur serta kontrak upstream.
- [ ] Manifest hanya memuat feature/pkg non-default dengan title dan description; struktur hasil generate mengikuti profil aplikasi dan standar upstream.
- [ ] DI, infrastructure, dan lifecycle hanya memuat komponen yang diperlukan.
- [ ] Daftar dependency Go konsisten dengan kode hasil generate.
- [ ] Preview dan ZIP dari plan yang sama memiliki path serta isi file yang sama; metadata timestamp ZIP boleh berbeda.
- [ ] Sync bersamaan tidak mengubah hasil plan yang sudah dibuat.
- [ ] Kandidat upstream gagal validasi tidak menggantikan snapshot aktif.
- [ ] Error transformasi tidak menghasilkan ZIP yang terlihat sukses.
- [ ] Download tidak menyimpan ZIP hasil generate dan validasi tidak menyimpan log besar; hanya status serta ringkasan diagnostik berukuran terbatas yang dipersistenkan.
- [ ] Default dan pengaturan admin yang ditawarkan benar-benar diterapkan aplikasi.

## 13. Perubahan yang diperlukan pada GoStarter saat ini

| Area kode saat ini | Perubahan yang diperlukan |
| --- | --- |
| `internal/generator/model.go` | Ganti boolean tetap dengan pilihan ekstensi dinamis dan model rencana |
| `internal/generator/transform.go` | Pisahkan perencanaan file, render komposisi, transformasi terstruktur, dan validasi |
| `internal/generator/explore.go` | Bangun preview dari hasil materialisasi rencana |
| `internal/generator/zipper.go` | Kemas hasil yang sudah tervalidasi; propagasikan error penulisan/penutupan ZIP |
| `internal/source/fetcher.go` | Ambil full SHA dan unduh snapshot immutable |
| `internal/source/cache.go` | Identitas snapshot, kandidat/aktif, aktivasi atomik, retensi |
| `internal/handler/api.go` | Katalog dinamis, endpoint resolve, pipeline bersama, error terstruktur |
| `assets/js/app.js` | Hapus katalog hardcoded; tampilkan resolusi, lock, alasan, kompatibilitas |
| Admin/config/database | Hubungkan pengaturan runtime, status validasi, dan snapshot aktif |
| Tes dan workflow | Gunakan fixture template nyata serta matriks hasil generate |

Nama package baru dan pembagian file internal aplikasi dapat ditetapkan saat implementasi. Struktur internal GoStarter tidak harus sama dengan struktur proyek yang dihasilkannya.

## 14. Tahapan implementasi dan hasil review

| Tahap | Yang disiapkan | Bukti selesai |
| --- | --- | --- |
| 1. Implementasi kontrak | Parser/validator schema awal, graph ekstensi, ownership dan konvensi pada profil aplikasi | Manifest upstream yang sudah dibuat lolos validator produksi |
| 2. Resolver | Pilihan eksplisit/efektif, dependency lock, kompatibilitas, plan | Tes graph dan API resolusi lulus |
| 3. Generator | Template arsitektur/infrastructure milik aplikasi, adapter, transformasi AST, DI, config/module | Core minimal dan fixture feature berhasil build |
| 4. UI/API | Katalog dinamis, lock/alasan, preview/ZIP dengan plan | UI dan CLI menghasilkan proyek setara |
| 5. Upstream lifecycle | Kandidat, validasi, aktivasi, rollback, retensi | Kandidat gagal tidak mengganggu snapshot aktif |
| 6. Kualitas dan migrasi | Matriks protokol/Go, kompatibilitas API lama, dokumentasi | Seluruh kriteria penerimaan yang disepakati terpenuhi |

Pekerjaan utama berada di GoStarter. Persiapan upstream difokuskan pada manifest yang mereferensikan kode asli. Tahap generator belum selesai bila template/adapter aplikasi belum dapat mengintegrasikan ekstensi sesuai kontrak dan standar upstream.

## 15. Keputusan yang masih perlu direview

Keputusan berikut belum ditetapkan pengguna; nilai di kolom usulan adalah rancangan awal:

| Keputusan | Usulan awal | Konsekuensi |
| --- | --- | --- |
| Bentuk modifikasi file | Otomatis pada pipeline generate | Editor manual browser menjadi fase terpisah |
| Auth dan User | Auth memerlukan User bawaan | Kontrak lebih sederhana; provider eksternal menyusul |
| Database proyek hasil generate | PostgreSQL mengikuti kebutuhan core upstream saat ini | Selalu terkunci jika core masih membutuhkannya; terpisah dari penyimpanan GoStarter |
| Penyimpanan GoStarter | PostgreSQL untuk data kontrol; persistent volume untuk artefak | Mengikuti pembagian pada bagian 16 |
| Evolusi manifest | YAML schema v1 awal sudah disiapkan | Field baru hanya ditambahkan saat kebutuhan non-konvensional terbukti |
| Wire | Render deklarasi dari template aplikasi lalu jalankan code generation terkontrol | Memerlukan toolchain serta worker yang sesuai |
| Aktivasi upstream | Otomatis setelah pemeriksaan wajib lulus | Sync lebih lama, snapshot aktif tetap tersedia |
| API lama | Adapter sementara dengan batas deprecation | Transisi CLI/UI lebih mudah, perlu aturan request ambigu |

Kontrak awal, schema struktural, dan manifest upstream dibuat sebelum eksekusi roadmap. Tahap berikutnya mengikuti roadmap; implementasi parser/validator produksi, profil/template, serta fixture core minimal/User/Auth belum dikerjakan.

Kepemilikan template arsitektur/infrastructure di GoStarter serta penghapusan REST-only dari cakupan saat ini sudah diputuskan pengguna, sehingga tidak termasuk keputusan terbuka di tabel ini. Branch REST-only merupakan rencana pengembangan berikutnya.

## 16. Penyimpanan aplikasi GoStarter

### Kondisi saat ini

PostgreSQL sudah dipakai untuk akun admin (`admin_users`), sesi (`admin_sessions`), dan pengaturan (`admin_settings`). Arsip template tersimpan di disk `.cache/boilerplate.zip` dan dibaca ke memori. Proyek hasil generate tidak disimpan sebagai data proyek permanen di PostgreSQL.

### Usulan pembagian penyimpanan

| Data | Penyimpanan | Masa simpan/peran |
| --- | --- | --- |
| Akun, sesi, pengaturan admin | PostgreSQL | Data persisten; sesi memiliki expiry |
| Identitas snapshot, full SHA, checksum, manifest terverifikasi, versi kontrak | PostgreSQL | Metadata untuk katalog, reproduksi, dan validasi |
| Referensi snapshot aktif dan status kandidat | PostgreSQL | Sumber kebenaran aktivasi snapshot |
| Status job sinkronisasi/validasi dan ringkasan diagnostik | PostgreSQL | Riwayat operasional berukuran terbatas dengan kebijakan retensi; tanpa log mentah besar |
| Rencana proyek, konfigurasi kanonik, fingerprint, referensi snapshot/template/adapter | PostgreSQL, payload terstruktur/JSONB | TTL; bukan workspace pengguna permanen |
| Arsip source upstream | Persistent volume | Snapshot immutable untuk generate, validasi, dan rollback |
| File hasil materialisasi untuk preview/build/validasi | Memori atau direktori kerja sementara | Dibersihkan setelah proses selesai; dapat dibentuk ulang dari plan dan snapshot |
| ZIP hasil generate | Tidak disimpan | Dibuat saat download dan langsung dikirim melalui response stream |
| Output validasi mentah | Tidak diarsipkan | Diproses dengan buffer terbatas untuk mengambil ringkasan error, lalu dibuang |
| Map file/katalog untuk mempercepat pembacaan | Memori proses | Cache yang dapat dibangun ulang |

Usulan tabel tambahan: `template_snapshots`, `upstream_state`, `generation_plans`, dan `validation_jobs`. Schema SQL final mengikuti kebutuhan implementasi; tabel akun/sesi/pengaturan yang ada tetap digunakan. Manifest di database merupakan salinan terverifikasi dari snapshot, bukan katalog terpisah yang diedit hingga berbeda dari upstream.

Arsip source upstream ditulis ke lokasi sementara, diverifikasi, lalu difinalkan pada path immutable. Setelah arsip siap, transaksi PostgreSQL dapat memperbarui referensi snapshot aktif. Jika proses gagal sebelum aktivasi, snapshot aktif sebelumnya tetap dipakai dan arsip yatim dapat dibersihkan kemudian. Jangan menganggap transaksi database juga mengatomikkan operasi filesystem. Mekanisme ini hanya untuk snapshot upstream, bukan penyimpanan ZIP hasil generate.

Untuk deployment awal satu instance, persistent volume sudah cukup sebagai lokasi snapshot upstream. Bila memakai beberapa instance atau worker terpisah, semuanya harus memiliki akses ke snapshot yang sama; object storage dapat digunakan pada tahap itu tanpa mengubah kontrak upstream. PostgreSQL sendiri tidak menggantikan kebutuhan akses bersama ke file source. Tidak diperlukan storage untuk ZIP hasil generate atau arsip log validasi besar.

Ketersediaan PostgreSQL menjadi syarat untuk membuat plan persisten dan mengganti snapshot aktif dalam rancangan ini. Cache yang lengkap dapat tetap melayani katalog atau artefak lama sesuai kebijakan fallback, tetapi operasi yang membutuhkan state baru harus mengembalikan status tidak tersedia secara jelas.

Database GoStarter dan database aplikasi hasil generate adalah dua hal berbeda. Pemakaian PostgreSQL untuk menyimpan akun admin atau plan GoStarter tidak otomatis mewajibkan PostgreSQL pada setiap proyek hasil generate; kebutuhan database proyek ditentukan oleh core dan dependensi upstream.

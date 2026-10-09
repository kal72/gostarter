# Roadmap Development GoStarter

Tanggal: 8 Oktober 2026.

Status: tahap 0–7 selesai. Pengembangan kode tahap 8 selesai; pekerjaan operasional/staging dilewati atas instruksi pengguna, tanpa menyatakan gate rilis lulus. [Hasil tahap 0](stage-0-baseline.md) · [Hasil tahap 1](stage-1-contract-profile.md) · [Hasil tahap 2](stage-2-resolver-plan.md) · [Hasil tahap 3A](stage-3a-core-templates.md) · [Hasil tahap 3B](stage-3b-feature-pkg.md) · [Hasil tahap 3C](stage-3c-materialization.md) · [Hasil tahap 4](stage-4-storage.md) · [Hasil tahap 5](stage-5-worker.md) · [Hasil tahap 6](stage-6-api.md) · [Hasil tahap 7](stage-7-ui-admin.md) · [Hasil lokal tahap 8](stage-8-release.md).

Dokumen acuan: [Rancangan dan kontrak upstream](generator-upstream-contract-review.md).

Kontrak sebelum roadmap: [Kontrak upstream v1](contracts/upstream-manifest-v1.md) dan [schema JSON](contracts/gostarter.schema.json). File `gostarter.yaml` sudah dipush pengguna dan tersedia pada commit upstream `05cf3ee561ddd7a454c2d9641c540d4c7d750ebb` (`dev`); sudah dikonsumsi parser, resolver, dan renderer development; integrasi API/worker produksi mengikuti tahap berikutnya.

## 1. Target rilis awal

GoStarter mampu membaca katalog ekstensi dari manifest upstream, menambahkan dan mengunci dependensi otomatis, lalu menghasilkan proyek yang mengikuti struktur dan arsitektur upstream. Template arsitektur dan infrastructure dimiliki GoStarter; implementasi feature/usecase dan pkg diambil dari upstream.

Hasil akhir yang harus dapat didemonstrasikan:

1. Core minimal, User, dan Auth berhasil digenerate serta dikompilasi dari source upstream nyata.
2. Ekstensi baru dengan jenis integrasi yang sudah didukung dapat ditambahkan lewat kode ekstensi dan manifest upstream tanpa mengubah katalog/UI atau engine GoStarter.
3. Preview dan ZIP berasal dari plan, snapshot, serta versi template aplikasi yang sama.
4. ZIP dikirim langsung tanpa disimpan; log validasi besar tidak diarsipkan.
5. Upstream yang gagal validasi tidak menggantikan snapshot aktif.

### Batas yang sudah ditetapkan

- Perubahan upstream diusahakan hanya penambahan `gostarter.yaml`; tidak memindahkan direktori atau mengubah source menjadi `.tmpl`.
- Manifest hanya mendeklarasikan non-default dalam satu `extensions`: `kind: feature | pkg`, `id`, `title`, `description`, `path`, dan `requires` opsional. Core, infrastructure, arsitektur, protokol, dan titik integrasi global disimpan pada profil internal GoStarter, bukan manifest upstream.
- GoStarter membuat template bootstrap, DI, infrastructure, transport, dan konfigurasi sendiri mengikuti standar upstream.
- Dependency transitif otomatis ditambahkan dan tidak bisa dihapus selama diperlukan.
- PostgreSQL menyimpan data kontrol, metadata snapshot, plan, status job, dan ringkasan diagnostik terbatas.
- Persistent volume hanya menyimpan snapshot source upstream. Workspace build/validasi bersifat sementara.
- REST-only dikeluarkan dari UI, API, template aktif, dan target rilis awal. Dukungan berikutnya menunggu branch upstream khusus.
- Dual tetap mencakup gRPC dan HTTP REST gateway. gRPC-only hanya ditawarkan setelah manifest, template, dan tes menyatakan kompatibel.
- Tidak mencakup editor kode manual, merge proyek yang sudah diunduh, marketplace, atau multi-upstream dalam satu proyek.

## 2. Urutan, status, dan milestone

Prasyarat sebelum tahap 0: kontrak awal ditulis pada upstream dan didokumentasikan di GoStarter. Artefak awal sudah tersedia; pemeriksaan kontrak tidak berarti tahap 0 atau tahap 1 implementasi telah dijalankan. Validasi produksi dan tes hasil generator tetap tugas roadmap.

| Tahap | Fokus | Prasyarat | Status | Hasil yang dapat direview |
| --- | --- | --- | --- | --- |
| 0 | Baseline dan penyelarasan scope | Tidak ada | Selesai | Baseline tes, inventory awal, REST-only dihapus |
| 1 | Implementasi kontrak dan profil default | 0 + kontrak awal | Selesai | Parser/validator v1 dan ownership profil aplikasi |
| 2 | Resolver dan model plan | 1 | Selesai | Dependency graph, alasan lock, konfigurasi kanonik |
| 3 | Template aplikasi dan generator | 2 | Selesai | Core/User/Auth berhasil build dari template nyata |
| 4 | PostgreSQL dan snapshot immutable | 1–3 | Selesai | Metadata persisten, storage snapshot, plan repository |
| 5 | Worker validasi dan aktivasi snapshot | 3–4 | Selesai | Kandidat tervalidasi, aktivasi aman, rollback |
| 6 | API plan, preview, dan ZIP streaming | 2–5 | Selesai | Jalur UI/API/CLI memakai pipeline yang sama |
| 7 | UI dinamis dan admin | 6 | Selesai | Katalog manifest, dependency lock, status snapshot |
| 8 | Integrasi, migrasi, dan kesiapan rilis | 0–7 | Kode lokal selesai; operasional dilewati | Matriks tes lulus dan release checklist selesai |

Milestone A: selesai tahap 2, kontrak dan resolusi stabil.

Milestone B: selesai tahap 3, generator lokal membuktikan hasil proyek dapat dibangun.

Milestone C: selesai tahap 6, backend baru dapat dipakai lewat API/CLI dengan state persisten.

Milestone D: selesai tahap 8, rilis awal siap dipromosikan setelah validasi staging.

Urutan tersebut adalah jalur utama eksekusi. Perkiraan kalender ditentukan setelah tahap 0–1 mengungkap jumlah integrasi nyata; tidak mengasumsikan semua pkg atau feature saling independen.

## 3. Cara mengeksekusi roadmap

- Kerjakan satu tahap atau bagian kecilnya dalam satu perubahan yang mudah direview. Tahap 3 dipecah menjadi 3A–3C.
- Gunakan checklist tugas dan gate selesai di bawah sebagai sumber status; jangan menandai selesai hanya karena kode sudah ditulis.
- Setelah satu tahap: catat perubahan, hasil pengujian, keterbatasan, serta langkah berikutnya pada tabel status/catatan eksekusi.
- Gate selesai adalah syarat teknis untuk lanjut, bukan kewajiban meminta persetujuan ulang pada setiap tahap.
- Jika pengguna meminta seluruh roadmap dieksekusi, lanjut berurutan selama tidak ada keputusan produk baru atau blocker eksternal. Jika meminta satu tahap, batasi implementasi pada tahap tersebut.
- Pertahankan perubahan pengguna yang sudah ada. Jangan menganggap file yang sudah staged sebagai perubahan yang dibuat untuk roadmap.
- Ikuti instruksi repository, termasuk penggunaan prefix `rtk` untuk perintah shell.
- Jangan mengaktifkan pipeline yang belum memenuhi kontraknya di lingkungan produksi. Selama pembangunan, gunakan fixture dan jalur development yang eksplisit.

## 4. Tahap 0 — Baseline dan penyelarasan scope

**Tujuan:** mendapatkan landasan pengujian dan menghapus janji fitur yang tidak lagi masuk scope.

Tugas:

- [x] T0.1 Catat versi toolchain yang benar-benar berhasil build/test; selesaikan ketidakselarasan Go lokal tanpa menurunkan versi module untuk menyembunyikan error.
- [x] T0.2 Jalankan tes/race/vet yang relevan dan simpan ringkasan hasil, termasuk keterbatasan tes database saat ini.
- [x] T0.3 Perluas inventory awal pada kontrak v1 menjadi inventory eksekusi snapshot: core, feature, pkg, infrastructure, generated protobuf/OpenAPI, migrasi, serta import antar-komponen. Default tidak dipindahkan ke manifest.
- [x] T0.4 Tandai cache yang belum memiliki full commit SHA sebagai fixture analisis, bukan snapshot produksi tervalidasi. Jangan menebak SHA asalnya.
- [x] T0.5 Hapus REST-only dari katalog backend, form UI/admin, logika transformasi, contoh README/cURL, dan tes yang mengharapkan dukungan REST-only.
- [x] T0.6 Tolak `protocol=rest` pada POST generate/explore, GET starter.zip, dan payload Base64; pertahankan HTTP REST gateway pada dual.
- [x] T0.7 Tangani default admin lama bernilai `rest` secara eksplisit melalui migrasi ke default dual yang tercatat. Input pengguna `rest` tetap ditolak, tidak dikonversi diam-diam.

Area utama: `internal/handler`, `internal/generator`, `web`, `internal/db`, `README.md`.

Gate selesai:

- Tidak ada jalur publik yang masih menghasilkan REST-only.
- Tes penolakan REST-only mencakup UI metadata, JSON, query, dan Base64.
- Baseline build/test dapat direproduksi dan inventory awal tersedia.

Temuan persiapan kontrak: build upstream lulus, tetapi `TestLoad_Defaults`, `TestLoad_DotEnv`, dan `TestLoad_EnvOverride` pada `infrastructure/config` gagal di checkout bersih tanpa `.env`. Tetapkan fixture/default konfigurasi yang reproducible pada tahap ini; jangan menyembunyikan kegagalan dengan melewati tes. Jika diperlukan perubahan source upstream, identifikasi perubahan kecil yang spesifik terhadap config/test tersebut sebelum pelaksanaannya; jangan memperluasnya menjadi refactor arsitektur.

## 5. Tahap 1 — Implementasi kontrak dan profil default

**Tujuan:** mengimplementasikan parser/validator kontrak awal yang sudah dibuat, beserta ownership dan konvensi pada profil internal aplikasi.

Tugas:

- [x] T1.1 Implementasikan model typed/parser dari schema v1 yang sudah disiapkan: root `schema_version` + `extensions`; setiap entri memiliki `id`, `kind`, `title`, `description`, `path`, dan `requires` opsional. Tolak field asing, duplicate key, alias/merge/custom tag, dan versi tak didukung.
- [x] T1.2 Bedakan sumber file secara eksplisit: kode upstream yang disalin, output template GoStarter, shared component, dan file yang dikeluarkan. Referensi source standar tidak otomatis berarti file tersebut disalin.
- [x] T1.3 Petakan semua file penting, termasuk protobuf, generated registration, OpenAPI, migrasi, dan file konfigurasi. Tidak ada penghapusan hanya berdasarkan nama direktori feature.
- [x] T1.4 Implementasikan konvensi provider/constructor, registrasi service, konfigurasi, resource lifecycle, module requirements, konflik, dan kompatibilitas pada profil aplikasi sesuai source nyata. Manifest v1 tidak memuat field integrasi arbitrer.
- [x] T1.5 Tetapkan registry template milik aplikasi berdasarkan architecture ID/revision, dengan versi template terpisah dari schema dan commit upstream.
- [x] T1.6 Implementasikan parser/validator dengan error terstruktur: kode, extension ID, file/simbol, dan penyebab.
- [x] T1.7 Uji manifest `gostarter.yaml` yang sudah dibuat di upstream dengan validator produksi dan fixture GoStarter. Koreksi hanya jika source membuktikan perlu; hindari perubahan kode bisnis upstream.
- [x] T1.8 Lengkapi panduan kontrak awal dengan contoh penambahan feature/pkg dan tes negatif. Data yang bisa diturunkan dari `go.mod` atau source tidak diduplikasi tanpa kebutuhan.

Usulan lokasi: `internal/manifest/`, registry pada `internal/generator/`, input unit test kecil di dalam kode tes, panduan di `docs/`.

Gate selesai:

- Manifest kandidat lolos validator terhadap source nyata.
- Duplicate ID, path bentrok/tidak valid, dependency hilang/siklus, schema tak dikenal, dan simbol yang hilang menghasilkan error yang jelas.
- Kepemilikan output template dan file upstream tidak bertabrakan.
- Inventory membedakan dependency wajib core dari library yang memang opsional. Logger/Contextutil/Errorutil/Routine/Transaction/JWT Utils tetap default sesuai profil awal; tidak menjadi pilihan yang dapat dihapus dalam manifest.

Manifest awal sudah tersedia pada commit upstream `05cf3ee561ddd7a454c2d9641c540d4c7d750ebb` (`dev`). Tahap 5 tetap harus mengunduh dan memvalidasi commit/ref yang dipilih secara immutable sebelum aktivasi produksi; fallback manifest tersembunyi tidak dibolehkan.

## 6. Tahap 2 — Dependency resolver dan plan kanonik

**Tujuan:** satu keputusan resolusi yang digunakan UI, API, CLI, preview, dan generator.

Tugas:

- [x] T2.1 Ganti model inti boolean per ekstensi dengan `requested_extensions`; compatibility adapter lama tetap terpisah dari resolver.
- [x] T2.2 Hitung closure dependency ekstensi dari pilihan pengguna dan gabungkan dengan default profil aplikasi, deduplikasi, urutan stabil, serta seluruh alasan `required_by`. `requires` manifest hanya merujuk ID ekstensi lain, tidak ID core.
- [x] T2.3 Hasilkan `selected_by_user`, `locked`, dan dependency path untuk penjelasan UI.
- [x] T2.4 Terapkan konflik serta kompatibilitas Go/protokol pada seluruh hasil resolusi, bukan hanya pilihan langsung.
- [x] T2.5 Normalisasi metadata proyek dan bentuk plan: source snapshot, template/engine version, resolved extensions, file actions, konfigurasi efektif, dan fingerprint.
- [x] T2.6 Buat resolver sebagai fungsi yang tidak mengubah cache/source dan dapat diuji tanpa server/database.

Gate selesai melalui tes perilaku:

- Pilih Auth → seluruh dependency tersedia dan terkunci.
- Hapus Auth → dependency otomatis yang tidak diperlukan lagi dilepas.
- Pilih User, lalu Auth, lalu hapus Auth → User tetap aktif.
- Dependency bersama/core tetap aktif setelah salah satu pemakai dihapus.
- Menghilangkan dependency dari request tidak mematikannya jika masih dibutuhkan.
- Input ekuivalen menghasilkan resolusi/fingerprint stabil; perubahan snapshot/template mengubah identitas plan.
- Siklus, konflik, ID tidak dikenal, protokol unsupported, dan versi Go incompatible ditolak.

## 7. Tahap 3 — Template GoStarter dan mesin generator

**Tujuan:** membuktikan komposisi source upstream dan template aplikasi menghasilkan proyek Go yang utuh.

### 3A. Core dan infrastructure

- [x] T3.1 Buat template entrypoint, bootstrap, DI, transport dual, konfigurasi, logging/telemetry, serta infrastructure yang diperlukan core, mengikuti standar upstream.
- [x] T3.2 Embed/version template pada aplikasi; tetapkan kompatibilitas profil terhadap struktur/simbol snapshot upstream. ID/revision profil tidak diwajibkan menjadi field manifest ekstensi.
- [x] T3.3 Render provider, koneksi bersama, resource initialization, dan shutdown. Jangan membuat instance database terpisah untuk tiap feature tanpa kebutuhan.
- [x] T3.4 Hilangkan registrasi endpoint User/Auth yang tetap dari core minimal; health dan komponen core lain mengikuti profil tahap 1. AuthInterceptor dan JWT Utils tetap default pada profil awal agar penghapusan feature Auth tidak menghilangkan validasi identitas pada endpoint User; perubahan kebijakan ini memerlukan review terpisah.

Gate 3A: proyek core minimal berhasil build tanpa dependency lokal hilang, dan struktur output mengikuti upstream.

### 3B. Feature dan pkg

- [x] T3.5 Integrasikan User, lalu Auth, melalui manifest dan extension source yang asli.
- [x] T3.6 Render provider/injector Wire dan jalankan versi codegen yang ditetapkan. Jangan membawa wiring upstream yang masih mereferensikan feature tidak terpilih.
- [x] T3.7 Ubah module/import serta referensi Go melalui operasi terstruktur; jaga protobuf import, generated code, dan konfigurasi deployment agar konsisten.
- [x] T3.8 Gabungkan env/config dan module requirements. Pastikan `go.mod`/`go.sum` cocok dengan hasil komposisi; gunakan toolchain serta dependency versions yang ditetapkan.
- [x] T3.9 Integrasikan seluruh ekstensi yang hendak ditawarkan saat rilis. Ekstensi yang belum dapat divalidasi tidak boleh ditandai supported.

Gate 3B: core, User, dan Auth build dari source nyata; menghapus feature tidak meninggalkan provider, import, config, migrasi, atau registrasi yang menggantung.

### 3C. Konsistensi materialisasi

- [x] T3.10 Hasilkan file actions dengan owner dan alasan; cegah output template menimpa source ekstensi tanpa aturan.
- [x] T3.11 Pastikan pipeline tidak mengubah byte source/cache asli dan menghasilkan isi file deterministik untuk input/versi yang sama.
- [x] T3.12 Tambahkan template gRPC-only hanya jika sesuai kontrak upstream dan lulus build/runtime test. Jangan buat template REST-only.
- [x] T3.13 Tetapkan identitas toolchain/codegen dan environment yang memengaruhi output. Nilai acak/waktu tidak boleh mengubah source antara preview dan download.

Area utama: `templates/`, `internal/generator/`, test fixtures. Prefix package internal baru dapat disesuaikan agar tetap sederhana.

Gate tahap 3: jalur generator lokal menghasilkan file yang parse/format/build dengan benar; perubahan tak kompatibel pada upstream menimbulkan error, bukan transformasi parsial diam-diam.

## 8. Tahap 4 — Storage metadata dan source snapshot

**Tujuan:** menyimpan identitas source/plan secara persisten tanpa menyimpan proyek hasil generate.

Tugas:

- [x] T4.1 Tambahkan migrasi additive untuk `template_snapshots`, `upstream_state`, `generation_plans`, dan `validation_jobs`; pertahankan akun/sesi/pengaturan lama.
- [x] T4.2 Simpan snapshot identity, full SHA, checksum, manifest terverifikasi, kompatibilitas, status, dan referensi lokasi arsip.
- [x] T4.3 Ubah fetcher: resolve ref ke full SHA lalu unduh isi commit yang sama; jangan mengambil metadata dan arsip dari branch bergerak secara terpisah.
- [x] T4.4 Simpan arsip per snapshot pada volume persisten melalui staging file dan finalisasi immutable.
- [x] T4.5 Simpan konfigurasi plan, hasil resolusi, snapshot/template/engine versions, expiry, status validasi, dan digest hasil. Tidak menyimpan source output atau ZIP dalam PostgreSQL.
- [x] T4.6 Implementasikan retention snapshot yang mempertahankan snapshot aktif, kandidat yang sedang diproses, dan snapshot yang masih dirujuk plan valid.
- [x] T4.7 Atur masa berlaku plan, jumlah snapshot yang dipertahankan, batas job, dan ukuran ringkasan diagnostik melalui config tervalidasi.
- [x] T4.8 Hubungkan parameter connection pool dan session lifetime ke implementasi agar config yang ditawarkan benar-benar digunakan.

Gate selesai:

- Tes PostgreSQL nyata membuktikan migrasi, repository metadata, expiry plan, dan restart persistence.
- Dua repository/ref tidak berbagi cache tanpa identitas; full SHA dan checksum terverifikasi.
- Proses gagal tidak mengganti referensi snapshot aktif.
- Snapshot yang masih diperlukan tidak terhapus oleh cleanup.
- Tidak ada tabel/blob/path permanen untuk ZIP hasil atau log validasi besar.

Candidate belum otomatis aktif pada tahap ini; aktivasi memerlukan pemeriksaan tahap 5.

## 9. Tahap 5 — Worker validasi dan aktivasi snapshot

**Tujuan:** setiap snapshot aktif kompatibel dengan engine/template aplikasi dan hasil proyek memenuhi pemeriksaan yang diwajibkan.

Tugas:

- [x] T5.1 Implementasikan worker dengan concurrency terbatas dan job state di PostgreSQL; rilis awal tidak membutuhkan broker tambahan.
- [x] T5.2 Tentukan job claim/lease/retry yang aman terhadap restart dan proses bersamaan. Kegagalan tidak menciptakan status sukses atau job yang selamanya running.
- [x] T5.3 Jalankan validasi manifest, materialisasi, codegen, format/build, dan fixture wajib pada temporary workspace dengan deadline/resource budget terkonfigurasi.
- [x] T5.4 Ambil output command melalui buffer terbatas; simpan hanya status dan ringkasan diagnostik. Bersihkan workspace, binary sementara, dan buffer setelah selesai/gagal.
- [x] T5.5 Implementasikan kandidat snapshot: `downloaded → validating → ready` atau `failed`.
- [x] T5.6 Aktifkan hanya kandidat ready, secara transaksi, dengan pemeriksaan bahwa source/config job masih relevan. Job lama yang selesai belakangan tidak boleh menimpa pilihan source atau aktivasi yang lebih baru.
- [x] T5.7 Pertahankan snapshot aktif saat kandidat gagal; rollback hanya ke snapshot yang masih ada dan kompatibel dengan engine/template saat ini.
- [x] T5.8 Terapkan perubahan interval sync admin ke worker dan hindari sync bersamaan untuk source yang sama.
- [x] T5.9 Sediakan validasi per-plan menggunakan worker yang sama. Pilihan ekstensi baru tidak diasumsikan valid hanya karena matriks kandidat pernah lulus.

Gate selesai:

- Kandidat dengan simbol hilang/build gagal tidak aktif.
- Sinkronisasi gagal tidak merusak generate dari snapshot aktif.
- Restart, timeout, job ganda, konfigurasi source berubah, serta aktivasi bersamaan memiliki hasil konsisten.
- Branch tanpa manifest ditolak sebagai kandidat v1; manifest fixture development tidak dipakai diam-diam.
- Tidak ada log besar atau ZIP hasil yang tertinggal di storage.

## 10. Tahap 6 — API terpadu, preview, dan ZIP streaming

**Tujuan:** menghubungkan engine ke penggunaan nyata dengan plan yang terikat ke input dan versi tertentu.

Kontrak endpoint yang difinalkan pada tahap ini:

| Endpoint | Perilaku |
| --- | --- |
| `GET /api/v1/info` | Katalog/default/supported versions dari snapshot aktif dan template kompatibel |
| `POST /api/v1/resolve` | Normalisasi, resolusi, simpan plan, dan mulai validasi bila diperlukan |
| `GET /api/v1/plans/{id}` | Status validasi, expiry, dependency reasons, dan diagnostik terbatas |
| `POST /api/v1/explore` | Preview plan ready; plan belum ready menghasilkan status pending yang jelas |
| `POST /api/v1/generate` | Stream ZIP plan ready setelah materialisasi dan pemeriksaan identitas output |
| `GET /starter.zip` | Parsing request CLI lalu pipeline resolusi/validasi/generate yang sama |

Tugas:

- [x] T6.1 Gunakan status plan `pending`, `validating`, `ready`, `failed`, `expired`; simpan hasil/digest validasi sebelum ready.
- [x] T6.2 Kembalikan status pending/202 beserta URL status untuk pekerjaan asynchronous. Dokumentasikan agar client tidak menyimpan JSON pending sebagai file ZIP.
- [x] T6.3 Tetapkan batas tunggu untuk jalur CLI synchronous. Jika belum selesai, berikan respons pending dan cara melanjutkan; contoh cURL baru harus memeriksa status sebelum download.
- [x] T6.4 Regenerate file dari source snapshot dan template version plan untuk preview/download. Cocokkan digest isi dengan hasil tervalidasi sebelum mulai streaming; mismatch membatalkan operasi.
- [x] T6.5 Jika engine/template lama tidak tersedia setelah deployment, nyatakan plan expired/incompatible dan minta resolve ulang; jangan mengganti snapshot/template diam-diam.
- [x] T6.6 Perbaiki propagasi error ZIP termasuk finalisasi writer. Validasi terjadi sebelum response download dimulai; error jaringan di tengah stream diperlakukan sebagai transfer gagal.
- [x] T6.7 Pertahankan adapter boolean lama sementara: field absent tidak dianggap penolakan dependency. Jika `extensions` dan boolean hadir bersama, terima nilai yang konsisten dan tolak konflik eksplisit.
- [x] T6.8 Samakan parameter query/Base64 antara UI, API, README, dan tes; seluruh jalur menolak REST-only dan unknown extension.
- [x] T6.9 Beri error terstruktur untuk snapshot tidak tersedia, database unavailable, template mismatch, plan expired, serta validasi gagal.

Gate selesai:

- UI API, JSON langsung, query, dan Base64 menghasilkan resolusi yang sama untuk input ekuivalen.
- Preview dan isi ZIP setara untuk plan yang sama, termasuk saat sync terjadi di tengah sesi.
- ZIP langsung dikirim; temporary source workspace dibersihkan setelah operasi.
- Client dapat membedakan pending, gagal, dan ZIP sukses tanpa menebak dari nama file.
- Plan gagal/expired dan input REST-only tidak menghasilkan ZIP sukses.

## 11. Tahap 7 — UI dinamis dan admin

**Tujuan:** menampilkan kontrak dan keputusan backend secara jelas kepada pengguna.

Tugas:

- [x] T7.1 Hapus katalog ekstensi hardcoded di JavaScript; render dari metadata snapshot aktif.
- [x] T7.2 Simpan pilihan eksplisit pengguna; tampilkan core dan dependency otomatis dari respons resolver, dengan lock serta `required_by`.
- [x] T7.3 Nonaktifkan tombol hapus dependency terkunci. Lepaskan dependency tak terpakai setelah parent dihapus; pertahankan yang dipilih eksplisit atau masih dibutuhkan.
- [x] T7.4 Tampilkan kelompok Features/Packages dari `kind`, judul/deskripsi dari manifest, search, kompatibilitas profil, hitungan pilihan/dependency, dan protokol yang benar-benar didukung.
- [x] T7.5 Terapkan default admin hanya saat inisialisasi atau reset; jangan menimpa input yang sudah diedit pengguna.
- [x] T7.6 Kelola resolve async: abaikan respons pilihan lama, perbarui plan ketika input berubah, tampilkan pending/failed/ready/expired, dan aktifkan preview/download hanya untuk plan yang sesuai input terkini.
- [x] T7.7 Buat perintah cURL yang mengikuti kontrak pending/download tahap 6 dan tidak menyimpan JSON sebagai ZIP.
- [x] T7.8 Admin menampilkan source/ref/full SHA, snapshot aktif/kandidat, ringkasan validasi, sync, rollback, serta interval yang benar-benar diterapkan.
- [x] T7.9 Sediakan petunjuk ringkas jika manifest/template tidak kompatibel. Detail engine tetap di diagnostik admin, bukan memenuhi form generator.

Gate selesai melalui pengujian browser:

- Pilih/hapus Auth dan User menunjukkan dependency lock yang benar.
- Respons resolve yang datang terlambat tidak mengembalikan pilihan lama atau memakai plan yang salah.
- Ekstensi baru sesuai kontrak muncul tanpa mengedit UI.
- REST-only tidak muncul di generator maupun default admin.
- Preview/download pending, gagal, dan sukses dapat dipahami serta digunakan.
- Perubahan default admin berlaku pada sesi form berikutnya dan perubahan interval mengubah jadwal worker.

## 12. Tahap 8 — Integrasi, migrasi, dan kesiapan rilis

**Tujuan:** rilis awal siap digunakan dan tidak hanya lulus tes unit.

Tugas:

- [x] T8.1 Jalankan matriks source nyata: core minimal, setiap ekstensi beserta closure dependency, Auth/User, semua ekstensi yang kompatibel, dependency bersama, serta penghapusan feature.
- [x] T8.2 Uji dual dan gRPC-only bila supported pada seluruh versi Go yang ditawarkan. Versi yang belum dibuktikan kompatibel dikeluarkan dari katalog.
- [x] T8.3 Jalankan smoke/runtime test dengan resource sementara yang diperlukan, termasuk database dan endpoint domain melalui transport yang didukung.
- [x] T8.4 Tambahkan CI untuk schema, resolver, generator, database repository, dan fixture hasil generate. Pisahkan smoke suite cepat dari matriks integrasi yang lebih mahal.
- [x] T8.5 Uji beberapa request bersamaan, job restart, snapshot rollback, expiry plan, cleanup file sementara, disk penuh/download upstream gagal, serta client download terputus pada titik yang relevan.
- [x] T8.6 Sinkronkan Dockerfile/deployment dengan kebutuhan worker, codegen/build toolchain, akses dependency, temporary workspace, database, volume snapshot, dan readiness. Jangan menganggap runtime binary-only cukup untuk build validator.
- [x] T8.7 Rencanakan cutover cache lama ke snapshot beridentitas: jangan menandainya valid tanpa manifest, full SHA yang dapat dibuktikan, dan hasil pemeriksaan.
- [x] T8.8 Dokumentasikan cara menambah ekstensi lewat manifest, versi kontrak/template, API plan, konfigurasi storage/retention, serta recovery snapshot.
- [ ] T8.9 Migrasi/default compatibility diverifikasi di staging; catat batas deprecation API boolean. Hapus implementasi lama yang tidak lagi dipakai setelah parity dibuktikan.
- [ ] T8.10 Catat readiness rilis dan hasil staging. Promosi/deployment mengikuti instruksi eksekusi pengguna pada saat itu; roadmap ini sendiri tidak men-deploy apa pun.

Status T8.1–T8.8 berlaku untuk implementasi dan tes lokal. CI hosted, download browser, migrasi/recovery staging serta audit legacy belum dinyatakan lulus. T8.9–T8.10 dan gate rilis tetap terbuka; lihat [hasil dan batas tahap 8](stage-8-release.md).

Gate rilis:

- Seluruh checklist penerimaan pada dokumen kontrak terpenuhi untuk scope awal.
- Manifest upstream tersedia pada ref yang dipakai produksi dan kompatibel dengan template aplikasi.
- Kandidat gagal tidak mengganggu snapshot aktif; rollback terbukti bekerja pada pasangan snapshot/template yang kompatibel.
- Tidak ada source output, ZIP hasil, atau log besar yang dipersistenkan.
- Data admin lama tetap tersedia setelah migrasi.
- Batas dukungan, prosedur upgrade, dan hasil pengujian tercatat tanpa klaim semua kombinasi runtime sudah teruji.

## 13. Keputusan implementasi awal

Nilai berikut menjadi default perencanaan, bukan perluasan scope. Jika source nyata membuktikan asumsi salah, catat penyesuaiannya sebelum melanjutkan pekerjaan yang bergantung padanya.

| Area | Default pelaksanaan |
| --- | --- |
| Manifest | Satu `gostarter.yaml`, schema v1; hanya feature/pkg non-default, dengan title dan description wajib |
| Default dan konvensi | Profil internal aplikasi yang versioned; tidak ditulis ulang dalam manifest upstream |
| Core | Ditentukan hasil inventory; dependency yang masih dipakai core tetap wajib |
| Auth/User | Auth memerlukan User bawaan untuk rilis awal |
| Integrasi baru | Didukung bila kontrak/template engine tersedia; tidak menjanjikan sembarang kode dapat diintegrasikan otomatis |
| Wire | Template aplikasi merender deklarasi; codegen versi tertentu dijalankan sebelum build validation |
| Snapshot aktif | Kandidat otomatis aktif setelah gate validasi; kandidat usang tidak boleh menimpa state baru |
| Worker | Worker terbatas dengan job PostgreSQL; tanpa broker tambahan untuk rilis awal |
| Plan | Persisten dengan expiry; referensi versi immutable, tanpa menyimpan output proyek |
| ZIP/log | Stream ZIP; simpan hanya ringkasan diagnostik terbatas |
| REST-only | Ditolak saat ini; tidak ada template konversi dual ke REST-only |

## 14. Pengembangan setelah rilis awal — branch REST-only

Tahap ini tidak menjadi prasyarat atau alasan menunda rilis awal.

Prasyarat: branch upstream khusus REST-only sudah tersedia, manifest ekstensi dapat diperiksa, dan profil aplikasi untuk branch tersebut mendeklarasikan dukungan protokol serta standar arsitekturnya.

Rencana berikutnya:

1. Inventaris perbedaan branch dan kompatibilitas ekstensi; jangan mengasumsikan manifest/extension ID pada dua branch selalu identik.
2. Tambahkan kumpulan template GoStarter yang sesuai standar branch REST-only.
3. Validasi registrasi handler, auth middleware, lifecycle, dan infrastructure pada branch tersebut.
4. Tentukan UX pemilihan source/profil yang eksplisit serta namespace snapshot/plan per branch.
5. Tawarkan REST-only hanya setelah matriks build/runtime branch tersebut lulus.

Nama branch, jadwal, dan pilihan source UI belum ditetapkan. Tidak membuat branch baru atau memodifikasi upstream untuk REST-only selama tahap 0–8.

## 15. Catatan eksekusi

Gunakan format berikut setiap kali sebuah tahap atau sub-tahap diselesaikan:

```text
Tahap / task ID:
Status:
Perubahan dan artefak:
Validasi yang dijalankan dan hasil:
Keterbatasan / blocker:
Perubahan kontrak bila ada:
Langkah berikutnya:
```

Tahap 0 selesai: lihat [baseline dan hasil validasi](stage-0-baseline.md) serta [inventory eksekusi](inventory/stage-0-upstream.json). REST-only dihapus/ditolak, migrasi setting lama diuji pada PostgreSQL sementara, cache lama diberi status unverified, dan race/vet/build kedua project lulus. Perbaikan config upstream diterapkan lokal pada dua file, belum di-commit/push. Tidak ada perubahan manifest. Saat penutupan tahap 0, langkah berikutnya tahap 1. Tahap 1 kini selesai sesuai catatan terbaru di bawah.

Persiapan kontrak: file upstream `gostarter.yaml`, schema struktural, serta panduan v1 sudah dibuat sebelum roadmap dijalankan. Katalog awal berisi 2 feature dan 8 pkg non-default. Catatan verifikasi persiapan berada pada panduan kontrak; pembuatan kontrak tidak menandai implementasi parser/resolver selesai.

### Eksekusi tahap 1 — 7 Oktober 2026

- Status: T1.1–T1.8 selesai. [Laporan dan batas validasi](stage-1-contract-profile.md).
- Artefak: parser/diagnostik v1, profil default dan registry output template, validator source, CLI read-only, fixture source upstream reproducible, serta panduan penambahan feature/pkg dan tes negatif.
- Validasi: manifest aktual dan fixture 169 file lolos validator produksi; race test, vet, build, dan fuzz parser lulus.
- Kontrak upstream tidak berubah. Tidak ada perubahan baru pada source upstream dalam tahap 1.
- Rendering template, resolver pilihan/lock, serta wiring ke API/worker mengikuti tahap berikutnya; status validasi statis tidak mengaktifkan snapshot produksi.
- Langkah berikutnya: tahap 2 — dependency resolver dan plan kanonik.

### Eksekusi tahap 2

- Status: T2.1–T2.6 selesai. [Hasil, perilaku dan batas plan](stage-2-resolver-plan.md).
- Artefak: resolver murni, model plan kanonik, fingerprint/verifikasi, adapter legacy terpisah, CLI analisis lokal, dan tes perilaku.
- Validasi: seluruh race test/vet/build lulus; percobaan Auth pada upstream aktual menambahkan User/Cryptoutil/Validator terkunci dan mempertahankan tujuh core default.
- Kontrak/source upstream tidak berubah. Plan fixture tidak mengklaim commit/provenance produksi.
- Rendering/materialisasi, API plan, dan snapshot persisten tetap mengikuti tahap berikutnya.
- Langkah berikutnya: tahap 3A — template core dan infrastructure.


### Eksekusi tahap 3A — 8 Oktober 2026

- Status: T3.1–T3.4 selesai. [Hasil dan batas renderer core](stage-3a-core-templates.md).
- Artefak: 37 template aplikasi embedded v2, digest template/tool pada plan engine v2, komposisi core dual, codegen protobuf/OpenAPI dan Wire terpaku versinya, serta CLI development `render-project`.
- Lifecycle: satu pool PostgreSQL dan pasangan provider telemetry dibagikan lewat Wire; cleanup parsial/deferred dan shutdown idempotent, dengan drain HTTP → gRPC → routine → database → telemetry → logger.
- Validasi: core fixture dan checkout upstream aktual berhasil codegen/build; smoke health gRPC/HTTP, JWT, urutan/cleanup resource, deadline shutdown, race test, vet, dan build GoStarter lulus.
- Batas: smoke database memakai pool/pinger pengujian, belum proses PostgreSQL nyata dalam tahap ini. Renderer menolak ekstensi dan gRPC-only sampai integrasinya dibuktikan. API/UI lama belum dialihkan. Tidak ada perubahan upstream, ZIP, atau arsip log besar.
- Langkah berikutnya: tahap 3B — integrasi feature User/Auth dan pkg, wiring terpilih, migrasi/OpenAPI, serta konfigurasi/module dari closure ekstensi.


### Eksekusi tahap 3B — 8 Oktober 2026

- Status: T3.5–T3.9 selesai. [Hasil integrasi feature/pkg](stage-3b-feature-pkg.md).
- Artefak: template v3 dengan provider/handler/public methods berdasarkan feature aktif, proto terpilih, config Auth kondisional, requirements dari registry versi upstream, dan CLI `-extensions`.
- Validasi: 12 kombinasi source upstream aktual berhasil codegen, build seluruh package, dan race test domain/pkg; core/User/Auth, delapan pkg satu per satu, dan semua ekstensi bersama. Tes policy/registrasi terpilih, penghapusan feature, serta feature/pkg contoh baru dari manifest juga lulus.
- Tes HTTP client awalnya terhalang bind localhost di sandbox; pengulangan dengan izin localhost lulus tanpa perubahan source upstream.
- Batas: belum startup/crud/auth end-to-end pada PostgreSQL nyata, gRPC-only belum dirender, API/UI belum dialihkan. Tidak ada perubahan kontrak/source upstream atau ZIP/log besar yang disimpan.
- Langkah berikutnya: tahap 3C — konsistensi materialisasi, identitas environment/output, dan pembuktian kompatibilitas gRPC-only sebelum ditawarkan.


### Eksekusi tahap 3C — 8 Oktober 2026

- Status: T3.10–T3.13 selesai; tahap 3 dan milestone B selesai. [Hasil materialisasi dan batas validasi](stage-3c-materialization.md).
- Artefak: pipeline `Materialize` dengan audit output/owner, receipt/digest, verifikasi terhadap plan, cleanup workspace, template v4 dan plan engine v3 dengan environment generation tetap.
- gRPC-only: bootstrap/config/deployment tanpa server HTTP, Swagger, atau OpenAPI output; adapter gateway handler upstream tetap sebagai library. Kontrak/source upstream tidak berubah.
- Validasi: delapan kombinasi dual/gRPC (core, User, Auth, semua pkg bersama) dimaterialisasi dua kali dengan environment host berbeda; seluruh byte/receipt sama. Build linux/amd64 CGO=0, runtime gRPC/JWT/lifecycle, smoke dual, failure cleanup, race/vet/build lulus.
- Batas: belum database end-to-end/container deployment, API/UI produksi belum dialihkan. Receipt bukan tanda tangan autentisitas; persistence/provenance produksi mengikuti tahap 4–5. Tidak menyimpan ZIP atau log besar.
- Langkah berikutnya: tahap 4 — PostgreSQL dan snapshot immutable, metadata/plan repository, serta retensi snapshot.


### Eksekusi tahap 4 — 8 Oktober 2026

- Status: T4.1–T4.8 selesai. [Storage, gate dan batas operasional](stage-4-storage.md).
- Artefak: migrasi additive empat tabel, repository snapshot/source state/plan/job, arsip source immutable dengan checksum dan finalisasi exclusive, service storage dan CLI development `capture-snapshot`.
- Fetcher mengikat metadata/arsip ke full SHA yang sama; repository/ref masuk identitas. Plan ID berbeda dari fingerprint agar expiry tidak mencegah resolve ulang. Retention melindungi pointer aktif/kandidat, plan valid dan pending job.
- Config retention/expiry/queue/diagnostic divalidasi; pool startup dan session lifetime digunakan. Sesi memakai jam PostgreSQL untuk menghindari ketergantungan pada sinkronisasi jam host.
- Validasi: PostgreSQL 18.4 nyata pada schema disposable membuktikan migrasi, reconnect persistence, expiry/re-resolve, queue concurrency, capture/retention concurrency dan alur service. Race test, vet, build dan whitespace lulus.
- Batas: worker/aktivasi belum dibuat, API/UI legacy belum dialihkan. Finalisasi disk + SQL bukan distributed transaction; orphan setelah crash belum direkonsiliasi otomatis, dan pembacaan arsip hilang/korup ditolak. Detail pemulihan pada laporan tahap 4.
- Kontrak/source upstream tidak berubah. PostgreSQL tidak menyimpan source output atau ZIP hasil; volume hanya menyimpan arsip source upstream.
- Langkah berikutnya: tahap 5 — worker validasi dan aktivasi snapshot.


### Eksekusi tahap 5 — 8 Oktober 2026

- Status: T5.1–T5.9 selesai. [Worker, hasil validasi dan batas operasional](stage-5-worker.md).
- Artefak: worker standalone bounded, lease/heartbeat/recovery, policy key, candidate epoch, aktivasi transaksi, rollback compare-and-swap dan CLI; source sync lease dengan admin settings yang dibaca ulang.
- Validasi: 24 kombinasi dual/gRPC dari katalog dinamis berhasil codegen/build/smoke melalui worker nyata; plan Auth + Pagination juga ready. Compile failure, manifest hilang, timeout, expired plan, lease lama, source/config berubah dan ABA kandidat tidak mengaktifkan hasil lama.
- PostgreSQL disposable membuktikan recovery, concurrency/queue limit, rollback, interval sync, diagnostic bounded serta cleanup workspace; seluruh race test/vet/build/whitespace lulus. Pembatalan Unix membunuh process group termasuk child.
- Batas: API/UI/cache legacy belum dialihkan; worker production tidak diaktifkan sebagai daemon pada environment pengguna. Budget belum berupa hard limit RAM/CPU/sandbox OS. Orphan arsip source pasca crash disk/SQL tetap memiliki batas tahap 4; lihat laporan untuk cleanup workspace yang dikenal database.
- Kontrak/source upstream tidak berubah. Tidak menyimpan output ZIP atau log besar.
- Langkah berikutnya: tahap 6 — API terpadu untuk resolve/status plan, preview dan ZIP streaming.

### Hasil tahap 6 — API plan, preview, ZIP

- API publik memakai snapshot aktif tervalidasi dan PostgreSQL; resolve/status plan, JSON/query/Base64 dan adapter boolean melalui pipeline yang sama.
- Pending memakai 202 JSON + status URL; CLI menunggu maksimal 10 detik, UI melakukan polling dan memeriksa content type sebelum download.
- Preview/ZIP regenerate snapshot yang dipin dan memeriksa digest isi serta validation identity; expired/failed/incompatible/mismatch ditolak sebelum stream.
- ZIP deterministik, error finalisasi/transfer dipropagasikan, workspace sementara dibersihkan; server tidak lagi menjalankan legacy cache download/auto-sync.
- Verifikasi: unit/regresi, integrasi PostgreSQL dengan real build + smoke plan dan pembandingan preview/ZIP, race suite, vet/build, syntax JS. Tes expiry DB lama diperlebar dari 100 ms menjadi 2 detik agar stabil saat build paralel.
- Status: T6.1–T6.9 dan milestone C selesai. [Kontrak API, hasil dan batas operasional](stage-6-api.md). Berikutnya tahap 7: katalog UI dinamis, dependency lock, state async lengkap, admin snapshot/rollback.

### Hasil tahap 7 — UI dinamis dan admin

- Status: T7.1–T7.9 selesai. [Perubahan, pengujian, dan batas verifikasi](stage-7-ui-admin.md).
- Katalog memakai metadata manifest; pilihan eksplisit terpisah dari dependency terkunci. Resolve async membatalkan state lama dan mengunci output sampai plan terbaru ready.
- Admin menampilkan source/ref/full SHA, validasi dan snapshot; rollback dilindungi autentikasi serta revision/epoch. Default dan interval tersimpan tanpa menghilangkan source/ref.
- Validasi: race suite Go dengan PostgreSQL disposable, build/vet, tes state JavaScript, serta browser memakai fixture katalog/admin. Integrasi API memverifikasi real build/smoke dan kesetaraan preview/ZIP.
- Batas: event penyimpanan ZIP oleh download manager browser belum terverifikasi karena alat pengujian tertahan; lanjutkan pemeriksaan ini pada staging tahap 8. Screenshot memakai fixture, bukan bukti aktivasi upstream produksi.
- Kontrak dan source upstream tidak berubah. Fixture lokal dihentikan setelah pemeriksaan.
- Berikutnya tahap 8: integrasi source nyata, CI, migrasi/deployment worker dan kesiapan rilis; belum dimulai.

### Hasil lokal tahap 8 — integrasi dan persiapan rilis

- T8.1–T8.8 selesai pada lingkungan lokal; T8.9–T8.10 menunggu staging. Tahap 8 belum ditandai selesai sebagai rilis.
- Source upstream nyata diuji pada matriks dual/gRPC Go 1.27.1, termasuk worker, reproducibility dan endpoint User dengan PostgreSQL disposable. Failure/recovery/rollback/expiry dan transfer terputus diuji pada lapisan terkait.
- CI reusable, image runtime lengkap, deployment server+worker, migrasi serial, health/readiness, Compose dan runbook disiapkan. Cache image diperbaiki setelah tes offline menemukan dependency transitif yang belum diprovision.
- Tidak ada deployments/push, perubahan database produksi, modifikasi upstream, ZIP hasil atau log besar yang dipersistenkan.
- Berikutnya: eksekusi [checklist staging](stage-8-operations.md), verifikasi download browser, audit legacy dan review kesiapan promosi. [Detail hasil lokal](stage-8-release.md).

### Arahan setelah tahap 8

Pengguna meminta melewati pekerjaan tahap 8 yang bukan pengembangan kode. CI hosted, deployment, review staging dan promosi tidak dijalankan; hasil lokal tetap berlaku dan gate rilis tidak dianggap lulus. Audit/penghapusan legacy yang bergantung pada parity staging tetap ditunda.

Pekerjaan kode berikutnya yang tercantum adalah dukungan branch REST-only pada bagian 14. Nama/source branch belum ditentukan; daftar branch pada checkout upstream saat pemeriksaan hanya dev/main beserta remote-tracking-nya. Perlu identitas branch REST-only sebelum menentukan template/profil dan mengimplementasikannya; tidak membuat konversi REST-only dari upstream dual secara spekulatif.

### Pembersihan kode legacy setelah tahap 8

Atas instruksi eksplisit pengguna, audit referensi dan penghapusan implementasi lama dijalankan sebagai pengembangan kode tanpa menunggu deployments/staging. Transform generator lama, mutable cache/auto-sync, model konfigurasi lama serta adapter internal `FromLegacy` yang hanya dipakai self-test dihapus. Parser boolean API dan jalur snapshot/fetch/materialization/preview/ZIP aktif tetap dipertahankan. Detail serta hasil verifikasi ada pada [audit cleanup](legacy-cleanup.md). T8.9 tetap memerlukan verifikasi compatibility staging; REST-only tetap ditunda.

### Revisi produk — sinkronisasi manual admin

Permintaan pengguna menggantikan auto-sync dengan sync manual melalui admin. Interval/jadwal otomatis pada catatan historis tahap 5–8 tidak lagi berlaku. Startup dan penyimpanan setting tidak mengunduh upstream; worker hanya memproses permintaan manual persisten. Validasi/aktivasi kandidat tetap mengikuti pipeline yang sama. [Perilaku dan migrasi](manual-admin-sync.md).

## Revisi pengujian: unit test saja

Sesuai keputusan terbaru, salinan upstream dan seluruh direktori `testdata` dihapus. Tes integrasi PostgreSQL, materialisasi proyek, dan mock browser beserta job CI integrasi/container dihapus; CI menjalankan unit test Go/UI, race detector, vet, dan build. Catatan tahap sebelumnya merupakan hasil historis, bukan daftar tes yang masih tersedia. Input unit test dibuat inline atau pada direktori sementara. Dependency runtime Docker dipisahkan ke `deployments/build-runtime`; validasi kandidat dan plan pada aplikasi tetap berjalan, sync tetap manual via Admin, dan REST-only tetap ditunda.

## Revisi runtime: satu proses aplikasi

`cmd/server` kini menjalankan web/API, worker validasi, dan pemrosesan request sync manual Admin dalam satu binary. `make start`, Docker, dan Compose menjalankan satu proses/container. Binary `validation-worker` dan script supervisor dua proses dihapus. Migrasi terpisah memakai `gostarter -migrate-only`; rollback tetap lewat Admin. Catatan tahap sebelumnya tentang worker standalone merupakan riwayat implementasi.

## Revisi stack web

Chi menggantikan ServeMux. Halaman statis dipindahkan ke komponen templ, HTMX menangani form admin/status dan fragmen kartu ekstensi, Alpine menangani interaksi lokal, Tailwind dikompilasi lokal. Desain grid modal dan admin dua kolom tetap dipertahankan. API JSON, worker dalam satu proses, serta sync manual Admin tetap berlaku. Tidak ada testdata atau suite integrasi baru. Detail build: [web-stack.md](web-stack.md).

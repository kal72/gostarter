# Kontrak Upstream GoStarter v1

Status: parser dan validator statis v1 serta profil default sudah diimplementasikan pada tahap 1. Resolver pilihan dan plan kanonik sudah diimplementasikan pada tahap 2; render template, build hasil generate, dan aktivasi snapshot belum diimplementasikan. Lihat [hasil tahap 1](../stage-1-contract-profile.md).

Sumber: repository `go-feature-based-boilerplate`, source commit `f986364fb4fff06a69bfd322fa9d95206c9b7e95` saat kontrak disusun. Manifest kemudian dipush pengguna pada commit `05cf3ee561ddd7a454c2d9641c540d4c7d750ebb` (`dev`).

File upstream: `gostarter.yaml` pada root repository.

Schema struktural: [gostarter.schema.json](gostarter.schema.json).

Dokumen ini menggantikan contoh manifest lama yang mencantumkan core, architecture, infrastructure, dan titik integrasi global. Bagian default tersebut dimiliki profil template internal GoStarter.

## 1. Bentuk kontrak

Root hanya memiliki `schema_version` dan `extensions`. Satu daftar dipakai untuk `kind: feature` maupun `kind: pkg`; UI dapat menampilkannya pada kelompok berbeda.

| Field | Wajib | Ketentuan |
| --- | --- | --- |
| `schema_version` | Ya | Bilangan bulat `1`; versi lain ditolak sampai didukung |
| `extensions` | Ya | Daftar ekstensi non-default; daftar kosong diperbolehkan untuk core saja |
| `id` | Ya | Unik global, stabil, maksimal 64 karakter, huruf kecil/angka/hyphen, dimulai huruf |
| `kind` | Ya | `feature` atau `pkg` |
| `title` | Ya | Nama UI, satu baris, 1–80 karakter, tanpa whitespace di awal/akhir |
| `description` | Ya | Deskripsi UI singkat, satu baris, 1–160 karakter, tanpa whitespace di awal/akhir |
| `path` | Ya | Direktori source asli, relatif terhadap root upstream; `internal/<name>` atau `pkg/<name>` sesuai kind |
| `requires` | Tidak | Daftar ID ekstensi lain; jika tidak ditulis berarti kosong |

Tidak ada `default_on`, `core`, `architecture`, `infrastructure`, `protocols`, `template_version`, script hook, atau daftar template dalam manifest ini. Versi template, protokol, source module, dan batas default ditangani profil aplikasi dan pemeriksaan source. Field yang belum dikenal tidak boleh diabaikan diam-diam.

Tidak ada kategori tambahan pada v1: `kind` sudah cukup untuk memisahkan Features dan Packages. Deskripsi berupa plain text; frontend menampilkannya sebagai teks, bukan HTML.

`path` memilih seluruh subtree, termasuk subpackage dan tes. Jangan mengartikan `feature` sebagai hanya direktori `usecase/`: DTO, entity, repository, handler, validator, dan provider termasuk satu feature.

## 2. Katalog awal berdasarkan source nyata

| ID | Kind | Path | Dependency non-default langsung |
| --- | --- | --- | --- |
| user | feature | internal/user | cryptoutil, validator |
| auth | feature | internal/auth | user, cryptoutil, validator |
| cryptoutil | pkg | pkg/cryptoutil | — |
| httpclient | pkg | pkg/httpclient | — |
| idempotency | pkg | pkg/idempotency | — |
| pagination | pkg | pkg/pagination | — |
| pointer | pkg | pkg/pointer | — |
| stringutil | pkg | pkg/stringutil | — |
| timeutil | pkg | pkg/timeutil | — |
| validator | pkg | pkg/validator | — |

`auth` mendeklarasikan seluruh dependency langsung walaupun sebagian juga sudah transitif melalui User. Resolver menduplikasi nol kali: setiap ekstensi hanya dimaterialisasikan satu kali.

Memilih Auth menghasilkan Auth, User, Cryptoutil, dan Validator, ditambah core default yang dikelola aplikasi. `requires` tidak berisi ID core, path Go, atau module eksternal.

Pkg Idempotency menyertakan implementasi memory dan Redis. Import `github.com/redis/go-redis/v9` merupakan dependency module Go, bukan dependency terhadap `infrastructure/cache/redis` upstream. Menyertakan paket tersebut tidak otomatis memerlukan Redis server aktif atau wiring Redis; penggunaan RedisStorage membutuhkan client saat dipakai. Jangan mengada-adakan dependency ekstensi Redis dari kemiripan nama.

## 3. Default profil GoStarter, tidak ditulis di manifest

Baseline berikut mengikuti source yang diperiksa dan menjadi input implementasi profil template pada roadmap. Ini bukan field tambahan upstream.

| Komponen | Alasan tetap default |
| --- | --- |
| `internal/health` | Layanan health/readiness dasar |
| `pkg/logger` | Logger bootstrap dan middleware |
| `pkg/contextutil` | Digunakan logger, middleware, dan handler |
| `pkg/errorutil` | Digunakan error interceptor dan typed error fitur |
| `pkg/routine` | Digunakan lifecycle bootstrap |
| `pkg/transaction` | Digunakan provider/transaction manager PostgreSQL |
| `pkg/jwtutil` | Digunakan AuthInterceptor pada middleware default saat ini |
| PostgreSQL/config/telemetry/middleware/bootstrap | Infrastructure dan arsitektur default yang dirender template GoStarter |

JWT Utils tidak dimasukkan sebagai ekstensi opsional pada kontrak awal: meskipun contoh diskusi pernah menampilkannya, source nyata mengimpornya dari middleware default. Menghapus fitur Auth tidak otomatis menghapus validasi JWT pada endpoint lain. Ini juga mempertahankan mekanisme identitas caller yang dipakai User Update/Delete.

Jika kelak JWT Utils atau Routine hendak dibuat opsional, ubah profil/template aplikasi dan buktikan core tidak lagi membutuhkannya sebelum memindahkannya ke katalog. Manifest tidak boleh menandai dependency core sebagai sesuatu yang bisa dihapus pengguna.

Package atau feature baru yang tidak ada dalam manifest dan tidak ada dalam daftar default profil tidak otomatis dianggap core. Aktivasi snapshot harus melaporkan komponen yang belum diklasifikasikan. Infrastructure opsional seperti Redis/Mongo dan contoh external service mengikuti dukungan profil aplikasi; tidak dimasukkan sebagai `kind: pkg` palsu.

## 4. Aturan dependency dan validasi

1. ID dan path harus unik; path harus menunjuk direktori nyata tanpa symlink yang keluar repository.
2. Semua ID `requires` harus tersedia di `extensions`, tidak boleh merujuk diri sendiri, tidak berulang, dan tidak membentuk siklus.
3. Resolver menambahkan dependency transitif dan menyimpan alasan penguncian. Pilihan eksplisit pengguna terpisah dari hasil resolusi.
4. Dependency yang masih diperlukan tetap terkunci; dependency otomatis yang tidak lagi diperlukan dilepas setelah parent dihapus. Pilihan eksplisit pengguna tetap dipertahankan.
5. Import ke ekstensi non-default lain harus memiliki dependency langsung yang dideklarasikan. Import core ditangani profil aplikasi. Dependency runtime juga harus dinyatakan bila tidak dapat disimpulkan dari import.
6. SQL foreign key, referensi proto, provider, dan kebutuhan runtime harus diperiksa selain import. Manifest sederhana bukan bukti hasil generate sudah dapat dikompilasi.
7. `schema_version` harus integer, bukan string, float, atau boolean. YAML duplicate key, alias, merge key, dan custom tag ditolak agar maknanya tidak ambigu.
8. JSON Schema memvalidasi bentuk data. Unik ID/path, closure graph, keberadaan source, serta kesesuaian profil membutuhkan pemeriksaan semantik tersendiri.

Kontrak v1 sengaja tidak menyediakan metadata integrasi arbitrer. Ekstensi mengikuti konvensi yang didukung profil aplikasi; jika memerlukan pola baru, tahap kontrak berikutnya menambahkan aturan/field yang versioned setelah kebutuhannya jelas. Jangan membuat generator menebak simbol atau mengabaikan integrasi yang tidak dikenal.

## 5. File pendamping dan konvensi integrasi

`path` bukan satu-satunya file yang diperlukan oleh sebuah feature. GoStarter bertanggung jawab menyusun file pendamping melalui konvensi dan mapping internal yang diverifikasi.

| Area | Konvensi/profil GoStarter |
| --- | --- |
| Provider feature | Root package memiliki `ProviderSet` pada `provider.go` |
| Handler feature | `<path>/handler` menyediakan `Handler` dan registrasi gRPC/gateway sesuai interface upstream |
| Proto | `api/proto/<nama-direktori-feature>/` |
| Generated protobuf | `gen/pb/<nama-direktori-feature>/`; regeneration mengikuti toolchain upstream, tidak edit manual source upstream |
| Migrasi User | Pasangan `migrations/000001_create_users_table.{up,down}.sql` |
| Migrasi Auth | Pasangan `migrations/000002_create_refresh_tokens_table.{up,down}.sql`; foreign key membutuhkan User |
| OpenAPI gabungan | Spec dibentuk ulang dari proto terpilih, termasuk root OpenAPI dan embed file yang konsisten; tidak menyalin spec semua fitur tanpa seleksi |
| Bootstrap/DI/infrastructure | Output template aplikasi, tidak sekaligus disalin mentah dari upstream |

Mapping migrasi awal disimpan pada profil aplikasi karena file upstream tidak memakai ID feature secara persis. Untuk feature baru dengan file pendamping non-konvensional, perlu mapping/kontrak pendukung sebelum dinyatakan supported; jangan mengklaim entri katalog saja cukup untuk semua kemungkinan integrasi.

Source protokol diperiksa dari branch/snapshot yang dipilih. REST-only belum didukung dan tetap ditolak. Keberadaan manifest tidak mengubah dukungan transport atau membuat branch baru.

## 6. Batas verifikasi kontrak awal

Pembuatan manifest mencakup pemeriksaan YAML, field, path, ID, dependency, dan kecocokan dengan import source saat ini. Pengecekan upstream mengikuti instruksi repository pada salinan kerja sementara agar source bisnis tidak berubah.

Belum merupakan implementasi parser produksi, resolver, template generator, atau bukti bahwa proyek hasil generate dapat dibangun. Saat panduan ini pertama disusun, seluruh tahap roadmap belum dimulai; validasi kontrak ini menjadi prasyarat tahap 0. Tahap 0 kini selesai, dengan hasil pada [baseline tahap 0](../stage-0-baseline.md).

Manifest kini tersedia pada commit upstream `05cf3ee561ddd7a454c2d9641c540d4c7d750ebb`. Push manifest tidak berarti aktivasi snapshot produksi sudah dilakukan; engine harus memvalidasi commit/ref yang diunduh pada tahap 5.

### Hasil pemeriksaan persiapan

| Pemeriksaan | Hasil |
| --- | --- |
| Parse YAML dan penolakan duplicate key/alias/merge/tag | Lulus pada manifest aktual |
| Field, tipe, panjang title/description, kind, dan pola path | Lulus terhadap ketentuan schema v1 |
| Unik ID/path, seluruh dependency tersedia, graph tanpa siklus | Lulus; 10 ekstensi |
| Path source, import lintas-ekstensi termasuk test, dan foreign key Auth → User | Lulus pemeriksaan statis |
| Klasifikasi seluruh root `internal/` dan `pkg/` | 10 non-default dan 7 default; tidak ada root yang belum diklasifikasikan |
| JSON schema | Dokumen JSON valid; validator schema/semantik produksi tetap tahap 1 |
| `gofmt -w -s .` | Dijalankan pada salinan tracked source; tidak menghasilkan perubahan byte file Go |
| `make build` | Lulus pada salinan source dengan toolchain Go 1.27.1 |
| `go test -race ./...` | Belum lulus penuh: tiga tes `infrastructure/config` gagal pada checkout tanpa `.env`; package lain lulus setelah tes HTTP client diulang dengan izin localhost |

Tes konfigurasi yang gagal adalah `TestLoad_Defaults`, `TestLoad_DotEnv`, dan `TestLoad_EnvOverride`. Tes mengasumsikan default/isi `.env`, sementara loader pada source yang diperiksa tidak memberi default untuk semua field dan tidak membind seluruh key env sebelum unmarshal. Temuan baseline ini tidak berasal dari manifest; pemeriksaan dilakukan pada salinan tracked source yang belum berisi manifest baru. Tidak ada source bisnis diperbaiki dalam pekerjaan kontrak ini. Penetapan baseline config yang reproducible menjadi pekerjaan tahap 0 sebelum gate pengujian dinyatakan lulus.

Pembaruan tahap 0 (7 Oktober 2026): perbaikan loader/default/fixture config sudah diterapkan pada dua file upstream. Race test, vet, dan build lulus pada salinan tracked source bersih yang identik dengan working tree setelah perbaikan. Perbaikan config belum di-commit/push dan tidak mengubah manifest. Lihat baseline tahap 0 untuk inventory, perintah pengujian, dan batas verifikasinya.

## 7. Menggunakan validator tahap 1

Dari root GoStarter:

```sh
rtk proxy go run ./tools/validate-upstream -source /path/to/go-feature-based-boilerplate
```

Output `static_contract_valid` berarti manifest, coverage ownership, dan konvensi source lolos pemeriksaan statis. Itu belum berarti hasil generate lulus Wire/protoc/build/runtime atau snapshot boleh diaktifkan. Opsi `-details` menampilkan ownership file dan module requirement yang diturunkan dari source; output tidak disimpan otomatis.

Kode parser berada di `internal/manifest`, profil/registry/validator source di `internal/profile`. Registry awal: architecture `go-feature-based-clean`, revision `1`, template version `1`. Versi schema, profil, template, dan commit source terpisah. Template registry tahap ini berisi descriptor output; isi template dan rendering dikerjakan pada tahap 3. API generator lama belum memakai pipeline baru ini.

### Contoh menambah pkg

Tambahkan implementasi generic pada `pkg/money`, lalu entri berikut ke daftar `extensions`:

```yaml
- id: money
  kind: pkg
  title: Money Helpers
  description: Rounding and formatting helpers for monetary values.
  path: pkg/money
```

Root package harus memiliki source Go non-test. Import ke pkg opsional lain harus memiliki `requires` langsung, termasuk import dalam tes. Import ke business feature, infrastructure, atau bootstrap ditolak untuk menjaga pkg tetap generic. Import library eksternal harus tersedia pada `go.mod`; versi tidak ditulis ulang pada manifest. Pkg yang hanya menyediakan library tidak otomatis menyalakan resource infrastructure.

### Contoh menambah feature/usecase

Tambahkan subtree `internal/order` mengikuti standar feature upstream, lalu:

```yaml
- id: orders
  kind: feature
  title: Orders
  description: Create and retrieve customer orders.
  path: internal/order
  requires: [money]
```

Contoh ini mengasumsikan entri `money` di atas juga tersedia. ID katalog boleh berbeda dari nama direktori; file pendamping diturunkan dari basename `order`.

Konvensi profil revision 1:

1. `internal/order/provider.go` mendeklarasikan `ProviderSet = wire.NewSet(...)` yang menyertakan `handler.NewHandler`. Referensi deklarasi local di ProviderSet harus tersedia.
2. `internal/order/handler/handler.go` mendeklarasikan `Handler` dan `NewHandler(...) *Handler`. Method receiver `*Handler` mengimplementasikan `RegisterGRPC(*grpc.Server)` dan `RegisterGateway(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error`.
3. `api/proto/order/` memiliki tepat satu service dengan package protobuf eksplisit dan nama service qualified yang tidak bentrok. Generated registration tersedia di `gen/pb/order/`; handler memanggil fungsi registration server dan gateway yang sesuai.
4. Semua dependency feature/pkg langsung, termasuk tes, dideklarasikan pada `requires`. Default profil tidak ditambahkan ke `requires`.
5. Jika feature membutuhkan SQL migration baru, resource lifecycle, konfigurasi, atau integrasi non-konvensional, tambahkan mapping/dukungan pada revision profil GoStarter sebelum source itu dapat divalidasi. V1 belum memiliki konvensi migrasi umum; jangan menaruh field integrasi arbitrer pada manifest. Migrasi User/Auth awal sudah dipetakan secara eksplisit.

Jalankan generator protobuf resmi upstream setelah menambah proto. Validator tidak menjalankan script upstream, mengedit generated source, menginstal plugin, atau menjalankan hook dari manifest.

### Contoh kegagalan yang ditolak

| Perubahan | Kode diagnostik |
| --- | --- |
| Field `default_on`/`architecture` ditambahkan | `unknown_field` |
| ID berulang, path dipakai dua ekstensi | `duplicate_id`, `path_conflict` |
| `requires` hilang atau membentuk siklus | `missing_dependency`, `dependency_cycle` |
| Default Routine/JWT Utils dijadikan opsional | `default_conflict` |
| Import Go/proto ekstensi lain tanpa dependency langsung | `undeclared_dependency`, `undeclared_proto_dependency` |
| Provider/constructor/generated registration hilang | `missing_symbol`, `missing_provider_dependency` |
| ProviderSet tidak memasukkan handler constructor | `missing_provider_constructor` |
| Handler registration/signature tidak sesuai | `missing_registration`, `invalid_signature` |
| Service protobuf qualified bentrok | `proto_conflict` |
| File baru tidak memiliki ownership/mapping | `unclassified_file` |
| Source membutuhkan module yang tidak ada di go.mod | `missing_module` |
| REST-only atau Go di bawah floor profil | `incompatible_protocol`, `incompatible_go` |

Diagnostik memakai `code`, `extension_id` bila diketahui, `file`, `symbol` bila relevan, dan `cause`; error YAML juga dapat menyertakan `line`. `requires` tetap deklarasi dependency. Closure, `required_by`, dependency path, lock, dan plan kanonik kini tersedia pada [tahap 2](../stage-2-resolver-plan.md); integrasi ke UI/API mengikuti tahap berikutnya.

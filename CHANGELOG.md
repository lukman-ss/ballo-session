# Changelog

Semua perubahan penting pada proyek ini akan dicatat di file ini.

## [Belum dirilis]

### Ditambahkan
- Package inti `session` dengan kontrak `Store`, `Manager`, dan `Session`.
- `MemoryStore` untuk development/test, termasuk garbage collection sederhana.
- Session ID acak aman dengan `crypto/rand`.
- Expiry, invalidasi, dan regenerasi ID session.
- Cookie konfigurasi eksplisit: Secure, HttpOnly, SameSite, Path, MaxAge.
- Akses session via `context.Context` request.
- Penanganan store error dan cookie invalid tanpa membocorkan data internal.
- Adapter `net/http` (`HTTPAdapter`).
- Adapter untuk framework Ballo (`BalloAdapter`).
- Dokumentasi dasar di README.md.
- Basis pemeriksaan: gofmt, vet, dan tes unit.
- MemoryStore.Get tidak lagi menghapus entry di bawah RLock; penghapusan dilakukan di bawah WriteLock dengan pemeriksaan ulang.
- MemoryStore.Get mengembalikan salinan session untuk menghindari berbagi pointer ke banyak goroutine tanpa sinkronisasi.
- Session.Data dilindungi mutex untuk operasi set/get/deleteKey concurrent-safe.
- MemoryStore.Close aman dipanggil lebih dari sekali.
- generateID memeriksa error crypto/rand dan panic jika gagal.
- Validasi Manager (nil store, nama cookie kosong) dan CookieConfig yang aman.
- Audit dan penambahan tes konkurensi; semua tes lolos dengan race detector (`go test ./... -race`).

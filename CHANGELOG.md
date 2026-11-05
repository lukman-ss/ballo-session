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

### Diperbaiki
- `MemoryStore.Get` proteksi pembacaan `ExpiredAt` dan penyalinan `Session.Data` dengan mutex session.
- `gc()` tidak lagi memanggil `After` pada `ExpiredAt` tanpa lock session.
- `Session.Data` sekarang unexported, hanya diakses melalui method `Set`, `Get`, `DeleteKey`.
- Salinan session dari `MemoryStore.Get` aman dari mutasi eksternal.

### Ditambahkan (tes)
- `TestMemoryStoreGetConcurrentSetDeleteKey`: verifikasi Get aman di bawah concurrent Set/DeleteKey.
- `TestMemoryStoreGCConcurrent`: verifikasi GC tidak menyebabkan race saat membaca ExpiredAt.
- `TestMemoryStoreNoInternalPointerMutation`: pastikan salinan tidak memengaruhi store.

### Diperbarui
- README: dokumentasi thread‑safety `Session.Data`, contoh API publik.
- CHANGELOG: format standar, kategorisasi perubahan.

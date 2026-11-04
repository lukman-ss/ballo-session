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

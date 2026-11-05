# ballo-session

Server-side session management for Ballo framework (Go).

## Rationale

Package inti (`session`) sengaja tetap mandiri tanpa dependensi framework Ballo.
Adapter baru dibuat hanya ketika kontrak API yang tersedia memungkinkannya.
Saat ini tersedia adapter `net/http` dan `ballo`-`ballo-http`.

## Fitur

- `Store` interface dan `MemoryStore` (hanya development/test)
- Session ID acak aman (`crypto/rand`)
- Expiry, invalidasi, dan regenerasi ID session
- Cookie: Secure, HttpOnly, SameSite, Path, MaxAge eksplisit
- Akses session dari `context.Context` request
- Penanganan store error dan cookie invalid yang tidak bocorkan data internal

## Instalasi

```bash
go get github.com/lukman-ss/ballo-session
```

## Contoh penggunaan

### net/http

```go
package main

import (
	"log"
	"net/http"

	"github.com/lukman-ss/ballo-session/session"
)

func main() {
	store := session.NewMemoryStore()
	defer store.Close()

	config := session.DefaultCookieConfig()
	config.Path = "/"
	mgr := session.NewManager(store, config)
	adapter := session.NewHTTPAdapter(mgr)

	http.Handle("/", adapter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := mgr.GetSession(r)
		if err != nil {
			http.Error(w, "no session", http.StatusUnauthorized)
			return
		}

		sess.Set("user", "lukman")

		w.Write([]byte("ok"))
	})))

	log.Println("listening on :8080")
	http.ListenAndServe(":8080", nil)
}
```

### Framework Ballo

```go
package main

import (
	"net/http"

	"github.com/lukman-ss/ballo"
	"github.com/lukman-ss/ballo-session/session"
)

func main() {
	store := session.NewMemoryStore()
	defer store.Close()

	mgr := session.NewManager(store, session.DefaultCookieConfig())
	adapter := session.NewBalloAdapter(mgr)

	e := ballo.New()
	e.Use(adapter.Middleware())

	e.GET("/", func(req *ballo.Request) (*ballo.Response, error) {
		sess, ok := session.SessionFromContext(req.Context)
		if !ok {
			return ballo.Text(401, "no session"), nil
		}
		sess.Set("user", "lukman")
		resp := ballo.JSON(200, map[string]string{"status": "ok"})
		return resp, nil
	})

	http.ListenAndServe(":8080", e)
}
```

## Batasan keamanan

- Jangan menyimpan seluruh isi session dalam cookie plaintext. Cookie hanya menyimpan session ID.
- `MemoryStore` hanya untuk development dan test, bukan production.
- `SameSiteStrict` cocok untuk session sensitif; gunakan `SameSiteLax` jika perlu navigasi lintas situs.
- Gunakan HTTPS (Secure cookie) di production.
- Tidak ada dukungan persistence atau distributed session saat ini; implementasikan `Store` sendiri untuk itu.
- `Session` aman untuk akses concurrent melalui `Set`, `Get`, `DeleteKey`. Field `Session.ID` dan `Session.ExpiredAt` hanya boleh diakses dari `Store` implementations.
- Konfigurasi bersama (Manager, CookieConfig) divalidasi saat pembuatan; gunakan salinan konfigurasi yang tidak dapat diubah secara tidak sengaja.
- `Session.Data` tidak diekspor; data hanya diakses melalui method `Set`, `Get`, `DeleteKey`. Salinan session yang dikembalikan oleh `MemoryStore.Get` terpisah dari store sehingga modifikasi pada salinan tidak memengaruhi sesi yang tersimpan.

### Migrasi API

`Session.Data` sebelumnya diekspor sebagai `session.Data`. Versi baru menjadikannya private (`data`). Pengguna harus mengganti akses langsung ke field dengan metode:

```go
// Dulu
value := sess.Data["key"]

// Sekarang
value, ok := sess.Get("key")
```

Untuk menulis data, gunakan `sess.Set("key", value)`. Untuk menghapus, gunakan `sess.DeleteKey("key")`.

Jika Anda memerlukan semua data sekaligus, iterasikan dengan `sess.Get` per key yang diketahui atau simpan daftar key terpisah.

## Adapter

- `session.NewHTTPAdapter(mgr)` — middleware `net/http` kompatibel.
- `session.NewBalloAdapter(mgr)` — middleware `ballo.Handler` kompatibel (memerlukan `github.com/lukman-ss/ballo` dan `github.com/lukman-ss/ballo-http`).

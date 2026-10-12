package main

import (
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//go:embed static
var staticFiles embed.FS

type Enlace struct {
	Codigo   string    `json:"codigo"`
	Destino  string    `json:"destino"`
	Visitas  int       `json:"visitas"`
	CreadoEn time.Time `json:"creado_en"`
}

type Almacen struct {
	mu      sync.Mutex
	ruta    string
	enlaces map[string]*Enlace
}

const alfabeto = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func nuevoAlmacen(dir string) (*Almacen, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	a := &Almacen{ruta: filepath.Join(dir, "enlaces.json"), enlaces: map[string]*Enlace{}}
	datos, err := os.ReadFile(a.ruta)
	if errors.Is(err, os.ErrNotExist) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	return a, json.Unmarshal(datos, &a.enlaces)
}

// guardar debe llamarse con el mutex tomado.
func (a *Almacen) guardar() error {
	datos, err := json.MarshalIndent(a.enlaces, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.ruta + ".tmp"
	if err := os.WriteFile(tmp, datos, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.ruta)
}

func generarCodigo(n int) string {
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alfabeto))))
		b[i] = alfabeto[idx.Int64()]
	}
	return string(b)
}

func (a *Almacen) crear(destino string) (*Enlace, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	codigo := generarCodigo(6)
	for a.enlaces[codigo] != nil {
		codigo = generarCodigo(6)
	}
	e := &Enlace{Codigo: codigo, Destino: destino, CreadoEn: time.Now().UTC()}
	a.enlaces[codigo] = e
	return e, a.guardar()
}

func (a *Almacen) visitar(codigo string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.enlaces[codigo]
	if e == nil {
		return "", false
	}
	e.Visitas++
	if err := a.guardar(); err != nil {
		log.Printf("no se pudo guardar: %v", err)
	}
	return e.Destino, true
}

func (a *Almacen) listar() []*Enlace {
	a.mu.Lock()
	defer a.mu.Unlock()
	lista := make([]*Enlace, 0, len(a.enlaces))
	for _, e := range a.enlaces {
		lista = append(lista, e)
	}
	return lista
}

func responderJSON(w http.ResponseWriter, estado int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(estado)
	json.NewEncoder(w).Encode(v)
}

func entorno(clave, defecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return defecto
}

func main() {
	puerto := entorno("PORT", "8080")
	dirDatos := entorno("DATA_DIR", "./data")

	almacen, err := nuevoAlmacen(dirDatos)
	if err != nil {
		log.Fatalf("no se pudo abrir el almacen en %s: %v", dirDatos, err)
	}

	estaticos, _ := fs.Sub(staticFiles, "static")
	mux := http.NewServeMux()

	mux.Handle("GET /", http.FileServerFS(estaticos))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
	})

	mux.HandleFunc("GET /api/enlaces", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, almacen.listar())
	})

	mux.HandleFunc("POST /api/enlaces", func(w http.ResponseWriter, r *http.Request) {
		var cuerpo struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
			responderJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
			return
		}
		u, err := url.ParseRequestURI(cuerpo.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			responderJSON(w, http.StatusBadRequest, map[string]string{"error": "la URL debe empezar con http:// o https://"})
			return
		}
		e, err := almacen.crear(cuerpo.URL)
		if err != nil {
			responderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		responderJSON(w, http.StatusCreated, e)
	})

	mux.HandleFunc("GET /r/{codigo}", func(w http.ResponseWriter, r *http.Request) {
		destino, ok := almacen.visitar(r.PathValue("codigo"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, destino, http.StatusFound)
	})

	log.Printf("Acortador escuchando en :%s (datos en %s)", puerto, dirDatos)
	log.Fatal(http.ListenAndServe(":"+puerto, mux))
}

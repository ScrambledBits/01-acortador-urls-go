# Acortador de URLs (Go)

Aplicación web mínima que convierte direcciones largas en enlaces cortos, cuenta cuántas veces se visita cada enlace y guarda todo en un archivo JSON. Está escrita en Go usando **solo la biblioteca estándar** (sin dependencias externas) y la interfaz web va incrustada dentro del binario.

> Proyecto con fines exclusivamente educativos. No tiene autenticación, ni límites de uso, ni pretende ser un servicio real.

---

## Nivel y objetivos de aprendizaje

**Nivel:** inicial.

Al dockerizar esta aplicación vas a practicar:

- Compilar un lenguaje compilado dentro de un contenedor.
- **Builds multi-etapa** (*multi-stage builds*): una imagen grande para compilar y una imagen diminuta para ejecutar.
- Comparar tamaños de imagen (cientos de MB contra unos pocos MB).
- **Volúmenes** para que los datos sobrevivan cuando el contenedor se borra.
- Configurar una aplicación mediante **variables de entorno**.
- Ejecutar el proceso con un **usuario sin privilegios**.

---

## Stack tecnológico

| Componente | Tecnología |
|---|---|
| Lenguaje | Go 1.24 o superior |
| Servidor HTTP | `net/http` (biblioteca estándar, con el enrutador por método y ruta de Go 1.22+) |
| Persistencia | Archivo `enlaces.json` en disco |
| Interfaz | HTML + JavaScript sin frameworks, incrustado con `embed` |
| Dependencias externas | Ninguna |

---

## Arquitectura

```
 Navegador ──HTTP──▶  acortador (binario Go, puerto 8080)
                         │
                         └──▶ DATA_DIR/enlaces.json   (archivo en disco)
```

Es un **único proceso**. No necesita base de datos: lee el archivo JSON al arrancar y lo reescribe cada vez que se crea un enlace o se registra una visita.

---

## Estructura del proyecto

```
01-acortador-urls-go/
├── main.go            # Servidor HTTP, rutas y lógica de almacenamiento
├── go.mod             # Definición del módulo Go (sin dependencias)
├── static/
│   └── index.html     # Interfaz web (se incrusta en el binario al compilar)
├── README.md
└── CHANGELOG.md
```

Detalles importantes de cada archivo:

- **`main.go`**
  - La directiva `//go:embed static` mete la carpeta `static/` *dentro* del binario en tiempo de compilación. Por eso, una vez compilado, el ejecutable **no necesita** la carpeta `static/` a su lado.
  - `nuevoAlmacen()` crea el directorio de datos si no existe y carga `enlaces.json` si ya existía.
  - `guardar()` escribe primero en un archivo temporal y luego lo renombra, para no dejar el JSON a medias si el proceso muere.
  - Un `sync.Mutex` evita que dos peticiones simultáneas corrompan los datos.
- **`go.mod`**: declara el nombre del módulo y la versión mínima de Go. No hay `go.sum` porque no hay dependencias.

---

## Variables de entorno

| Variable | Valor por defecto | Descripción |
|---|---|---|
| `PORT` | `8080` | Puerto TCP en el que escucha el servidor. |
| `DATA_DIR` | `./data` | Carpeta donde se guarda `enlaces.json`. Se crea si no existe. |

---

## API y rutas

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/` | Interfaz web para crear y listar enlaces. |
| `GET` | `/health` | Verificación de salud. Responde `{"estado":"ok"}`. |
| `GET` | `/api/enlaces` | Lista todos los enlaces en JSON. |
| `POST` | `/api/enlaces` | Crea un enlace. Cuerpo: `{"url": "https://..."}`. Responde `201`. |
| `GET` | `/r/{codigo}` | Redirige (HTTP 302) al destino y suma una visita. `404` si no existe. |

Ejemplo de respuesta de `POST /api/enlaces`:

```json
{
  "codigo": "aB3xYz",
  "destino": "https://ejemplo.com/una/ruta/larga",
  "visitas": 0,
  "creado_en": "2026-01-01T12:00:00Z"
}
```

Validaciones: la URL debe empezar con `http://` o `https://` y tener dominio; si no, responde `400` con `{"error": "..."}`.

---

## Ejecución local sin Docker (referencia)

Solo como referencia para entender qué hace la app; el objetivo del ejercicio es **no** tener que instalar Go en tu máquina.

1. Requiere tener Go instalado (versión indicada en `go.mod` o superior).
2. Desde la raíz del proyecto, el comando `go` puede ejecutar directamente el paquete principal, o compilarlo a un binario y luego ejecutarlo.
3. Abre el navegador en el puerto configurado.
4. Verás aparecer una carpeta `data/` con el archivo `enlaces.json`.

---

## Reto: dockerizar la aplicación

> Las siguientes son **pistas**, no soluciones. Consulta la documentación oficial de Docker cuando tengas dudas.

### Parte 1: el `Dockerfile`

1. **Elige la imagen de compilación.** Busca en Docker Hub la imagen oficial de Go. Usa una etiqueta con versión explícita (nunca `latest`) que sea igual o mayor a la versión de `go.mod`.
2. **Define un directorio de trabajo** dentro de la imagen.
3. **Aprovecha la caché de capas.** Aunque este proyecto no tiene dependencias, acostúmbrate a copiar primero los archivos de definición del módulo y luego el resto del código. Piensa: ¿qué archivos cambian con menos frecuencia?
4. **Compila un binario estático.** Investiga qué variable de entorno de Go desactiva `cgo`; sin ella, el binario podría depender de bibliotecas del sistema que no existirán en la imagen final.
5. **Crea una segunda etapa** a partir de una imagen mínima. Compara al menos dos opciones: `alpine`, una imagen *distroless* y `scratch`. ¿Cuál es la más pequeña? ¿Cuál te permite entrar con una *shell* para depurar?
6. **Copia solo el binario** desde la etapa de compilación. Recuerda: la carpeta `static/` ya va dentro del binario.
7. **Ejecuta como usuario sin privilegios.** Algunas imágenes mínimas ya traen un usuario `nonroot`; en otras tendrás que crearlo o indicar un UID numérico.
8. **Documenta el puerto** que usa la app y define el comando de arranque en forma *exec* (arreglo JSON).
9. **Crea un `.dockerignore`.** ¿Qué no debería entrar al contexto de construcción? (pista: la carpeta de datos local, `.git`, archivos del editor).
10. Construye la imagen y **compara su tamaño** con el de la imagen de compilación.

### Parte 2: el `docker-compose.yml`

1. Define **un servicio** para el acortador que se construya a partir de tu `Dockerfile`.
2. **Publica el puerto** del contenedor en tu máquina. Prueba a cambiar el puerto interno usando la variable `PORT` y ajusta el mapeo en consecuencia.
3. Declara la variable `DATA_DIR` apuntando a una ruta absoluta dentro del contenedor (por ejemplo, algo bajo `/data`).
4. Declara un **volumen con nombre** y móntalo en esa ruta.
5. **Cuidado con los permisos:** si ejecutas como usuario sin privilegios, ese usuario debe poder escribir en la carpeta montada. Investiga cómo se inicializa un volumen con nombre a partir del contenido y los permisos de la imagen.
6. Agrega una **política de reinicio**.
7. (Opcional) Agrega un **healthcheck**. Ojo: si tu imagen final no tiene `curl` ni `wget`, piensa en alternativas (¿una imagen base distinta? ¿un pequeño programa?).

### Criterios de aceptación

- [ ] La imagen final pesa **menos de 30 MB** (la imagen de compilación pesa cientos de MB).
- [ ] La aplicación abre en el navegador y permite crear enlaces.
- [ ] Los enlaces cortos redirigen y el contador de visitas aumenta.
- [ ] Si eliminas y vuelves a crear el contenedor, **los enlaces siguen ahí**.
- [ ] Si eliminas también el volumen, los enlaces desaparecen (y entiendes por qué).
- [ ] El proceso dentro del contenedor **no** corre como `root`.

### Retos extra

- Pasa la versión de Go como **argumento de construcción** para poder cambiarla sin editar el `Dockerfile`.
- Construye la imagen para **otra arquitectura** (por ejemplo, `amd64` si estás en ARM, o al revés).
- Reemplaza el volumen con nombre por un **bind mount** a una carpeta local y abre `enlaces.json` con tu editor mientras la app corre.
- Investiga cómo reducir aún más el binario con *flags* del enlazador de Go.

---

## Errores comunes

| Síntoma | Causa probable |
|---|---|
| `exec format error` o `no such file or directory` al arrancar en la imagen mínima | El binario se compiló con `cgo` activo o para otra arquitectura. |
| `permission denied` al crear `enlaces.json` | El usuario del contenedor no puede escribir en `DATA_DIR`. |
| La app arranca pero el navegador no conecta | El puerto publicado no coincide con `PORT`, o falta publicar el puerto. |
| Los datos se pierden al recrear el contenedor | `DATA_DIR` no apunta a la ruta donde montaste el volumen. |

---

## Referencias

- Imagen oficial de Go: <https://hub.docker.com/_/golang>
- Builds multi-etapa: <https://docs.docker.com/build/building/multi-stage/>
- Volúmenes: <https://docs.docker.com/engine/storage/volumes/>
- Archivo `.dockerignore`: <https://docs.docker.com/build/concepts/context/#dockerignore-files>
- Paquete `embed` de Go: <https://pkg.go.dev/embed>
- Imágenes distroless: <https://github.com/GoogleContainerTools/distroless>

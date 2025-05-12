package main

import (
	"archive/zip"
	crand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed stt.zip
var htmlContent embed.FS

var videoExtensions = []string{".mp4", ".mkv", ".mov", ".webm", ".avi"}

func extractEmbeddedFiles() error {
	filesToExtract := []struct {
		sourceName  string
		targetName  string
		permissions os.FileMode
	}{
		{"stt.zip", "stt.zip", 0755},
	}

	for _, file := range filesToExtract {
		// Verificar si el archivo ya existe
		if _, err := os.Stat(file.targetName); err == nil {
			// El archivo ya existe, saltarlo
			fmt.Printf("El archivo %s ya existe, omitiendo extracción\n", file.targetName)
			continue
		}

		// Leer el archivo embebido
		fileData, err := htmlContent.ReadFile(file.sourceName)
		if err != nil {
			fmt.Printf("Error al leer %s embebido: %v\n", file.sourceName, err)
			continue // Continuar con el siguiente archivo en caso de error
		}

		// Escribir el archivo
		err = os.WriteFile(file.targetName, fileData, file.permissions)
		if err != nil {
			fmt.Printf("Error al extraer %s: %v\n", file.targetName, err)
		} else {
			fmt.Printf("Archivo %s extraído exitosamente\n", file.targetName)
		}
	}

	return nil
}

func extractZip(zipPath string, dest string) error {
	// Abre el archivo ZIP
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Itera sobre los archivos en el ZIP
	for _, f := range r.File {
		// Crea la ruta completa del archivo a extraer
		fPath := filepath.Join(dest, f.Name)

		// Crea los directorios necesarios
		if f.FileInfo().IsDir() {
			os.MkdirAll(fPath, os.ModePerm)
			continue
		}

		// Crea el archivo
		outFile, err := os.Create(fPath)
		if err != nil {
			return err
		}
		defer outFile.Close()

		// Copia el contenido del archivo ZIP al nuevo archivo
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateThumbnail(videoPath string) error {
	// Crear directorio de miniaturas si no existe
	thumbnailDir := "./static/thumbnails"
	os.MkdirAll(thumbnailDir, os.ModePerm)

	// Nombre de la miniatura
	videoFileName := filepath.Base(videoPath)
	thumbnailPath := filepath.Join(thumbnailDir, videoFileName+".jpg")

	// Si la miniatura ya existe, no la regeneramos
	if _, err := os.Stat(thumbnailPath); err == nil {
		return nil
	}

	// Comando FFmpeg para generar miniatura
	cmd := exec.Command("ffmpeg",
		"-i", filepath.Join("./static", videoPath),
		"-ss", "00:00:05",
		"-vframes", "1",
		"-q:v", "2",
		thumbnailPath)

	// Ejecutar comando
	if err := cmd.Run(); err != nil {
		log.Printf("Error generando miniatura para %s: %v", videoPath, err)
		return err
	}

	return nil
}

func listVideoFiles(dir string) ([]string, error) {
	var videos []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isVideoFile(info.Name()) {
			relativePath, _ := filepath.Rel(dir, path)

			// Generar miniatura
			generateThumbnail(relativePath)

			videos = append(videos, relativePath)
		}
		return nil
	})
	if len(videos) > 1 {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(videos), func(i, j int) {
			videos[i], videos[j] = videos[j], videos[i]
		})
	}
	return limitSlice(videos, 1000), err
}

func limitSlice(videos []string, limit int) []string {
	if len(videos) <= limit {
		return videos
	}
	return videos[:limit]
}

func isVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, validExt := range videoExtensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

func indexHandler(w http.ResponseWriter) {
	videos, err := listVideoFiles("./static")
	if err != nil {
		http.Error(w, "Error listing videos", http.StatusInternalServerError)
		return
	}

	// Renderizar template con videos
	tmpl, err := os.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Error reading template", http.StatusInternalServerError)
		return
	}

	videoList := strings.Join(videos, ",")
	renderedHTML := strings.ReplaceAll(string(tmpl), "{{VIDEO_LIST}}", videoList)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(renderedHTML))
}

func getCurrentDateTimeString() string {
	return time.Now().Format("2006_01_02_15_04_05")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Validar que sea un método POST
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Limitar tamaño de archivo a 100 MB
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20) // 100 MB

	// Procesar archivo multimedia
	file, header, err := r.FormFile("multimedia-upload")
	if err != nil {
		http.Error(w, "Error al procesar el archivo: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validar extensión del archivo
	ext := filepath.Ext(header.Filename)
	if strings.ToLower(ext) != ".mp4" {
		http.Error(w, "Solo se permiten archivos MP4", http.StatusBadRequest)
		return
	}

	// Calcular hash del archivo
	hash, err := calculateFileHash(file)
	if err != nil {
		http.Error(w, "Error al calcular hash del archivo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Verificar si el hash ya existe en la base de datos
	db, _ := sql.Open("sqlite3", "./valids.db")
	defer db.Close()
	exists, err := checkHashExists(db, hash)
	if err != nil {
		http.Error(w, "Error al verificar hash en la base de datos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "El archivo ya ha sido subido previamente", http.StatusConflict)
		return
	}

	// Reiniciar puntero del archivo para lectura
	if _, err := file.Seek(0, 0); err != nil {
		http.Error(w, "Error al reiniciar archivo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Obtener nombre del archivo sin extensión
	baseFileName := strings.TrimSuffix(header.Filename, ext)

	// Generar nombre de archivo con fecha
	currentDateTime := getCurrentDateTimeString()
	newFileName := fmt.Sprintf("%s_%s%s", baseFileName, currentDateTime, ext)

	// Crear directorio si no existe
	uploadDir := "./static"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		http.Error(w, "Error al crear directorio", http.StatusInternalServerError)
		return
	}

	// Ruta completa del archivo
	filePath := filepath.Join(uploadDir, newFileName)

	// Crear archivo
	out, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error al crear archivo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	// Copiar contenido del archivo
	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, "Error al guardar archivo: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Verificar tamaño final del archivo
	fileInfo, err := out.Stat()
	if err != nil {
		http.Error(w, "Error al obtener información del archivo", http.StatusInternalServerError)
		return
	}

	if fileInfo.Size() > 100<<20 { // 100 MB
		os.Remove(filePath) // Eliminar archivo si excede el tamaño
		http.Error(w, "El archivo excede el tamaño máximo de 100 MB", http.StatusBadRequest)
		return
	}

	// Guardar hash en la base de datos
	err = saveFileHash(db, hash, newFileName)
	if err != nil {
		http.Error(w, "Error al guardar hash en la base de datos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respuesta exitosa
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Archivo subido exitosamente"))
}

// Calcular hash SHA-256 del archivo
func calculateFileHash(file multipart.File) (string, error) {
	// Reiniciar puntero del archivo
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	// Crear hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Convertir hash a string hexadecimal
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Verificar si el hash ya existe en la base de datos
func checkHashExists(db *sql.DB, hash string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM file_hashes WHERE hash = ?", hash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func createDatabase() {
	db, _ := sql.Open("sqlite3", "./valids.db")
	db.Exec(`CREATE TABLE IF NOT EXISTS file_hashes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hash TEXT NOT NULL UNIQUE,
    filename TEXT,
    upload_date TEXT DEFAULT ''
);`)
	db.Close()
}

// Guardar hash en la base de datos
func saveFileHash(db *sql.DB, hash string, filename string) error {
	_, err := db.Exec("INSERT INTO file_hashes (hash, filename, upload_date) VALUES (?, ?, ?)",
		hash, filename, "")
	return err
}

func main() {
	// Crear directorio de miniaturas
	extractEmbeddedFiles()
	createDatabase()
	extractZip("stt.zip", ".")
	os.MkdirAll("./static/thumbnails", os.ModePerm)

	// Mapa para rastrear sesiones válidas y sus timestamps
	var sessionMutex sync.Mutex
	validSessions := make(map[string]time.Time)

	// Limpieza periódica de sesiones antiguas
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			sessionMutex.Lock()
			now := time.Now()
			for token, timestamp := range validSessions {
				if now.Sub(timestamp) > 30*time.Minute {
					delete(validSessions, token)
				}
			}
			sessionMutex.Unlock()
		}
	}()

	// Handler para la página principal
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Solo procesar la ruta raíz exacta
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Generar un token de sesión para la página principal
		sessionToken := generateSessionToken()

		sessionMutex.Lock()
		validSessions[sessionToken] = time.Now()
		sessionMutex.Unlock()

		// Establecer cookie de sesión
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   7200, // 30 minutos
		})

		// Llamar al manejador original
		indexHandler(w)
	})

	// Handler para la ruta de carga
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		// Verificar token de sesión
		if !hasValidSession(r, validSessions, &sessionMutex) {
			http.Error(w, "Acceso no autorizado", http.StatusForbidden)
			return
		}

		uploadHandler(w, r)
	})

	// Handler para recursos estáticos
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		// Verificar token de sesión y referer
		if !hasValidSession(r, validSessions, &sessionMutex) || !hasValidReferer(r) {
			http.Error(w, "Acceso no autorizado", http.StatusForbidden)
			return
		}

		// No permitir listar directorios
		if r.URL.Path == "/static/" || r.URL.Path == "/static" {
			http.Error(w, "Acceso no autorizado", http.StatusForbidden)
			return
		}

		// Servir el archivo estático
		fs := http.FileServer(http.Dir("."))
		//r.URL.Path = r.URL.Path
		fs.ServeHTTP(w, r)
	})

	// Handler para estilos
	http.HandleFunc("/styles/", func(w http.ResponseWriter, r *http.Request) {
		// Verificar token de sesión y referer
		if !hasValidSession(r, validSessions, &sessionMutex) || !hasValidReferer(r) {
			http.Error(w, "Acceso no autorizado", http.StatusForbidden)
			return
		}

		// No permitir listar directorios
		if r.URL.Path == "/styles/" || r.URL.Path == "/styles" {
			http.Error(w, "Acceso no autorizado", http.StatusForbidden)
			return
		}

		// Servir el archivo de estilos
		fs := http.FileServer(http.Dir("."))
		//r.URL.Path = r.URL.Path
		fs.ServeHTTP(w, r)
	})

	fmt.Println("Servidor iniciado en http://localhost:8087")
	log.Fatal(http.ListenAndServe(":8087", nil))
}

// Función para generar un token de sesión aleatorio
func generateSessionToken() string {
	b := make([]byte, 32)
	crand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Función para verificar si la solicitud tiene una sesión válida
func hasValidSession(r *http.Request, validSessions map[string]time.Time, mutex *sync.Mutex) bool {
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		return false
	}

	mutex.Lock()
	timestamp, exists := validSessions[cookie.Value]
	valid := exists && time.Since(timestamp) < 30*time.Minute
	if valid {
		// Actualizar el timestamp para extender la sesión
		validSessions[cookie.Value] = time.Now()
	}
	mutex.Unlock()

	return valid
}

// Función para verificar si el referer es válido
func hasValidReferer(r *http.Request) bool {
	referer := r.Header.Get("Referer")
	if referer == "" {
		return false
	}

	// Verificar que el referer sea del mismo host
	refURL, err := url.Parse(referer)
	if err != nil {
		return false
	}

	// Comprobar que el referer sea del mismo host y puerto
	return refURL.Host == r.Host
}

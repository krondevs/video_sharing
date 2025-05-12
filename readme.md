# VideoSharing 🎥 - Plataforma Local de Compartición de Videos

## 🌟 Descripción del Proyecto

VideoSharing es una aplicación web ligera y segura desarrollada en Go, diseñada para compartir videos de manera privada y anónima en redes locales.

## ✨ Características Principales

- 🔒 Sesiones seguras con tokens de acceso
- 📤 Subida de videos sin registro
- 🎞 Generación automática de miniaturas
- 💾 Base de datos SQLite3 integrada
- 🛡️ Protección contra descargas masivas
- 👥 Navegación completamente anónima

## 🛠 Requisitos Técnicos

### Dependencias
- Go 1.18+ 
- FFmpeg (para generación de miniaturas)
- SQLite3

### Librerías Go
- `github.com/mattn/go-sqlite3`

## 🚀 Instalación Multiplataforma

VideoSharing se compila directamente desde fuentes en Windows, macOS y Linux.

### Requisitos Previos

- Instalar Go desde [golang.org](https://golang.org/dl/)
- Instalar FFmpeg 
- Windows: Descargar desde [FFmpeg oficial](https://ffmpeg.org/download.html)
- macOS: `brew install ffmpeg`
- Linux: `sudo apt-get install ffmpeg`

### Compilación en Windows

```powershell
# Clonar el repositorio
git clone https://github.com/tu-usuario/videosharing.git
cd videosharing

# Descargar dependencias
go mod tidy

# Compilar
go build -o videosharing.exe
```

### Compilación en macOS/Linux

```bash
# Clonar el repositorio
git clone https://github.com/tu-usuario/videosharing.git
cd videosharing

# Descargar dependencias
go mod tidy

# Compilar
go build -o videosharing
```

## 🔧 Configuración

### Estructura de Directorios

```
videosharing/
├── static/
│   └── thumbnails/  # Miniaturas generadas
├── templates/       # Plantillas HTML
├── valids.db        # Base de datos SQLite
└── stt.zip          # Recursos embebidos
```

### Configuraciones Predeterminadas

- **Puerto**: 8087
- **Directorio de Videos**: `./static`
- **Máximo Tamaño de Subida**: 100 MB
- **Formatos Soportados**: 
- .mp4
- .mkv
- .mov
- .webm
- .avi

## 🔐 Seguridad

- Tokens de sesión con caducidad de 30 minutos
- Validación de referer
- Hash único para prevenir subidas duplicadas
- Protección contra listado de directorios

## 📋 Funcionalidades

- Subida anónima de videos
- Generación automática de miniaturas
- Listado aleatorio de videos (máximo 1000)
- Base de datos de hashes para control de duplicados

## 🤝 Contribuciones

1. Haz un Fork del repositorio
2. Crea una rama (`git checkout -b feature/nueva-caracteristica`)
3. Commit de cambios (`git commit -m 'Añadir nueva característica'`)
4. Push a la rama (`git push origin feature/nueva-caracteristica`)
5. Crear un Pull Request

## ⚠️ Advertencias

- Uso exclusivo en redes locales privadas
- No compartir contenido sensible o ilegal
- Mantener actualizada la aplicación

## 📄 Licencia

Proyecto bajo Licencia MIT.

## 🐞 Reportar Errores

Abre un issue en GitHub o contacta con el equipo de desarrollo.
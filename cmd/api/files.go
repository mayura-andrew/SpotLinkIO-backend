package main

import (
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strings"

    "github.com/julienschmidt/httprouter"
)

// serveAvatarHandler serves avatar images by ID with proper fallback to default avatar
func (app *application) serveAvatarHandler(w http.ResponseWriter, r *http.Request) {
    params := httprouter.ParamsFromContext(r.Context())
    id := params.ByName("id")

    // Clean the ID to prevent directory traversal
    id = filepath.Clean(strings.TrimSpace(id))
    if id == "" || id == "." || strings.Contains(id, "/") || strings.Contains(id, "\\") {
        http.NotFound(w, r)
        return
    }

    // Define the avatar directory
    avatarDir := "../../uploads/avatars"

    // First try to find the avatar with any supported extension
    extensions := []string{".jpg", ".jpeg", ".png", ".gif"}
    var filePath string
    var found bool

    for _, ext := range extensions {
        testPath := filepath.Join(avatarDir, id+ext)
        if _, err := os.Stat(testPath); err == nil {
            filePath = testPath
            found = true
            break
        }
    }

    // If avatar not found, serve a default avatar
    if !found {
        // Log this but don't crash the application
        app.logger.PrintInfo(fmt.Sprintf("Avatar not found for ID: %s, using default", id), nil)
        
        // Use a default avatar (ensure this file exists)
        filePath = filepath.Join(avatarDir, "default.png")
        if _, err := os.Stat(filePath); err != nil {
            // If default avatar doesn't exist, return a 404
            http.NotFound(w, r)
            return
        }
    }

    // Set appropriate content type based on file extension
    extension := strings.ToLower(filepath.Ext(filePath))
    contentType := "application/octet-stream" // default
    
    switch extension {
    case ".jpg", ".jpeg":
        contentType = "image/jpeg"
    case ".png":
        contentType = "image/png"
    case ".gif":
        contentType = "image/gif"
    }
    
    w.Header().Set("Content-Type", contentType)
    w.Header().Set("Cache-Control", "public, max-age=604800") // Cache for a week
    
    // Serve the file
    http.ServeFile(w, r, filePath)
}

// servePDFHandler serves PDF files by ID
func (app *application) servePDFHandler(w http.ResponseWriter, r *http.Request) {
    params := httprouter.ParamsFromContext(r.Context())
    id := params.ByName("id")

    // Clean the ID to prevent directory traversal
    id = filepath.Clean(strings.TrimSpace(id))
    if id == "" || id == "." || strings.Contains(id, "/") || strings.Contains(id, "\\") {
        http.NotFound(w, r)
        return
    }

    // Define the PDF directory
    pdfDir := "../../uploads"
    filePath := filepath.Join(pdfDir, id+".pdf")

    // Check if file exists
    if _, err := os.Stat(filePath); err != nil {
        http.NotFound(w, r)
        return
    }

    // Set appropriate content type
    w.Header().Set("Content-Type", "application/pdf")
    w.Header().Set("Cache-Control", "no-store") // Don't cache sensitive documents
    
    // Serve the file
    http.ServeFile(w, r, filePath)
}

// serveFilesHandler is a more general file handler that can serve various types of files
func (app *application) serveFilesHandler(w http.ResponseWriter, r *http.Request) {
    params := httprouter.ParamsFromContext(r.Context())
    fileType := params.ByName("type") // e.g., "avatars", "pdfs", etc.
    id := params.ByName("id")

    // Clean and validate the input
    fileType = filepath.Clean(strings.TrimSpace(fileType))
    id = filepath.Clean(strings.TrimSpace(id))
    
    if fileType == "" || id == "" || fileType == "." || id == "." || 
       strings.Contains(fileType, "/") || strings.Contains(id, "/") ||
       strings.Contains(fileType, "\\") || strings.Contains(id, "\\") {
        http.NotFound(w, r)
        return
    }

    // Define allowed file types and their directories
    allowedTypes := map[string]string{
        "avatars": "../../uploads/avatars",
        "pdfs":    "../../uploads",
        // Add other file types as needed
    }

    // Check if the file type is allowed
    baseDir, allowed := allowedTypes[fileType]
    if !allowed {
        http.NotFound(w, r)
        return
    }

    // For specific file types, use specialized handlers
    if fileType == "avatars" {
        app.serveAvatarHandler(w, r)
        return
    } else if fileType == "pdfs" {
        app.servePDFHandler(w, r)
        return
    }

    // Generic file serving for any other allowed types
    var extensions []string
    switch fileType {
    case "pdfs":
        extensions = []string{".pdf"}
    default:
        extensions = []string{".jpg", ".jpeg", ".png", ".gif", ".pdf"}
    }

    var filePath string
    var found bool

    for _, ext := range extensions {
        testPath := filepath.Join(baseDir, id+ext)
        if _, err := os.Stat(testPath); err == nil {
            filePath = testPath
            found = true
            break
        }
    }

    if !found {
        http.NotFound(w, r)
        return
    }

    // Set appropriate content type based on file extension
    extension := strings.ToLower(filepath.Ext(filePath))
    contentType := "application/octet-stream" // default
    
    switch extension {
    case ".jpg", ".jpeg":
        contentType = "image/jpeg"
    case ".png":
        contentType = "image/png"
    case ".gif":
        contentType = "image/gif"
    case ".pdf":
        contentType = "application/pdf"
    }
    
    w.Header().Set("Content-Type", contentType)
    
    // Cache static assets but not sensitive documents
    if fileType == "avatars" {
        w.Header().Set("Cache-Control", "public, max-age=604800") // Cache for a week
    } else {
        w.Header().Set("Cache-Control", "no-store") // Don't cache
    }
    
    // Serve the file
    http.ServeFile(w, r, filePath)
}
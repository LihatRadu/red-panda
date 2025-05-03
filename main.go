package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/disintegration/imaging"
)

const (
    uploadDir = "./uploads"
    convertedDir = "./converted"
    maxUploadSize = 10 << 20 
)

func convertHandler(w http.ResponseWriter, r *http.Request)  {
   
   if r.Method != http.MethodPost {
     http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
     return
   }

    err := r.ParseMultipartForm(maxUploadSize)
    if err != nil {
        http.Error(w, "File too big", http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("file")
    if err !=nil {
        http.Error(w, "", http.StatusBadRequest)
        return
    }
    defer file.Close()

    filename := header.Filename
    if !strings.HasSuffix(strings.ToLower(filename), ".png") {
        http.Error(w, "Only PNG files allowed", http.StatusBadRequest)
        return
    }

    uploadPath := filepath.Join(uploadDir, fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename))
    dst, err := os.Create(uploadPath)
    if err != nil {
        http.Error(w, "Save failed", http.StatusBadRequest)
        return
    }
    defer dst.Close()

    _, err = io.Copy(dst, file)
    if err != nil {
        http.Error(w, "Failed to save", http.StatusBadRequest)
        return
    }

    convertedPath, err := convertToJPG(uploadPath)
    if err != nil {
        http.Error(w, "Failed conversion", http.StatusBadRequest)
        return
    }

    convertedFile, err := os.Open(convertedPath)
    if err != nil {
        http.Error(w, "File not  found", http.StatusBadRequest)
        return
    }
    defer convertedFile.Close()

    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.TrimSuffix(filename, ".png") + ".jpg"))
    io.Copy(w, convertedFile)
}

func convertToJPG(inputPath string) (string, error) {
    img, err := imaging.Open(inputPath)
    if err != nil {
        return "", fmt.Errorf("failed to open image: %v", err)
    }

    outputFilename := strings.TrimSuffix(filepath.Base(inputPath), ".png") + ".jpg"
    outputPath := filepath.Join(convertedDir, outputFilename)

    err = imaging.Save(img, outputPath, imaging.JPEGQuality(90))
    if err != nil {
        return "", fmt.Errorf("failed to save image: %v", err)
    }
    return outputPath,  nil
}

func main()  {
   os.MkdirAll(uploadDir, os.ModePerm)
   os.Mkdir(convertedDir, os.ModePerm)

    http.Handle("/", templ.Handler(indexPage()))
    http.Handle("/convert", http.HandlerFunc(convertHandler))
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    port := ":9000"
    fmt.Printf("Server running on port: ", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

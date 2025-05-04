package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/disintegration/imaging"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

const (
    uploadDir = "./uploads"
    convertedDir = "./converted"
    maxUploadSize = 20 << 20 
)

var supportedFormats = map[string]bool{
    ".png": true,
    ".jpg": true,
    ".webp": true,
    ".jpeg": true,
    ".svg": true,
    ".tiff": true,
    ".tif": true,
    ".bmp": true,
    ".gif": true,
    ".psd": true,
}
func convertHandler(w http.ResponseWriter, r *http.Request)  {
   
   if r.Method != http.MethodPost {
     http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
     return
   }

    fileID := fmt.Sprintf("%d", time.Now().UnixNano())
    progress[fileID] = 0

    err := r.ParseMultipartForm(maxUploadSize)
    if err != nil {
        delete(progress, fileID)
        http.Error(w, "File too big", http.StatusBadRequest)
        return
    }
    progress[fileID] = 10

    file, header, err := r.FormFile("file")
    if err !=nil {
        delete(progress, fileID)
        http.Error(w, "", http.StatusBadRequest)
        return
    }
    defer file.Close()

    filename := header.Filename
    ext := strings.ToLower(filepath.Ext(filename))
    if !supportedFormats[ext] {
        delete(progress, fileID)
        http.Error(w, "Only PNG files allowed", http.StatusBadRequest)
        return
    }
    progress[fileID] = 20

    uploadPath := filepath.Join(uploadDir, fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename))
    dst, err := os.Create(uploadPath)
    if err != nil {
        delete(progress, fileID)
        http.Error(w, "Save failed", http.StatusBadRequest)
        return
    }
    defer dst.Close()

    _, err = io.Copy(dst, file)
    if err != nil {
        delete(progress, fileID)
        http.Error(w, "Failed to save", http.StatusBadRequest)
        return
    }
    progress[fileID] = 40

    convertedPath, err := convertToJPG(uploadPath, fileID)
    if err != nil {
        delete(progress, fileID)
        http.Error(w, "Failed conversion", http.StatusBadRequest)
        return
    }
    progress[fileID] = 90

    convertedFile, err := os.Open(convertedPath)
    if err != nil {
        delete(progress, fileID)
        http.Error(w, "File not  found", http.StatusBadRequest)
        return
    }
    defer convertedFile.Close()

    progress[fileID] = 100

    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.TrimSuffix(filename, ext) + ".jpg"))
    io.Copy(w, convertedFile)

    go func() {
        time.Sleep(30 * time.Second)
        delete(progress, fileID)
    }()
}

func convertToJPG(inputPath, fileID string) (string, error) {

    ext := strings.ToLower(filepath.Ext(inputPath))
    var img image.Image
    var err error

    switch ext {
    case ".png", ".jpg", ".jpeg", ".bmp":
        img, err = imaging.Open(inputPath)
    case ".gif":
       f, err := os.Open(inputPath)
       if err != nil {
         return "", err
       }
       defer f.Close()
       img, err = gif.Decode(f)
    case ".webp":
       f, err := os.Open(inputPath)
       if err != nil {
         return "", err
       }
       defer f.Close()
       img, err = webp.Decode(f)
    case ".svg":
       icon, err := oksvg.ReadIcon(inputPath)
       if err != nil {
         return "", err
       }

       w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
       img = image.NewRGBA(image.Rect(0,0, w, h))
       scanner := rasterx.NewScannerGV(w, h, img.(draw.Image), img.Bounds())
       raster := rasterx.NewDasher(w,h, scanner)
       icon.Draw(raster, 1.0)
    case ".tiff", ".tif":
       f, err := os.Open(inputPath)
       if err != nil {
         return "", err
       }
       defer f.Close()
       img, err = tiff.Decode(f)
    case ".psd":
      img, err = imaging.Open(inputPath)
    default:
      return "", fmt.Errorf("unsupported format: %s", ext)
    }
    if err != nil {
        return "", err
    }

    progress[fileID] = 60

    outputFilename := strings.TrimSuffix(filepath.Base(inputPath), ext) + ".jpg"
    outputPath := filepath.Join(convertedDir, outputFilename)

    progress[fileID] = 80

    err = imaging.Save(img, outputPath, imaging.JPEGQuality(90))
    if err != nil {
        return "", fmt.Errorf("failed to save image: %v", err)
    }
    return outputPath,  nil
}

var progress = make(map[string]int)

func progressHandler(w http.ResponseWriter, r *http.Request)  {
    fileID :=  r.URL.Query().Get("id")
    if progress, exists := progress[fileID]; exists {
        fmt.Fprintf(w, "%d", progress)
        return
    }
    fmt.Fprintf(w, "0")
}

func main()  {
   os.MkdirAll(uploadDir, os.ModePerm)
   os.MkdirAll(convertedDir, os.ModePerm)

    http.Handle("/", templ.Handler(indexPage()))
    http.Handle("/convert", http.HandlerFunc(convertHandler))
    http.Handle("/progress", http.HandlerFunc(progressHandler))
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    port := ":9000"
    fmt.Printf("Server running on port: %s", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

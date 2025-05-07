package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
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
    "github.com/h2non/filetype"
    //"golang.org/x/image/webp"
    //"github.com/chai2010/webp"
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

var outputFormats = []string{"jpg", "png", "webp", "gif", "bmp"}

type ConversionRequest struct{
    File []byte
    Filename string
    Format string
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
        http.Error(w, "Invalid file upload", http.StatusBadRequest)
        return
    }
    defer file.Close()

    if err := validateFileSize(header.Size); err != nil {
        delete(progress, fileID)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }


    outputFormat := r.FormValue("format")
    if !contains(outputFormats, outputFormat) {
        delete(progress, fileID)
        http.Error(w, "", http.StatusBadRequest)
        return
    }

    filename := sanitizeFilename(header.Filename)
    ext := strings.ToLower(filepath.Ext(filename))
    if !supportedFormats[ext] {
        delete(progress, fileID)
        http.Error(w, "Unsupported file format", http.StatusBadRequest)
        return
    }

    if err := validateMIMEType(file, filename); err != nil {
        delete(progress, fileID)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := validateFileSignature(file, ext); err != nil {
       delete(progress, fileID)
       http.Error(w, err.Error(), http.StatusBadRequest)
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

    convertedPath, err := convertToJPG(uploadPath, fileID, outputFormat)
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

    contentType := getContentType(outputFormat)
    w.Header().Set("Content-Type", contentType)
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.TrimSuffix(filename, ext) + "." + outputFormat))
    io.Copy(w, convertedFile)

    go func() {
        time.Sleep(30 * time.Second)
        delete(progress, fileID)
    }()
}

func convertToJPG(inputPath, fileID, outputFormat string) (string, error) {

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
//       img, err = webp.Decode(f)
    case ".svg":
//       svgFile.Seek(0,0)
        svgFile, err := os.Open(inputPath)
        if err != nil {
            return "", err
        }
        defer svgFile.Close()

        if err := validateSVG(svgFile); err != nil {
            return "", fmt.Errorf("Invalid SVG: %v", err)
        }
        svgFile.Seek(0,0)
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

    outputFilename := strings.TrimSuffix(filepath.Base(inputPath), ext) + "." + outputFormat
    outputPath := filepath.Join(convertedDir, outputFilename)

    progress[fileID] = 80

    err = imaging.Save(img, outputPath, imaging.JPEGQuality(90))
    if err != nil {
        return "", fmt.Errorf("failed to save image: %v", err)
    }

    switch outputFormat {
    case "jpg", "jpeg":
        err = imaging.Save(img, outputPath, imaging.JPEGQuality(90))
    case "png":
        err = imaging.Save(img, outputPath, imaging.PNGCompressionLevel(png.DefaultCompression))
    case "webp":
     //   err = imaging.Save(img, outputPath, imaging.WebPQuality(90))
    case "gif":
        err = imaging.Save(img, outputPath)
    case "bmp":
       err = imaging.Save(img, outputPath)
    default:
       return "", fmt.Errorf("unsupported output format: %s", outputFormat)
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

func getContentType(format string) string {
    switch format {
    case "jpg", "jpeg":
        return "image/jpeg"
    case "png":
       return "image/png"
    case "webp":
       return "image/webp"
    case "gif":
      return "image/gif"
    case "bmp":
      return "image/bmp"
    default:
       return "application/octet-stream"
    }
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}

func validateFileSignature(file io.ReadSeeker, expectedExt string) error {
    header := make([]byte, 512)
    _, err := file.Read(header)
    if err != nil {
        return fmt.Errorf("failed to read file header")
    }
    file.Seek(0,0)

    kind, err := filetype.Match(header)
    if err != nil {
        return fmt.Errorf("failed to detect the type of the file")
    }

    allowedTypes := map[string]string{
        ".png":  "image/png",
        ".jpg":  "image/jpeg",
        ".jpeg": "image/jpeg",
        ".gif":  "image/gif",
        ".bmp":  "image/bmp",
        ".tiff": "image/tiff",
        ".webp": "image/webp",
    }

    expectedMIME, ok := allowedTypes[expectedExt]
    if !ok {
        return fmt.Errorf("unsupported file extension.")
    }

    if kind.MIME.Value != expectedMIME {
        return fmt.Errorf("file content does not match the extension. Expected %s instead of %s", expectedMIME, kind.MIME.Value)
    }

    return nil
}

func validateMIMEType(file io.ReadSeeker, filename string) error {
    
//    ext := strings.ToLower(filepath.Ext(filename))

    buffer := make([]byte, 512)
    _, err := file.Read(buffer)
    if err != nil {
        return fmt.Errorf("failed to read MIME for file detection.")
    }
    file.Seek(0,0)

    mimeType := http.DetectContentType(buffer)

    allowedMIMEs := map[string]bool{
        "image/png":              true,
        "image/jpeg":             true,
        "image/gif":              true,
        "image/bmp":              true,
        "image/tiff":             true,
        "image/webp":             true,
        "application/octet-stream": true, // For some image types
    }

    if !allowedMIMEs[mimeType] {
        return fmt.Errorf("invalid MIME type: %s", mimeType)
    }

    return nil
}

func validateFileSize(size int64) error {
    
    if size > maxUploadSize {
        return fmt.Errorf("file size has been exceeded the limit of %d bytes.", maxUploadSize)
    }

    return nil
}

func sanitizeFilename(filename string) string {
    
    filename = filepath.Base(filename)
    filename = strings.ReplaceAll(filename, "/", "_")
    filename = strings.ReplaceAll(filename, "\\", "_")
    filename = strings.ReplaceAll(filename, ":", "_")

    return filename
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

package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "errors"
    "io"
    "mime/multipart"
    "net/http"
)

// allowedImageContentTypes are the sniffed MIME types we accept (BUG-71).
var allowedImageContentTypes = map[string]bool{
    "image/jpeg": true,
    "image/png":  true,
    "image/webp": true,
}

// validateImageContent sniffs the first bytes of the upload instead of
// trusting the file extension.
func validateImageContent(fh *multipart.FileHeader) error {
    f, err := fh.Open()
    if err != nil {
        return errors.New("could not read uploaded file")
    }
    defer f.Close()
    buf := make([]byte, 512)
    n, err := io.ReadFull(f, buf)
    if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
        return errors.New("could not read uploaded file")
    }
    if !allowedImageContentTypes[http.DetectContentType(buf[:n])] {
        return errors.New("file content is not a valid jpg, png or webp image")
    }
    return nil
}

// randSuffix returns 8 random hex chars so concurrent uploads never collide.
func randSuffix() string {
    b := make([]byte, 4)
    if _, err := rand.Read(b); err != nil {
        return "00000000"
    }
    return hex.EncodeToString(b)
}
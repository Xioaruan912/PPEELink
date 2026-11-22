package middlewares

import (
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// StaticFS serves embedded static assets with pooled gzip compression,
// proper Vary headers, and immutable cache control for fingerprinted bundles.
func StaticFS(root fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if !strings.HasPrefix(reqPath, "/static/") {
			c.Next()
			return
		}
		clean := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(reqPath, "/static/")), "/")
		if clean == "" || clean == "." || strings.Contains(clean, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		file, err := root.Open(clean)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil || info.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}

		ct := mime.TypeByExtension(filepath.Ext(clean))
		if ct == "" {
			ct = "application/octet-stream"
		}
		c.Header("Content-Type", ct)
		c.Header("Cache-Control", "public, max-age=31536000, immutable")

		acceptsGzip := strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip")
		if acceptsGzip && isCompressible(ct) && info.Size() >= 512 {
			c.Header("Content-Encoding", "gzip")
			c.Header("Vary", "Accept-Encoding")
			c.Status(http.StatusOK)
			gz := gzipWriterPool.Get().(*gzip.Writer)
			gz.Reset(c.Writer)
			defer func() {
				_ = gz.Close()
				gz.Reset(io.Discard)
				gzipWriterPool.Put(gz)
			}()
			_, _ = io.Copy(gz, file)
			return
		}

		c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, file)
	}
}

func isCompressible(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/") ||
		strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "application/javascript") ||
		strings.Contains(ct, "application/x-javascript") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "image/svg+xml") ||
		strings.Contains(ct, "font/") ||
		strings.Contains(ct, "application/wasm")
}

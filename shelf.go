package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var port = flag.String("p", "8888", "Port to listen on")
var storage = flag.String("s", "/var/lib/shelf", "Path to store files")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:	%v -p <port> -s <storage-path>\n\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func errorResponse(w http.ResponseWriter, err error) {
	if err == ErrNotFound {
		http.Error(w, "NotFound", 404)
		return
	}
	log.Println("error:", err)
	http.Error(w, "Internal Server Error", 500)
}

type File interface {
	io.ReadSeeker
	io.Closer
	Size() int64
	ModTime() time.Time
	Type() string
	ETag() string
}

type Filesystem interface {
	Open(name string) (File, error)
	Put(name string, r io.Reader, typ string) error
	Delete(name string) error
}

var ErrNotFound = errors.New("file not found")

func handler(fs Filesystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "HEAD", "GET":
			file, err := fs.Open(req.URL.Path)
			if err != nil {
				errorResponse(w, err)
				return
			}
			defer file.Close()
			log.Println("GET", req.RequestURI)
			w.Header().Set("Content-Length", strconv.FormatInt(file.Size(), 10))
			w.Header().Set("Content-Type", file.Type())
			w.Header().Set("Etag", file.ETag())
			http.ServeContent(w, req, req.URL.Path, file.ModTime(), file)
		case "PUT":
			err := fs.Put(req.URL.Path, req.Body, req.Header.Get("Content-Type"))
			if err != nil {
				errorResponse(w, err)
				return
			}
			log.Println("PUT", req.RequestURI)
		case "DELETE":
			err := fs.Delete(req.URL.Path)
			if err != nil {
				errorResponse(w, err)
				return
			}
			log.Println("DELETE", req.RequestURI)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func main() {
	flag.Parse()

	log.Println("Shelf serving files on " + *port + " from " + *storage)
	log.Fatal(http.ListenAndServe(":"+*port, handler(NewOSFilesystem(*storage))))
}

package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	//go:embed static/css
	css embed.FS
	//go:embed static/icons
	icons embed.FS
	//go:embed static/browserconfig.xml
	browserconfig string
	//go:embed static/site.webmanifest
	webmanifest string
	//go:embed views/index.html
	index string
)

func main() {
	port := flag.String("port", "8080", "a string")
	flag.Parse()

	// Echo instance
	e := echo.New()

	// Middleware
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Secure())
	e.Use(middleware.CSRF())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Static("/static/resources", "static/resources")

	e.GET("/static/css/*", echo.WrapHandler(http.StripPrefix("/", http.FileServer(http.FS(css)))))
	e.GET("/static/icons/*", echo.WrapHandler(http.StripPrefix("/", http.FileServer(http.FS(icons)))))
	e.GET("/static/browserconfig.xml", func(c echo.Context) error {
		return c.XML(http.StatusOK, browserconfig)
	})
	e.GET("/static/site.webmanifest", func(c echo.Context) error {
		return c.JSON(http.StatusOK, webmanifest)
	})

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, index)
	})
	e.GET("/precise/:folder", func(c echo.Context) error {
		return c.HTML(http.StatusOK, index)
	})

	e.GET("/api/files/:dir", func(c echo.Context) error {
		path := filepath.Join("static/resources/", c.Param("dir"))
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return c.String(http.StatusNotFound, "Not found.")
		}
		files := get_files(path)
		files_json, _ := json.Marshal(files)
		return c.JSONBlob(http.StatusOK, files_json)
	})
	e.GET("/api/files", func(c echo.Context) error {
		path := filepath.Join("static/resources/")
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return c.String(http.StatusNotFound, "Not found.")
		}
		files := get_files(path)
		files_json, _ := json.Marshal(files)
		return c.JSONBlob(http.StatusOK, files_json)
	})

	// Start server
	e.Logger.Fatal(e.Start(":" + *port))

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	// Use a buffered channel to avoid missing signals as recommended for signal.Notify
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func get_files(dir string) []map[string]string {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	file_map_slice := make([]map[string]string, 0)

	for _, file := range files {
		if !file.IsDir() {
			path := filepath.Join(dir, file.Name())
			mtype, err := mimetype.DetectFile(path)
			if err != nil {
				log.Fatal(err)
			}
			switch strings.Split(mtype.String(), "/")[0] {
			case
				"image",
				"video":
				file_map := map[string]string{
					"type": mtype.String(),
					"url":  dir + "/" + file.Name(),
				}
				file_map_slice = append(file_map_slice, file_map)
			}
		}
	}
	return file_map_slice
}

func getAllFilenames(fs *embed.FS, path string) (out []string, err error) {
	if len(path) == 0 {
		path = "."
	}
	entries, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		fp := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			res, err := getAllFilenames(fs, fp)
			if err != nil {
				return nil, err
			}
			out = append(out, res...)
			continue
		}
		out = append(out, fp)
	}
	return
}

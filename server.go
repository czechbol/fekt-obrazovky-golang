package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"golang.org/x/crypto/acme/autocert"
)

var index string = `<!DOCTYPE html>
<html>

<head>
    <title>FEKT Obrazovky</title>

    <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
    <link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
    <link rel="manifest" href="/site.webmanifest">
    <link rel="mask-icon" href="/safari-pinned-tab.svg" color="#5bbad5">
    <meta name="msapplication-TileColor" content="#2d89ef">
    <meta name="theme-color" content="#ffffff">

    <link rel="stylesheet" href="/css/page.css" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <script>
        var i = 0
        let files
        var slideTime = 15000

        window.onload = update

        async function build() {
            const container = document.getElementById("container")
            container.remove()
            newContainer = document.createElement("div")
            newContainer.id = "container"
            if (files.length === 0) {
                let h3 = document.createElement("h3")
                let node = document.createTextNode("No image or video");
                h3.appendChild(node)
                newContainer.appendChild(h3);
                let meta = document.createElement("meta")
                meta.httpEquiv = "refresh";
                meta.content = "60";
                newContainer.appendChild(meta);
            } else {
                files.forEach(function (file, i) {
                    let element
                    if (file.type.includes("image")) {
                        element = document.createElement("img")
                        element.classList.add("invisible")
                        element.id = "background-" + i
                        element.src = file.url
                    } else if (file.type.includes("video")) {
                        element = document.createElement("video")
                        element.classList.add("invisible")
                        element.id = "background-" + i
                        element.muted = true
                        const subelement = document.createElement("source")
                        subelement.id = "background-" + i + "-source"
                        subelement.src = file.url
                        subelement.type = file.type
                        const node = document.createTextNode("Your browser does not support HTML5 video.");
                        element.appendChild(subelement)
                        element.appendChild(node)
                    }
                    newContainer.appendChild(element);
                })
            }
            document.body.appendChild(newContainer)
        }

        function change() {
            if (i >= files.length) {
                document.location.reload(true)
                return
            }
            var prev_file = files[i]
            if (i > 0) {
                var j = i - 1;
                var old = document.getElementById("background-" + j);
                old.classList.remove('visible');
                old.classList.add('invisible');
            }
            var item = document.getElementById("background-" + i);
            item.classList.remove('invisible');
            item.classList.add('visible');
            var file = files[i];
            i++;

            if (file.type.includes("image")) {
                setTimeout(change, slideTime);
            } else {
                item.addEventListener('ended', function () { change() }, false);
                item.load();
                item.play();
            }
        }

        async function update() {
            let response
            if (window.location.pathname === "/") {
                response = await fetch("/api/files/")
            } else {
                var substring = "/precise/"
                const path = window.location.pathname.slice(substring.length)
                response = await fetch("/api/files/" + path)

            }
            files = await response.json()
            await build()
            i = 0
            if (i < files.length) {
                await change()
            }
        }
    </script>
</head>

<body>
    <div id="container"></div>
</body>

</html>`

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.AutoTLSManager.Cache = autocert.DirCache("/var/www/.cache")
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.Secure())
	e.Use(middleware.CSRF())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())
	e.Static("/", "static")

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, index)
	})
	e.GET("/precise/:folder", func(c echo.Context) error {
		return c.HTML(http.StatusOK, index)
	})

	e.GET("/api/files/:dir", func(c echo.Context) error {
		path := filepath.Join("static/resources/", c.Param("dir"))
		fmt.Println(path)
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
		fmt.Println(path)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return c.String(http.StatusNotFound, "Not found.")
		}
		files := get_files(path)
		files_json, _ := json.Marshal(files)
		return c.JSONBlob(http.StatusOK, files_json)
	})

	// Start server
	e.Logger.Fatal(e.Start(":1324"))

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
	dir_path := strings.TrimPrefix(dir, "static")
	fmt.Println(dir_path)
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
					"url":  dir_path + "/" + file.Name(),
				}
				file_map_slice = append(file_map_slice, file_map)
			}
		}
	}
	return file_map_slice
}

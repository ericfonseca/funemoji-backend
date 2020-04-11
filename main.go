package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

var cache = make(map[string]*bytes.Buffer)
var maxCacheSize = 25000 //50000 * 25KB ~ 0.6 GB

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func Generate(w http.ResponseWriter, r *http.Request) {

	// CORS support
	setupResponse(&w, r)
	if r.Method == "OPTIONS" {
		return
	}

	top := r.URL.Query().Get("top")
	bottom := r.URL.Query().Get("bottom")
	percentStr := r.URL.Query().Get("percent")

	if top == "" || bottom == "" || percentStr == "" {
		fmt.Print("top, bottom, or percent missing cannot proceed")
		w.WriteHeader(400)
		return
	}

	percent, err := strconv.ParseInt(percentStr, 10, 32)
	if err != nil {
		fmt.Printf("percent %s was not parseable", percentStr)
		w.WriteHeader(400)
		return
	}

	if percent < 0 || percent > 100 {
		fmt.Printf("percent outside of allowed bounds: %d", percent)
		w.WriteHeader(400)
		return
	}

	cachedEmojiFile := fmt.Sprintf("emoji_%+q_%+q_%d", top, bottom, percent)
	buf, found := cache[cachedEmojiFile]
	if found {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
		w.Header().Set("Cache-Hit", "true")

		// by default writes 200 ok header
		if _, err := w.Write(buf.Bytes()); err != nil {
			fmt.Print("could not write image")
			w.WriteHeader(500)
		}
		return
	}

	topImageFile, err := os.Open(fmt.Sprintf("./assets/%s.png", top))
	if err != nil {
		fmt.Printf("first emoji %s.png not found", top)
		w.WriteHeader(500)
		return
	}

	topImage, err := png.Decode(topImageFile)
	if err != nil {
		fmt.Print("could not decode first image")
		w.WriteHeader(500)
		return
	}

	bottomImageFile, err := os.Open(fmt.Sprintf("./assets/%s.png", bottom))
	if err != nil {
		fmt.Printf("second emoji %s.png not found", bottom)
		w.WriteHeader(500)
		return
	}

	bottomImage, err := png.Decode(bottomImageFile)
	if err != nil {
		fmt.Print("could not decode second image")
		w.WriteHeader(500)
		return
	}

	width := topImage.Bounds().Dx()
	height := topImage.Bounds().Dy()
	dy := int(float64(height) * (float64(percent) / 100))

	full := image.Rectangle{image.Point{0, 0}, image.Point{width, height}}
	topHalf := image.Rectangle{image.Point{0, 0}, image.Point{width, dy}}
	bottomHalf := image.Rectangle{image.Point{0, dy}, image.Point{width, height}}

	rgba := image.NewRGBA(full)
	draw.Draw(rgba, topHalf, topImage, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, bottomHalf, bottomImage, image.Point{0, dy}, draw.Src)

	buf = bytes.NewBuffer(nil)
	if err := png.Encode(buf, rgba); err != nil {
		fmt.Print("could not encode output image")
		w.WriteHeader(500)
		return
	}

	if len(cache) < maxCacheSize {
		cache[cachedEmojiFile] = buf
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
	w.Header().Set("Cache-Hit", "true")

	// by default writes 200 ok header
	if _, err := w.Write(buf.Bytes()); err != nil {
		fmt.Print("could not write image")
		w.WriteHeader(500)
		return
	}
}

func Serve(w http.ResponseWriter, r *http.Request) {
	fileLoc := fmt.Sprintf("./build%s", r.RequestURI)
	content, err := ioutil.ReadFile(fileLoc)
	if err != nil {
		fmt.Printf("file could not be read, err: %s", err.Error())
		w.WriteHeader(404)
		return
	}
	w.Write(content)
}

func redirectTLS(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://funemoji.club"+r.RequestURI, http.StatusMovedPermanently)
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/generate", Generate)
	router.HandleFunc("/", Serve)
	go func() {
		redirectRouter := mux.NewRouter()
		redirectRouter.HandleFunc("/", redirectTLS)
		err := http.ListenAndServe(":80", redirectRouter)
		fmt.Printf("%s | couldnt start server on port 80, err: %s\n", time.Now(), err.Error())
	}()
	err := http.ListenAndServeTLS(":443", "/etc/letsencrypt/live/funemoji.club/fullchain.pem", "/etc/letsencrypt/live/funemoji.club/privkey.pem", router)
	fmt.Printf("%s | couldnt start server on port 443, err: %s\n", time.Now(), err.Error())
}

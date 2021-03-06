package main

import (
	// "context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	// "os"
	"strconv"
	"strings"
	"time"

	// "github.com/gorilla/handlers"
	// "github.com/die-net/lrucache"
	// "github.com/die-net/lrucache/twotier"
	"github.com/gorilla/mux"
	// "github.com/gregjones/httpcache"
	// "github.com/gregjones/httpcache/diskcache"
	"gopkg.in/gographics/imagick.v3/imagick"
)

// var cache httpcache.Cache

// var mw


func main() {
	listenPort := flag.Int("port", 8080, "Listening port")
	listenHost := ""
	flag.Parse()

	router := mux.NewRouter().StrictSlash(true).UseEncodedPath()
	router.HandleFunc("/", indexAction)
	router.HandleFunc("/v1/{wildcard:.*}", handleV1MethodsAction)
	router.HandleFunc("/v2/images/{source}", imageAction)
	// loggedRouter := handlers.LoggingHandler(os.Stdout, router)

	// tempDir, _ := ioutil.TempDir("", "image-service")
	// cache = twotier.New(lrucache.New(2048, 604800), diskcache.New(tempDir))

	imagick.Initialize()
	defer imagick.Terminate()

	listen := fmt.Sprintf("%s:%d", listenHost, *listenPort)
	log.Printf("🚀 Listening on %v", listen)
	log.Fatal(http.ListenAndServe(listen, router))
}

func indexAction(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
}

func handleV1MethodsAction(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Location", strings.Replace(req.URL.RequestURI(), "/v1/", "/v2/", 1))
	res.WriteHeader(http.StatusPermanentRedirect)
}

func imageAction(res http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	source, _ := url.QueryUnescape(params["source"])

	httpClient := &http.Client{
		// Transport: httpcache.NewTransport(cache),
		Timeout:   time.Second * 30,
	}

	sourceResponse, err := httpClient.Get(source)
	if err != nil {
		res.WriteHeader(sourceResponse.StatusCode)
	}
	sourceObject, _ := ioutil.ReadAll(sourceResponse.Body)
	defer sourceResponse.Body.Close()
	log.Printf("👌 Downloaded %s with Content-Length %v", source, strconv.Itoa(len(sourceObject)))

	mw := imagick.NewMagickWand()
	err = mw.ReadImageBlob(sourceObject)
	if (err != nil) {
		log.Panic(err.Error())		
	}
	log.Printf("😎 Loaded %s into wand with canvas %vx%v", source, mw.GetImageWidth(), mw.GetImageHeight())

	formatImage(res, req, mw)
	resizeImage(res, req, mw)

	img := mw.GetImageBlob()
	res.WriteHeader(http.StatusOK)

	imgLength := strconv.Itoa(len(img))
	log.Printf("💯 Serving %s with Content-Length %v", source, imgLength)
	res.Header().Set("Content-Length", imgLength)

	res.Write(img)
	defer mw.Destroy()
}

func resizeImage(res http.ResponseWriter, req *http.Request, mw *imagick.MagickWand) {
	var height, width uint
	queryString := req.URL.Query()

	heights, heightOk := queryString["height"]
	widths, widthOk := queryString["width"]

	if !heightOk && !widthOk {
		return
	}

	// Assume no fill, yet.
	if heightOk && len(heights) > 0 {
		IHeight, _ := strconv.Atoi(heights[0])
		height = uint(IHeight)
	} else {
		height = mw.GetImageHeight()
	}
	if widthOk && len(widths) > 0 {
		IWidth, _ := strconv.Atoi(widths[0])
		width = uint(IWidth)
	} else {
		width = mw.GetImageWidth()
	}

	if height > 50000 || width > 50000 {
		return
	}

	fits, fitOk := queryString["fit"]
	fit := "clip"
	if fitOk {
		fit = fits[0]
	}

	switch fit {
	// "clamp":

	// "crop":

	// "fill":

	// "max":

	// "min":

	case "scale":
		resizeAndScaleImage(mw, width, height)

	case "clip", "contain":
		fallthrough
	default:
		resizeAndClipImage(mw, width, height)

	}
}

func resizeAndClipImage(mw *imagick.MagickWand, width uint, height uint) {
	var widthRatio, heightRatio, ratio float64

	widthRatio = float64(width) / float64(mw.GetImageWidth())
	heightRatio = float64(height) / float64(mw.GetImageHeight())

	ratio = heightRatio
	if widthRatio < heightRatio {
		ratio = widthRatio
	}

	width = uint(float64(mw.GetImageWidth()) * ratio)
	height = uint(float64(mw.GetImageHeight()) * ratio)

	resizeAndScaleImage(mw, width, height)
}

func resizeAndScaleImage(mw *imagick.MagickWand, width uint, height uint) {
	mw.ResizeImage(width, height, imagick.FILTER_LANCZOS)
}

func formatImage(res http.ResponseWriter, req *http.Request, mw *imagick.MagickWand) {
	queryString := req.URL.Query()
	imageFormats, ok := queryString["format"]

	if !ok || len(imageFormats) == 0 {
		return
	}

	imageFormat := imageFormats[0]

	var imageFormatMap = map[string]string{
		"jpg":  "image/jpg",
		"png":  "image/png",
		"webp": "image/webp",
		"svg":  "image/svg",
		"gif":  "image/gif",
	}

	mw.SetFormat(imageFormat)

	res.Header().Set("Content-Type", imageFormatMap[imageFormat])

	return
}

package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jasonwinn/geocoder"
	"github.com/muesli/smartcrop"
	"github.com/muesli/smartcrop/nfnt"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"gocv.io/x/gocv"
)

var (
	inputDir  = flag.String("in", "", "Source Path")
	outputDir = flag.String("out", "", "Target Path")
)
var RATIO float64 = 0.75

type ImgInfo struct {
	Title         string
	Content       string
	DatePosted    string
	OriginUrl     string
	ThumbsUrl     string
	Camera        string
	DateTaken     string
	TakenLocation string
}

func (f *ImgInfo) setOriginUrl(originUrl string) {
	f.OriginUrl = originUrl
}

func (f *ImgInfo) setThumbsUrl(thumbsURL string) {
	f.ThumbsUrl = thumbsURL
}

func extractExif(f *os.File) *exif.Exif {
	// Optionally register camera makenote data parsing - currently Nikon and
	// Canon are supported.
	exif.RegisterParsers(mknote.All...)
	x, err := exif.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	return x
}

func extractCam(x *exif.Exif) string {
	camModel, _ := x.Get(exif.Model) // normally, don't ignore errors!
	camModelValue, _ := camModel.StringVal()
	return camModelValue
}

func extractFocal(x *exif.Exif) (int64, int64) {
	focal, _ := x.Get(exif.FocalLength)
	numer, denom, _ := focal.Rat2(0) // retrieve first (only) rat. value
	return numer, denom
}

func centerlizedRect(cx, cy int, targetImage image.Rectangle, ratio float64) image.Rectangle {
	width := targetImage.Max.X
	x0 := 0
	x1 := width
	height := targetImage.Max.Y
	shouldWitdh := width
	shouldHeight := int(float64(width) * ratio)
	y0 := int(
		math.Min(
			math.Max(0, float64(cy-shouldHeight/2)),
			float64(height-shouldHeight),
		),
	)
	y1 := y0 + shouldHeight
	if height < shouldHeight {
		shouldHeight = height
		y0 = 0
		y1 = height
		shouldWitdh = int(float64(height) / ratio)
		x0 = int(
			math.Min(
				math.Max(0, float64(cx-shouldWitdh/2)),
				float64(width-shouldWitdh),
			),
		)
		x1 = x0 + shouldWitdh
	}
	return image.Rect(x0, y0, x1, y1)
}

func processImage(filePath string, outputDir string) (string, string) {

	src, err := imaging.Open(filePath)

	if err != nil {
		log.Fatal(err)
	}

	rawFileName := filePath[strings.LastIndex(filePath, "/")+1 : strings.LastIndex(filePath, ".")]

	/**
	 * preprocessing: resize file to <=1240 witdh
	 */
	outOriginPath := outputDir + "/" + rawFileName + ".jpg"
	resizedFile := imaging.Resize(src, 1240, 0, imaging.Lanczos)
	err1 := imaging.Save(resizedFile, outOriginPath)

	if err1 != nil {
		log.Fatal(err1)
	}

	/**
	 * phase1: small image wich fit thumb scale
	 *
	 * further resize origin file to <=400 witdth
	 */
	thumbPhase1Path := outputDir + "/" + rawFileName + "_tmp.jpg"
	thumbPhase1 := imaging.Resize(resizedFile, 400, 0, imaging.Lanczos)
	err2 := imaging.Save(thumbPhase1, thumbPhase1Path)

	if err2 != nil {
		log.Fatal(err2)
	}

	/**
	 * phase2: crop thumbPhase1 image to target ratio img
	 *
	 */
	log.Println(thumbPhase1.Bounds())
	f, _ := os.Open(thumbPhase1Path)
	img, _, _ := image.Decode(f)

	/**
	 * phase2:
	 * check opencv face detect first
	 */

	log.Printf("trying to detect faces for cropping\n")
	cvImg := gocv.IMRead(thumbPhase1Path, gocv.IMReadColor)
	classifierPath := "data/haarcascade_frontalface_default.xml"
	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()
	if !classifier.Load(classifierPath) {
		log.Fatalf("Error reading cascade file: %v\n", classifierPath)
	}
	// cv Detect faces.
	faces := classifier.DetectMultiScaleWithParams(cvImg, 1.1, 16, 0, image.Pt(30, 30), image.Pt(thumbPhase1.Bounds().Max.X, thumbPhase1.Bounds().Max.Y))

	var bestCrop image.Rectangle
	if len(faces) > 0 {
		log.Printf("face detected\n")
		//calculate the center of faces
		cx := 0
		cy := 0
		n := 0
		for _, face := range faces {
			cx += face.Min.X + face.Max.X
			cy += face.Min.Y + face.Max.Y
			n += 2
		}
		cx = cx / n
		cy = cy / n

		bestCrop = centerlizedRect(cx, cy, thumbPhase1.Bounds(), RATIO)

	} else {
		log.Printf("no faces detected, fall back to smart crop\n")
		/**
		* using smart crop
		 */
		targetRect := centerlizedRect(0, 0, thumbPhase1.Bounds(), RATIO)
		log.Println(targetRect.Bounds())

		analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
		tmpCrop, _ := analyzer.FindBestCrop(img, targetRect.Size().X, targetRect.Size().Y)
		bestCrop = tmpCrop

	}

	log.Printf("generate crop rect: %v\n", bestCrop)

	/**
	* outputing the image
	 */
	type SubImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	croppedimg := img.(SubImager).SubImage(bestCrop)

	thumbPhase2Path := outputDir + "/" + rawFileName + "_thumb.jpg"
	thumbPhase2File, err4 := os.OpenFile(thumbPhase2Path, os.O_CREATE|os.O_WRONLY, 0755)
	if err4 != nil {
		log.Fatalf("Unable to open output file: %v", err)
	}
	jpeg.Encode(thumbPhase2File, croppedimg, nil)
	os.Remove(thumbPhase1Path)

	return rawFileName + ".jpg", rawFileName + "_thumb.jpg"
}

func geoCode(lat float64, long float64) string {
	if lat == 0 && long == 0 {
		return ""
	}
	log.Println(lat, long)
	loc, err := geocoder.ReverseGeocode(lat, long)
	if err != nil {
		panic(err.Error())
	}
	address := []string{loc.Street, loc.City, loc.State, loc.County}
	return strings.Join(address, ", ")
}

func extractImgInfo(f *os.File) ImgInfo {

	x := extractExif(f)
	camModel := extractCam(x)
	// numer, denom := extractFocal(x) // retrieve first (only) rat. value
	tt, _ := x.DateTime()
	lat, long, _ := x.LatLong()

	title := ""
	content := ""
	location := geoCode(lat, long)

	return ImgInfo{
		title,
		content,
		time.Now().Format(time.RFC3339),
		"",
		"",
		camModel,
		tt.String(),
		location,
	}
}

func getTemplate() *template.Template {
	t := template.New("Person template")
	tmpl := `---
title: {{ .Title }}
datePosted: {{ .DatePosted }}
image: 
    origin: "{{ .OriginUrl }}"
    thumb: "{{ .ThumbsUrl }}"
exif:
  camera: "{{ .Camera }}"
  dateTaken: {{ .DateTaken }}
  location:
    name: "{{ .TakenLocation }}"
---

{{ .Content }}
	`
	t, err := t.Parse(tmpl)
	if err != nil {
		log.Fatal("Parse: ", err)
	}
	return t
}

func mergeTemplate(info ImgInfo, tmpl *template.Template) string {
	buf := new(bytes.Buffer)
	tmpl.Execute(buf, info)
	return buf.String()
}

func main() {
	flag.Parse()
	if len(*inputDir) == 0 || len(*outputDir) == 0 {
		log.Fatal("Usage: ImgPost -in {inDir} -out {outDir}")
	}

	geocoder.SetAPIKey("APNfRnF0ZgBM8cq4t3GEwUW9dFfRrNxr")

	files, err := ioutil.ReadDir(*inputDir)
	if err != nil {
		log.Fatal(err)
	}

	template := getTemplate()

	for _, fileInfo := range files {
		if strings.HasSuffix(fileInfo.Name(), "jpg") ||
			strings.HasSuffix(fileInfo.Name(), "png") {
			rawFileName := fileInfo.Name()[:strings.LastIndex(fileInfo.Name(), ".")]
			fmt.Println(fileInfo.Name())
			//file
			filePath := *inputDir + "/" + fileInfo.Name()
			f, err := os.Open(filePath)
			if err != nil {
				log.Fatal(err)
			}
			info := extractImgInfo(f)
			imageURL, thumbsURL := processImage(filePath, *outputDir)
			info.setOriginUrl(imageURL)
			info.setThumbsUrl(thumbsURL)
			markdown := mergeTemplate(info, template)

			ioutil.WriteFile(*outputDir+"/"+rawFileName+".md", []byte(markdown), 0644)
		}
	}

}

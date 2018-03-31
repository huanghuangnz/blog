package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
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
)

var (
	inputDir  = flag.String("in", "", "Source Path")
	outputDir = flag.String("out", "", "Target Path")
)

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

func (f *ImgInfo) setThumbsUrl(thumbsUrl string) {
	f.ThumbsUrl = thumbsUrl
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

func cropToRatio(rect image.Rectangle, ratio float64) image.Rectangle {
	width := rect.Max.X
	x0 := 0
	x1 := width
	height := rect.Max.Y
	shouldWitdh := width
	shouldHeight := int(float64(width) * ratio)
	y0 := (height - shouldHeight) / 2
	y1 := y0 + shouldHeight
	if height < shouldHeight {
		shouldHeight = height
		y0 = 0
		y1 = height
		shouldWitdh = int(float64(height) / ratio)
		x0 = (width - shouldWitdh) / 2
		x1 = x0 + shouldWitdh
	}
	return image.Rect(x0, y0, x1, y1)
}

func processImage(filePath string, outputDir string) (string, string) {
	// img, err := jpeg.Decode(f)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	src, err := imaging.Open(filePath)

	if err != nil {
		log.Fatal(err)
	}

	rawFileName := filePath[strings.LastIndex(filePath, "/")+1 : strings.LastIndex(filePath, ".")]

	//resize file to <=1240 witdh
	outOriginPath := outputDir + "/" + rawFileName + ".jpg"
	resizedFile := imaging.Resize(src, 1240, 0, imaging.Lanczos)
	err1 := imaging.Save(resizedFile, outOriginPath)

	if err1 != nil {
		log.Fatal(err1)
	}

	//further resize origin file to <=400 witdth
	thumb_phase1_path := outputDir + "/" + rawFileName + "_tmp.jpg"
	thumb_phase1 := imaging.Resize(resizedFile, 400, 0, imaging.Lanczos)
	err2 := imaging.Save(thumb_phase1, thumb_phase1_path)

	if err2 != nil {
		log.Fatal(err2)
	}

	//crop thumb_phase1 image to target ratio img using seam
	log.Println(thumb_phase1.Bounds())
	rect := cropToRatio(thumb_phase1.Bounds(), 0.75)
	log.Println(rect.Bounds())

	f, _ := os.Open(thumb_phase1_path)
	img, _, _ := image.Decode(f)

	analyzer := smartcrop.NewAnalyzer(nfnt.NewDefaultResizer())
	topCrop, _ := analyzer.FindBestCrop(img, rect.Size().X, rect.Size().Y)

	// The crop will have the requested aspect ratio, but you need to copy/scale it yourself
	fmt.Printf("Top crop: %+v\n", topCrop)

	type SubImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	croppedimg := img.(SubImager).SubImage(topCrop)

	thumb_phase2_path := outputDir + "/" + rawFileName + "_thumb.jpg"
	thumb_phase2_file, err4 := os.OpenFile(thumb_phase2_path, os.O_CREATE|os.O_WRONLY, 0755)
	if err4 != nil {
		log.Fatalf("Unable to open output file: %v", err)
	}

	jpeg.Encode(thumb_phase2_file, croppedimg, nil)

	os.Remove(thumb_phase1_path)

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
			imageUrl, thumbsUrl := processImage(filePath, *outputDir)
			info.setOriginUrl(imageUrl)
			info.setThumbsUrl(thumbsUrl)
			markdown := mergeTemplate(info, template)

			ioutil.WriteFile(*outputDir+"/"+rawFileName+".md", []byte(markdown), 0644)
		}
	}

}

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jasonwinn/geocoder"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
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

	img1 := imaging.Resize(src, 1240, 0, imaging.Lanczos)
	err1 := imaging.Save(img1, outputDir+"/"+rawFileName+".jpg")

	if err1 != nil {
		log.Fatal(err1)
	}

	thumb := imaging.Resize(src, 400, 0, imaging.Lanczos)
	err2 := imaging.Save(thumb, outputDir+"/"+rawFileName+"_thumb.jpg")

	if err2 != nil {
		log.Fatal(err2)
	}

	return rawFileName + ".jpg", rawFileName + "_thumb.jpg"
}

func geoCode(lat float64, long float64) string {
	log.Println(lat, long)
	loc, err := geocoder.ReverseGeocode(lat, long)
	if err != nil {
		panic("THERE WAS SOME ERROR!!!!!")
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
	tmpl := `
---
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
	geocoder.SetAPIKey("APNfRnF0ZgBM8cq4t3GEwUW9dFfRrNxr")

	inputDir := os.Args[1]
	outputDir := os.Args[2]
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		log.Fatal(err)
	}

	template := getTemplate()

	for _, fileInfo := range files {
		if strings.HasSuffix(fileInfo.Name(), "jpg") {
			rawFileName := fileInfo.Name()[:strings.LastIndex(fileInfo.Name(), ".")]
			fmt.Println(fileInfo.Name())
			//file
			filePath := inputDir + "/" + fileInfo.Name()
			f, err := os.Open(filePath)
			if err != nil {
				log.Fatal(err)
			}
			info := extractImgInfo(f)
			imageUrl, thumbsUrl := processImage(filePath, outputDir)
			info.setOriginUrl(imageUrl)
			info.setThumbsUrl(thumbsUrl)
			markdown := mergeTemplate(info, template)

			ioutil.WriteFile(outputDir+"/"+rawFileName+".md", []byte(markdown), 0644)
		}
	}

}

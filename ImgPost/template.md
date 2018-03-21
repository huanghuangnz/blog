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
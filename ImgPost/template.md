---
title: {{ .Content }}
datePosted: {{ .DatePosted }}
image: 
    origin: "{{ .OriginUrl }}"
    thumb: "{{ .ThumbsUrl }}"
exif:
  camera: "{{ .OriginUrl }}"
  dateTaken: {{ .DateTaken }}
  location:
    name: "{{ .TakenLocation }}"
---

{{ .Content }}
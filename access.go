package main

import "time"

type Access struct {
	Date     time.Time
	IP       string
	ShortURL string
	UA       string
}

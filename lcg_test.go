package main

import (
	"fmt"
	"testing"

	"github.com/iochen/shorturl/utils/base64"
)

func TestLength(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		fmt.Println(string(base64.Encode(uint64(i), length(uint64(i)))))
	}
}

func TestLCG_FromLast(t *testing.T) {
	var last uint64 = 54
	var length = 1
	for i := 0; i < 10000; i++ {
		lcg := &LCG{
			A: 5,
			B: 7,
		}
		last, length = lcg.FromLast(uint64(i), last)
		fmt.Println(i, string(base64.Encode(last, length)))
	}
}

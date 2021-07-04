package base64

import (
	"fmt"
	"testing"
)

func TestEncode(t *testing.T) {
	for i := 0; i < 1000; i++ {
		fmt.Println(string(Encode(uint64(i), 2)))
	}
}

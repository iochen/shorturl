package base64

const CHAR = "0123456789-abcdefghijklmnopqrstuvwxyz_ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func Encode(raw uint64, length int) []byte {
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		b[length-i-1] = CHAR[(raw>>(uint(i)*6))&63]
	}
	return b
}

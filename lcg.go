package main

type LCG struct {
	A uint64 `yaml:"A"`
	B uint64 `yaml:"B"`
}

func (lcg *LCG) FromLast(num, last uint64) (uint64, int) {
	l := length(num)
	return lcg.calculate(last, 1<<(6*uint(l))), l
}

func (lcg *LCG) calculate(n, m uint64) uint64 {
	return (lcg.A*n + lcg.B) % m
}

func length(num uint64) int {
	var compare uint64
	for i := 0; i < 10; i++ {
		if num < compare {
			return i
		} else {
			compare += 1 << (6 * uint64(i+1))
		}
	}
	return 10
}

package sparklines

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"math"
)

var (
	lightSV = [2]float64{0.40, 0.85}
	darkSV  = [2]float64{0.95, 0.65}
	// satVal is the default saturation-value to use.
	satVal = lightSV
)

// HexColor returns the color in #XXXXXXXX format.
func HexColor(c color.Color) string {
	r, g, b, a := c.RGBA()

	return fmt.Sprintf(
		"#%02X%02X%02X%02X",
		uint8(float64(r)/0xFFFF*0xFF),
		uint8(float64(g)/0xFFFF*0xFF),
		uint8(float64(b)/0xFFFF*0xFF),
		uint8(float64(a)/0xFFFF*0xFF),
	)
}

// HashColor hashes the given string into a color.RGBA.
func HashColor(v ...interface{}) color.RGBA64 {
	hash := fnv.New32()
	hash.Write([]byte(fmt.Sprint(v...)))

	h := float64(hash.Sum32()) / math.MaxUint32

	return hsvrgb(
		hashClamp(h, 0, 360, 64),
		satVal[0],
		satVal[1],
	)
}

// hashClamp converts the given u32 hash to a number within [min, max],
// optionally rounded if round is not 0. Hash must be within [0, 1].
func hashClamp(hash, min, max, round float64) float64 {
	if round > 0 {
		hash = math.Round(hash*round) / round
	}

	r := max - min
	n := min + (hash * r)

	return n
}

// hsvrgb is taken from lucasb-eyer/go-colorful, licensed under the MIT license.
func hsvrgb(h, s, v float64) color.RGBA64 {
	Hp := h / 60.0
	C := v * s
	X := C * (1.0 - math.Abs(math.Mod(Hp, 2.0)-1.0))

	m := v - C
	r, g, b := 0.0, 0.0, 0.0

	switch {
	case 0.0 <= Hp && Hp < 1.0:
		r = C
		g = X
	case 1.0 <= Hp && Hp < 2.0:
		r = X
		g = C
	case 2.0 <= Hp && Hp < 3.0:
		g = C
		b = X
	case 3.0 <= Hp && Hp < 4.0:
		g = X
		b = C
	case 4.0 <= Hp && Hp < 5.0:
		r = X
		b = C
	case 5.0 <= Hp && Hp < 6.0:
		r = C
		b = X
	}

	return color.RGBA64{
		R: uint16((m + r) * 0xFFFF),
		G: uint16((m + g) * 0xFFFF),
		B: uint16((m + b) * 0xFFFF),
		A: 0xFFFF,
	}
}

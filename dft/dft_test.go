package dft

import (
	"fmt"
	"math"
	"math/cmplx"
	"strconv"
	"testing"
)

const (
	tolerance = 2.5e-15
)

// stepFloat creates input for testing step response.
func stepFloat(n int) (out []float64) {
	out = make([]float64, n)
	for idx := 0; idx < n>>1; idx++ {
		out[idx] = 1
	}
	return
}

// stepCmplx creates input for testing step response.
func stepCmplx(n int) (out []complex128) {
	out = make([]complex128, n)
	for idx := 0; idx < n>>1; idx++ {
		out[idx] = 1
	}
	return
}

func dftError(i, j []complex128) float64 {
	var err float64
	for idx := range i {
		err += cmplx.Abs(i[idx] - j[idx])
	}
	return err / float64(len(i))
}

func naiveDFT(f []complex128, sign float64) {
	n := len(f)
	h := make([]complex128, n)
	phi := sign * 2.0 * math.Pi / float64(n)
	for w := 0; w < n; w++ {
		var t complex128
		for k := 0; k < n; k++ {
			t += f[k] * cmplx.Rect(1, phi*float64(k)*float64(w))
		}
		h[w] = t
	}
	copy(f, h)
}

type floatDft struct {
	Size int
	Fn   func(ri, ii, ro, io []float64)
}

var floatDfts = []floatDft{
	{2, DftFloat2},
	{3, DftFloat3},
	{4, DftFloat4},
	{5, DftFloat5},
	{6, DftFloat6},
	{7, DftFloat7},
	{8, DftFloat8},
	{9, DftFloat9},
	{10, DftFloat10},
	{11, DftFloat11},
	{12, DftFloat12},
	{13, DftFloat13},
	{14, DftFloat14},
	{15, DftFloat15},
	{16, DftFloat16},
}

func TestFloatDFT(t *testing.T) {
	for _, dft := range floatDfts {
		t.Run(strconv.FormatInt(int64(dft.Size), 10), func(t *testing.T) {
			re := stepFloat(dft.Size)
			im := make([]float64, dft.Size)
			dft.Fn(re, im, re, im)

			genOut := make([]complex128, dft.Size)
			for idx := range genOut {
				genOut[idx] = complex(re[idx], im[idx])
			}

			naiveOut := stepCmplx(dft.Size)
			naiveDFT(naiveOut, -1.0)

			err := dftError(genOut, naiveOut)
			t.Logf("DFT%d Error: %0.12g", dft.Size, err)
			if err > tolerance {
				t.Fail()
			}
		})
	}
}

type cmplxDft struct {
	Size int
	Fn   func(xi, xo []complex128)
}

var cmplxDfts = []cmplxDft{
	{2, DftCmplx2},
	{3, DftCmplx3},
	{4, DftCmplx4},
	{5, DftCmplx5},
	{6, DftCmplx6},
	{7, DftCmplx7},
	{8, DftCmplx8},
	{9, DftCmplx9},
	{10, DftCmplx10},
	{11, DftCmplx11},
	{12, DftCmplx12},
	{13, DftCmplx13},
	{14, DftCmplx14},
	{15, DftCmplx15},
	{16, DftCmplx16},
}

func TestCmplxDFT(t *testing.T) {
	for _, dft := range cmplxDfts {
		t.Run(strconv.FormatInt(int64(dft.Size), 10), func(t *testing.T) {
			xi := stepCmplx(dft.Size)
			dft.Fn(xi, xi)

			naiveOut := stepCmplx(dft.Size)
			naiveDFT(naiveOut, -1.0)

			err := dftError(xi, naiveOut)
			t.Logf("DFT%d Error: %0.3g", dft.Size, err)
			if err > tolerance {
				t.Fail()
			}
		})
	}
}

func BenchmarkFloatDFT(b *testing.B) {
	for _, dft := range floatDfts {
		b.Run(fmt.Sprintf("Naive DFT N=%d", dft.Size), func(b *testing.B) {
			ri := make([]complex128, dft.Size)

			b.SetBytes(int64(dft.Size))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				naiveDFT(ri, -1.0)
			}
		})

		b.Run(fmt.Sprintf("In Place Float DFT N=%2d", dft.Size), func(b *testing.B) {
			ri := make([]float64, dft.Size)
			ii := make([]float64, dft.Size)

			b.SetBytes(int64(dft.Size))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				dft.Fn(ri, ii, ri, ii)
			}
		})

		b.Run(fmt.Sprintf("Out of Place Float DFT N=%d", dft.Size), func(b *testing.B) {
			ri := make([]float64, dft.Size)
			ii := make([]float64, dft.Size)
			ro := make([]float64, dft.Size)
			io := make([]float64, dft.Size)

			b.SetBytes(int64(dft.Size))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				dft.Fn(ri, ii, ro, io)
			}
		})
	}
}

func BenchmarkCmplxDFT(b *testing.B) {
	for _, dft := range cmplxDfts {
		b.Run(fmt.Sprintf("Naive Cmplx DFT N=%d", dft.Size), func(b *testing.B) {
			ri := make([]complex128, dft.Size)

			b.SetBytes(int64(dft.Size))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				naiveDFT(ri, -1.0)
			}
		})

		b.Run(fmt.Sprintf("In Place Cmplx DFT N=%2d", dft.Size), func(b *testing.B) {
			xi := make([]complex128, dft.Size)

			b.SetBytes(int64(dft.Size))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				dft.Fn(xi, xi)
			}
		})

		b.Run(fmt.Sprintf("Out of Cmplx Place DFT N=%d", dft.Size), func(b *testing.B) {
			xi := make([]complex128, dft.Size)
			xo := make([]complex128, dft.Size)

			b.SetBytes(int64(dft.Size))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				dft.Fn(xi, xo)
			}
		})
	}
}

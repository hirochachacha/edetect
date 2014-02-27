package edetect

import "testing"

func Test(t *testing.T) {
	input := "aa"
	detector, err := Open()
	if err != nil {
		panic(err)
	}
	defer detector.Close()

	charset, err := detector.Run(input)
	if err != nil {
		panic(err)
	}

	println(charset.Name)
	println(charset.Confidence)
	println(charset.Language)

	println("detectAll")

	charsets, err := detector.RunAll(input)
	if err != nil {
		panic(err)
	}
	for _, charset := range charsets {
		println(charset.Name)
		println(charset.Confidence)
		println(charset.Language)
	}

	encodings, err := detector.SupportedEncodings()
	if err != nil {
		panic(err)
	}

	for _, encoding := range encodings {
		println(encoding)
	}
}

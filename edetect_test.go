package edetect

import "testing"

// import "io/ioutil"

func Test(t *testing.T) {
	// input := []byte("aa")
	input := []byte("ああ")
	// input, _ := ioutil.ReadFile("/usr/bin/ls")

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
	println(charset.Mime)

	println("detectAll")

	charsets, err := detector.RunAll(input)
	if err != nil {
		panic(err)
	}
	for _, charset := range charsets {
		println(charset.Name)
		println(charset.Confidence)
		println(charset.Language)
		println(charset.Mime)
	}

	encodings, err := detector.SupportedEncodings()
	if err != nil {
		panic(err)
	}

	for _, encoding := range encodings {
		println(encoding)
	}
}

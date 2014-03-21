package convert

import "testing"
import "os"

func TestConvert(t *testing.T) {
	utf8 := []byte("こんにちは世界")
	sjis, err := Convert(utf8, "utf8", "sjis")
	if err != nil {
		panic(err)
	}
	utf8, err = Convert(sjis, "sjis", "utf8")
	if err != nil {
		panic(err)
	}
	if string(utf8) != "こんにちは世界" {
		t.Error("unexpected")
	}
}

func TestReadCloser(t *testing.T) {
	f, err := os.Open("sjis.txt")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	reader, err := NewReadCloser(f, "sjis", "utf8")
	if err != nil {
		panic(err)
	}
	defer reader.Close()

	utf8 := "こんにちは世界"
	buf := make([]byte, len(utf8))

	reader.Read(buf)
	if string(buf) != utf8 {
		t.Error("unexpected")
	}
}

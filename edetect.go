package edetect

// #cgo LDFLAGS: -licuuc -licudata -licui18n -lmagic
// #include <stdlib.h>
// #include <magic.h>
// #include <unicode/ucsdet.h>
//
// const UCharsetMatch* ucsd_run(UCharsetDetector* ucsd, const char* input, size_t input_len, UErrorCode* u_err) {
//   const UCharsetMatch* ucsm;
//
//   ucsdet_setText(ucsd, input, input_len, u_err);
//   if U_FAILURE(*u_err) {
//     return NULL;
//   }
//
//   ucsm = ucsdet_detect(ucsd, u_err);
//   if U_FAILURE(*u_err) {
//     return NULL;
//   }
//
//   return ucsm;
// }
//
// const UCharsetMatch** ucsd_runAll(UCharsetDetector* ucsd, int32_t* matchesFound, const char* input, size_t input_len, UErrorCode* u_err) {
//   const UCharsetMatch** ucsms;
//
//   ucsdet_setText(ucsd, input, input_len, u_err);
//   if U_FAILURE(*u_err) {
//     return NULL;
//   }
//
//   ucsms = ucsdet_detectAll(ucsd, matchesFound, u_err);
//   if U_FAILURE(*u_err) {
//     return NULL;
//   }
//
//   return ucsms;
// }
//
// int u_failure(UErrorCode u_err) {
//   return U_FAILURE(u_err);
// }
//
import "C"
import "unsafe"
import "errors"
import "runtime"
import "reflect"
import "strings"

type Charset struct {
	Name       string
	Confidence int
	Language   string
	Mime       string
}

type Detector struct {
	ucsd  *C.UCharsetDetector
	magic C.magic_t
}

func ucsdOpen() (*C.UCharsetDetector, error) {
	uErr := C.UErrorCode(C.U_ZERO_ERROR)
	ucsd, err := C.ucsdet_open(&uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}
	return ucsd, nil
}

func magicError(magic C.magic_t) error {
	errorStr, err := C.magic_error(magic)
	if err != nil {
		// hope it unreachable!
		panic("unreachable")
	}
	if errorStr == nil {
		return nil
	}
	return errors.New(C.GoString(errorStr))
}

func magicOpen() (C.magic_t, error) {
	magic, err := C.magic_open(C.MAGIC_MIME_TYPE)
	if err != nil {
		return nil, err
	}
	if magic == nil {
		err = magicError(magic)
		if err == nil {
			panic("unreachable")
		}
		return nil, err
	}

	// magic_load set ENOENT, even if it succeed,
	// so ignore second value.
	code, _ := C.magic_load(magic, nil)
	if int(code) != 0 {
		err = magicError(magic)
		if err == nil {
			panic("unreachable")
		}
		return nil, err
	}

	return magic, nil
}

func Open() (*Detector, error) {
	ucsd, err := ucsdOpen()
	if err != nil {
		return nil, err
	}

	magic, err := magicOpen()
	if err != nil {
		return nil, err
	}

	detector := &Detector{
		ucsd:  ucsd,
		magic: magic,
	}

	runtime.SetFinalizer(detector, func(detector *Detector) {
		detector.Close()
	})

	return detector, nil
}

func (detector *Detector) Close() error {
	_, err1 := C.ucsdet_close(detector.ucsd)
	_, err2 := C.magic_close(detector.magic)

	if err1 != nil {
		return err1
	}
	return err2
}

func (detector *Detector) EnableInputFilter(filter bool) (bool, error) {
	var err error
	var result C.UBool
	if filter {
		result, err = C.ucsdet_enableInputFilter(detector.ucsd, C.UBool(1))
	} else {
		result, err = C.ucsdet_enableInputFilter(detector.ucsd, C.UBool(0))
	}
	if err != nil {
		return false, err
	}
	return (int(result) != 0), nil
}

func (detector *Detector) IsInputFilterEnabled(filter bool) (bool, error) {
	result, err := C.ucsdet_isInputFilterEnabled(detector.ucsd)
	if err != nil {
		return false, err
	}
	return (int(result) != 0), nil
}

func (detector *Detector) SetDeclaredEncoding(encoding string) error {
	cencoding := C.CString(encoding)
	defer C.free(unsafe.Pointer(cencoding))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	_, err := C.ucsdet_setDeclaredEncoding(detector.ucsd, cencoding, C.int32_t(len(encoding)), &uErr)
	if err != nil {
		return err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return err
	}

	return nil
}

func (detector *Detector) SupportedEncodings() ([]string, error) {
	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	uenum, err := C.ucsdet_getAllDetectableCharsets(detector.ucsd, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	defer C.uenum_close(uenum)

	ccount, err := C.uenum_count(uenum, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	encodings := make([]string, 0)

	var length C.int32_t
	for i := int(ccount); i > 0; i-- {
		cencoding, err := C.uenum_next(uenum, &length, &uErr)
		if err != nil {
			return nil, err
		}
		if err = uErrorToGoError(uErr); err != nil {
			return nil, err
		}
		encodings = append(encodings, C.GoString(cencoding))
	}

	return encodings, nil
}

func (detector *Detector) detectMime(cinput *C.char, cinputLen C.size_t) (string, error) {
	// magic_buffer set EINVAL, even if it succeed(binary?),
	// so ignore second value.
	mimeStr, _ := C.magic_buffer(detector.magic, unsafe.Pointer(cinput), cinputLen)
	if mimeStr == nil {
		err := magicError(detector.magic)
		if err == nil {
			panic("unreachable")
		}
		return "", err
	}
	return C.GoString(mimeStr), nil
}

func (detector *Detector) Run(input []byte) (*Charset, error) {
	cinput := (*C.char)(unsafe.Pointer(&input[0]))

	cinputLen := C.size_t(len(input))

	mime, err := detector.detectMime(cinput, cinputLen)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(mime, "text") {
		return makeCharset(nil, mime)
	}

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	uCharsetMatch, err := C.ucsd_run(detector.ucsd, (*C.char)(cinput), cinputLen, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	return makeCharset(uCharsetMatch, mime)
}

func (detector *Detector) RunAll(input []byte) ([]*Charset, error) {
	cinput := (*C.char)(unsafe.Pointer(&input[0]))

	cinputLen := C.size_t(len(input))

	mime, err := detector.detectMime(cinput, cinputLen)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(mime, "text") {
		charset, err := makeCharset(nil, mime)
		if err != nil {
			return nil, err
		}
		return []*Charset{charset}, nil
	}

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	var matchesFound C.int32_t

	uCharsetMatches, err := C.ucsd_runAll(detector.ucsd, &matchesFound, (*C.char)(cinput), cinputLen, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	length := int(matchesFound)

	var umatches []*C.UCharsetMatch
	var matches []*Charset

	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&umatches)))
	sliceHeader.Cap = length
	sliceHeader.Len = length
	sliceHeader.Data = uintptr(unsafe.Pointer(uCharsetMatches))

	for _, uCharsetMatch := range umatches {
		charset, err := makeCharset(uCharsetMatch, mime)
		if err != nil {
			return nil, err
		}

		matches = append(matches, charset)
	}
	return matches, nil
}

func uErrorToGoError(uErr C.UErrorCode) error {
	i, err := C.u_failure(uErr)
	if err != nil {
		// hope it unreachable!
		panic(err)
	}
	if int(i) == 0 {
		return nil
	}

	cerrStr := C.u_errorName(uErr)
	errStr := C.GoString(cerrStr)
	return errors.New(errStr)
}

func makeCharset(uCharsetMatch *C.UCharsetMatch, mime string) (*Charset, error) {
	if uCharsetMatch == nil {
		charset := &Charset{
			Confidence: 100,
			Mime:       mime,
		}
		return charset, nil
	}
	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	cname, err := C.ucsdet_getName(uCharsetMatch, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	cconfidence, err := C.ucsdet_getConfidence(uCharsetMatch, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	clang, err := C.ucsdet_getLanguage(uCharsetMatch, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	charset := &Charset{
		Name:       C.GoString(cname),
		Confidence: int(cconfidence),
		Language:   C.GoString(clang),
		Mime:       mime,
	}
	return charset, nil
}

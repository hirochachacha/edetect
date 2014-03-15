package edetect

// #cgo LDFLAGS: -licuuc -licudata -licui18n
// #include <stdlib.h>
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

type Charset struct {
	Name       string
	Confidence int
	Language   string
}

type Detector struct {
	ucsd *C.UCharsetDetector
}

func Open() (*Detector, error) {
	uErr := C.UErrorCode(C.U_ZERO_ERROR)
	ucsd, err := C.ucsdet_open(&uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	detector := &Detector{ucsd}

	runtime.SetFinalizer(detector, func(detector *Detector) {
		detector.Close()
	})

	return detector, nil
}

func (detector *Detector) Close() error {
	_, err := C.ucsdet_close(detector.ucsd)
	return err
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

func (detector *Detector) Run(input string) (*Charset, error) {
	cinput := C.CString(input)
	defer C.free(unsafe.Pointer(cinput))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	uCharsetMatch, err := C.ucsd_run(detector.ucsd, cinput, C.size_t(len(input)), &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	charset, err := uCharsetMatchToGoCharset(uCharsetMatch)
	if err != nil {
		return nil, err
	}

	return charset, nil
}

func (detector *Detector) RunAll(input string) ([]*Charset, error) {
	cinput := C.CString(input)
	defer C.free(unsafe.Pointer(cinput))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	var matchesFound C.int32_t

	uCharsetMatches, err := C.ucsd_runAll(detector.ucsd, &matchesFound, cinput, C.size_t(len(input)), &uErr)
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
		charset, err := uCharsetMatchToGoCharset(uCharsetMatch)
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

func uCharsetMatchToGoCharset(uCharsetMatch *C.UCharsetMatch) (*Charset, error) {
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
	}
	return charset, nil
}

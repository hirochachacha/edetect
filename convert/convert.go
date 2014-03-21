package convert

// #cgo LDFLAGS: -licuuc -licudata -licui18n
// #include <stdlib.h>
// #include <unicode/ucnv.h>
//
// int u_failure(UErrorCode u_err) {
//   return U_FAILURE(u_err);
// }
//
import "C"
import "unsafe"
import "io"
import "runtime"

const (
	defaultBufSize = 4096
)

type UError struct {
	s string
}

func (err *UError) Error() string {
	return err.s
}

func Convert(input []byte, from string, to string) ([]byte, error) {
	cfrom := C.CString(from)
	defer C.free(unsafe.Pointer(cfrom))

	cto := C.CString(to)
	defer C.free(unsafe.Pointer(cto))

	src := (*C.char)(unsafe.Pointer(&input[0]))
	srcLen := C.int32_t(len(input))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	// get dstLen
	// ignore ENOENT
	dstLen, _ := C.ucnv_convert(cto, cfrom, nil, 0, src, srcLen, &uErr)
	if uErr != C.U_BUFFER_OVERFLOW_ERROR {
		return nil, uErrorToGoError(uErr)
	}
	uErr = C.UErrorCode(C.U_ZERO_ERROR)

	output := make([]byte, int(dstLen))
	dst := (*C.char)(unsafe.Pointer(&output[0]))

	dstLen, err := C.ucnv_convert(cto, cfrom, dst, dstLen, src, srcLen, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}

	return output, nil
}

type ReadCloser struct {
	r    io.Reader
	from *C.UConverter
	to   *C.UConverter

	// input buffer
	ibuf []byte
	ilen int

	// output buffer
	obuf   []byte
	ostart int
	oend   int
}

func NewReadCloser(r io.Reader, from string, to string) (*ReadCloser, error) {
	ufrom, err := ucnvOpen(from)
	if err != nil {
		return nil, err
	}

	uto, err := ucnvOpen(to)
	if err != nil {
		return nil, err
	}

	reader := &ReadCloser{
		r:    r,
		from: ufrom,
		to:   uto,
		ibuf: make([]byte, defaultBufSize),
		obuf: make([]byte, defaultBufSize),
	}

	runtime.SetFinalizer(reader, func(reader *ReadCloser) {
		reader.Close()
	})

	return reader, nil
}

func (r *ReadCloser) Close() error {
	_, err1 := C.ucnv_close(r.from)
	_, err2 := C.ucnv_close(r.to)
	if err1 != nil {
		return err1
	}
	return err2
}

func (r *ReadCloser) Read(p []byte) (int, error) {
	plen := len(p)

	if plen == 0 {
		return 0, nil
	}

	n := 0
	olen := r.oend - r.ostart

	// flush write buffer if exist
	if olen > 0 {
		if olen > plen {
			copy(p, r.obuf[r.ostart:plen])
			r.ostart += plen
			return plen, nil
		}
		copy(p, r.obuf[r.ostart:r.oend])
		r.ostart = 0
		r.oend = 0
		if olen == plen {
			return plen, nil
		}
		n = olen
	}

	// reallocate read buffer or set Len
	if plen != len(r.ibuf) {
		if plen > cap(r.ibuf) {
			r.ibuf = make([]byte, plen, plen*2)
		} else {
			r.ibuf = r.ibuf[:plen]
		}
	}

	// fill read buffer
	ilen, err := r.r.Read(r.ibuf)
	r.ilen = ilen
	if err != nil {
		return n, err
	}

	if ilen == 0 {
		return n, io.EOF
	}

	src := (*C.char)(unsafe.Pointer(&r.ibuf[0]))
	srcLimit := (*C.char)(unsafe.Pointer(&r.ibuf[ilen]))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	oMaxLen, err := ucnvMaxLen(ilen, r.from, r.to)
	if err != nil {
		return n, err
	}

	// reallocate write buffer
	if oMaxLen > cap(r.obuf) {
		r.obuf = make([]byte, oMaxLen*2)
	}
	dst := (*C.char)(unsafe.Pointer(&r.obuf[0]))
	dstLimit := (*C.char)(unsafe.Pointer(&r.obuf[oMaxLen]))

	dstStart := uintptr(unsafe.Pointer(dst))

	// fill write buffer by C.ucnv_convertEx
	_, err = C.ucnv_convertEx(r.to, r.from, &dst, dstLimit, &src, srcLimit, nil, nil, nil, nil, C.UBool(1), C.UBool(1), &uErr)
	if err != nil {
		return n, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return n, err
	}

	r.ilen = 0

	olen = int(uintptr(unsafe.Pointer(dst)) - dstStart)

	// flush write buffer
	if olen > plen-n {
		copy(p[n:], r.obuf[:plen-n])
		r.ostart = plen - n
		r.oend = olen
		return plen, nil
	}
	copy(p[n:], r.obuf[:olen])
	return olen + n, nil
}

// provide previous internal input result for error handling
func (r *ReadCloser) Input() []byte {
	return r.ibuf[:r.ilen]
}

// provide previous internal output result for error handling
func (r *ReadCloser) Output() []byte {
	return r.obuf[r.ostart:r.oend]
}

type WriteCloser struct {
	w    io.Writer
	from *C.UConverter
	to   *C.UConverter
	obuf []byte
	olen int
}

func NewWriteCloser(w io.Writer, from string, to string) (*WriteCloser, error) {
	ufrom, err := ucnvOpen(from)
	if err != nil {
		return nil, err
	}

	uto, err := ucnvOpen(to)
	if err != nil {
		return nil, err
	}

	writer := &WriteCloser{
		w:    w,
		from: ufrom,
		to:   uto,
		obuf: make([]byte, 0, defaultBufSize),
	}

	runtime.SetFinalizer(writer, func(writer *WriteCloser) {
		writer.Close()
	})

	return writer, nil
}

func (w *WriteCloser) Write(p []byte) (int, error) {
	plen := len(p)
	src := (*C.char)(unsafe.Pointer(&p[0]))
	srcLimit := (*C.char)(unsafe.Pointer(&p[plen]))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)

	oMaxLen, err := ucnvMaxLen(plen, w.from, w.to)
	if err != nil {
		return 0, err
	}

	// reallocate write buffer
	if oMaxLen > cap(w.obuf) {
		w.obuf = make([]byte, oMaxLen*2)
	}
	dst := (*C.char)(unsafe.Pointer(&w.obuf[0]))
	dstLimit := (*C.char)(unsafe.Pointer(&w.obuf[oMaxLen]))

	dstStart := uintptr(unsafe.Pointer(dst))

	// fill write buffer by C.ucnv_convertEx
	_, err = C.ucnv_convertEx(w.to, w.from, &dst, dstLimit, &src, srcLimit, nil, nil, nil, nil, C.UBool(1), C.UBool(1), &uErr)
	if err != nil {
		return 0, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return 0, err
	}

	w.olen = int(uintptr(unsafe.Pointer(dst)) - dstStart)

	n, err := w.w.Write(w.obuf[:w.olen])
	if err != nil {
		return n, err
	}

	return n, nil
}

// provide previous internal output result for error handling
func (w *WriteCloser) Output() []byte {
	return w.obuf[:w.olen]
}

func (w *WriteCloser) Close() error {
	_, err1 := C.ucnv_close(w.from)
	_, err2 := C.ucnv_close(w.to)
	if err1 != nil {
		return err1
	}
	return err2
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
	return &UError{errStr}
}

func ucnvOpen(encoding string) (*C.UConverter, error) {
	cencoding := C.CString(encoding)
	defer C.free(unsafe.Pointer(cencoding))

	uErr := C.UErrorCode(C.U_ZERO_ERROR)
	ucnv, err := C.ucnv_open(cencoding, &uErr)
	if err != nil {
		return nil, err
	}
	if err = uErrorToGoError(uErr); err != nil {
		return nil, err
	}
	return ucnv, nil
}

func ucnvMaxLen(ilen int, from *C.UConverter, to *C.UConverter) (int, error) {
	min, err := C.ucnv_getMinCharSize(from)
	if err != nil {
		return 0, err
	}

	max, err := C.ucnv_getMaxCharSize(to)
	if err != nil {
		return 0, err
	}

	return (ilen / int(min)) * int(max), nil
}

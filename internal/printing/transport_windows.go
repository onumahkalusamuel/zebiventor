//go:build windows

package printing

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

// Raw printing through the Windows spooler. Sending with datatype "RAW"
// bypasses the driver's rendering, so ESC/POS reaches the printer untouched
// whatever the driver's page size is set to — this is what makes dynamic
// receipt length work on Windows.

var (
	winspool             = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinter      = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinter  = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
	procEnumPrinters     = winspool.NewProc("EnumPrintersW")
)

type docInfo1 struct {
	docName    *uint16
	outputFile *uint16
	datatype   *uint16
}

type printerInfo4 struct {
	printerName *uint16
	serverName  *uint16
	attributes  uint32
}

func sendSystem(_ context.Context, name string, data []byte) error {
	pName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var h syscall.Handle
	if r, _, e := procOpenPrinter.Call(uintptr(unsafe.Pointer(pName)), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return fmt.Errorf("cannot open printer %q: %v", name, e)
	}
	defer procClosePrinter.Call(uintptr(h))

	docName, _ := syscall.UTF16PtrFromString("Zebiventor receipt")
	raw, _ := syscall.UTF16PtrFromString("RAW")
	di := docInfo1{docName: docName, datatype: raw}
	if r, _, e := procStartDocPrinter.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&di))); r == 0 {
		return fmt.Errorf("cannot start print job: %v", e)
	}
	defer procEndDocPrinter.Call(uintptr(h))
	if r, _, e := procStartPagePrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("cannot start page: %v", e)
	}
	defer procEndPagePrinter.Call(uintptr(h))

	for len(data) > 0 {
		var written uint32
		r, _, e := procWritePrinter.Call(uintptr(h), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&written)))
		if r == 0 {
			return fmt.Errorf("write to printer failed: %v", e)
		}
		if written == 0 {
			return fmt.Errorf("printer accepted no data")
		}
		data = data[written:]
	}
	return nil
}

// ListPrinters returns installed printer names (local and network connections).
func ListPrinters() ([]string, error) {
	const flags = 0x2 | 0x4 // PRINTER_ENUM_LOCAL | PRINTER_ENUM_CONNECTIONS
	var needed, returned uint32
	procEnumPrinters.Call(flags, 0, 4, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		return []string{}, nil
	}
	buf := make([]byte, needed)
	r, _, e := procEnumPrinters.Call(flags, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r == 0 {
		return nil, fmt.Errorf("listing printers failed: %v", e)
	}
	infos := unsafe.Slice((*printerInfo4)(unsafe.Pointer(&buf[0])), returned)
	names := make([]string, 0, returned)
	for _, pi := range infos {
		names = append(names, utf16PtrToString(pi.printerName))
	}
	return names, nil
}

func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var s []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		s = append(s, c)
	}
	return syscall.UTF16ToString(s)
}

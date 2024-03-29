//go:build windows && directsound
// +build windows,directsound

package directsound

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	win32 "github.com/heucuva/go-win32"
	winmm "github.com/heucuva/go-winmm"
	"golang.org/x/sys/windows"
)

var (
	// ErrDirectSound is an error returned by the directsound system
	ErrDirectSound = errors.New("directsound error")

	errDirectSound = fmt.Errorf("%w: in DirectSound", ErrDirectSound)

	dsoundDll              = windows.NewLazySystemDLL("dsound.dll")
	directSoundCreate8Proc = dsoundDll.NewProc("DirectSoundCreate8")
)

type directSoundVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr

	CreateSoundBuffer    uintptr
	GetCaps              uintptr
	DuplicateSoundBuffer uintptr
	SetCooperativeLevel  uintptr
	Compact              uintptr
	GetSpeakerConfig     uintptr
	SetSpeakerConfig     uintptr
	Initialize           uintptr
}

type DirectSound struct {
	vtbl *directSoundVtbl
}

func createDevice(deviceID *syscall.GUID) (*DirectSound, error) {
	var obj *DirectSound
	deviceIDPtr := uintptr(unsafe.Pointer(deviceID))
	objPtr := uintptr(unsafe.Pointer(&obj))
	retVal, _, _ := directSoundCreate8Proc.Call(deviceIDPtr, objPtr, 0)
	if retVal != 0 {
		return nil, fmt.Errorf("%w: DirectSoundCreate8 returned %0.8x", errDirectSound, retVal)
	}

	hwnd := win32.GetDesktopWindow()

	if err := obj.setCooperativeLevel(hwnd, DSSCL_PRIORITY); err != nil {
		obj.release()
		return nil, err
	}
	return obj, nil
}

// NewDSound returns a new DirectSound interface for the preferred device
func NewDSound(preferredDeviceName string) (*DirectSound, error) {
	var deviceID *syscall.GUID
	if preferredDeviceName != "" {
		// TODO: determine GUID for provided preferred device name here
		// preferredDeviceName = &syscall.GUID{ ... }
	}
	return createDevice(deviceID)
}

func (ds *DirectSound) addRef() error {
	retVal, _, _ := syscall.Syscall(ds.vtbl.AddRef, 1, uintptr(unsafe.Pointer(ds)), 0, 0)
	if retVal != 0 {
		return fmt.Errorf("%w: AddRef returned %0.8x", errDirectSound, retVal)
	}
	return nil
}

func (ds *DirectSound) release() error {
	retVal, _, _ := syscall.Syscall(ds.vtbl.Release, 1, uintptr(unsafe.Pointer(ds)), 0, 0)
	if retVal != 0 {
		return fmt.Errorf("%w: Release returned %0.8x", errDirectSound, retVal)
	}
	return nil
}

func (ds *DirectSound) setCooperativeLevel(hwnd windows.HWND, level uint32) error {
	retVal, _, _ := syscall.Syscall(ds.vtbl.SetCooperativeLevel, 3, uintptr(unsafe.Pointer(ds)), uintptr(hwnd), uintptr(level))
	if retVal != 0 {
		return fmt.Errorf("%w: SetCooperativeLevel returned %0.8x", errDirectSound, retVal)
	}
	return nil
}

type dsBufferDesc struct {
	Size        uint32
	Flags       DSBCAPS
	BufferBytes uint32
	Reserved    uint32
	WfxFormat   *winmm.WAVEFORMATEX
}

// CreateSoundBufferPrimary creates a primary sound buffer
func (ds *DirectSound) CreateSoundBufferPrimary(channels int, samplesPerSec int, bitsPerSample int) (*Buffer, *winmm.WAVEFORMATEX, error) {
	bd := dsBufferDesc{
		Flags: DSBCAPS_PRIMARYBUFFER,
	}
	bd.Size = uint32(unsafe.Sizeof(bd))

	var buffer *Buffer
	retVal, _, _ := syscall.Syscall6(ds.vtbl.CreateSoundBuffer, 4, uintptr(unsafe.Pointer(ds)), uintptr(unsafe.Pointer(&bd)), uintptr(unsafe.Pointer(&buffer)), 0, 0, 0)
	if retVal != 0 {
		return nil, nil, fmt.Errorf("%w: CreateSoundBuffer(primary) returned %0.8x", errDirectSound, retVal)
	}

	wfx := winmm.WAVEFORMATEX{
		WFormatTag:     winmm.WAVE_FORMAT_PCM,
		NChannels:      uint16(channels),
		NSamplesPerSec: uint32(samplesPerSec),
		WBitsPerSample: uint16(bitsPerSample),
	}
	wfx.CbSize = uint16(unsafe.Sizeof(wfx))
	wfx.NBlockAlign = uint16(channels * bitsPerSample / 8)
	wfx.NAvgBytesPerSec = wfx.NSamplesPerSec * uint32(wfx.NBlockAlign)

	if err := buffer.setFormat(wfx); err != nil {
		buffer.Release()
		return nil, nil, err
	}

	return buffer, &wfx, nil
}

// CreateSoundBufferSecondary creates a secondary sound buffer
func (ds *DirectSound) CreateSoundBufferSecondary(wfx *winmm.WAVEFORMATEX, bufferSize int) (*Buffer, error) {
	bd := dsBufferDesc{
		Flags:       DSBCAPS_GETCURRENTPOSITION2 | DSBCAPS_GLOBALFOCUS | DSBCAPS_CTRLALL,
		BufferBytes: uint32(bufferSize),
		WfxFormat:   wfx,
	}
	bd.Size = uint32(unsafe.Sizeof(bd))

	var buffer *Buffer
	retVal, _, _ := syscall.Syscall6(ds.vtbl.CreateSoundBuffer, 4, uintptr(unsafe.Pointer(ds)), uintptr(unsafe.Pointer(&bd)), uintptr(unsafe.Pointer(&buffer)), 0, 0, 0)
	if retVal != 0 {
		return nil, fmt.Errorf("%w: CreateSoundBuffer(secondary) returned %0.8x", errDirectSound, retVal)
	}

	return buffer, nil
}

// Close cleans up the DirectSound device
func (ds *DirectSound) Close() error {
	return ds.release()
}

package television

import (
	"fmt"
	"gopher2600/errors"
	"strings"
)

// HeadlessTV is the minimalist implementation of the Television interface - a
// television without a screen. Fuller implementations of the television can
// use this as the basis of the emulation by struct embedding. The
// InitHeadlessTV() method is useful in this regard.
type HeadlessTV struct {
	// spec is the specification of the tv type (NTSC or PAL)
	Spec *Specification

	// if the most recently received signal is not as expected, according to
	// the television protocol definition in the Stella Programmer's Guide, the
	// the outOfSpec flags will be true
	outOfSpec bool

	// state of the television
	// * the current horizontal position. the position where the next pixel will be
	//  drawn. also used to check we're receiving the correct signals at the
	//  correct time.
	// * the current frame
	// * the current scanline number
	HorizPos *TVState
	FrameNum *TVState
	Scanline *TVState

	// record of signal attributes from the last call to Signal()
	prevSignal SignalAttributes

	// vsyncCount records the number of consecutive colorClocks the vsync signal
	// has been sustained. we use this to help correctly implement vsync.
	vsyncCount int

	// the scanline at which vblank is turned off and on
	//  - top mask ranges from 0 to VBlankOff-1
	//  - bottom mask ranges from VBlankOn to Spec.ScanlinesTotal
	VBlankOff int
	VBlankOn  int

	// callback hooks from Signal() - these are used by outer-structs to
	// hook into and add extra gubbins to the Signal() function
	HookNewFrame    func() error
	HookNewScanline func() error
	HookSetPixel    func(x, y int32, red, green, blue byte, vblank bool) error
}

// NewHeadlessTV creates a new instance of HeadlessTV for a minimalist
// implementation of a televsion for the VCS emulation
func NewHeadlessTV(tvType string) (*HeadlessTV, error) {
	tv := new(HeadlessTV)

	err := InitHeadlessTV(tv, tvType)
	if err != nil {
		return nil, err
	}

	return tv, nil
}

// InitHeadlessTV initialises an instance of HeadlessTV. useful for television
// types that want to "inherit" the basic functionality of HeadlessTV by
// embedding. those types can call InitHeadlessTV() on the embedded field
func InitHeadlessTV(tv *HeadlessTV, tvType string) error {
	switch strings.ToUpper(tvType) {
	case "NTSC":
		tv.Spec = SpecNTSC
	case "PAL":
		tv.Spec = SpecPAL
	default:
		return fmt.Errorf("unsupport tv type (%s)", tvType)
	}

	// empty callbacks
	tv.HookNewFrame = func() error { return nil }
	tv.HookNewScanline = func() error { return nil }
	tv.HookSetPixel = func(x, y int32, r, g, b byte, vblank bool) error { return nil }

	// initialise TVState
	tv.HorizPos = &TVState{label: "Horiz Pos", shortLabel: "HP", value: -tv.Spec.ClocksPerHblank, valueFormat: "%d"}
	tv.FrameNum = &TVState{label: "Frame", shortLabel: "FR", value: 0, valueFormat: "%d"}
	tv.Scanline = &TVState{label: "Scanline", shortLabel: "SL", value: 0, valueFormat: "%d"}

	// vblank off/on values. initialise with ideal values
	tv.VBlankOff = tv.Spec.ScanlinesPerVBlank + tv.Spec.ScanlinesPerVSync
	tv.VBlankOn = tv.Spec.ScanlinesTotal - tv.Spec.ScanlinesPerOverscan

	return nil
}

// MachineInfoTerse returns the television information in terse format
func (tv HeadlessTV) MachineInfoTerse() string {
	specExclaim := ""
	if tv.outOfSpec {
		specExclaim = " !!"
	}
	return fmt.Sprintf("%s %s %s%s", tv.FrameNum.MachineInfoTerse(), tv.Scanline.MachineInfoTerse(), tv.HorizPos.MachineInfoTerse(), specExclaim)
}

// MachineInfo returns the television information in verbose format
func (tv HeadlessTV) MachineInfo() string {
	s := strings.Builder{}
	outOfSpec := ""
	if tv.outOfSpec {
		outOfSpec = " !!"
	}
	s.WriteString(fmt.Sprintf("TV (%s)%s:\n", tv.Spec.ID, outOfSpec))
	s.WriteString(fmt.Sprintf("   %s\n", tv.FrameNum))
	s.WriteString(fmt.Sprintf("   %s\n", tv.Scanline))
	s.WriteString(fmt.Sprintf("   %s", tv.HorizPos))

	return s.String()
}

// map String to MachineInfo
func (tv HeadlessTV) String() string {
	return tv.MachineInfo()
}

// Reset all the values for the television
func (tv *HeadlessTV) Reset() error {
	tv.HorizPos.value = -tv.Spec.ClocksPerHblank
	tv.FrameNum.value = 0
	tv.Scanline.value = 0
	tv.vsyncCount = 0
	tv.prevSignal = SignalAttributes{}
	tv.VBlankOff = -1
	tv.VBlankOn = -1
	return nil
}

// Signal is principle method of communication between the VCS and televsion
//
// the function will panic if an unexpected signal is received (or not received,
// as the case may be).
//
// if a signal is not entirely unexpected but is none-the-less out-of-spec then
// then the tv object will be flagged outOfSpec and the emulation allowed to
// continue.
func (tv *HeadlessTV) Signal(attr SignalAttributes) error {
	if attr.HSync && !tv.prevSignal.HSync {
		if tv.HorizPos.value < -52 || tv.HorizPos.value > -49 {
			panic(fmt.Sprintf("bad HSYNC (on at %d)", tv.HorizPos.value))
		}
	} else if !attr.HSync && tv.prevSignal.HSync {
		if tv.HorizPos.value < -36 || tv.HorizPos.value > -33 {
			panic(fmt.Sprintf("bad HSYNC (off at %d)", tv.HorizPos.value))
		}
	}
	if attr.CBurst && !tv.prevSignal.CBurst {
		if tv.HorizPos.value < -28 || tv.HorizPos.value > -17 {
			panic("bad CBURST (on)")
		}
	} else if !attr.CBurst && tv.prevSignal.CBurst {
		if tv.HorizPos.value < -19 || tv.HorizPos.value > -16 {
			panic("bad CBURST (off)")
		}
	}

	// simple implementation of vsync
	if attr.VSync {
		tv.vsyncCount++
	} else {
		if tv.vsyncCount >= tv.Spec.VsyncClocks {
			tv.outOfSpec = tv.vsyncCount != tv.Spec.VsyncClocks

			tv.FrameNum.value++
			tv.Scanline.value = 0
			tv.vsyncCount = 0

			err := tv.HookNewFrame()
			if err != nil {
				return err
			}

			// some roms turn off vblank multiple times before the end of the frame. to
			// prevent recording additional VBLANK signals, we make sure to
			// reset the VBlankOff value at the end of the frame
			//
			// ROMs affected:
			//	* Custer's Revenge
			//	* Ladybug
			tv.VBlankOff = -1
		}
	}

	// start a new scanline if a frontporch signal has been received
	if attr.FrontPorch {
		tv.HorizPos.value = -tv.Spec.ClocksPerHblank
		tv.Scanline.value++
		err := tv.HookNewScanline()
		if err != nil {
			return err
		}

		if tv.Scanline.value > tv.Spec.ScanlinesTotal {
			// we've not yet received a correct vsync signal
			// continue with an implied VSYNC
			tv.outOfSpec = true

			// repeat the last scanline (over and over if necessary)
			tv.Scanline.value--
		}
	} else {
		tv.HorizPos.value++

		// check to see if frontporch has been encountered
		// we're panicking because this shouldn't ever happen
		if tv.HorizPos.value > tv.Spec.ClocksPerVisible {
			panic("no FRONTPORCH")
		}
	}

	// note the scanline when vblank is turned on/off. plus, only record the
	// off signal if it hasn't been set before this frame
	if tv.VBlankOff == -1 && !attr.VBlank && tv.prevSignal.VBlank {
		tv.VBlankOff = tv.Scanline.value
	}
	if attr.VBlank && !tv.prevSignal.VBlank {
		// some ROMS do not turn on VBlank until the beginning of the frame
		// this means that the value of vblank on will be less than vblank off.
		// to remedy this, we record a value of ScanlinesTotal+1 instead of 0.
		//
		// ROMs affected:
		//  * Gauntlet
		if tv.Scanline.value == 0 {
			tv.VBlankOn = tv.Spec.ScanlinesTotal + 1
		} else {
			tv.VBlankOn = tv.Scanline.value
		}
	}

	// record the current signal settings so they can be used for reference
	tv.prevSignal = attr

	// decode color
	red, green, blue := byte(0), byte(0), byte(0)
	if attr.Pixel <= 256 {
		col := tv.Spec.Colors[attr.Pixel]
		red, green, blue = byte((col&0xff0000)>>16), byte((col&0xff00)>>8), byte(col&0xff)
	}

	// current coordinates
	x := int32(tv.HorizPos.value) + int32(tv.Spec.ClocksPerHblank)
	y := int32(tv.Scanline.value)

	return tv.HookSetPixel(x, y, red, green, blue, attr.VBlank)
}

// RequestTVState returns the TVState object for the named state. television
// implementations in other packages will difficulty extending this function
// because TVStateReq does not expose its members. (although it may need to if
// television is running in it's own goroutine)
func (tv *HeadlessTV) RequestTVState(request TVStateReq) (*TVState, error) {
	switch request {
	default:
		return nil, errors.NewGopherError(errors.UnknownTVRequest, request)
	case ReqFramenum:
		return tv.FrameNum, nil
	case ReqScanline:
		return tv.Scanline, nil
	case ReqHorizPos:
		return tv.HorizPos, nil
	}
}

// RequestTVInfo returns the TVState object for the named state
func (tv *HeadlessTV) RequestTVInfo(request TVInfoReq) (string, error) {
	switch request {
	default:
		return "", errors.NewGopherError(errors.UnknownTVRequest, request)
	case ReqTVSpec:
		return tv.Spec.ID, nil
	}
}

// RequestCallbackRegistration is used to hook custom functionality into the televsion
func (tv *HeadlessTV) RequestCallbackRegistration(request CallbackReq, channel chan func(), callback func()) error {
	// the HeadlessTV implementation does nothing currently
	return errors.NewGopherError(errors.UnknownTVRequest, request)
}

// RequestSetAttr is used to set a television attibute
func (tv *HeadlessTV) RequestSetAttr(request SetAttrReq, args ...interface{}) error {
	return errors.NewGopherError(errors.UnknownTVRequest, request)
}

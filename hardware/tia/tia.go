package tia

import (
	"fmt"
	"gopher2600/hardware/memory"
	"gopher2600/hardware/tia/polycounter"
	"gopher2600/hardware/tia/video"
	"gopher2600/television"
)

const vblankMask = 0x02
const vsyncMask = 0x02
const vsyncLatchTriggerMask = 0x40
const vsyncGroundedPaddleMask = 0x80

// TIA contains all the sub-components of the VCS TIA sub-system
type TIA struct {
	tv  television.Television
	mem memory.ChipBus

	colorClock *polycounter.Polycounter

	// motion clock is an out-of-phase colorClock, running 2 cycles ahead of
	// the main color clock (according to the document, "Atari 2600 TIA
	// Hardware Notes" by Andrew Towers) - currently used to indicate when
	// calling tickFutures()
	motionClock bool

	Hmove *hmove
	rsync *rsync

	// TIA state -- controlled by the CPU
	vsync  bool
	vblank bool

	// TIA state -- set automatically by the TIA
	hblank bool
	hsync  bool
	wsync  bool

	Video *video.Video
	// TODO: audio
}

// MachineInfoTerse returns the TIA information in terse format
func (tia TIA) MachineInfoTerse() string {
	return fmt.Sprintf("%s %s %s", tia.colorClock.MachineInfoTerse(), tia.rsync.MachineInfoTerse(), tia.Hmove.MachineInfoTerse())
}

// MachineInfo returns the TIA information in verbose format
func (tia TIA) MachineInfo() string {
	return fmt.Sprintf("TIA:\n   colour clock: %v\n   %v\n   %v", tia.colorClock, tia.rsync, tia.Hmove)
}

// map String to MachineInfo
func (tia TIA) String() string {
	return tia.MachineInfo()
}

// NewTIA creates a TIA, to be used in a VCS emulation
func NewTIA(tv television.Television, mem memory.ChipBus) *TIA {
	tia := new(TIA)
	tia.tv = tv
	tia.mem = mem

	// TODO: audio

	tia.colorClock = polycounter.New6Bit()
	tia.colorClock.SetResetPoint(56) // "010100"

	tia.Hmove = newHmove(tia.colorClock)
	if tia.Hmove == nil {
		return nil
	}

	tia.rsync = newRsync(tia.colorClock)
	if tia.rsync == nil {
		return nil
	}

	tia.hblank = true

	tia.Video = video.NewVideo(tia.colorClock)
	if tia.Video == nil {
		return nil
	}

	// TODO: audio

	return tia
}

// ReadTIAMemory checks for side effects in the TIA sub-system
func (tia *TIA) ReadTIAMemory() {
	service, register, value := tia.mem.ChipRead()
	if !service {
		return
	}

	switch register {
	case "VSYNC":
		tia.vsync = value&vsyncMask == vsyncMask
		// TODO: do something with controller settings below
		_ = value&vsyncLatchTriggerMask == vsyncLatchTriggerMask
		_ = value&vsyncGroundedPaddleMask == vsyncGroundedPaddleMask
		service = false
	case "VBLANK":
		tia.vblank = value&vblankMask == vblankMask
		service = false
	case "WSYNC":
		tia.wsync = true
		service = false
	case "RSYNC":
		tia.rsync.set()
		service = false
	case "HMOVE":
		tia.Video.PrepareSpritesForHMOVE()
		tia.Hmove.set()
		service = false
	}

	if !service {
		return
	}

	service = !tia.Video.ReadVideoMemory(register, value)

	// TODO: TIA audio memory
}

// StepVideoCycle moves the state of the tia forward one video cycle
// returns the state of the CPU (conceptually, we're attaching the result of
// this function to pin 3 of the 6507)
func (tia *TIA) StepVideoCycle() bool {
	frontPorch := false
	cburst := false

	// color clock
	if tia.colorClock.MatchEnd(16) && !tia.Hmove.isActive() {
		// HBLANK off (early)
		tia.hblank = false
	} else if tia.colorClock.MatchEnd(18) && tia.Hmove.isActive() {
		// HBLANK off (late)
		tia.hblank = false
	} else if tia.colorClock.MatchEnd(4) {
		tia.hsync = true
	} else if tia.colorClock.MatchEnd(8) {
		tia.hsync = false
	} else if tia.colorClock.MatchEnd(12) {
		cburst = true
	}

	// motion clock is an out-of-phase color clock. note that the motion clock
	// does not care about HMOVE.
	if tia.colorClock.MatchEnd(15) {
		tia.motionClock = true
	} else if tia.colorClock.MatchEnd(56) {
		tia.motionClock = false
	}

	// set up new scanline if colorClock has ticked its way to the reset point or if
	// an rsync has matured (see rsync.go commentary)
	if tia.rsync.tick() {
		frontPorch = true
		tia.wsync = false
		tia.hblank = true
		tia.Hmove.reset()
		tia.Video.NewScanline()
		tia.colorClock.Reset()
	} else if tia.colorClock.Tick() {
		frontPorch = true
		tia.wsync = false
		tia.hblank = true
		tia.Hmove.reset()
		tia.Video.NewScanline()
		// not sure if we need to reset rsync
	}

	// HMOVE clock stuffing
	if ct, ok := tia.Hmove.tick(); ok {
		tia.Video.TickSpritesForHMOVE(ct)
	}

	// tick all sprites according to hblank
	if !tia.hblank {
		tia.Video.TickSprites()
	}

	// tick playfield and scheduled writes
	// -- important that this happens after TickSprites because we want
	// position resets to happen *after* sprite ticking; in particular, when
	// the draw signal has been resolved
	tia.Video.TickPlayfield()
	tia.Video.TickFutures(tia.motionClock)

	// decide on pixel color
	pixelColor := television.VideoBlack
	if !tia.hblank {
		pixelColor = television.ColorSignal(tia.Video.Pixel())
	}

	// at the end of the video cycle we want to finally signal the televison
	err := tia.tv.Signal(television.SignalAttributes{
		VSync:      tia.vsync,
		VBlank:     tia.vblank,
		FrontPorch: frontPorch,
		HSync:      tia.hsync,
		CBurst:     cburst,
		Pixel:      pixelColor})
	if err != nil {
		panic(err)
	}

	// set collision registers
	tia.Video.Collisions.SetMemory(tia.mem)

	return !tia.wsync
}

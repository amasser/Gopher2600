// This file is part of Gopher2600.
//
// Gopher2600 is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Gopher2600 is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Gopher2600.  If not, see <https://www.gnu.org/licenses/>.

// ROMs used to test PAL switching and resizing:
// * Pitfall
// * Hero
// * Chiphead
// * Bang!
// * Ladybug
// * Hack Em Hangly Pacman
// * Andrew Davies' Chess
// * Communist Mutants From Space
// * Mega Bitmap Demo

package television

import (
	"fmt"
	"strings"

	"github.com/jetsetilly/gopher2600/curated"
)

// the number of additional lines over the NTSC spec that is allowed before the
// TV flips to the PAL specification.
const excessScanlinesNTSC = 40

// the number of synced frames where we can expect things to be in flux.
const leadingFrames = 5

// the number of synced frames required before the tv frame is considered to "stable".
const stabilityThreshold = 20

type state struct {
	// television specification (NTSC or PAL)
	spec Spec

	// auto flag indicates that the tv type/specification should switch if it
	// appears to be outside of the current spec.
	//
	// in practice this means that if auto is true then we start with the NTSC
	// spec and move to PAL if the number of scanlines exceeds the NTSC maximum
	auto bool

	// state of the television
	//	- the current horizontal position. the position where the next pixel will be
	//  drawn. also used to check we're receiving the correct signals at the
	//  correct time.
	horizPos int
	//	- the current frame
	frameNum int
	//	- the current scanline number
	scanline int
	//  - the current synced frame number. a synced frame is one which was
	//  generated from a valid VSYNC/VBLANK sequence. we use this to detect:
	//   * whether the image is "stable"
	//   * whether specification changes should still occur
	syncedFrameNum int

	// is current frame as a result of a VSYNC flyback or not (a "natural"
	// flyback). we use this in the context of newFrame() so we should probably
	// think of this as the previous frame.
	syncedFrame bool

	// record of signal attributes from the last call to Signal()
	lastSignal SignalAttributes

	// vsyncCount records the number of consecutive colorClocks the vsync signal
	// has been sustained. we use this to help correctly implement vsync.
	vsyncCount int

	// top and bottom of screen as detected by vblank/color signal
	top    int
	bottom int

	// list of signals sent to pixel renderers since the beginning of the
	// current frame
	signalHistory []signalHistoryEntry

	// the index to write the next signal
	signalHistoryIdx int
}

type signalHistoryEntry struct {
	x   int
	y   int
	sig SignalAttributes
}

// reference is a reference implementation of the Television interface. In all
// honesty, it's most likely the only implementation required.
type reference struct {
	// spec on creation ID is the string that was to ID the television
	// type/spec on creation. because the actual spec can change, the ID field
	// of the Spec type can not be used for things like regression
	// test recreation etc.
	reqSpecID string

	// frame resizer
	resizer resizer

	// framerate limiter
	lmtr limiter

	// list of renderer implementations to consult
	renderers []PixelRenderer

	// list of refresher implementations to consult
	refreshers []PixelRefresher

	// list of frametrigger implementations to consult
	frameTriggers []FrameTrigger

	// list of audio mixers to consult
	mixers []AudioMixer

	state *state
}

// NewReference creates a new instance of the reference television type,
// satisfying the Television interface.
func NewReference(spec string) (Television, error) {
	tv := &reference{
		resizer:   &simpleResizer{},
		reqSpecID: strings.ToUpper(spec),
		state:     &state{},
	}

	// set specification
	err := tv.SetSpec(spec)
	if err != nil {
		return nil, err
	}

	// initialise frame rate limiter
	tv.lmtr.init()
	tv.SetFPS(-1)

	// empty list of renderers
	tv.renderers = make([]PixelRenderer, 0)

	return tv, nil
}

func (tv reference) String() string {
	s := strings.Builder{}
	s.WriteString(fmt.Sprintf("FR=%04d SL=%03d HP=%03d", tv.state.frameNum, tv.state.scanline, tv.state.horizPos-HorizClksHBlank))
	return s.String()
}

// Snapshot implements the Television interface.
func (tv *reference) Snapshot() TelevisionState {
	n := *tv.state
	n.signalHistory = make([]signalHistoryEntry, len(tv.state.signalHistory))
	copy(n.signalHistory, tv.state.signalHistory)
	return &n
}

// RestoreSnapshot implements the Television interface.
func (tv *reference) RestoreSnapshot(s TelevisionState) {
	if s == nil {
		return
	}

	tv.state = s.(*state)

	for _, r := range tv.refreshers {
		r.Refresh(true)
	}

	for i, e := range tv.state.signalHistory {
		col := tv.state.spec.getColor(e.sig.Pixel)
		for _, r := range tv.refreshers {
			r.RefreshPixel(e.x, e.y, col.R, col.G, col.B, e.sig.VBlank, i >= tv.state.signalHistoryIdx)
		}
	}

	for _, r := range tv.refreshers {
		r.Refresh(false)
	}
}

// AddPixelRenderer implements the Television interface.
func (tv *reference) AddPixelRenderer(r PixelRenderer) {
	tv.renderers = append(tv.renderers, r)
	tv.frameTriggers = append(tv.frameTriggers, r)
}

// AddPixelRefresher implements the Television interface.
func (tv *reference) AddPixelRefresher(r PixelRefresher) {
	tv.refreshers = append(tv.refreshers, r)
}

// AddFrameTrigger implements the Television interface.
func (tv *reference) AddFrameTrigger(f FrameTrigger) {
	tv.frameTriggers = append(tv.frameTriggers, f)
}

// AddAudioMixer implements the Television interface.
func (tv *reference) AddAudioMixer(m AudioMixer) {
	tv.mixers = append(tv.mixers, m)
}

// Reset implements the Television interface.
func (tv *reference) Reset() error {
	// we definitely do not call this on television initialisation because the
	// rest of the system may not be yet be in a suitable state

	err := tv.SetSpec(tv.reqSpecID)
	if err != nil {
		return err
	}

	tv.state.horizPos = 0
	tv.state.frameNum = 0
	tv.state.scanline = 0
	tv.state.syncedFrameNum = 0
	tv.state.vsyncCount = 0
	tv.state.lastSignal = SignalAttributes{}

	return nil
}

// End implements the Television interface.
func (tv reference) End() error {
	var err error

	// call new frame for all renderers
	for f := range tv.renderers {
		err = tv.renderers[f].EndRendering()
	}

	// flush audio for all mixers
	for f := range tv.mixers {
		err = tv.mixers[f].EndMixing()
	}

	return err
}

// Signal implements the Television interface.
func (tv *reference) Signal(sig SignalAttributes) error {
	// mix audio before we do anything else
	if sig.AudioUpdate {
		for f := range tv.mixers {
			err := tv.mixers[f].SetAudio(sig.AudioData)
			if err != nil {
				return err
			}
		}
	}

	// examine signal for resizing possibility
	tv.resizer.examine(tv, sig)

	// a Signal() is by definition a new color clock. increase the horizontal count
	tv.state.horizPos++

	// once we reach the scanline's back-porch we'll reset the horizPos counter
	// and wait for the HSYNC signal. we do this so that the front-porch and
	// back-porch are 'together' at the beginning of the scanline. this isn't
	// strictly technically correct but it's convenient to think about
	// scanlines in this way (rather than having a split front and back porch)
	if tv.state.horizPos >= HorizClksScanline {
		tv.state.horizPos = 0

		// bump scanline counter
		tv.state.scanline++

		// reached end of screen without synchronisation. fly-back naturally.
		if tv.state.scanline > tv.state.spec.ScanlinesTotal {
			err := tv.newFrame(false)
			if err != nil {
				return err
			}
		} else {
			// if we're not at end of screen then indicate new scanline
			err := tv.newScanline()
			if err != nil {
				return err
			}
		}

		// checkRate evey scanline. see checkRate() commentary for why this is
		tv.lmtr.checkRate()
	}

	// check vsync signal at the time of the flyback
	//
	// !!TODO: replace VSYNC signal with extended HSYNC signal
	if sig.VSync && !tv.state.lastSignal.VSync {
		tv.state.vsyncCount = 0
	} else if !sig.VSync && tv.state.lastSignal.VSync {
		if tv.state.vsyncCount > 0 {
			err := tv.newFrame(true)
			if err != nil {
				return err
			}
		}
	}

	// we've "faked" the flyback signal above when horizPos reached
	// horizClksScanline. we need to handle the real flyback signal however, by
	// making sure we're at the correct horizPos value.  if horizPos doesn't
	// equal 16 at the front of the HSYNC or 36 at then back of the HSYNC, then
	// it indicates that the RSYNC register was used last scanline.
	if sig.HSync && !tv.state.lastSignal.HSync {
		tv.state.horizPos = 16

		// count vsync lines at start of hsync
		if sig.VSync || tv.state.lastSignal.VSync {
			tv.state.vsyncCount++
		}
	}
	if !sig.HSync && tv.state.lastSignal.HSync {
		tv.state.horizPos = 36
	}

	// doing nothing with CBURST signal

	// decode color using the regular color signal
	col := tv.state.spec.getColor(sig.Pixel)
	for f := range tv.renderers {
		err := tv.renderers[f].SetPixel(tv.state.horizPos, tv.state.scanline,
			col.R, col.G, col.B, sig.VBlank)
		if err != nil {
			return err
		}
	}

	// record the current signal settings so they can be used for reference
	tv.state.lastSignal = sig

	e := signalHistoryEntry{
		x:   tv.state.horizPos,
		y:   tv.state.scanline,
		sig: sig,
	}

	if tv.state.signalHistoryIdx >= len(tv.state.signalHistory) {
		tv.state.signalHistory = append(tv.state.signalHistory, e)
	} else {
		tv.state.signalHistory[tv.state.signalHistoryIdx] = e
	}
	tv.state.signalHistoryIdx++

	return nil
}

func (tv *reference) newScanline() error {
	// notify renderers of new scanline
	for f := range tv.renderers {
		err := tv.renderers[f].NewScanline(tv.state.scanline)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tv *reference) newFrame(synced bool) error {
	// a synced frame is one which was generated from a valid VSYNC/VBLANK sequence
	if tv.state.syncedFrame {
		tv.state.syncedFrameNum++
	}

	// specification change
	if tv.state.syncedFrameNum > leadingFrames && tv.state.syncedFrameNum < stabilityThreshold {
		if tv.state.auto && !tv.state.syncedFrame && tv.state.scanline > excessScanlinesNTSC {
			// flip from NTSC to PAL
			if tv.state.spec.ID == SpecNTSC.ID {
				_ = tv.SetSpec("PAL")
			}
		}
	}

	// commit any resizing that maybe pending
	err := tv.resizer.commit(tv)
	if err != nil {
		return err
	}

	// prepare for next frame
	tv.state.frameNum++
	tv.state.scanline = 0
	tv.resizer.prepare(tv)
	tv.state.syncedFrame = synced

	// process all FrameTriggers
	for f := range tv.frameTriggers {
		err = tv.frameTriggers[f].NewFrame(tv.state.frameNum, tv.IsStable())
		if err != nil {
			return err
		}
	}

	// reset signal history for next frame
	tv.state.signalHistoryIdx = 0

	return nil
}

// IsStable implements the Television interface.
func (tv reference) IsStable() bool {
	return tv.state.syncedFrameNum >= stabilityThreshold
}

// GetLastSignal implements the Television interface.
func (tv *reference) GetLastSignal() SignalAttributes {
	return tv.state.lastSignal
}

// GetState implements the Television interface.
func (tv *reference) GetState(request StateReq) (int, error) {
	switch request {
	case ReqFramenum:
		return tv.state.frameNum, nil
	case ReqScanline:
		return tv.state.scanline, nil
	case ReqHorizPos:
		return tv.state.horizPos - HorizClksHBlank, nil
	default:
		return 0, curated.Errorf("television: unhandled tv state request (%v)", request)
	}
}

// SetSpec implements the Television interface.
func (tv *reference) SetSpec(spec string) error {
	switch strings.ToUpper(spec) {
	case "NTSC":
		tv.state.spec = SpecNTSC
		tv.state.auto = false
	case "PAL":
		tv.state.spec = SpecPAL
		tv.state.auto = false
	case "AUTO":
		tv.state.spec = SpecNTSC
		tv.state.auto = true
	default:
		return curated.Errorf("television: unsupported spec (%s)", spec)
	}

	tv.state.top = tv.state.spec.ScanlineTop
	tv.state.bottom = tv.state.spec.ScanlineBottom
	tv.resizer.prepare(tv)

	for f := range tv.renderers {
		err := tv.renderers[f].Resize(tv.state.spec, tv.state.top, tv.state.bottom-tv.state.top)
		if err != nil {
			return err
		}
	}

	// allocate enough memory for a TV screen that stays within the limits of
	// the specification
	tv.state.signalHistory = make([]signalHistoryEntry, HorizClksScanline*(tv.state.bottom-tv.state.top))
	tv.state.signalHistoryIdx = 0

	return nil
}

// GetReqSpecID implements the Television interface.
func (tv *reference) GetReqSpecID() string {
	return tv.reqSpecID
}

// GetSpec implements the Television interface.
func (tv reference) GetSpec() Spec {
	return tv.state.spec
}

// SetFPSCap implements the Television interface. Reasons for turning the cap
// off include performance measurement. The debugger also turns the cap off and
// replaces it with its own. The FPS limiter in this television implementation
// works at the frame level which is not fine grained enough for effective
// limiting of rates less than 1fps.
func (tv *reference) SetFPSCap(limit bool) {
	tv.lmtr.limit = limit
}

// SetFPS implements the Television interface. A negative value resets the FPS
// to the specification's ideal value.
func (tv *reference) SetFPS(fps float32) {
	if fps == -1 {
		fps = tv.state.spec.FramesPerSecond
	}
	tv.lmtr.setRate(fps, tv.state.spec.ScanlinesTotal)
}

// GetReqFPS implements the Television interface.
func (tv *reference) GetReqFPS() float32 {
	return tv.lmtr.requested
}

// GetActualFPS implements the Television interface. Note that FPS measurement
// still works even when frame capping is disabled.
func (tv *reference) GetActualFPS() float32 {
	return tv.lmtr.actual
}
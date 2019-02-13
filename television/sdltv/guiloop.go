package sdltv

import (
	"gopher2600/television"
	"runtime"

	"github.com/veandco/go-sdl2/sdl"
)

// guiLoop listens for SDL events and is run concurrently. critical sections
// protected by tv.guiLoopLock
func (tv *SDLTV) guiLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		ev := sdl.WaitEvent()
		switch ev := ev.(type) {

		// close window
		case *sdl.QuitEvent:
			tv.SetFeature(television.ReqSetVisibility, false)
			tv.onWindowClose.dispatch()

		case *sdl.KeyboardEvent:
			if ev.Type == sdl.KEYDOWN {
				switch ev.Keysym.Sym {
				case sdl.K_BACKQUOTE:
					tv.scr.toggleMasking()
					tv.update()
				}
			}

		case *sdl.MouseButtonEvent:
			if ev.Type == sdl.MOUSEBUTTONDOWN {
				switch ev.Button {

				case sdl.BUTTON_LEFT:
					tv.onMouseButtonLeft.dispatch()

				case sdl.BUTTON_RIGHT:
					sx, sy := tv.scr.renderer.GetScale()

					// convert X pixel value to horizpos equivalent
					// the opposite of pixelX() and also the scalining applied
					// by the SDL renderer
					if tv.scr.unmasked {
						tv.lastMouseHorizPos = int(float32(ev.X)/sx) - tv.Spec.ClocksPerHblank
					} else {
						tv.lastMouseHorizPos = int(float32(ev.X) / sx)
					}

					// convert Y pixel value to scanline equivalent
					// the opposite of pixelY() and also the scalining applied
					// by the SDL renderer
					if tv.scr.unmasked {
						tv.lastMouseScanline = int(float32(ev.Y) / sy)
					} else {
						tv.lastMouseScanline = int(float32(ev.Y)/sy) + int(tv.scr.stb.visibleTopReference)
					}

					tv.onMouseButtonRight.dispatch()
				}
			}

		case *sdl.MouseMotionEvent:
			// TODO: panning of zoomed image

		case *sdl.MouseWheelEvent:
			// TODO: zoom image

		default:
		}
	}
}

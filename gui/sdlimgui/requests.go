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
//
// *** NOTE: all historical versions of this file, as found in any
// git repository, are also covered by the licence, even when this
// notice is not present ***

package sdlimgui

import (
	"github.com/jetsetilly/gopher2600/debugger"
	"github.com/jetsetilly/gopher2600/errors"
	"github.com/jetsetilly/gopher2600/gui"
)

type featureRequest struct {
	request gui.FeatureReq
	args    []interface{}
}

// SetFeature implements gui.GUI interface
func (img *SdlImgui) SetFeature(request gui.FeatureReq, args ...interface{}) (returnedErr error) {
	img.featureReq <- featureRequest{request: request, args: args}
	return <-img.featureErr
}

// featureRequests have been handed over to the featureReq channel. we service
// any requests on that channel here.
func (img *SdlImgui) serviceFeatureRequests(request featureRequest) {
	// lazy (but clear) handling of type assertion errors
	defer func() {
		if r := recover(); r != nil {
			img.featureErr <- errors.New(errors.PanicError, "sdlImgui.serviceFeatureRequests()", r)
		}
	}()

	var err error

	switch request.request {
	case gui.ReqSetEventChan:
		img.events = request.args[0].(chan gui.Event)

	case gui.ReqSetVisibleOnStable:

	case gui.ReqSetVisibility:
		img.wm.dbgScr.setOpen(request.args[0].(bool))

	case gui.ReqToggleVisibility:
		img.wm.dbgScr.setOpen(!img.wm.dbgScr.isOpen())

	case gui.ReqSetAltColors:
		img.wm.dbgScr.useAltPixels = request.args[0].(bool)

	case gui.ReqToggleAltColors:
		img.wm.dbgScr.useAltPixels = !img.wm.dbgScr.useAltPixels

	case gui.ReqSetCropping:
		img.wm.dbgScr.setCropping(request.args[0].(bool))

	case gui.ReqToggleCropping:
		img.wm.dbgScr.setCropping(!img.wm.dbgScr.cropped)

	case gui.ReqSetOverlay:
		img.wm.dbgScr.setOverlay(request.args[0].(bool))

	case gui.ReqToggleOverlay:
		img.wm.dbgScr.setOverlay(!img.wm.dbgScr.overlay)

	case gui.ReqIncScale:
		img.setScale(0.1, true)

	case gui.ReqDecScale:
		img.setScale(-0.1, true)

	case gui.ReqSetScale:
		img.setScale(request.args[0].(float32), false)

	case gui.ReqSetPause:
		img.pause(request.args[0].(bool))

	case gui.ReqAddDebugger:
		img.lz.Dbg = request.args[0].(*debugger.Debugger)

	case gui.ReqSetPlaymode:
		err = img.setPlaymode(request.args[0].(bool))

	case gui.ReqSavePrefs:
		err = img.prefs.Save()

	default:
		err = errors.New(errors.UnsupportedGUIRequest, request)
	}

	if err == nil {
		img.featureErr <- nil
	} else {
		img.featureErr <- errors.New(errors.SDLImgui, err)
	}
}

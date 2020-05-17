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
	"fmt"
	"strings"

	"github.com/inkyblackness/imgui-go/v2"
	"github.com/jetsetilly/gopher2600/disassembly"
	"github.com/jetsetilly/gopher2600/reflection"
	"github.com/jetsetilly/gopher2600/television"
)

const winScreenTitle = "TV Screen"

type winScreen struct {
	windowManagement
	img *SdlImgui
	scr *screen

	// is screen currently pointed at
	isHovered bool

	// the tv screen has captured mouse input
	isCaptured bool

	// is the popup break menu active
	isPopup bool

	// last mouse position (adjusted to be equivalent to horizpos and scanline)
	mx, my int

	threeDigitDim imgui.Vec2
	fiveDigitDim  imgui.Vec2
}

func newWinScreen(img *SdlImgui) (managedWindow, error) {
	win := &winScreen{
		img: img,
		scr: img.screen,
	}

	return win, nil
}

func (win *winScreen) init() {
	win.threeDigitDim = imguiGetFrameDim("FFF")
	win.fiveDigitDim = imguiGetFrameDim("FFFF")
}

func (win *winScreen) destroy() {
}

func (win *winScreen) id() string {
	return winScreenTitle
}

func (win *winScreen) draw() {
	if !win.open {
		return
	}

	imgui.SetNextWindowPosV(imgui.Vec2{8, 28}, imgui.ConditionFirstUseEver, imgui.Vec2{0, 0})

	// if isCaptured flag is set then change the title and border colors of the
	// TV Screen window.
	if win.isCaptured {
		imgui.PushStyleColor(imgui.StyleColorTitleBgActive, win.img.cols.CapturedScreenTitle)
		imgui.PushStyleColor(imgui.StyleColorBorder, win.img.cols.CapturedScreenBorder)
	}

	imgui.BeginV(winScreenTitle, &win.open, imgui.WindowFlagsAlwaysAutoResize)

	// once the window has been drawn then remove any additional styling
	if win.isCaptured {
		imgui.PopStyleColorV(2)
	}

	imgui.Spacing()

	// actual display
	var w, h float32
	if win.scr.cropped {
		w = win.scr.scaledCroppedWidth()
		h = win.scr.scaledCroppedHeight()
	} else {
		w = win.scr.scaledWidth()
		h = win.scr.scaledHeight()
	}

	// overlay texture on top of screen texture
	imagePos := imgui.CursorScreenPos()
	imgui.Image(imgui.TextureID(win.scr.screenTexture), imgui.Vec2{w, h})
	if win.scr.overlay {
		imgui.SetCursorScreenPos(imagePos)
		imgui.Image(imgui.TextureID(win.scr.overlayTexture), imgui.Vec2{w, h})
	}

	// popup menu on right mouse button
	win.isPopup = imgui.BeginPopupContextItem()
	if win.isPopup {
		imgui.Text("Break")
		imgui.Separator()
		if imgui.Selectable(fmt.Sprintf("Scanline=%d", win.my)) {
			win.img.term.pushCommand(fmt.Sprintf("BREAK SL %d", win.my))
		}
		if imgui.Selectable(fmt.Sprintf("Horizpos=%d", win.mx)) {
			win.img.term.pushCommand(fmt.Sprintf("BREAK HP %d", win.mx))
		}
		if imgui.Selectable(fmt.Sprintf("Scanline=%d & Horizpos=%d", win.my, win.mx)) {
			win.img.term.pushCommand(fmt.Sprintf("BREAK SL %d & HP %d", win.my, win.mx))
		}
		imgui.EndPopup()
		win.isPopup = false
	}

	// if mouse is hovering over the image. note that if popup menu is active
	// then imgui.IsItemHovered() is false by definition
	win.isHovered = imgui.IsItemHovered()
	if win.isHovered {
		// *** CRIT SECTION
		win.scr.crit.section.RLock()

		// get mouse position and transform it so it relates to the underlying
		// image
		mp := imgui.MousePos().Minus(imagePos)
		mp.X = mp.X / win.scr.scaledCroppedWidth()
		mp.Y = mp.Y / win.scr.scaledCroppedHeight()

		imageSz := win.scr.crit.cropPixels.Bounds().Size()

		if win.scr.cropped {
			mp.X *= float32(imageSz.X)
			mp.X += float32(television.HorizClksHBlank)
			mp.Y *= float32(imageSz.Y)
			mp.Y += float32(win.scr.crit.topScanline)
		} else {
			mp.X *= float32(imageSz.X)
			mp.Y *= float32(imageSz.Y)
		}

		win.mx = int(mp.X)
		win.my = int(mp.Y)

		// get reflection information
		var res reflection.ResultWithBank
		if win.mx < len(win.scr.crit.reflection) && win.my < len(win.scr.crit.reflection[win.mx]) {
			res = win.scr.crit.reflection[win.mx][win.my]
		}

		win.scr.crit.section.RUnlock()
		// *** CRIT SECTION END ***

		// present tooltip showing pixel coords and CPU state
		if !win.isCaptured {
			fmtRes, _ := win.img.lz.Dsm.FormatResult(res.Bank, res.Res, disassembly.EntryLevelBlessed)
			if fmtRes.Address != "" {
				imgui.BeginTooltip()
				imgui.Text(fmt.Sprintf("Scanline: %d", win.my))
				imgui.Text(fmt.Sprintf("Horiz Pos: %d", win.mx-television.HorizClksHBlank))

				imgui.PushStyleColor(imgui.StyleColorText, win.img.cols.DisasmBreakAddress)
				if win.img.lz.Cart.NumBanks > 1 {
					imgui.Text(fmt.Sprintf("%s [bank %d]", fmtRes.Address, res.Bank))
				} else {
					imgui.Text(fmtRes.Address)
				}
				imgui.PopStyleColor()

				imgui.PushStyleColor(imgui.StyleColorText, win.img.cols.DisasmMnemonic)
				imgui.Text(fmtRes.Mnemonic)
				imgui.PopStyleColor()

				if fmtRes.Operand != "" {
					imgui.SameLine()
					imgui.PushStyleColor(imgui.StyleColorText, win.img.cols.DisasmOperand)
					imgui.Text(fmtRes.Operand)
					imgui.PopStyleColor()
				}

				imgui.EndTooltip()
			}
		}
	}

	// tv status line
	imguiText("Frame:")
	imguiText(fmt.Sprintf("%-4d", win.img.lz.TV.Frame))
	imgui.SameLineV(0, 15)
	imguiText("Scanline:")
	imguiText(fmt.Sprintf("%-4d", win.img.lz.TV.Scanline))
	imgui.SameLineV(0, 15)
	imguiText("Horiz Pos:")
	imguiText(fmt.Sprintf("%-4d", win.img.lz.TV.HP))

	// fps indicator
	imgui.SameLineV(0, 20)
	imgui.AlignTextToFramePadding()
	if win.img.paused {
		imguiText("no fps")
	} else {
		if win.img.lz.TV.ReqFPS < 1.0 {
			imguiText("< 1 fps")
		} else {
			imguiText(fmt.Sprintf("%03.1f fps", win.img.lz.TV.AcutalFPS))
		}
	}

	// include tv signal information
	imgui.SameLineV(0, 20)
	signal := strings.Builder{}
	if win.img.lz.TV.LastSignal.VSync {
		signal.WriteString("VSYNC ")
	}
	if win.img.lz.TV.LastSignal.VBlank {
		signal.WriteString("VBLANK ")
	}
	if win.img.lz.TV.LastSignal.CBurst {
		signal.WriteString("CBURST ")
	}
	if win.img.lz.TV.LastSignal.HSync {
		signal.WriteString("HSYNC ")
	}
	imgui.Text(signal.String())

	// display toggles
	imgui.Spacing()
	imgui.Checkbox("Debug Colours", &win.scr.useAltPixels)
	imgui.SameLine()
	if imgui.Checkbox("Cropping", &win.scr.cropped) {
		win.scr.setCropping(win.scr.cropped)
	}
	imgui.SameLine()
	imgui.Checkbox("Pixel Perfect", &win.scr.pixelPerfect)
	imgui.SameLine()
	imgui.Checkbox("Overlay", &win.scr.overlay)

	imgui.End()
}

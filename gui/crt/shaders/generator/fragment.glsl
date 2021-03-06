uniform int ImageType;
uniform int DrawMode; // 0 == running; 1 = show drawing; 2 = "goto coords"
uniform int Cropped; // false <= 0; true > 0
uniform vec2 ScreenDim;
uniform vec2 CropScreenDim;
uniform float ScalingX;
uniform float ScalingY;
uniform float LastX; 
uniform float LastY;
uniform float Hblank;
uniform float TopScanline;
uniform float BotScanline;
uniform float AnimTime;
uniform float RandSeed;

uniform int CRT;
uniform float InputGamma; 
uniform float OutputGamma; 
uniform int Mask;
uniform int Scanlines;
uniform int Noise;
uniform float MaskBrightness;
uniform float ScanlinesBrightness;
uniform float NoiseLevel;
uniform int Vignette;
uniform int MaskScanlineScaling;

uniform sampler2D Texture;
in vec2 Frag_UV;
in vec4 Frag_Color;
out vec4 Out_Color;

bool isNearEqual(float x, float y, float epsilon)
{
	return abs(x - y) <= epsilon;
}

const float cursorSize = 1.0;

// Gold Noise taken from: https://www.shadertoy.com/view/ltB3zD
// Coprighted to dcerisano@standard3d.com not sure of the licence

// Gold Noise ©2015 dcerisano@standard3d.com
// - based on the Golden Ratio
// - uniform normalized distribution
// - fastest static noise generator function (also runs at low precision)
float PHI = 1.61803398874989484820459;  // Φ = Golden Ratio   
float gold_noise(in vec2 xy){
	return fract(tan(distance(xy*PHI, xy)*RandSeed)*xy.x);
}

void main()
{
	// imgui texture
	if (ImageType == 0) {
		Out_Color = vec4(Frag_Color.rgb, Frag_Color.a * texture(Texture, Frag_UV.st).r);
		return;
	}

	// if this is the overlay texture then we're done
	if (ImageType == 2) {
		Out_Color = Frag_Color * texture(Texture, Frag_UV.st);
		return;
	}

	vec2 coords = Frag_UV.xy;

	// bring geometry values into workable range
	float hblank;
	float topScanline;
	float botScanline;
	float lastX;
	float lastY;

	// the size of one texel (used for painting and cursor positioning)
	float texelX;
	float texelY;

	// debug tv screen texture
	if (ImageType == 1) {
		if (Cropped > 0) {
			texelX = ScalingX / CropScreenDim.x;
			texelY = ScalingY / CropScreenDim.y;
			hblank = Hblank / CropScreenDim.x;
			lastX = LastX / CropScreenDim.x;
			topScanline = 0;
			botScanline = (BotScanline - TopScanline) / CropScreenDim.y;

			// the LastY coordinate refers to the full-frame scanline. the cropped
			// texture however counts from zero at the visible edge so we need to
			// adjust the lastY value by the TopScanline value.
			//
			// note that there's no need to do this for LastX because the
			// horizontal position is counted from -68 in all instances.
			lastY = (LastY - TopScanline) / CropScreenDim.y;
		} else {
			texelX = ScalingX / ScreenDim.x;
			texelY = ScalingY / ScreenDim.y;
			hblank = Hblank / ScreenDim.x;
			topScanline = TopScanline / ScreenDim.y;
			botScanline = BotScanline / ScreenDim.y;
			lastX = LastX / ScreenDim.x;
			lastY = LastY / ScreenDim.y;
		}

		// pixels are texels without the scaling applied
		float pixelX = texelX / ScalingX;
		float pixelY = texelY / ScalingY;

		// if the entire frame is being shown then plot the screen guides
		if (Cropped < 0) {
			if (isNearEqual(coords.x, hblank, pixelX) ||
			   isNearEqual(coords.y, topScanline, pixelY) ||
			   isNearEqual(coords.y, botScanline, pixelY)) {
				Out_Color.r = 1.0;
				Out_Color.g = 1.0;
				Out_Color.b = 1.0;
				Out_Color.a = 0.2;
				return;
			}
		}

		// when DrawMode is greater than 0 then there's some additional image
		// processing we need to perform
		if (DrawMode == 1) {
			// draw cursor if pixel is at the last x/y position
			if (lastY >= 0 && lastX >= 0) {
				if (isNearEqual(coords.y, lastY+texelY, cursorSize*texelY) && isNearEqual(coords.x, lastX+texelX, cursorSize*texelX/2)) {
					Out_Color.r = 1.0;
					Out_Color.g = 1.0;
					Out_Color.b = 1.0;
					Out_Color.a = AnimTime;
					return;
				}
			}

			// draw off-screen cursor for HBLANK
			if (lastX < 0 && isNearEqual(coords.y, lastY+texelY, cursorSize*texelY) && isNearEqual(coords.x, 0, cursorSize*texelX/2)) {
				Out_Color.r = 1.0;
				Out_Color.a = AnimTime;
				return;
			}

			// for cropped screens there are a few more conditions that we need to
			// consider for drawing an off-screen cursor
			if (Cropped > 0) {
				// when VBLANK is active but HBLANK is off
				if (isNearEqual(coords.x, lastX, cursorSize * texelX/2)) {
					// top of screen
					if (lastY < 0 && isNearEqual(coords.y, 0, cursorSize*texelY)) {
						Out_Color.r = 1.0;
						Out_Color.a = AnimTime;
						return;
					}
				
					// bottom of screen (knocking a pixel off the scanline
					// boundary check to make sure the cursor is visible)
					if (lastY > botScanline-pixelY && isNearEqual(coords.y, botScanline, cursorSize*texelY)) {
						Out_Color.r = 1.0;
						Out_Color.a = AnimTime;
						return;
					}
				}

				// when HBLANK and VBLANK are both active
				if (lastX < 0 && isNearEqual(coords.x, 0, cursorSize*texelX/2)) {
					// top/left corner of screen
					if (lastY < 0 && isNearEqual(coords.y, 0, cursorSize*texelY)) {
						Out_Color.r = 1.0;
						Out_Color.a = AnimTime;
						return;
					}

					// bottom/left corner of screen (knocking a pixel off the
					// scanline boundary check to make sure the cursor is
					// visible)
					if (lastY > botScanline-pixelY && isNearEqual(coords.y, botScanline, cursorSize*texelY)) {
						Out_Color.r = 1.0;
						Out_Color.a = AnimTime;
						return;
					}
				}
			}

			// painting effect draws pixels with faded alpha if lastX and lastY
			// are less than rendering coords.
			//
			// as a special case, we ignore the first scanline and do not fade the
			// previous image on a brand new frame. note that we're using the
			// unadjusted LastY value for this
			if (LastY > 0) {
				if (coords.y > lastY+texelY || (isNearEqual(coords.y, lastY+texelY, texelY) && coords.x > lastX+texelX)) {
					Out_Color = Frag_Color * texture(Texture, Frag_UV.st);
					Out_Color.a = 0.5;
					return;
				}
			}
		}

		// special handling for "Goto Coords" mode. the effect we want is for
		// the selected coords to be obvious immediately. we don't want to see
		// any screen drawing but we do want the alpha fade.
		if (DrawMode == 2) {
			if (coords.y > lastY+texelY || (isNearEqual(coords.y, lastY+texelY, texelY) && coords.x > lastX+texelX)) {
				Out_Color = Frag_Color * texture(Texture, Frag_UV.st);
				Out_Color.a = 0.5;
				return;
			}
		}

	} else {
		texelX = ScalingX / CropScreenDim.x;
		texelY = ScalingY / CropScreenDim.y;
	}

	// set basic color
	Out_Color = Frag_Color * texture(Texture, Frag_UV.st);

	// if pixel-perfect	rendering is selected then there's nothing much more to do
	if (CRT == 0 && ImageType != 4) {
		return;
	}

	// only apply CRT effects on the "cropped" area of the screen. we can think
	// of the cropped area as the "play" area
	if (Cropped < 0 && (coords.x < hblank || coords.y < topScanline || coords.y > botScanline)) {
		Out_Color = Frag_Color * texture(Texture, Frag_UV.st);
		return;
	}

	// basic CRT effects
	// -----------------
	// some ideas taken from the crt-pi.glsl shader which is part of lib-retro
	//
	// https://github.com/libretro/glsl-shaders/blob/master/crt/shaders/crt-pi.glsl
	
	int scaling = MaskScanlineScaling + 1;	
	
	// noise
	if (Noise == 1) {
		float r;
		r = gold_noise(gl_FragCoord.xy);
		if (r < 0.33) {
			Out_Color.r *= max(1.0-NoiseLevel, gold_noise(gl_FragCoord.xy));
		} else if (r < 0.66) {
			Out_Color.g *= max(1.0-NoiseLevel, gold_noise(gl_FragCoord.xy));
		} else {
			Out_Color.b *= max(1.0-NoiseLevel, gold_noise(gl_FragCoord.xy));
		}
	}

	// input gamma
	Out_Color.rgb = pow(Out_Color.rgb, vec3(InputGamma));
	
	// masking
	if (Mask == 1) {
		vec3 mask;

		/* if (fract(gl_FragCoord.x * 0.5) < 0.5) { */

		float oneCol = gl_FragCoord.x/gl_FragCoord.x;
		if ( isNearEqual(mod(gl_FragCoord.x, scaling*oneCol), 0.0, oneCol) ) {
			mask = vec3(MaskBrightness, 1.0, MaskBrightness);
		} else {
			mask = vec3(1.0, MaskBrightness, 1.0);
		}
		Out_Color = vec4(Out_Color.rgb * mask, 1.0);
	}

	// scanline effect
	if (Scanlines == 1) {
		/* if (fract(gl_FragCoord.y * 0.5) < 0.5) { */

		float oneLine = gl_FragCoord.y/gl_FragCoord.y;
		if ( isNearEqual(mod(gl_FragCoord.y, scaling*oneLine), 0.0, oneLine) ) {
			Out_Color.a = Out_Color.a * ScanlinesBrightness;
		}
	}

	// output gamma
	Out_Color.rgb = pow(Out_Color.rgb, vec3(1.0/OutputGamma));

	// vignette effect
	if (Vignette == 1) {
		float vignette;
		if (Cropped > 0) {
			vignette = (10.0*coords.x*coords.y*(1.0-coords.x)*(1.0-coords.y));
		} else {
			// f is used to factor the vignette value. In the "cropped" branch we
			// use a factor value of 10. to visually mimic the vignette effect a
			// value of about 25 is required (using Pitfall as a template). I don't
			// understand this well enough to say for sure what the relationship
			// between 25 and 10 is, but the following ratio between
			// cropped/uncropped widths gives us a value of 23.5
			float f =ScreenDim.x/(ScreenDim.x-CropScreenDim.x);
			vignette = (f*(coords.x-hblank)*(coords.y-topScanline)*(1.0-coords.x)*(1.0-coords.y));
		}
		Out_Color.rgb *= pow(vignette, 0.10) * 1.2;
	}
}

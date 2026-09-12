package utils

/*
#include "webp_animation_decoder.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const maxDecodedAnimationBytes = 256 << 20

type decodedWebPAnimation struct {
	width      int
	height     int
	frames     [][]byte
	timestamps []int
}

func decodeWebPAnimation(inputData []byte) (*decodedWebPAnimation, error) {
	if len(inputData) == 0 {
		return nil, fmt.Errorf("animated WebP input is empty")
	}

	input := C.CBytes(inputData)
	if input == nil {
		return nil, fmt.Errorf("failed to allocate animated WebP input")
	}
	defer C.free(input)

	decoder := C.watgbridge_webp_anim_decoder_new(
		(*C.uint8_t)(input), C.size_t(len(inputData)))
	if decoder == nil {
		return nil, fmt.Errorf("libwebp could not initialize the animation decoder")
	}
	defer C.WebPAnimDecoderDelete(decoder)

	var info C.WebPAnimInfo
	if C.WebPAnimDecoderGetInfo(decoder, &info) == 0 {
		return nil, fmt.Errorf("libwebp could not read animation metadata")
	}

	width, height := int(info.canvas_width), int(info.canvas_height)
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil, fmt.Errorf("invalid animated WebP dimensions %dx%d", width, height)
	}
	frameBytes := width * height * 4
	if frameBytes <= 0 || frameBytes > maxDecodedAnimationBytes {
		return nil, fmt.Errorf("animated WebP frame is too large: %d bytes", frameBytes)
	}

	animation := &decodedWebPAnimation{width: width, height: height}
	totalBytes := 0
	for C.WebPAnimDecoderHasMoreFrames(decoder) != 0 {
		var (
			frame     *C.uint8_t
			timestamp C.int
		)
		if C.WebPAnimDecoderGetNext(decoder, &frame, &timestamp) == 0 || frame == nil {
			return nil, fmt.Errorf("libwebp failed while decoding animation frame %d", len(animation.frames)+1)
		}
		if int(timestamp) <= 0 || (len(animation.timestamps) > 0 && int(timestamp) <= animation.timestamps[len(animation.timestamps)-1]) {
			return nil, fmt.Errorf("animated WebP has invalid frame timestamp %d", int(timestamp))
		}
		if totalBytes+frameBytes > maxDecodedAnimationBytes {
			return nil, fmt.Errorf("decoded animated WebP exceeds %d MiB memory limit", maxDecodedAnimationBytes>>20)
		}

		animation.frames = append(animation.frames, C.GoBytes(unsafe.Pointer(frame), C.int(frameBytes)))
		animation.timestamps = append(animation.timestamps, int(timestamp))
		totalBytes += frameBytes

		// Telegram video stickers may be at most three seconds long. The frame
		// returned here covers the cutoff, so no later frame can affect output.
		if int(timestamp) >= telegramVideoStickerMaxDurationMS {
			break
		}
	}

	if len(animation.frames) == 0 {
		return nil, fmt.Errorf("animated WebP contains no frames")
	}
	return animation, nil
}

#include "webp_animation_decoder.h"

#include <stddef.h>

extern int WebPAnimDecoderOptionsInitInternal(WebPAnimDecoderOptions*, int);
extern WebPAnimDecoder* WebPAnimDecoderNewInternal(
    const WebPData*, const WebPAnimDecoderOptions*, int);

enum {
  WATGBRIDGE_WEBP_MODE_RGBA = 1,
  WATGBRIDGE_WEBP_DEMUX_ABI_VERSION = 0x0107
};

WebPAnimDecoder* watgbridge_webp_anim_decoder_new(const uint8_t* bytes,
                                                   size_t size) {
  WebPData data;
  WebPAnimDecoderOptions options;
  data.bytes = bytes;
  data.size = size;
  if (!WebPAnimDecoderOptionsInitInternal(
          &options, WATGBRIDGE_WEBP_DEMUX_ABI_VERSION)) {
    return NULL;
  }
  options.color_mode = WATGBRIDGE_WEBP_MODE_RGBA;
  options.use_threads = 1;
  return WebPAnimDecoderNewInternal(
      &data, &options, WATGBRIDGE_WEBP_DEMUX_ABI_VERSION);
}

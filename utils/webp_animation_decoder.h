#ifndef WATGBRIDGE_WEBP_ANIMATION_DECODER_H_
#define WATGBRIDGE_WEBP_ANIMATION_DECODER_H_

#include <stddef.h>
#include <stdint.h>

typedef struct {
  const uint8_t* bytes;
  size_t size;
} WebPData;

typedef struct WebPAnimDecoder WebPAnimDecoder;

typedef struct {
  int color_mode;
  int use_threads;
  uint32_t padding[7];
} WebPAnimDecoderOptions;

typedef struct {
  uint32_t canvas_width;
  uint32_t canvas_height;
  uint32_t loop_count;
  uint32_t bgcolor;
  uint32_t frame_count;
  uint32_t pad[4];
} WebPAnimInfo;

int WebPAnimDecoderGetInfo(const WebPAnimDecoder*, WebPAnimInfo*);
int WebPAnimDecoderGetNext(WebPAnimDecoder*, uint8_t**, int*);
int WebPAnimDecoderHasMoreFrames(const WebPAnimDecoder*);
void WebPAnimDecoderDelete(WebPAnimDecoder*);

WebPAnimDecoder* watgbridge_webp_anim_decoder_new(const uint8_t* bytes,
                                                   size_t size);

#endif

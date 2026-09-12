package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"

	// "image/color"
	"image/draw"
	"os"
	"os/exec"
	"strconv"

	"watgbridge/state"

	"github.com/watgbridge/tgsconverter/libtgsconverter"
	"github.com/watgbridge/webp"
	"go.uber.org/zap"
)

func TGSConvertToWebp(tgsStickerData []byte, updateId int64) ([]byte, error) {
	logger := state.State.Logger
	defer logger.Sync()
	cacheVariant := fmt.Sprintf("tgs-to-webp-v1:%s:%s",
		state.State.Config.WhatsApp.StickerMetadata.PackName,
		state.State.Config.WhatsApp.StickerMetadata.AuthorName,
	)
	if cachedData, found := readStickerCache(cacheVariant, "webp", tgsStickerData); found {
		return cachedData, nil
	}

	opt := libtgsconverter.NewConverterOptions()
	opt.SetExtension("webp")
	var (
		quality float32 = 100
		fps     uint    = 30
	)
	for quality > 2 && fps > 5 {
		logger.Debug("trying to convert tgs to webp",
			zap.Int64("updateId", updateId),
			zap.Float32("quality", quality),
			zap.Uint("fps", fps),
		)
		opt.SetFPS(fps)
		opt.SetWebpQuality(quality)
		webpStickerData, err := libtgsconverter.ImportFromData(tgsStickerData, opt)
		if err != nil {
			logger.Debug("TGS conversion attempt failed",
				zap.Int64("updateId", updateId),
				zap.Float32("quality", quality),
				zap.Uint("fps", fps),
				zap.Error(err),
			)
			return nil, err
		}

		finalOutput := webpStickerData
		if outputDataWithExif, exifErr := WebpWriteExifData(webpStickerData); exifErr == nil {
			finalOutput = outputDataWithExif
		} else {
			logger.Debug("failed to add EXIF data to TGS conversion attempt",
				zap.Int64("updateId", updateId),
				zap.Float32("quality", quality),
				zap.Uint("fps", fps),
				zap.Error(exifErr),
			)
		}

		if len(finalOutput) < 1024*1024 {
			writeStickerCache(cacheVariant, "webp", tgsStickerData, finalOutput)
			return finalOutput, nil
		}
		logger.Debug("TGS conversion attempt exceeds WhatsApp size limit",
			zap.Int64("updateId", updateId),
			zap.Float32("quality", quality),
			zap.Uint("fps", fps),
			zap.Int("bytes", len(finalOutput)),
			zap.Int("limit_bytes", 1024*1024),
		)
		quality /= 2
		fps = uint(float32(fps) / 1.5)
	}
	return nil, fmt.Errorf("sticker has a lot of data which cannot be handled by WhatsApp")
}

func WebmConvertToWebp(webmStickerData []byte, updateId int64) ([]byte, error) {
	logger := state.State.Logger
	defer logger.Sync()
	cacheVariant := fmt.Sprintf("webm-to-webp-v2-riff:%s:%s",
		state.State.Config.WhatsApp.StickerMetadata.PackName,
		state.State.Config.WhatsApp.StickerMetadata.AuthorName,
	)
	if cachedData, found := readStickerCache(cacheVariant, "webp", webmStickerData); found {
		return cachedData, nil
	}

	ffmpegExec := state.State.Config.FfmpegExecutable
	if ffmpegExec == "" {
		ffmpegExec = "ffmpeg"
	}

	var (
		quality = 75
		fps     = 15
	)

	for quality >= 30 && fps >= 8 {
		vf := fmt.Sprintf("fps=%d,scale=512:512:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=512:512:(ow-iw)/2:(oh-ih)/2:color=#00000000,format=rgba", fps)
		cmd := exec.Command(ffmpegExec,
			"-i", "-",
			"-c:v", "libwebp",
			"-loop", "0",
			"-preset", "default",
			"-an",
			"-vsync", "0",
			"-quality", strconv.Itoa(quality),
			"-compression_level", "6",
			"-vf", vf,
			"-f", "webp",
			"-",
		)

		var outputBuf, stderr bytes.Buffer
		cmd.Stdin = bytes.NewReader(webmStickerData)
		cmd.Stdout = &outputBuf
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			logger.Debug("ffmpeg webm conversion attempt failed",
				zap.Int64("updateId", updateId),
				zap.Int("quality", quality),
				zap.Int("fps", fps),
				zap.Error(err),
				zap.String("stderr", stderr.String()),
			)
			quality -= 15
			fps -= 2
			continue
		}

		outputBytes, repairErr := repairStreamingWebP(outputBuf.Bytes())
		if repairErr != nil {
			logger.Debug("ffmpeg webm conversion attempt produced invalid WebP",
				zap.Int64("updateId", updateId),
				zap.Int("quality", quality),
				zap.Int("fps", fps),
				zap.Error(repairErr),
			)
			quality -= 15
			fps -= 2
			continue
		}

		finalOutput, exifErr := WebpWriteExifData(outputBytes)
		if exifErr != nil {
			logger.Debug("failed to add EXIF data to WEBM conversion attempt",
				zap.Int64("updateId", updateId),
				zap.Int("quality", quality),
				zap.Int("fps", fps),
				zap.Error(exifErr),
			)
			quality -= 15
			fps -= 2
			continue
		}

		if len(finalOutput) <= 500*1024 {
			writeStickerCache(cacheVariant, "webp", webmStickerData, finalOutput)
			return finalOutput, nil
		}
		logger.Debug("WEBM conversion attempt exceeds WhatsApp size limit",
			zap.Int64("updateId", updateId),
			zap.Int("quality", quality),
			zap.Int("fps", fps),
			zap.Int("bytes", len(finalOutput)),
			zap.Int("limit_bytes", 500*1024),
		)

		quality -= 15
		fps -= 2
	}

	return nil, fmt.Errorf("webm sticker could not be converted under WhatsApp size limit")
}

func WebpImagePad(inputData []byte, wPad, hPad int, updateId int64) ([]byte, error) {
	logger := state.State.Logger
	defer logger.Sync()
	cacheVariant := fmt.Sprintf("webp-pad-v1:%d:%d:%s:%s",
		wPad,
		hPad,
		state.State.Config.WhatsApp.StickerMetadata.PackName,
		state.State.Config.WhatsApp.StickerMetadata.AuthorName,
	)
	if cachedData, found := readStickerCache(cacheVariant, "webp", inputData); found {
		return cachedData, nil
	}

	inputImage, err := webp.DecodeRGBA(inputData)
	if err != nil {
		logger.Debug("failed to decode WebP before padding",
			zap.Int64("updateId", updateId),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to decode web image: %w", err)
	}

	var (
		wOffset = wPad / 2
		hOffset = hPad / 2
	)

	outputWidth := inputImage.Bounds().Dx() + wPad
	outputHeight := inputImage.Bounds().Dy() + hPad

	outputImage := image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	draw.Draw(outputImage, image.Rect(wOffset, hOffset, outputWidth-wOffset, outputHeight-hOffset), inputImage, image.Point{}, draw.Src)

	outputBytes, err := webp.EncodeRGBA(outputImage, 100)
	if err != nil {
		logger.Debug("failed to encode padded WebP",
			zap.Int64("updateId", updateId),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to encode padded data into Webp: %w", err)
	}

	if outputData, err := WebpWriteExifData(outputBytes); err == nil {
		writeStickerCache(cacheVariant, "webp", inputData, outputData)
		return outputData, nil
	} else {
		logger.Debug("failed to add EXIF data to padded WebP",
			zap.Int64("updateId", updateId),
			zap.Error(err),
		)
	}

	writeStickerCache(cacheVariant, "webp", inputData, outputBytes)
	return outputBytes, nil
}

// Fallback function to convert to GIF if WEBM conversion fails
func AnimatedWebpConvertToGif(inputData []byte, updateId string) ([]byte, error) {
	logger := state.State.Logger
	defer logger.Sync()
	const cacheVariant = "animated-webp-to-gif-v2"
	if cachedData, found := readStickerCache(cacheVariant, "gif", inputData); found {
		return cachedData, nil
	}

	cmd := exec.Command("convert",
		"webp:-",
		"-loop", "0",
		"-dispose", "previous",
		"gif:-",
	)

	var outputBuf, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(inputData)
	cmd.Stdout = &outputBuf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("convert command failed",
			zap.Error(err),
			zap.String("stderr", stderr.String()),
		)
		return nil, err
	}

	outputData := outputBuf.Bytes()
	if len(outputData) < 6 || (!bytes.Equal(outputData[:6], []byte("GIF87a")) && !bytes.Equal(outputData[:6], []byte("GIF89a"))) {
		return nil, fmt.Errorf("convert produced invalid GIF output")
	}
	writeStickerCache(cacheVariant, "gif", inputData, outputData)
	return outputData, nil
}

func repairStreamingWebP(inputData []byte) ([]byte, error) {
	if len(inputData) < 12 || !bytes.Equal(inputData[:4], []byte("RIFF")) || !bytes.Equal(inputData[8:12], []byte("WEBP")) {
		return nil, fmt.Errorf("output does not contain a WebP RIFF header")
	}
	actualSize := len(inputData) - 8
	if uint64(actualSize) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("WebP output is too large for a RIFF container")
	}
	declaredSize := binary.LittleEndian.Uint32(inputData[4:8])
	if declaredSize == uint32(actualSize) {
		return inputData, nil
	}
	if declaredSize != 0 {
		return nil, fmt.Errorf("WebP RIFF size is %d, actual size is %d", declaredSize, actualSize)
	}

	repaired := bytes.Clone(inputData)
	binary.LittleEndian.PutUint32(repaired[4:8], uint32(actualSize))
	return repaired, nil
}

func WebpWriteExifData(inputData []byte) ([]byte, error) {
	var (
		cfg           = state.State.Config
		logger        = state.State.Logger
		startingBytes = []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, 0x01, 0x00, 0x41, 0x57, 0x07, 0x00}
		endingBytes   = []byte{0x16, 0x00, 0x00, 0x00}
		b             bytes.Buffer
	)
	defer logger.Sync()

	exifFile, err := os.CreateTemp("", "raw*.exif")
	if err != nil {
		return nil, fmt.Errorf("failed to create exif file: %w", err)
	}
	defer os.Remove(exifFile.Name())

	inputFile, err := os.CreateTemp("", "input")
	if err != nil {
		return nil, fmt.Errorf("failed to create input data file: %w", err)
	}
	defer os.Remove(inputFile.Name())

	if _, err := b.Write(startingBytes); err != nil {
		return nil, err
	}

	jsonData := map[string]any{
		"sticker-pack-id":        "watgbridge.akshettrj.com.github.",
		"sticker-pack-name":      cfg.WhatsApp.StickerMetadata.PackName,
		"sticker-pack-publisher": cfg.WhatsApp.StickerMetadata.AuthorName,
		"emojis":                 []string{"😀"},
	}
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, err
	}

	jsonLength := (uint32)(len(jsonBytes))
	lenBuffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuffer, jsonLength)

	if _, err := b.Write(lenBuffer); err != nil {
		return nil, err
	}
	if _, err := b.Write(endingBytes); err != nil {
		return nil, err
	}
	if _, err := b.Write(jsonBytes); err != nil {
		return nil, err
	}

	if _, err := exifFile.Write(b.Bytes()); err != nil {
		return nil, err
	}
	exifFile.Close()

	if _, err := inputFile.Write(inputData); err != nil {
		return nil, err
	}
	inputFile.Close()

	cmd := exec.Command("webpmux",
		"-set", "exif", exifFile.Name(),
		inputFile.Name(),
		"-o", "-",
	)

	var outputBuf, stderr bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("failed to run webpmux command",
			zap.Error(err),
			zap.String("stderr", stderr.String()),
		)
		return nil, err
	}

	outputData, err := repairStreamingWebP(outputBuf.Bytes())
	if err != nil {
		logger.Debug("webpmux produced invalid WebP output", zap.Error(err))
		return nil, err
	}
	return outputData, nil
}

func GenerateVideoThumbnail(videoData []byte) ([]byte, error) {
	logger := state.State.Logger
	defer logger.Sync()
	const cacheVariant = "media-thumbnail-jpeg-v1"
	if cachedData, found := readStickerCache(cacheVariant, "jpg", videoData); found {
		return cachedData, nil
	}

	ffmpegExec := state.State.Config.FfmpegExecutable
	if ffmpegExec == "" {
		ffmpegExec = "ffmpeg"
	}

	cmd := exec.Command(ffmpegExec,
		"-i", "-",
		"-vframes", "1",
		"-vf", "scale=160:160:force_original_aspect_ratio=decrease",
		"-f", "image2",
		"-c:v", "mjpeg",
		"-q:v", "8",
		"-y",
		"-",
	)

	var outputBuf, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(videoData)
	cmd.Stdout = &outputBuf
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("ffmpeg thumbnail generation failed",
			zap.Error(err),
			zap.String("stderr", stderr.String()),
		)
		return nil, err
	}

	outputData := outputBuf.Bytes()
	if len(outputData) < 4 || outputData[0] != 0xff || outputData[1] != 0xd8 || outputData[len(outputData)-2] != 0xff || outputData[len(outputData)-1] != 0xd9 {
		logger.Debug("ffmpeg thumbnail generation produced invalid JPEG output",
			zap.Int("bytes", len(outputData)),
		)
		return nil, fmt.Errorf("ffmpeg produced invalid JPEG thumbnail output")
	}
	writeStickerCache(cacheVariant, "jpg", videoData, outputData)
	return outputData, nil
}

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"watgbridge/state"

	"go.uber.org/zap"
)

const (
	telegramVideoStickerMaxBytes      = 256 * 1024
	telegramVideoStickerMaxDurationMS = 3000
	telegramVideoStickerMaxFPS        = 30
)

var telegramVideoStickerCRFs = [...]int{30, 35, 40, 45, 50, 55, 60, 63}

func AnimatedWebpConvertToWebm(inputData []byte, updateId string) ([]byte, error) {
	const cacheVariant = "animated-webp-to-webm-v3-seekable"
	if cachedData, found := readStickerCache(cacheVariant, "webm", inputData); found {
		return cachedData, nil
	}

	logger := state.State.Logger
	defer logger.Sync()

	animation, err := decodeWebPAnimation(inputData)
	if err != nil {
		logger.Debug("failed to decode animated WebP",
			zap.String("updateId", updateId),
			zap.Error(err),
		)
		return nil, err
	}

	durationMS := animation.timestamps[len(animation.timestamps)-1]
	if durationMS > telegramVideoStickerMaxDurationMS {
		durationMS = telegramVideoStickerMaxDurationMS
	}
	fps := chooseAnimationFrameRate(animation.timestamps, durationMS)
	frameIndexes := resampleAnimationFrames(animation.timestamps, durationMS, fps)
	if len(frameIndexes) == 0 {
		return nil, fmt.Errorf("animated WebP produced no frames after resampling")
	}

	logger.Debug("converting animated WebP directly to VP9",
		zap.String("updateId", updateId),
		zap.Int("source_frames", len(animation.frames)),
		zap.Int("output_frames", len(frameIndexes)),
		zap.Int("fps", fps),
		zap.Int("duration_ms", durationMS),
	)

	var lastErr error
	for _, crf := range telegramVideoStickerCRFs {
		outputData, stderr, encodeErr := encodeRGBAAnimationToWebM(animation, frameIndexes, fps, crf)
		if encodeErr != nil {
			lastErr = fmt.Errorf("ffmpeg animated WebP conversion at CRF %d failed: %w", crf, encodeErr)
			logger.Debug("animated WebP VP9 conversion attempt failed",
				zap.String("updateId", updateId),
				zap.Int("crf", crf),
				zap.Error(encodeErr),
				zap.String("stderr", stderr),
			)
			continue
		}
		if len(outputData) == 0 {
			lastErr = fmt.Errorf("ffmpeg produced empty VP9 output at CRF %d", crf)
			logger.Debug("animated WebP VP9 conversion attempt produced empty output",
				zap.String("updateId", updateId),
				zap.Int("crf", crf),
			)
			continue
		}
		if len(outputData) > telegramVideoStickerMaxBytes {
			lastErr = fmt.Errorf("VP9 output exceeds Telegram's size limit")
			logger.Debug("animated WebP VP9 conversion attempt exceeds Telegram size limit",
				zap.String("updateId", updateId),
				zap.Int("crf", crf),
				zap.Int("bytes", len(outputData)),
				zap.Int("limit_bytes", telegramVideoStickerMaxBytes),
			)
			continue
		}

		if validationErr := validateTelegramVideoSticker(outputData, len(frameIndexes), fps); validationErr != nil {
			lastErr = validationErr
			logger.Debug("animated WebP VP9 output failed validation",
				zap.String("updateId", updateId),
				zap.Int("crf", crf),
				zap.Error(validationErr),
			)
			continue
		}

		writeStickerCache(cacheVariant, "webm", inputData, outputData)
		return outputData, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("animated WebP could not be converted to a Telegram video sticker")
	}
	return nil, lastErr
}

func chooseAnimationFrameRate(timestamps []int, durationMS int) int {
	if durationMS <= 0 {
		return telegramVideoStickerMaxFPS
	}

	boundaries := make([]int, 0, len(timestamps))
	previousTimestamp := 0
	minimumDelay := durationMS
	maximumDelay := 0
	for _, timestamp := range timestamps {
		if timestamp >= durationMS {
			boundaries = append(boundaries, durationMS)
			timestamp = durationMS
		} else {
			boundaries = append(boundaries, timestamp)
		}
		delay := timestamp - previousTimestamp
		if delay < minimumDelay {
			minimumDelay = delay
		}
		if delay > maximumDelay {
			maximumDelay = delay
		}
		previousTimestamp = timestamp
		if timestamp == durationMS {
			break
		}
	}
	if maximumDelay-minimumDelay > 2 {
		return telegramVideoStickerMaxFPS
	}

	for fps := 1; fps <= telegramVideoStickerMaxFPS; fps++ {
		previousTick := 0
		matches := true
		for _, timestamp := range boundaries {
			tick := int(math.Round(float64(timestamp*fps) / 1000))
			quantizedMS := float64(tick*1000) / float64(fps)
			if tick <= previousTick || math.Abs(quantizedMS-float64(timestamp)) > 5 {
				matches = false
				break
			}
			previousTick = tick
		}
		if matches {
			return fps
		}
	}

	return telegramVideoStickerMaxFPS
}

func resampleAnimationFrames(timestamps []int, durationMS, fps int) []int {
	if len(timestamps) == 0 || durationMS <= 0 || fps <= 0 {
		return nil
	}

	frameCount := int(math.Round(float64(durationMS*fps) / 1000))
	if frameCount < 1 {
		frameCount = 1
	}
	indexes := make([]int, 0, frameCount)
	for sourceIndex, timestamp := range timestamps {
		if timestamp > durationMS {
			timestamp = durationMS
		}
		endTick := int(math.Round(float64(timestamp*fps) / 1000))
		if endTick > frameCount {
			endTick = frameCount
		}
		for len(indexes) < endTick {
			indexes = append(indexes, sourceIndex)
		}
		if len(indexes) == frameCount {
			break
		}
	}
	for len(indexes) < frameCount {
		indexes = append(indexes, len(timestamps)-1)
	}
	return indexes
}

func encodeRGBAAnimationToWebM(animation *decodedWebPAnimation, frameIndexes []int, fps, crf int) ([]byte, string, error) {
	ffmpegExec := state.State.Config.FfmpegExecutable
	if ffmpegExec == "" {
		ffmpegExec = "ffmpeg"
	}

	outputFile, outputPath, extraFiles, cleanup, err := newSeekableMediaFile("webm", 3)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create seekable WEBM output: %w", err)
	}
	defer cleanup()

	filter := "scale=512:512:force_original_aspect_ratio=decrease:force_divisible_by=2," +
		"pad=512:512:(ow-iw)/2:(oh-ih)/2:color=0x00000000,format=yuva420p"
	cmd := exec.Command(ffmpegExec,
		"-v", "error",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-video_size", fmt.Sprintf("%dx%d", animation.width, animation.height),
		"-framerate", strconv.Itoa(fps),
		"-i", "-",
		"-an",
		"-vf", filter,
		"-c:v", "libvpx-vp9",
		"-pix_fmt", "yuva420p",
		"-auto-alt-ref", "0",
		"-b:v", "0",
		"-crf", strconv.Itoa(crf),
		"-deadline", "good",
		"-cpu-used", "2",
		"-frames:v", strconv.Itoa(len(frameIndexes)),
		"-f", "webm",
		"-y",
		outputPath,
	)
	cmd.ExtraFiles = extraFiles

	readers := make([]io.Reader, 0, len(frameIndexes))
	for _, index := range frameIndexes {
		readers = append(readers, bytes.NewReader(animation.frames[index]))
	}
	var stderr bytes.Buffer
	cmd.Stdin = io.MultiReader(readers...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, stderr.String(), err
	}
	if _, err := outputFile.Seek(0, io.SeekStart); err != nil {
		return nil, stderr.String(), fmt.Errorf("failed to rewind WEBM output: %w", err)
	}
	outputData, err := io.ReadAll(outputFile)
	if err != nil {
		return nil, stderr.String(), fmt.Errorf("failed to read WEBM output: %w", err)
	}
	return outputData, stderr.String(), nil
}

type ffprobeStickerOutput struct {
	Streams []struct {
		CodecName string            `json:"codec_name"`
		Width     int               `json:"width"`
		Height    int               `json:"height"`
		FrameRate string            `json:"avg_frame_rate"`
		Frames    string            `json:"nb_read_frames"`
		Tags      map[string]string `json:"tags"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func validateTelegramVideoSticker(outputData []byte, expectedFrames, expectedFPS int) error {
	if len(outputData) > telegramVideoStickerMaxBytes {
		return fmt.Errorf("WEBM is %d bytes; limit is %d", len(outputData), telegramVideoStickerMaxBytes)
	}
	if len(outputData) < 4 || !bytes.Equal(outputData[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		return fmt.Errorf("VP9 output does not contain a WEBM header")
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=codec_name,width,height,avg_frame_rate,nb_read_frames:stream_tags=alpha_mode:format=duration",
		"-of", "json",
		"-i", "-",
	)
	var probeOutput, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(outputData)
	cmd.Stdout = &probeOutput
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var probe ffprobeStickerOutput
	if err := json.Unmarshal(probeOutput.Bytes(), &probe); err != nil {
		return fmt.Errorf("failed to parse ffprobe output: %w", err)
	}
	if len(probe.Streams) != 1 {
		return fmt.Errorf("WEBM contains %d video streams", len(probe.Streams))
	}
	stream := probe.Streams[0]
	if stream.CodecName != "vp9" {
		return fmt.Errorf("WEBM codec is %q, expected VP9", stream.CodecName)
	}
	if stream.Width != 512 || stream.Height != 512 {
		return fmt.Errorf("WEBM dimensions are %dx%d, expected 512x512", stream.Width, stream.Height)
	}
	if stream.Tags["alpha_mode"] != "1" {
		return fmt.Errorf("WEBM is missing VP9 alpha metadata")
	}
	frames, err := strconv.Atoi(stream.Frames)
	if err != nil || frames != expectedFrames {
		return fmt.Errorf("WEBM contains %q frames, expected %d", stream.Frames, expectedFrames)
	}
	fps, err := parseFrameRate(stream.FrameRate)
	if err != nil || fps > telegramVideoStickerMaxFPS+0.01 || math.Abs(fps-float64(expectedFPS)) > 0.01 {
		return fmt.Errorf("WEBM frame rate is %q, expected %d FPS", stream.FrameRate, expectedFPS)
	}
	if float64(frames)/fps > float64(telegramVideoStickerMaxDurationMS)/1000+0.01 {
		return fmt.Errorf("WEBM duration exceeds %d milliseconds", telegramVideoStickerMaxDurationMS)
	}
	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return fmt.Errorf("WEBM duration metadata is %q", probe.Format.Duration)
	}
	if duration > float64(telegramVideoStickerMaxDurationMS)/1000+0.01 {
		return fmt.Errorf("WEBM duration metadata exceeds %d milliseconds", telegramVideoStickerMaxDurationMS)
	}
	return nil
}

func parseFrameRate(value string) (float64, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid frame rate %q", value)
	}
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || denominator == 0 {
		return 0, fmt.Errorf("invalid frame rate denominator %q", parts[1])
	}
	return numerator / denominator, nil
}

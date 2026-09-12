package utils

import (
	"bytes"
	"encoding/binary"
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
	whatsAppGIFMaxDimension = 720
	whatsAppGIFMaxFPS       = 30
	whatsAppGIFMaxDuration  = 6.0
	whatsAppGIFTrimDuration = 5.9
)

type whatsAppGIFMetadata struct {
	width   uint32
	height  uint32
	seconds uint32
}

type whatsAppGIFProbe struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		Profile     string `json:"profile"`
		PixelFormat string `json:"pix_fmt"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		FrameRate   string `json:"avg_frame_rate"`
		HasBFrames  int    `json:"has_b_frames"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
}

type whatsAppGIFDetails struct {
	width       int
	height      int
	duration    float64
	fps         float64
	codec       string
	profile     string
	pixelFormat string
	hasBFrames  int
	audioTracks int
	fastStart   bool
}

func prepareTelegramAnimationForWhatsApp(inputData []byte) ([]byte, whatsAppGIFMetadata, error) {
	details, err := probeWhatsAppGIF(inputData)
	if err != nil {
		return nil, whatsAppGIFMetadata{}, fmt.Errorf("failed to inspect Telegram animation: %w", err)
	}

	reasons := whatsAppGIFConversionReasons(details)
	if len(reasons) == 0 {
		return inputData, whatsAppGIFMetadataFromDetails(details), nil
	}

	if logger := state.State.Logger; logger != nil {
		logger.Info("normalizing Telegram animation for WhatsApp GIF playback",
			zap.Strings("reasons", reasons),
			zap.String("codec", details.codec),
			zap.String("profile", details.profile),
			zap.String("pixel_format", details.pixelFormat),
			zap.Int("width", details.width),
			zap.Int("height", details.height),
			zap.Float64("fps", details.fps),
			zap.Float64("duration_seconds", details.duration),
			zap.Int("audio_tracks", details.audioTracks),
			zap.Bool("fast_start", details.fastStart),
		)
	}

	outputData, stderr, err := convertTelegramAnimationForWhatsApp(inputData, details)
	if err != nil {
		if logger := state.State.Logger; logger != nil {
			logger.Error("failed to normalize Telegram animation for WhatsApp GIF playback",
				zap.Strings("reasons", reasons),
				zap.String("ffmpeg_stderr", strings.TrimSpace(stderr)),
				zap.Error(err),
			)
		}
		return nil, whatsAppGIFMetadata{}, fmt.Errorf("failed to normalize Telegram animation: %w", err)
	}

	outputDetails, err := probeWhatsAppGIF(outputData)
	if err != nil {
		return nil, whatsAppGIFMetadata{}, fmt.Errorf("failed to validate normalized Telegram animation: %w", err)
	}
	if remainingReasons := whatsAppGIFConversionReasons(outputDetails); len(remainingReasons) > 0 {
		return nil, whatsAppGIFMetadata{}, fmt.Errorf("normalized Telegram animation is still incompatible: %s", strings.Join(remainingReasons, ", "))
	}

	return outputData, whatsAppGIFMetadataFromDetails(outputDetails), nil
}

func probeWhatsAppGIF(inputData []byte) (whatsAppGIFDetails, error) {
	inputFile, inputPath, extraFiles, cleanup, err := newSeekableMediaFile("mp4", 3)
	if err != nil {
		return whatsAppGIFDetails{}, fmt.Errorf("failed to create seekable probe input: %w", err)
	}
	defer cleanup()
	if _, err := inputFile.Write(inputData); err != nil {
		return whatsAppGIFDetails{}, fmt.Errorf("failed to buffer probe input: %w", err)
	}
	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		return whatsAppGIFDetails{}, fmt.Errorf("failed to rewind probe input: %w", err)
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name,profile,pix_fmt,width,height,avg_frame_rate,has_b_frames:format=format_name,duration",
		"-of", "json",
		"-i", inputPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.ExtraFiles = extraFiles
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return whatsAppGIFDetails{}, fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var probe whatsAppGIFProbe
	if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
		return whatsAppGIFDetails{}, fmt.Errorf("failed to decode ffprobe output: %w", err)
	}

	var details whatsAppGIFDetails
	videoStreams := 0
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			videoStreams++
			if videoStreams == 1 {
				details.width = stream.Width
				details.height = stream.Height
				details.codec = stream.CodecName
				details.profile = stream.Profile
				details.pixelFormat = stream.PixelFormat
				details.hasBFrames = stream.HasBFrames
				fps, err := parseFrameRate(stream.FrameRate)
				if err != nil {
					return whatsAppGIFDetails{}, err
				}
				details.fps = fps
			}
		case "audio":
			details.audioTracks++
		}
	}
	if videoStreams != 1 {
		return whatsAppGIFDetails{}, fmt.Errorf("expected one video stream, found %d", videoStreams)
	}

	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return whatsAppGIFDetails{}, fmt.Errorf("invalid duration %q", probe.Format.Duration)
	}
	details.duration = duration
	details.fastStart = hasMP4FastStart(inputData)
	if !strings.Contains(probe.Format.FormatName, "mp4") {
		details.fastStart = false
	}
	return details, nil
}

func whatsAppGIFConversionReasons(details whatsAppGIFDetails) []string {
	var reasons []string
	if details.codec != "h264" {
		reasons = append(reasons, "video codec is not H.264")
	}
	if details.pixelFormat != "yuv420p" {
		reasons = append(reasons, "pixel format is not yuv420p")
	}
	if details.audioTracks != 0 {
		reasons = append(reasons, "animation contains audio")
	}
	if details.width <= 0 || details.height <= 0 || details.width%2 != 0 || details.height%2 != 0 {
		reasons = append(reasons, "video dimensions are invalid or odd")
	}
	if details.width > whatsAppGIFMaxDimension || details.height > whatsAppGIFMaxDimension {
		reasons = append(reasons, "video dimensions exceed 720 pixels")
	}
	if details.fps <= 0 || details.fps > whatsAppGIFMaxFPS+0.01 {
		reasons = append(reasons, "frame rate exceeds 30 FPS")
	}
	if details.duration >= whatsAppGIFMaxDuration {
		reasons = append(reasons, "duration is not under six seconds")
	}
	if !details.fastStart {
		reasons = append(reasons, "MP4 metadata is not suitable for progressive playback")
	}
	return reasons
}

func convertTelegramAnimationForWhatsApp(inputData []byte, details whatsAppGIFDetails) ([]byte, string, error) {
	ffmpegExec := state.State.Config.FfmpegExecutable
	if ffmpegExec == "" {
		ffmpegExec = "ffmpeg"
	}

	inputFile, inputPath, inputExtraFiles, cleanupInput, err := newSeekableMediaFile("mp4", 3)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create seekable MP4 input: %w", err)
	}
	defer cleanupInput()
	if _, err := inputFile.Write(inputData); err != nil {
		return nil, "", fmt.Errorf("failed to buffer MP4 input: %w", err)
	}
	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		return nil, "", fmt.Errorf("failed to rewind MP4 input: %w", err)
	}

	outputChildFileDescriptor := 3 + len(inputExtraFiles)
	outputFile, outputPath, outputExtraFiles, cleanupOutput, err := newSeekableMediaFile("mp4", outputChildFileDescriptor)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create seekable MP4 output: %w", err)
	}
	defer cleanupOutput()

	filters := make([]string, 0, 2)
	if details.fps > whatsAppGIFMaxFPS+0.01 {
		filters = append(filters, "fps=30")
	}
	if details.width > whatsAppGIFMaxDimension || details.height > whatsAppGIFMaxDimension || details.width%2 != 0 || details.height%2 != 0 {
		filters = append(filters, "scale=w='min(720,iw)':h='min(720,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2")
	}

	args := []string{
		"-v", "error",
		"-i", inputPath,
		"-map", "0:v:0",
		"-an",
	}
	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}
	if details.duration >= whatsAppGIFMaxDuration {
		args = append(args, "-t", strconv.FormatFloat(whatsAppGIFTrimDuration, 'f', 1, 64))
	}
	if whatsAppGIFNeedsVideoEncoding(details) {
		args = append(args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-crf", "20",
			"-profile:v", "baseline",
			"-level:v", "3.1",
			"-pix_fmt", "yuv420p",
		)
	} else {
		args = append(args, "-c:v", "copy")
	}
	args = append(args,
		"-movflags", "+faststart",
		"-brand", "mp42",
		"-map_metadata", "-1",
		"-f", "mp4",
		"-y",
		outputPath,
	)

	cmd := exec.Command(ffmpegExec, args...)
	cmd.ExtraFiles = append(inputExtraFiles, outputExtraFiles...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, stderr.String(), err
	}
	if _, err := outputFile.Seek(0, io.SeekStart); err != nil {
		return nil, stderr.String(), fmt.Errorf("failed to rewind MP4 output: %w", err)
	}
	outputData, err := io.ReadAll(outputFile)
	if err != nil {
		return nil, stderr.String(), fmt.Errorf("failed to read MP4 output: %w", err)
	}
	return outputData, stderr.String(), nil
}

func whatsAppGIFNeedsVideoEncoding(details whatsAppGIFDetails) bool {
	return details.codec != "h264" ||
		details.pixelFormat != "yuv420p" ||
		details.width <= 0 || details.height <= 0 ||
		details.width%2 != 0 || details.height%2 != 0 ||
		details.width > whatsAppGIFMaxDimension || details.height > whatsAppGIFMaxDimension ||
		details.fps <= 0 || details.fps > whatsAppGIFMaxFPS+0.01 ||
		details.duration >= whatsAppGIFMaxDuration
}

func whatsAppGIFMetadataFromDetails(details whatsAppGIFDetails) whatsAppGIFMetadata {
	return whatsAppGIFMetadata{
		width:   uint32(details.width),
		height:  uint32(details.height),
		seconds: uint32(math.Ceil(details.duration)),
	}
}

func hasMP4FastStart(inputData []byte) bool {
	var moovOffset, mdatOffset int64 = -1, -1
	for offset := int64(0); offset+8 <= int64(len(inputData)); {
		size := int64(binary.BigEndian.Uint32(inputData[offset : offset+4]))
		atomType := string(inputData[offset+4 : offset+8])
		headerSize := int64(8)
		switch size {
		case 1:
			if offset+16 > int64(len(inputData)) {
				return false
			}
			size = int64(binary.BigEndian.Uint64(inputData[offset+8 : offset+16]))
			headerSize = 16
		case 0:
			size = int64(len(inputData)) - offset
		}
		if size < headerSize || offset+size > int64(len(inputData)) {
			return false
		}
		switch atomType {
		case "moov":
			moovOffset = offset
		case "mdat":
			mdatOffset = offset
		}
		offset += size
	}
	return moovOffset >= 0 && mdatOffset >= 0 && moovOffset < mdatOffset
}

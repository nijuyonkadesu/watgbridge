package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"watgbridge/state"

	"go.uber.org/zap"
)

const (
	stickerCacheMaxBytes    int64 = 1 << 30
	stickerCacheTargetBytes int64 = 900 << 20
)

var (
	stickerCacheDir = filepath.Join("downloads", ".cache", "stickers")

	stickerCacheInitOnce sync.Once
	stickerCacheMu       sync.RWMutex
	stickerCachePrune    = make(chan struct{}, 1)
	stickerCacheReady    bool
)

type stickerCacheFile struct {
	path    string
	size    int64
	modTime time.Time
}

func stickerCacheLogger() *zap.Logger {
	return state.State.Logger
}

func initializeStickerCache() bool {
	stickerCacheInitOnce.Do(func() {
		if err := os.MkdirAll(stickerCacheDir, os.ModePerm); err != nil {
			if logger := stickerCacheLogger(); logger != nil {
				logger.Warn("failed to initialize sticker conversion cache", zap.Error(err))
			}
			return
		}

		stickerCacheReady = true
		go func() {
			for range stickerCachePrune {
				pruneStickerCache()
			}
		}()
		requestStickerCachePrune()
	})

	return stickerCacheReady
}

func stickerCacheKey(variant string, inputData []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(variant))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(inputData)
	return hex.EncodeToString(hash.Sum(nil))
}

func stickerCachePath(variant, extension string, inputData []byte) string {
	key := stickerCacheKey(variant, inputData)
	extension = strings.TrimPrefix(extension, ".")
	return filepath.Join(stickerCacheDir, key+"."+extension)
}

func readStickerCache(variant, extension string, inputData []byte) ([]byte, bool) {
	if !initializeStickerCache() {
		return nil, false
	}

	cachePath := stickerCachePath(variant, extension, inputData)
	stickerCacheMu.RLock()
	outputData, err := os.ReadFile(cachePath)
	if err == nil && len(outputData) > 0 {
		now := time.Now()
		_ = os.Chtimes(cachePath, now, now)
	}
	stickerCacheMu.RUnlock()

	if err != nil || len(outputData) == 0 {
		return nil, false
	}

	if logger := stickerCacheLogger(); logger != nil {
		logger.Debug("sticker conversion cache hit",
			zap.String("variant", variant),
			zap.Int("bytes", len(outputData)),
		)
	}
	return outputData, true
}

func writeStickerCache(variant, extension string, inputData, outputData []byte) {
	if len(outputData) == 0 || !initializeStickerCache() {
		return
	}

	cachePath := stickerCachePath(variant, extension, inputData)
	stickerCacheMu.Lock()
	if existingInfo, err := os.Stat(cachePath); err == nil && existingInfo.Size() > 0 {
		now := time.Now()
		_ = os.Chtimes(cachePath, now, now)
		stickerCacheMu.Unlock()
		requestStickerCachePrune()
		return
	}

	tempFile, err := os.CreateTemp(stickerCacheDir, ".sticker-cache-*")
	if err == nil {
		if _, err = tempFile.Write(outputData); err == nil {
			err = tempFile.Close()
		} else {
			_ = tempFile.Close()
		}
		if err == nil {
			err = os.Rename(tempFile.Name(), cachePath)
		}
		if err != nil {
			_ = os.Remove(tempFile.Name())
		}
	}
	stickerCacheMu.Unlock()

	if err != nil {
		if logger := stickerCacheLogger(); logger != nil {
			logger.Warn("failed to store sticker conversion cache entry",
				zap.String("variant", variant),
				zap.Error(err),
			)
		}
		return
	}

	requestStickerCachePrune()
}

func requestStickerCachePrune() {
	select {
	case stickerCachePrune <- struct{}{}:
	default:
	}
}

func pruneStickerCache() {
	entries, err := os.ReadDir(stickerCacheDir)
	if err != nil {
		return
	}

	var (
		cacheFiles []stickerCacheFile
		totalBytes int64
	)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".sticker-cache-") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		cacheFiles = append(cacheFiles, stickerCacheFile{
			path:    filepath.Join(stickerCacheDir, entry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		totalBytes += info.Size()
	}

	if totalBytes <= stickerCacheMaxBytes {
		return
	}

	sort.Slice(cacheFiles, func(i, j int) bool {
		return cacheFiles[i].modTime.Before(cacheFiles[j].modTime)
	})

	removedFiles := 0
	removedBytes := int64(0)
	for _, cacheFile := range cacheFiles {
		if totalBytes <= stickerCacheTargetBytes {
			break
		}
		// A cache hit may have refreshed the file since the directory scan.
		// Leave that entry alone and let a later prune account for it.
		currentInfo, statErr := os.Stat(cacheFile.path)
		if statErr == nil && currentInfo.ModTime().After(cacheFile.modTime) {
			continue
		}
		if removeErr := os.Remove(cacheFile.path); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			continue
		}
		totalBytes -= cacheFile.size
		removedBytes += cacheFile.size
		removedFiles++
	}

	if removedFiles > 0 {
		if logger := stickerCacheLogger(); logger != nil {
			logger.Debug("pruned sticker conversion cache",
				zap.Int("files", removedFiles),
				zap.Int64("bytes", removedBytes),
				zap.Int64("remaining_bytes", totalBytes),
			)
		}
	}
}

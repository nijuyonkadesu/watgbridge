package utils

import "os"

func newTemporaryAnimationOutput() (*os.File, string, []*os.File, func(), error) {
	outputFile, err := os.CreateTemp("", "watgbridge-sticker-*.webm")
	if err != nil {
		return nil, "", nil, nil, err
	}
	return outputFile, outputFile.Name(), nil, func() {
		_ = outputFile.Close()
		_ = os.Remove(outputFile.Name())
	}, nil
}

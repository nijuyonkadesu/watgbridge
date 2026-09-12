package utils

import "os"

func newTemporaryMediaFile(extension string) (*os.File, string, []*os.File, func(), error) {
	outputFile, err := os.CreateTemp("", "watgbridge-media-*."+extension)
	if err != nil {
		return nil, "", nil, nil, err
	}
	return outputFile, outputFile.Name(), nil, func() {
		_ = outputFile.Close()
		_ = os.Remove(outputFile.Name())
	}, nil
}

package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// FfmpegAvailable returns true if ffmpeg is on PATH.
func FfmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// ExtractSegmentToWav uses ffmpeg to decode any supported format and write a WAV slice [startMs, endMs).
func ExtractSegmentToWav(srcPath, dstPath string, startMs, endMs int64) error {
	if !FfmpegAvailable() {
		return fmt.Errorf("ffmpeg not found on PATH (required for MP3 and non-WAV export)")
	}
	secStart := float64(startMs) / 1000.0
	durSec := float64(endMs-startMs) / 1000.0
	if durSec <= 0 {
		return fmt.Errorf("invalid segment duration")
	}
	cmd := exec.Command("ffmpeg", "-y",
		"-ss", strconv.FormatFloat(secStart, 'f', 6, 64),
		"-i", srcPath,
		"-t", strconv.FormatFloat(durSec, 'f', 6, 64),
		"-acodec", "pcm_s16le",
		"-ar", "44100",
		"-ac", "1",
		dstPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return nil
}

// TranscodeToWav16 converts full file to mono 16-bit PCM WAV (for processing).
func TranscodeToWav16(srcPath string) (tmpWav string, err error) {
	if !FfmpegAvailable() {
		return "", fmt.Errorf("ffmpeg not found")
	}
	f, err := os.CreateTemp("", "audiotag-*.wav")
	if err != nil {
		return "", err
	}
	_ = f.Close()
	tmpWav = f.Name()
	cmd := exec.Command("ffmpeg", "-y", "-i", srcPath, "-acodec", "pcm_s16le", "-ar", "44100", "-ac", "1", tmpWav)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmpWav)
		return "", err
	}
	return tmpWav, nil
}

func TempDir(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

func SafeJoin(dir, name string) string {
	return filepath.Join(dir, filepath.Base(name))
}

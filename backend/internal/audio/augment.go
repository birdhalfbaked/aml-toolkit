package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
)

// AddNoise reads WAV at path, adds Gaussian noise scaled by noiseRMS (0..1 of full scale), writes to dstPath.
func AddNoise(srcPath, dstPath string, noiseRMS float64, rng *rand.Rand) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := ReadWavInfo(f)
	if err != nil {
		return err
	}
	if info.BitsPerSample != 16 {
		return fmt.Errorf("augment: need 16-bit")
	}
	buf := make([]byte, info.DataSize)
	if _, err := f.Seek(info.DataOffset, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return err
	}
	n := len(buf) / 2
	for i := 0; i < n; i++ {
		v := int16(binary.LittleEndian.Uint16(buf[i*2:]))
		nz := rng.NormFloat64() * noiseRMS * 32767.0
		nv := float64(v) + nz
		if nv > 32767 {
			nv = 32767
		}
		if nv < -32768 {
			nv = -32768
		}
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(int16(math.Round(nv))))
	}
	return writeWavPCM(dstPath, info.SampleRate, info.Channels, info.BitsPerSample, buf)
}

// TimeShiftPad reads WAV, circularly shifts samples by shiftFrames (can be negative), writes dstPath.
func TimeShiftPad(srcPath, dstPath string, shiftFrames int) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := ReadWavInfo(f)
	if err != nil {
		return err
	}
	if info.BitsPerSample != 16 {
		return fmt.Errorf("shift: need 16-bit")
	}
	bytesPerFrame := info.Channels * 2
	totalFrames := info.DataSize / bytesPerFrame
	if totalFrames == 0 {
		return fmt.Errorf("empty")
	}
	shiftFrames = shiftFrames % totalFrames
	if shiftFrames < 0 {
		shiftFrames += totalFrames
	}
	buf := make([]byte, info.DataSize)
	if _, err := f.Seek(info.DataOffset, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return err
	}
	out := make([]byte, len(buf))
	frameBytes := bytesPerFrame
	for fr := 0; fr < totalFrames; fr++ {
		srcFr := (fr - shiftFrames + totalFrames) % totalFrames
		copy(out[fr*frameBytes:], buf[srcFr*frameBytes:(srcFr+1)*frameBytes])
	}
	return writeWavPCM(dstPath, info.SampleRate, info.Channels, info.BitsPerSample, out)
}

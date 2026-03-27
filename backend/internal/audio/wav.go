package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

type WavInfo struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	DataOffset    int64
	DataSize      int
}

func ReadWavInfo(r io.ReadSeeker) (*WavInfo, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	var riff [4]byte
	if _, err := io.ReadFull(r, riff[:]); err != nil {
		return nil, err
	}
	if string(riff[:]) != "RIFF" {
		return nil, fmt.Errorf("not RIFF")
	}
	var riffSize uint32
	if err := binary.Read(r, binary.LittleEndian, &riffSize); err != nil {
		return nil, err
	}
	var wave [4]byte
	if _, err := io.ReadFull(r, wave[:]); err != nil {
		return nil, err
	}
	if string(wave[:]) != "WAVE" {
		return nil, fmt.Errorf("not WAVE")
	}

	var info WavInfo
	for {
		var id [4]byte
		if _, err := io.ReadFull(r, id[:]); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		var chunkSize uint32
		if err := binary.Read(r, binary.LittleEndian, &chunkSize); err != nil {
			return nil, err
		}
		switch string(id[:]) {
		case "fmt ":
			var audioFormat uint16
			var numChannels uint16
			var sampleRate uint32
			var byteRate uint32
			var blockAlign uint16
			var bitsPerSample uint16
			if err := binary.Read(r, binary.LittleEndian, &audioFormat); err != nil {
				return nil, err
			}
			if audioFormat != 1 {
				return nil, fmt.Errorf("only PCM supported, got format %d", audioFormat)
			}
			if err := binary.Read(r, binary.LittleEndian, &numChannels); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &sampleRate); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &byteRate); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &blockAlign); err != nil {
				return nil, err
			}
			if err := binary.Read(r, binary.LittleEndian, &bitsPerSample); err != nil {
				return nil, err
			}
			info.SampleRate = int(sampleRate)
			info.Channels = int(numChannels)
			info.BitsPerSample = int(bitsPerSample)
			if chunkSize > 16 {
				if _, err := r.Seek(int64(chunkSize-16), io.SeekCurrent); err != nil {
					return nil, err
				}
			}
		case "data":
			off, _ := r.Seek(0, io.SeekCurrent)
			info.DataOffset = off
			info.DataSize = int(chunkSize)
			if _, err := r.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, err
			}
		default:
			if _, err := r.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return nil, err
			}
		}
	}
	if info.DataSize == 0 {
		return nil, fmt.Errorf("no data chunk")
	}
	return &info, nil
}

func DurationMs(info *WavInfo) int64 {
	bytesPerFrame := info.Channels * (info.BitsPerSample / 8)
	if bytesPerFrame == 0 || info.SampleRate == 0 {
		return 0
	}
	frames := info.DataSize / bytesPerFrame
	return int64(frames) * 1000 / int64(info.SampleRate)
}

// ExtractWavSegment writes a new WAV file with samples from startMs to endMs.
func ExtractWavSegment(srcPath, dstPath string, startMs, endMs int64) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := ReadWavInfo(f)
	if err != nil {
		return err
	}
	bytesPerFrame := info.Channels * (info.BitsPerSample / 8)
	if bytesPerFrame == 0 {
		return fmt.Errorf("invalid wav")
	}
	totalFrames := info.DataSize / bytesPerFrame
	startFrame := int64(info.SampleRate) * startMs / 1000
	endFrame := int64(info.SampleRate) * endMs / 1000
	if startFrame < 0 {
		startFrame = 0
	}
	if endFrame > int64(totalFrames) {
		endFrame = int64(totalFrames)
	}
	if endFrame <= startFrame {
		return fmt.Errorf("empty segment")
	}
	nFrames := int(endFrame - startFrame)
	dataStart := info.DataOffset + startFrame*int64(bytesPerFrame)
	dataLen := nFrames * bytesPerFrame
	if _, err := f.Seek(dataStart, io.SeekStart); err != nil {
		return err
	}
	buf := make([]byte, dataLen)
	if _, err := io.ReadFull(f, buf); err != nil {
		return err
	}
	return writeWavPCM(dstPath, info.SampleRate, info.Channels, info.BitsPerSample, buf)
}

func writeWavPCM(path string, sampleRate, channels, bits int, pcm []byte) error {
	var b bytes.Buffer
	dataSize := len(pcm)
	chunkSize := 36 + dataSize

	_, _ = b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(chunkSize))
	_, _ = b.WriteString("WAVE")
	_, _ = b.WriteString("fmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&b, binary.LittleEndian, uint32(sampleRate))
	byteRate := uint32(sampleRate * channels * (bits / 8))
	_ = binary.Write(&b, binary.LittleEndian, byteRate)
	blockAlign := uint16(channels * (bits / 8))
	_ = binary.Write(&b, binary.LittleEndian, blockAlign)
	_ = binary.Write(&b, binary.LittleEndian, uint16(bits))
	_, _ = b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(dataSize))
	b.Write(pcm)
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// TrimSilenceEdges returns new start/end ms by scanning from edges until RMS exceeds threshold (0..1 scale of int16 max).
func TrimSilenceEdges(srcPath string, threshold float64, windowMs int64) (newStartMs, newEndMs int64, err error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	info, err := ReadWavInfo(f)
	if err != nil {
		return 0, 0, err
	}
	if info.BitsPerSample != 16 {
		return 0, 0, fmt.Errorf("trim silence: need 16-bit PCM")
	}
	bytesPerFrame := info.Channels * (info.BitsPerSample / 8)
	totalFrames := info.DataSize / bytesPerFrame
	if totalFrames == 0 {
		return 0, 0, fmt.Errorf("empty audio")
	}
	windowFrames := int(info.SampleRate) * int(windowMs) / 1000
	if windowFrames < 1 {
		windowFrames = 1
	}
	buf := make([]byte, info.DataSize)
	if _, err := f.Seek(info.DataOffset, io.SeekStart); err != nil {
		return 0, 0, err
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0, 0, err
	}

	rmsWindow := func(startFrame int) float64 {
		end := startFrame + windowFrames
		if end > totalFrames {
			end = totalFrames
		}
		if startFrame < 0 {
			startFrame = 0
		}
		var sum float64
		n := 0
		for fr := startFrame; fr < end; fr++ {
			off := fr * bytesPerFrame
			for ch := 0; ch < info.Channels; ch++ {
				o := off + ch*2
				if o+2 > len(buf) {
					break
				}
				v := int16(binary.LittleEndian.Uint16(buf[o:]))
				sum += float64(v) * float64(v)
				n++
			}
		}
		if n == 0 {
			return 0
		}
		return math.Sqrt(sum / float64(n)) / 32768.0
	}

	startFrame := 0
	for startFrame < totalFrames-windowFrames {
		if rmsWindow(startFrame) > threshold {
			break
		}
		startFrame += windowFrames / 4
		if startFrame < 1 {
			startFrame = 1
		}
	}
	endFrame := totalFrames
	for endFrame > startFrame+windowFrames {
		if rmsWindow(endFrame-windowFrames) > threshold {
			break
		}
		endFrame -= windowFrames / 4
	}
	newStartMs = int64(startFrame) * 1000 / int64(info.SampleRate)
	newEndMs = int64(endFrame) * 1000 / int64(info.SampleRate)
	if newEndMs <= newStartMs {
		return 0, int64(totalFrames) * 1000 / int64(info.SampleRate), nil
	}
	return newStartMs, newEndMs, nil
}

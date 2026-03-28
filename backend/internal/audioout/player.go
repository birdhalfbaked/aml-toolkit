package audioout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"

	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"
)

// LayoutAccessor resolves the current on-disk library root (see [handlers.Server]).
type LayoutAccessor interface {
	CurrentLayout() *store.Layout
}

const playbackSR = beep.SampleRate(44100)

// defaultPositionEmitInterval is how often [Player.OnPosition] runs during playback.
// This drives the desktop UI playhead via Wails events (~10 updates/sec). True sub-millisecond
// rates are not practical across the JS bridge.
const defaultPositionEmitInterval = 100 * time.Millisecond

var speakerOnce sync.Once
var speakerInitErr error

func ensureSpeaker() error {
	speakerOnce.Do(func() {
		buf := playbackSR.N(100 * time.Millisecond)
		if buf < 1024 {
			buf = 1024
		}
		speakerInitErr = speaker.Init(playbackSR, buf)
	})
	return speakerInitErr
}

// WarmSpeaker initializes oto/beep audio once. Call from app startup so device errors show in logs
// before the first segment play (and to fail fast instead of timing out on the Wails bridge).
func WarmSpeaker() error {
	return ensureSpeaker()
}

// Player plays collection audio files through the system speaker using gopxl/beep.
type Player struct {
	repo    *repo.Repo
	layouts LayoutAccessor

	mu sync.Mutex

	inner beep.StreamSeekCloser
	file  *os.File
	sr    beep.SampleRate

	tickCancel context.CancelFunc

	// OnPosition is called from a background goroutine with playback position in milliseconds.
	OnPosition func(ms int64)

	// PositionEmitInterval is the ticker period for OnPosition while playing.
	// Zero means [defaultPositionEmitInterval].
	PositionEmitInterval time.Duration
}

func NewPlayer(rp *repo.Repo, layouts LayoutAccessor) *Player {
	return &Player{repo: rp, layouts: layouts}
}

func (p *Player) resolvePath(ctx context.Context, fileID int64) (beep.StreamSeekCloser, *os.File, beep.SampleRate, error) {
	a, err := p.repo.GetAudioFile(ctx, fileID)
	if err != nil || a == nil {
		return nil, nil, 0, fmt.Errorf("audio file not found")
	}
	col, err := p.repo.GetCollection(ctx, a.CollectionID)
	if err != nil || col == nil {
		return nil, nil, 0, fmt.Errorf("collection not found")
	}
	path := filepath.Join(p.layouts.CurrentLayout().CollectionDir(col.ProjectID, col.ID), a.StoredFilename)
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, err
	}
	lower := strings.ToLower(a.StoredFilename)
	switch {
	case strings.HasSuffix(lower, ".wav") || a.Format == "wav":
		s, format, err := wav.Decode(f)
		if err != nil {
			_ = f.Close()
			return nil, nil, 0, err
		}
		return s, f, format.SampleRate, nil
	case strings.HasSuffix(lower, ".mp3") || a.Format == "mp3":
		s, format, err := mp3.Decode(f)
		if err != nil {
			_ = f.Close()
			return nil, nil, 0, err
		}
		return s, f, format.SampleRate, nil
	default:
		_ = f.Close()
		return nil, nil, 0, fmt.Errorf("unsupported audio format for desktop playback")
	}
}

func (p *Player) stopLocked() {
	if p.tickCancel != nil {
		p.tickCancel()
		p.tickCancel = nil
	}
	speaker.Clear()
	if p.inner != nil {
		_ = p.inner.Close()
		p.inner = nil
	}
	if p.file != nil {
		_ = p.file.Close()
		p.file = nil
	}
}

// Stop stops playback and releases the current file.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

// seekInnerToMsLocked moves the decoded stream to approximately ms. mu must be held.
func (p *Player) seekInnerToMsLocked(ms int64) error {
	if p.inner == nil {
		return fmt.Errorf("no active stream")
	}
	pos := p.sr.N(time.Duration(ms) * time.Millisecond)
	if pos < 0 {
		pos = 0
	}
	if pos >= p.inner.Len() {
		pos = p.inner.Len() - 1
	}
	if pos < 0 {
		pos = 0
	}
	return p.inner.Seek(pos)
}

// restartResampledPlaybackLocked clears the speaker and plays a new Resampler over p.inner.
// beep.Resample buffers the source; seeking inner without rebuilding the Resampler breaks playback.
// mu must be held.
func (p *Player) restartResampledPlaybackLocked() {
	if p.tickCancel != nil {
		p.tickCancel()
		p.tickCancel = nil
	}
	speaker.Clear()
	stream := beep.Resample(4, p.sr, playbackSR, p.inner)
	// Do not call speaker.Lock() here: speaker.Play already locks the mixer mutex; double-lock deadlocks.
	speaker.Play(stream)
	p.startPositionLoop()
}

// Play opens fileID and starts playback at startMs (0 = file start).
// Resolution matches HTTP GET /api/files/:id/audio.
func (p *Player) Play(ctx context.Context, fileID int64, startMs int64) error {
	if err := ensureSpeaker(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopLocked()

	inner, f, sr, err := p.resolvePath(ctx, fileID)
	if err != nil {
		return err
	}

	p.inner = inner
	p.file = f
	p.sr = sr

	if startMs > 0 {
		if err := p.seekInnerToMsLocked(startMs); err != nil {
			p.stopLocked()
			return err
		}
	}

	p.restartResampledPlaybackLocked()
	return nil
}

func (p *Player) emitPositionIfPlaying() bool {
	p.mu.Lock()
	inner := p.inner
	sr := p.sr
	cb := p.OnPosition
	p.mu.Unlock()
	if inner == nil || cb == nil {
		return false
	}
	cb(sr.D(inner.Position()).Milliseconds())
	return true
}

func (p *Player) startPositionLoop() {
	if p.OnPosition == nil {
		return
	}
	interval := p.PositionEmitInterval
	if interval <= 0 {
		interval = defaultPositionEmitInterval
	}
	tctx, cancel := context.WithCancel(context.Background())
	p.tickCancel = cancel
	go func() {
		if !p.emitPositionIfPlaying() {
			return
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-tctx.Done():
				return
			case <-t.C:
				if !p.emitPositionIfPlaying() {
					return
				}
			}
		}
	}()
}

// SeekToMs seeks the current stream to approximately ms (from the start of the file).
func (p *Player) SeekToMs(ms int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inner == nil {
		return fmt.Errorf("no active playback")
	}
	if err := p.seekInnerToMsLocked(ms); err != nil {
		return err
	}
	p.restartResampledPlaybackLocked()
	if p.OnPosition != nil {
		p.OnPosition(p.sr.D(p.inner.Position()).Milliseconds())
	}
	return nil
}

// Pause locks the speaker so playback stalls (beep speaker.Lock semantics).
func (p *Player) Pause() {
	speaker.Lock()
}

// Resume unlocks the speaker after [Pause].
func (p *Player) Resume() {
	speaker.Unlock()
}

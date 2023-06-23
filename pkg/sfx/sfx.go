package sfx

import (
	"fmt"
	"os"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type Streamer struct {
	beep.StreamSeekCloser

	controller *effects.Volume
	ctrl       *beep.Ctrl

	data   []byte
	format beep.Format
	done   chan bool
	file   *os.File
	Base   float64
	Silent bool
}

func Init() error {
	// fmt.Println("----- initializing sfx -----")
	// fmt.Println("sample rate:", s.format.SampleRate)
	// fmt.Println("buffer size:", s.format.SampleRate.N(time.Second/30))
	// fmt.Println("-------------------")
	// bgm
	// sample rate: 44100
	// buffer size: 1469
	// sfx
	// sample rate: 22050
	// buffer size: 734
	return speaker.Init(44100, 1500) //s.format.SampleRate.N(time.Second/30))
}

func NewStreamer(filename string) (*Streamer, error) {
	body, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	// defer f.Close()

	// buff := io.NopCloser(bytes.NewReader(body))
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("invalid mp3 file: %s", filename)
	}

	return &Streamer{
		StreamSeekCloser: streamer,
		file:             f,
		data:             body,
		format:           format,
		done:             make(chan bool),
		Base:             2,
		Silent:           false,
	}, nil
}

func (s *Streamer) Play(volume float64) {
	speaker.Lock()
	// reinitialize the streamer so that we don't have to keep the file open
	// buff := io.NopCloser(bytes.NewReader(s.data))
	// streamer, _, err := mp3.Decode(buff)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// s.StreamSeekCloser = streamer
	// when the file isn't open, seek will panic
	// @TODO: find out if there's a way to seek from a buffer
	s.Seek(0)
	speaker.Unlock()

	s.ctrl = nil
	s.controller = &effects.Volume{
		Streamer: s.StreamSeekCloser,
		Base:     s.Base,
		Volume:   volume,
		Silent:   s.Silent,
	}
	speaker.Play(s.controller)
}

func (s *Streamer) PlayOnRepeat(volume float64) {
	speaker.Lock()
	// reinitialize the streamer so that we don't have to keep the file open
	// buff := io.NopCloser(bytes.NewReader(s.data))
	// streamer, _, err := mp3.Decode(buff)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// s.StreamSeekCloser = streamer
	s.Seek(0)
	speaker.Unlock()

	// @TODO: this is digsuting, come up with a way to fix it
	s.ctrl = &beep.Ctrl{Streamer: beep.Loop(-1, s.StreamSeekCloser), Paused: false}
	s.controller = &effects.Volume{
		Streamer: s.ctrl,
		Base:     s.Base,
		Volume:   volume,
		Silent:   s.Silent,
	}
	speaker.Play(s.controller)
}

func Stop() {
	speaker.Clear()
}

func (s *Streamer) Release() {
	s.Close()
}

func Close() {
	speaker.Close()
}

func (s *Streamer) SetVolume(volume float64) {
	if s.controller != nil {
		speaker.Lock()
		s.controller.Volume = volume
		speaker.Unlock()
	}
}

func (s *Streamer) GetVolume() float64 {
	if s.controller != nil {
		return s.controller.Volume
	}
	return 0
}

func (s *Streamer) Resume() {
	if s.ctrl != nil {
		s.ctrl.Paused = false
	}
}
func (s *Streamer) Pause() {
	if s.ctrl != nil {
		s.ctrl.Paused = true
	}
}

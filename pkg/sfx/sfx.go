package sfx

import (
	"log"
	"os"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type Streamer struct {
	beep.StreamSeekCloser

	data   []byte
	format beep.Format
	done   chan bool
	file   *os.File
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

func NewStreamer(filename string) *Streamer {
	body, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	// defer f.Close()

	// buff := io.NopCloser(bytes.NewReader(body))
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	return &Streamer{
		StreamSeekCloser: streamer,
		file:             f,
		data:             body,
		format:           format,
		done:             make(chan bool),
	}
}

func (s *Streamer) Play() {
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
	speaker.Play(s)
	// speaker.Play(beep.Seq(Sounds["54"], beep.Callback(func() {
	// done <- true
	// })))
}

func (s *Streamer) PlayOnRepeat() {
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
	speaker.Play(beep.Loop(-1, s.StreamSeekCloser))
}

func Stop() {
	speaker.Clear()
}

func (s *Streamer) Release() {
	s.Close()
	// s.done <- true
}

func Close() {
	speaker.Close()
}

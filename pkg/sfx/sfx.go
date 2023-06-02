package sfx

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type Streamer struct {
	beep.StreamSeekCloser

	data   []byte
	format beep.Format
}

func Init(s *Streamer) error {
	fmt.Println("----- initializing sfx -----")
	fmt.Println("sample rate:", s.format.SampleRate)
	fmt.Println("buffer size:", s.format.SampleRate.N(time.Second/30))
	fmt.Println("-------------------")
	// bgm
	// sample rate: 44100
	// buffer size: 1469
	// sfx
	// sample rate: 22050
	// buffer size: 734
	return speaker.Init(s.format.SampleRate, 1500) //s.format.SampleRate.N(time.Second/30))
}

func NewStreamer(filename string) *Streamer {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	body, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	buff := io.NopCloser(bytes.NewReader(body))
	streamer, format, err := mp3.Decode(buff)
	if err != nil {
		log.Fatal(err)
	}

	return &Streamer{
		StreamSeekCloser: streamer,
		data:             body,
		format:           format,
	}
}

func (s *Streamer) Play() {
	speaker.Lock()
	// reinitialize the streamer so that we don't have to keep the file open
	buff := io.NopCloser(bytes.NewReader(s.data))
	streamer, _, err := mp3.Decode(buff)
	if err != nil {
		log.Fatal(err)
	}
	s.StreamSeekCloser = streamer
	// when the file isn't open, seek will panic
	// @TODO: find out if there's a way to seek from a buffer
	// s.StreamSeekCloser.Seek(0)
	speaker.Unlock()
	speaker.Play(s)
	// speaker.Play(beep.Seq(Sounds["54"], beep.Callback(func() {
	// done <- true
	// })))
}

func (s *Streamer) Release() {
	s.Close()
}

func Close() {
	speaker.Close()
}

package proto

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	Magic      uint32 = 0x53435231 // "SCR1"
	MaxPCM     uint32 = 2 * 16000 * 2 * 60
	MaxText    uint32 = 1 << 20
	StatusOK   uint32 = 0
	StatusErr  uint32 = 1
	headerSize        = 12
)

type Request struct {
	SampleRate int
	PCM        []byte
}

type Reply struct {
	Status uint32
	Text   string
}

func WriteRequest(w io.Writer, sampleRate int, pcm []byte) error {
	if sampleRate <= 0 {
		return fmt.Errorf("bad sample rate %d", sampleRate)
	}
	if uint32(len(pcm)) > MaxPCM {
		return fmt.Errorf("audio too long (%d bytes)", len(pcm))
	}
	var hdr [headerSize]byte
	binary.LittleEndian.PutUint32(hdr[0:], Magic)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[8:], uint32(len(pcm)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(pcm) == 0 {
		return nil
	}
	_, err := w.Write(pcm)
	return err
}

func ReadRequest(r io.Reader) (Request, error) {
	var hdr [headerSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Request{}, err
	}
	magic := binary.LittleEndian.Uint32(hdr[0:])
	if magic != Magic {
		return Request{}, fmt.Errorf("bad magic 0x%x", magic)
	}
	sr := binary.LittleEndian.Uint32(hdr[4:])
	n := binary.LittleEndian.Uint32(hdr[8:])
	if sr == 0 {
		return Request{}, fmt.Errorf("bad sample rate %d", sr)
	}
	if n > MaxPCM {
		return Request{}, fmt.Errorf("audio too long (%d bytes)", n)
	}
	pcm := make([]byte, n)
	if _, err := io.ReadFull(r, pcm); err != nil {
		return Request{}, err
	}
	return Request{SampleRate: int(sr), PCM: pcm}, nil
}

func WriteReply(w io.Writer, status uint32, text string) error {
	b := []byte(text)
	if uint32(len(b)) > MaxText {
		return fmt.Errorf("text too long (%d bytes)", len(b))
	}
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:], status)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	_, err := w.Write(b)
	return err
}

func ReadReply(r io.Reader) (Reply, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Reply{}, err
	}
	status := binary.LittleEndian.Uint32(hdr[0:])
	n := binary.LittleEndian.Uint32(hdr[4:])
	if n > MaxText {
		return Reply{}, fmt.Errorf("text too long (%d bytes)", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return Reply{}, err
	}
	return Reply{Status: status, Text: string(b)}, nil
}

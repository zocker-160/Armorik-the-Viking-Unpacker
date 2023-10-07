package main

// WARNING: this code is a quick and dirty hackjob!!!

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

const WIDTH = 320
const HEIGHT = 200
const BPP = 2

const IMGSIZE = (WIDTH * HEIGHT) / 4

// CGA color palette
var COLOR_0 = color.Black
var COLOR_1 = color.NRGBA{0x55, 0xFF, 0xFF, 0xFF}
var COLOR_2 = color.NRGBA{0xFF, 0x55, 0xFF, 0xFF}
var COLOR_3 = color.White

type CGAImage struct {
	Data []byte
}

func (img *CGAImage) ColorModel() color.Model {
	return color.NRGBAModel
}

func (img *CGAImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, WIDTH, HEIGHT)
}

func (img *CGAImage) At(x, y int) color.Color {
	bits := (y * WIDTH * BPP) + x * BPP
	var value byte

	switch index := bits / 8; bits % 8 {
	case 0:
		value = img.Data[index] >> 6
	case 2:
		value = img.Data[index] >> 4
	case 4:
		value = img.Data[index] >> 2
	case 6:
		value = img.Data[index]
	}

	value = value & 0x03

	switch value {
	case 0x00:
		return COLOR_0
	case 0x01:
		return COLOR_1
	case 0x02:
		return COLOR_2
	case 0x03:
		return COLOR_3
	}

	panic(fmt.Errorf("Unknown color index %d", value))
}


type Header struct {
	Unk1, Unk2, Unk3, Unk4 uint16
}

func deinterlace(data []byte) (*bytes.Buffer, error) {
	// data is expected to be interlaced CGA
	// see: https://moddingwiki.shikadi.net/wiki/Raw_CGA_Data

	even := bytes.NewBuffer(data[:IMGSIZE/2])
	odd := bytes.NewBuffer(data[IMGSIZE/2:IMGSIZE])

	outbuf := new(bytes.Buffer)
	outbuf.Grow(IMGSIZE)

	row := make([]byte, WIDTH / 4)

	for i := 0; i < HEIGHT; i += 2 {
		even.Read(row)
		outbuf.Write(row)

		odd.Read(row)
		outbuf.Write(row)
	}

	return outbuf, nil
}

func parseImage(fr *bufio.Reader) (*bytes.Buffer, error) {
	decomb := new(bytes.Buffer)
	pos := 0

	for {
		peek, err := fr.Peek(4)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			
			panic(err)
		}
		if bytes.Equal(peek, []byte("\x00\x00\x00\x00")) {
			fmt.Println("next header reached")
			fr.ReadByte()
			break
		}

		controlByte, _ := fr.ReadByte()
		pos++

		if controlByte == 0 {
			//fmt.Println("skipped", controlByte)
			continue

		} else if controlByte == 0x80 {
			//fmt.Println("CGA plane change")
			continue

		} else if (controlByte & 0x80) == 0 {
			//fmt.Println("process", controlByte)

			for i := 0; i < int(controlByte); i++ {
				b1, _ := fr.ReadByte()
				b2, _ := fr.ReadByte()
				
				pos += 2

				decomb.WriteByte(b1)
				decomb.WriteByte(b2)
			}

		} else {
			//fmt.Println("process else", controlByte)

			controlByte = controlByte & 0x7F

			b1, _ := fr.ReadByte()
			b2, _ := fr.ReadByte()

			pos += 2

			for i := 0; i < int(controlByte); i++ {
				decomb.WriteByte(b1)
				decomb.WriteByte(b2)
			}
		}

		fmt.Println("LEN:", decomb.Len(), "POS:", pos+8)
	}

	fmt.Println("deinterlace")

	//return decomb, nil

	deint, _ := deinterlace(decomb.Bytes())
	return deint, nil
}

func main() {
	fmt.Println("Amorik the Viking extractor by zocker_160")

	filename := os.Args[1]

	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		panic(err)
	}

	filesize := stat.Size()
	fmt.Println("File Size:", filesize)

	fr := bufio.NewReader(file)

	for i := 0;; i++ {
		//n, _ := file.Seek(0, io.SeekCurrent)

		header := new(Header)
		err := binary.Read(fr, binary.LittleEndian, header)
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				fmt.Println("Header EOF")
				break
			}

			panic(err)
		}

		fmt.Println("header:", header, i)

		imgBuf, err := parseImage(fr)
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("EOF")
				break
			}

			panic(err)
		}

		
		fmt.Println("Writing file", i)

		os.Mkdir("out", os.FileMode(0775))

		outFile, err := os.Create(fmt.Sprintf("out/outfile_%d.png", i))
		if err != nil {
			panic(err)
		}
		
		var img image.Image = &CGAImage{Data: imgBuf.Bytes()}

		err = png.Encode(outFile, img)
		if err != nil {
			panic(err)
		}

		//imgBuf.WriteTo(outFile)

		outFile.Close()
	}

	fmt.Println("DONE!")
}

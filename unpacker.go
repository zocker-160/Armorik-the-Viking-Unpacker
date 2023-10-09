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
	"path/filepath"
)

// CGA color palette
var COLOR_0 = color.NRGBA{0x00, 0x00, 0x00, 0xFF}
var COLOR_1 = color.NRGBA{0x55, 0xFF, 0xFF, 0xFF}
var COLOR_2 = color.NRGBA{0xFF, 0x55, 0xFF, 0xFF}
var COLOR_3 = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}

type CGAImage struct {
	Width, Height, BPP int
	Data []byte
	HasAlpha bool
	AlphaData []byte
}

func (img *CGAImage) ColorModel() color.Model {
	return color.NRGBAModel
}

func (img *CGAImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, img.Width, img.Height)
}

func (img *CGAImage) At(x, y int) color.Color {
	bits := (y * img.Width * img.BPP) + x * img.BPP
	var value byte
	var color color.NRGBA

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
		color = COLOR_0
	case 0x01:
		color = COLOR_1
	case 0x02:
		color = COLOR_2
	case 0x03:
		color = COLOR_3
	default:
		panic(fmt.Errorf("Unknown color index %d", value))
	}

	if img.HasAlpha {
		var alphaValue byte

		switch index := bits / 8; bits % 8 {
		case 0:
			alphaValue = img.AlphaData[index] >> 6
		case 2:
			alphaValue = img.AlphaData[index] >> 4
		case 4:
			alphaValue = img.AlphaData[index] >> 2
		case 6:
			alphaValue = img.AlphaData[index]
		}

		if alphaValue == 0x03 {
			color.A = 0x00
		}
	}

	return color
}

func (img *CGAImage) Deinterlace() error {
	// see: https://moddingwiki.shikadi.net/wiki/Raw_CGA_Data

	data := img.Data
	imgsize := img.GetImageSize()

	even := bytes.NewBuffer(data[:imgsize/2])
	odd := bytes.NewBuffer(data[imgsize/2:imgsize])

	outbuf := new(bytes.Buffer)
	outbuf.Grow(imgsize)

	row := make([]byte, img.Width / 4)

	for i := 0; i < img.Height; i += 2 {
		even.Read(row)
		outbuf.Write(row)

		odd.Read(row)
		outbuf.Write(row)
	}

	img.Data = outbuf.Bytes()

	return nil
}

func (img *CGAImage) GetImageSize() int {
	return (img.Width * img.Height * img.BPP) / 8
}

type Header struct {
	Unk1, Unk2, Unk3, Unk4 uint16
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

	return decomb, nil
}

func parseBACKS(filename, filebase string) error {
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

		fmt.Println("deinterlace")
		
		img := &CGAImage{
			Width: 320, Height: 200, BPP: 2,
			HasAlpha: false,
			Data: imgBuf.Bytes(),
		}
		img.Deinterlace()

		fmt.Println("Writing file", i)

		os.Mkdir(filebase, os.FileMode(0775))

		outFile, err := os.Create(fmt.Sprintf("%s/outfile_%d.png", filebase, i))
		if err != nil {
			panic(err)
		}

		err = png.Encode(outFile, img)
		if err != nil {
			panic(err)
		}

		//imgBuf.WriteTo(outFile)
		outFile.Close()
	}

	return nil
}

func parseMOTIV(filename, filebase string) error {
	// TODO
}

func main() {
	fmt.Println("Amorik the Viking extractor by zocker_160")

	filename := os.Args[1]
	filebase := filepath.Base(filename)

	switch filebase {
	case "BACKS.BIN":
		if err := parseBACKS(filename, filebase); err != nil {
			panic(err)
		}
	case "MOTIFS.BIN":
		if err := parseMOTIV(filename, filebase); err != nil {
			panic(err)
		}
	default:
		fmt.Println("ERROR: unsupported file", filebase)
	}

	fmt.Println("DONE!")
}

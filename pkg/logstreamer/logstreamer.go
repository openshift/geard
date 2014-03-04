// Source from https://github.com/kvz/logstreamer
// ----------
// The MIT License (MIT)
// Copyright (c) 2013 Kevin van Zonneveld <kevin@vanzonneveld.net>

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the â€œSoftwareâ€), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
//
package logstreamer

import (
	"bytes"
	"io"
	"log"
	"os"
)

type Logstreamer struct {
	Logger    *log.Logger
	buf       *bytes.Buffer
	readLines string
	// If prefix == stdout, colors green
	// If prefix == stderr, colors red
	// Else, prefix is taken as-is, and prepended to anything
	// you throw at Write()
	prefix string
	// if true, saves output in memory
	record  bool
	persist string

	// Adds color to stdout & stderr if terminal supports it
	colorOkay  string
	colorFail  string
	colorReset string
}

func NewLogstreamer(logger *log.Logger, prefix string, record bool) *Logstreamer {
	streamer := &Logstreamer{
		Logger:     logger,
		buf:        bytes.NewBuffer([]byte("")),
		prefix:     prefix,
		record:     record,
		persist:    "",
		colorOkay:  "",
		colorFail:  "",
		colorReset: "",
	}

	if os.Getenv("TERM") == "xterm" {
		streamer.colorOkay = "\x1b[32m"
		streamer.colorFail = "\x1b[31m"
		streamer.colorReset = "\x1b[0m"
	}

	return streamer
}

func (l *Logstreamer) Write(p []byte) (n int, err error) {
	if n, err = l.buf.Write(p); err != nil {
		return
	}

	err = l.OutputLines()
	return
}

func (l *Logstreamer) Close() error {
	l.Flush()
	l.buf = bytes.NewBuffer([]byte(""))
	return nil
}

func (l *Logstreamer) Flush() error {
	var p []byte
	if n, err := l.buf.Read(p); err != nil || n == 0 {
		return err
	}

	l.out(string(p))
	return nil
}

func (l *Logstreamer) OutputLines() (err error) {
	for {
		line, err := l.buf.ReadString('\n')
		if err == io.EOF {
			if line != "" {
				l.out(line)
			}
			break
		}
		if err != nil {
			return err
		}

		l.out(line)
	}

	return nil
}

func (l *Logstreamer) FlushRecord() string {
	buffer := l.persist
	l.persist = ""
	return buffer
}

func (l *Logstreamer) out(str string) (err error) {
	if l.record == true {
		l.persist = l.persist + str
	}

	if l.prefix == "stdout" {
		str = l.colorOkay + l.prefix + l.colorReset + " " + str
	} else if l.prefix == "stderr" {
		str = l.colorFail + l.prefix + l.colorReset + " " + str
	} else {
		str = l.prefix + str
	}

	l.Logger.Print(str)

	return nil
}

// Copyright 2017 Łukasz Pankowski <lukpank at o2 dot pl>. All rights
// reserved.  This source code is licensed under the terms of the MIT
// license. See LICENSE file for details.

package jsonlexer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

type Lexer struct {
	r     io.Reader
	b     []byte
	start bool
	buf   [4096]byte
	err   error
	sbuf  bytes.Buffer
}

func New(r io.Reader) *Lexer {
	return &Lexer{r: r}
}

// Delim should be use for the following characters: "[]{}:"
func (l *Lexer) Delim(expected byte) error {
	b, err := l.nonSpaceByte()
	if err != nil {
		return err
	}
	l.b = l.b[1:]
	if b != expected {
		return fmt.Errorf("expected %q but found %q", expected, b)
	}
	if expected == '[' || expected == '{' {
		l.start = true
	}
	return nil
}

// More is used to check (and removing) of a comma (unless just after
// '{' or '['). Should not be called twice (one by one).
func (l *Lexer) More() (bool, error) {
	b, err := l.nonSpaceByte()
	if err != nil {
		return false, err
	}
	if b == ']' || b == '}' {
		return false, nil
	}
	if l.start {
		l.start = false
		return true, nil
	}
	if b != ',' {
		return false, fmt.Errorf("expected ',' but found %q", b)
	}
	l.b = l.b[1:]
	return true, nil
}

func (l *Lexer) EOF() error {
	b, err := l.nonSpaceByte()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return fmt.Errorf("expected EOF but found %q", b)
}

func (l *Lexer) Int64() (int64, error) {
	_, err := l.nonSpaceByte()
	if err != nil {
		return 0, err
	}
	j := -1
	for {
		for i, c := range l.b {
			if c == '-' && i == 0 {
				continue
			}
			if c < '0' || c > '9' {
				j = i
				break
			}
		}
		if j != -1 {
			break
		}
		if l.err == io.EOF {
			j = len(l.b)
			break
		}
		if len(l.b) == len(l.buf) {
			return 0, errors.New("int64 number to long")
		}
		n := copy(l.buf[:], l.b)
		var m int
		if l.err != nil {
			return 0, l.err
		}
		m, l.err = l.r.Read(l.buf[n:])
		if m == 0 && l.err != nil {
			if l.err == io.EOF {
				l.b = l.buf[:n+m]
				j = n + m
				break
			}
			return 0, l.err
		}
		l.b = l.buf[:n+m]
	}
	s := string(l.b[:j])
	l.b = l.b[j:]
	return strconv.ParseInt(s, 10, 64)
}

func (l *Lexer) Float64() (float64, error) {
	_, err := l.nonSpaceByte()
	if err != nil {
		return 0, err
	}
	j := -1
	for {
		for i, c := range l.b {
			if (c < '0' || c > '9') && c != '-' && c != '+' && c != '.' && c != 'e' && c != 'E' {
				j = i
				break
			}
		}
		if j != -1 {
			break
		}
		if l.err == io.EOF {
			j = len(l.b)
			break
		}
		if len(l.b) == len(l.buf) {
			return 0, errors.New("float64 number to long")
		}
		n := copy(l.buf[:], l.b)
		var m int
		if l.err != nil {
			return 0, l.err
		}
		m, l.err = l.r.Read(l.buf[n:])
		if m == 0 && l.err != nil {
			if l.err == io.EOF {
				l.b = l.buf[:n+m]
				j = n + m
				break
			}
			return 0, l.err
		}
		l.b = l.buf[:n+m]
	}
	s := string(l.b[:j])
	l.b = l.b[j:]
	return strconv.ParseFloat(s, 64)
}

func (l *Lexer) Bool() (bool, error) {
	b, err := l.nonSpaceByte()
	if err != nil {
		return false, err
	}
	var s string
	var v bool
	if b == 'f' {
		s = "false"
		v = false
	} else if b == 't' {
		s = "true"
		v = true
	} else {
		return false, errors.New(`expected true or false`)
	}
	for {
		for i, c := range l.b {
			if i == len(s) {
				l.b = l.b[i:]
				return v, nil
			}
			if c != s[i] {
				return false, errors.New(`expected true or false`)
			}
		}
		if len(l.b) >= len(s) || l.err == io.EOF {
			l.b = l.b[len(s):]
			return v, nil
		}
		if len(l.b) == len(l.buf) {
			return false, errors.New("bool value to long")
		}
		n := copy(l.buf[:], l.b)
		m, err := l.r.Read(l.buf[n:])
		if m == 0 && err != nil {
			if err == io.EOF {
				return false, io.ErrUnexpectedEOF
			}
			return false, err
		}
		l.b = l.buf[:n+m]
	}
}

func (l *Lexer) String() (string, error) {
	b, err := l.nonSpaceByte()
	if err != nil {
		return "", err
	}
	if b != '"' {
		return "", errors.New(`expected '"' to start string`)
	}
	j := len(l.b)
	for i, c := range l.b[1:] {
		if c == '\\' || c >= 0x80 {
			j = i + 1
			break
		}
		if c == '"' {
			j := i + 1
			s := string(l.b[1:j])
			l.b = l.b[j+1:]
			return s, nil
		}
	}
	l.sbuf.Reset()
	l.sbuf.Write(l.b[1:j])
	l.b = l.b[j:]

	escape := false
	for {
		k, err := l.complexStr(&escape)
		if err != nil {
			return "", err
		}
		if k != -1 {
			s := l.sbuf.String()
			l.sbuf.Reset()
			l.b = l.b[k+1:]
			return s, nil
		}

		n := copy(l.buf[:], l.b)
		var m int
		m, l.err = l.r.Read(l.buf[n:])
		if m == 0 && l.err != nil {
			if l.err == io.EOF {
				return "", errors.New(`expected '"' ending but EOF encountered`)
			}
			return "", l.err
		}
		l.b = l.buf[:n+m]
	}
}

func (l *Lexer) StringValue(expected string) error {
	b, err := l.nonSpaceByte()
	if err != nil {
		return err
	}
	if b != '"' {
		return errors.New(`expected '"' to start string`)
	}
	j := len(l.b)
	for i, c := range l.b[1:] {
		if c == '\\' || c >= 0x80 {
			j = i + 1
			break
		}
		if c == '"' {
			j := i + 1
			err := equal(l.b[1:j], expected)
			l.b = l.b[j+1:]
			return err
		}
	}
	l.sbuf.Reset()
	l.sbuf.Write(l.b[1:j])
	l.b = l.b[j:]

	escape := false
	for {
		k, err := l.complexStr(&escape)
		if err != nil {
			return err
		}
		if k != -1 {
			err := equal(l.sbuf.Bytes(), expected)
			l.sbuf.Reset()
			l.b = l.b[k+1:]
			return err
		}

		n := copy(l.buf[:], l.b)
		var m int
		m, l.err = l.r.Read(l.buf[n:])
		if m == 0 && l.err != nil {
			if l.err == io.EOF {
				return errors.New(`expected '"' ending but EOF encountered`)
			}
			return l.err
		}
		l.b = l.buf[:n+m]
	}
}

func equal(b []byte, s string) error {
	if len(b) != len(s) {
		return fmt.Errorf("expected string %q but got %q", s, b)
	}
	for i, c := range b {
		if c != s[i] {
			return fmt.Errorf("expected string %q but got %q", s, b)
		}
	}
	return nil
}

func (l *Lexer) complexStr(escape *bool) (int, error) {
	var x [2]byte
	i := 0
	for i < len(l.b) {
		c := l.b[i]
		if *escape {
			switch c {
			default:
				return 0, fmt.Errorf("unexpected escaped char %q", c)
			case '"', '\\', '/':
				l.sbuf.WriteByte(c)
			case 'b':
				l.sbuf.WriteByte('\b')
			case 'f':
				l.sbuf.WriteByte('\f')
			case 'n':
				l.sbuf.WriteByte('\n')
			case 'r':
				l.sbuf.WriteByte('\r')
			case 't':
				l.sbuf.WriteByte('\t')
			case 'u':
				if i+5 > len(l.b) {
					l.b = l.b[i:]
					return -1, nil
				}
				if _, err := hex.Decode(x[:], l.b[i+1:i+5]); err != nil {
					return 0, err
				}
				l.sbuf.WriteRune(rune(x[0])<<8 + rune(x[1]))
				i += 4

			}
			*escape = false
			i++
			continue
		}
		if c == '\\' {
			*escape = true
			i++
			continue
		}
		if c == '"' {
			return i, nil
		}
		if c >= 0x80 {
			if !utf8.FullRune(l.b[i:]) {
				l.b = l.b[i:]
				return -1, nil
			}
			r, n := utf8.DecodeRune(l.b[i:])
			l.sbuf.WriteRune(r)
			i += n
			continue
		}
		l.sbuf.WriteByte(c)
		i++
	}
	l.b = nil
	return -1, nil
}

func (l *Lexer) Skip() error {
	b, err := l.nonSpaceByte()
	if err != nil {
		return err
	}
	switch b {
	default:
		// TODO: support null
		return fmt.Errorf("unexpected byte %q", b)
	case '[':
		return l.skipArray()
	case '{':
		return l.skipDict()
	case '"':
		_, err := l.String()
		return err
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		_, err := l.Float64()
		return err
	case 't', 'f':
		_, err := l.Bool()
		return err
	}
}

func (l *Lexer) skipArray() error {
	if err := l.Delim('['); err != nil {
		return err
	}
	for {
		more, err := l.More()
		if err != nil {
			return err
		}
		if !more {
			break
		}
		if err := l.Skip(); err != nil {
			return err
		}
	}
	return l.Delim(']')
}

func (l *Lexer) skipDict() error {
	if err := l.Delim('{'); err != nil {
		return err
	}
	for {
		more, err := l.More()
		if err != nil {
			return err
		}
		if !more {
			break
		}
		if err := l.Skip(); err != nil {
			return err
		}
		if err := l.Delim(':'); err != nil {
			return err
		}
		if err := l.Skip(); err != nil {
			return err
		}
	}
	return l.Delim('}')
}

func (l *Lexer) nonSpaceByte() (byte, error) {
	for {
		if len(l.b) == 0 {
			if l.err != nil {
				return 0, l.err
			}
			var n int
			n, l.err = l.r.Read(l.buf[:])
			if n == 0 && l.err != nil {
				return 0, l.err
			}
			l.b = l.buf[:n]
		}
		for len(l.b) > 0 {
			b := l.b[0]
			if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
				return b, nil
			}
			l.b = l.b[1:]
		}
	}
}

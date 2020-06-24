// Copyright 2017 Łukasz Pankowski <lukpank at o2 dot pl>. All rights
// reserved.  This source code is licensed under the terms of the MIT
// license. See LICENSE file for details.

package jsonlexer_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/lukpank/jsonlexer"
)

func TestLexerEmptyArray(t *testing.T) {
	r := &readers{S: " [\r\n\t]"}
	for i := 0; i < 2*r.Len(); i++ {
		l := jsonlexer.New(r.Get(i))
		if err := l.Delim('['); err != nil {
			t.Fatalf("expected '[' but got error: %v", err)
		}
		if err := l.Delim(']'); err != nil {
			t.Fatalf("expected ']' but got error: %v", err)
		}
	}
}

func TestLexerEmptyArrayMore(t *testing.T) {
	for _, s := range []string{" [\r\n\t]", " [\r\n\t]\r\n"} {
		r := &readers{S: s}
		for i := 0; i < 2*r.Len(); i++ {
			l := jsonlexer.New(r.Get(i))
			if err := l.Delim('['); err != nil {
				t.Fatalf("expected '[' but got error: %v", err)
			}
			more, err := l.More()
			if err != nil {
				t.Fatalf("expected ',' but got error: %v", err)
			}
			if more {
				t.Fatal("expected false but got true")
			}
			if err := l.Delim(']'); err != nil {
				t.Fatalf("expected ']' but got error: %v", err)
			}
			if err := l.EOF(); err != nil {
				t.Fatalf("expected EOF but got error: %v", err)
			}
		}
	}
}

func TestLexerInt64(t *testing.T) {
	for i, s := range []string{" \r\n-123", " \r\n-123 "} {
		r := &readers{S: s}
		for j := 0; j < 2*r.Len(); j++ {
			t.Run(fmt.Sprintf("s%d/%d", i, j), func(t *testing.T) {
				l := jsonlexer.New(r.Get(j))
				expectedInt64(t, l, -123)
			})
		}
	}
}

func TestLexerArrayInt64(t *testing.T) {
	r := &readers{S: " [\r\n123, -84\t]"}
	for i := 0; i < 2*r.Len(); i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			l := jsonlexer.New(r.Get(i))
			if err := l.Delim('['); err != nil {
				t.Fatalf("expected '[' but got error: %v", err)
			}
			expectedMore(t, l)
			expectedInt64(t, l, 123)
			expectedMore(t, l)
			expectedInt64(t, l, -84)
			if err := l.Delim(']'); err != nil {
				t.Fatalf("expected ']' but got error: %v", err)
			}
		})
	}
}

func TestLexerFloat64(t *testing.T) {
	for i, s := range []string{" \r\n-1.5", " \r\n-1.5 "} {
		r := &readers{S: s}
		for j := 0; j < 2*r.Len(); j++ {
			t.Run(fmt.Sprintf("s%d/%d", i, j), func(t *testing.T) {
				l := jsonlexer.New(r.Get(j))
				expectedFloat64(t, l, -1.5)
			})
		}
	}
}

func TestLexerArrayFloat64(t *testing.T) {
	r := &readers{S: " [\r\n1e3, -3.25e2\t]"}
	for i := 0; i < 2*r.Len(); i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			l := jsonlexer.New(r.Get(i))
			if err := l.Delim('['); err != nil {
				t.Fatalf("expected '[' but got error: %v", err)
			}
			expectedMore(t, l)
			expectedFloat64(t, l, 1000)
			expectedMore(t, l)
			expectedFloat64(t, l, -325)
			if err := l.Delim(']'); err != nil {
				t.Fatalf("expected ']' but got error: %v", err)
			}
		})
	}
}

func TestLexerBool(t *testing.T) {
	for i, s := range []string{" \r\ntrue", " \r\ntrue ", " \r\nfalse", " \r\nfalse "} {
		r := &readers{S: s}
		for j := 0; j < 2*r.Len(); j++ {
			t.Run(fmt.Sprintf("s%d/%d", i, j), func(t *testing.T) {
				l := jsonlexer.New(r.Get(j))
				expectedBool(t, l, i < 2)
			})
		}
	}
}

func TestLexerArrayBool(t *testing.T) {
	r := &readers{S: " [\r\ntrue, false\t]"}
	for i := 0; i < 2*r.Len(); i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			l := jsonlexer.New(r.Get(i))
			if err := l.Delim('['); err != nil {
				t.Fatalf("expected '[' but got error: %v", err)
			}
			expectedMore(t, l)
			expectedBool(t, l, true)
			expectedMore(t, l)
			expectedBool(t, l, false)
			if err := l.Delim(']'); err != nil {
				t.Fatalf("expected ']' but got error: %v", err)
			}
		})
	}
}

func TestLexerString(t *testing.T) {
	cases := []struct{ input, output string }{
		{" \r\n\"test\"", "test"},
		{" \r\n\"test\" ", "test"},
		{`  "test\b\u0105ę\f\n\r\t"`, "test\bąę\f\n\r\t"},
		{` "test\b\u0105ę\f\n\r\t" `, "test\bąę\f\n\r\t"},
		{`  "test\b\u0105` + "\x80" + `ę\f\n\r\t"`, "test\bą\uFFFDę\f\n\r\t"},
		{` "test\b\u0105ę` + "\x80" + `\f\n\r\t" `, "test\bąę\uFFFD\f\n\r\t"},
	}
	for i, c := range cases {
		r := &readers{S: c.input}
		for j := 0; j < 2*r.Len(); j++ {
			t.Run(fmt.Sprintf("s%d/%s", i, r.Name(j)), func(t *testing.T) {
				l := jsonlexer.New(r.Get(j))
				expectedString(t, l, c.output)
			})
		}
	}
}

func TestLexerArrayString(t *testing.T) {
	r := &readers{S: " [\r\n\"test\", \"123\"\t]"}
	for i := 0; i < 2*r.Len(); i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			l := jsonlexer.New(r.Get(i))
			if err := l.Delim('['); err != nil {
				t.Fatalf("expected '[' but got error: %v", err)
			}
			expectedMore(t, l)
			expectedString(t, l, "test")
			expectedMore(t, l)
			expectedString(t, l, "123")
			if err := l.Delim(']'); err != nil {
				t.Fatalf("expected ']' but got error: %v", err)
			}
		})
	}
}

func TestLexerStringValue(t *testing.T) {
	cases := []struct{ input, output string }{
		{" \r\n\"test\"", "test"},
		{" \r\n\"test\" ", "test"},
		{`  "test\b\u0105ę\f\n\r\t"`, "test\bąę\f\n\r\t"},
		{` "test\b\u0105ę\f\n\r\t" `, "test\bąę\f\n\r\t"},
		{`  "test\b\u0105` + "\x80" + `ę\f\n\r\t"`, "test\bą\uFFFDę\f\n\r\t"},
		{` "test\b\u0105ę` + "\x80" + `\f\n\r\t" `, "test\bąę\uFFFD\f\n\r\t"},
	}
	for i, c := range cases {
		r := &readers{S: c.input}
		for j := 0; j < 2*r.Len(); j++ {
			t.Run(fmt.Sprintf("s%d/%s", i, r.Name(j)), func(t *testing.T) {
				l := jsonlexer.New(r.Get(j))
				expectedStringValue(t, l, c.output)
				l = jsonlexer.New(r.Get(j))
				err := l.StringValue(c.output[1:])
				if err == nil {
					t.Fatal("expected error")
				}
			})
		}
	}
}

func expectedMore(t *testing.T, l *jsonlexer.Lexer) {
	more, err := l.More()
	if err != nil {
		t.Fatalf("expected more but got error: %v", err)
	}
	if !more {
		t.Fatal("expected more but got false")
	}
}

func expectedInt64(t *testing.T, l *jsonlexer.Lexer, expected int64) {
	got, err := l.Int64()
	if err != nil {
		t.Fatalf("expected %d but got error: %v", expected, err)
	}
	if got != expected {
		t.Errorf("expected %d but got: %d", expected, got)
	}
}

func expectedFloat64(t *testing.T, l *jsonlexer.Lexer, expected float64) {
	got, err := l.Float64()
	if err != nil {
		t.Fatalf("expected %g but got error: %v", expected, err)
	}
	if got != expected {
		t.Errorf("expected %g but got: %g", expected, got)
	}
}

func expectedBool(t *testing.T, l *jsonlexer.Lexer, expected bool) {
	got, err := l.Bool()
	if err != nil {
		t.Fatalf("expected %t but got error: %v", expected, err)
	}
	if got != expected {
		t.Errorf("expected %t but got: %t", expected, got)
	}
}

func expectedString(t *testing.T, l *jsonlexer.Lexer, expected string) {
	got, err := l.String()
	if err != nil {
		t.Fatalf("expected %q but got error: %v", expected, err)
	}
	if got != expected {
		t.Errorf("expected %q but got: %q", expected, got)
	}
}

func expectedStringValue(t *testing.T, l *jsonlexer.Lexer, expected string) {
	err := l.StringValue(expected)
	if err != nil {
		t.Fatalf("expected %q but got error: %v", expected, err)
	}
}

func TestSplit1StringsReader(t *testing.T) {
	r := &split1StringReader{s: "test1", split: 2}
	b := make([]byte, 10)
	n, err := r.Read(b)
	if err != nil || n != 2 || string(b[:n]) != "te" {
		t.Fatalf(`expected "te" but got %q (%v, %q)`, b[:n], n, err)
	}
	n, err = r.Read(b)
	if err != nil && err != io.EOF || n != 3 || string(b[:n]) != "st1" {
		t.Fatalf(`expected "st1" but got %q (%d, %v)`, b[:n], n, err)
	}
	n, err = r.Read(b)
	if err != io.EOF || n != 0 {
		t.Fatalf(`expected (0, io.EOF) but got %q (%d, %v)`, b[:n], n, err)
	}
}

func TestSplitNStringsReader(t *testing.T) {
	r := &splitNStringReader{s: "test1", split: 2}
	b := make([]byte, 10)
	n, err := r.Read(b)
	if err != nil || n != 2 || string(b[:n]) != "te" {
		t.Fatalf(`expected "te" but got %q (%v, %q)`, b[:n], n, err)
	}
	n, err = r.Read(b)
	if err != nil || n != 2 || string(b[:n]) != "st" {
		t.Fatalf(`expected "st1" but got %q (%d, %v)`, b[:n], n, err)
	}
	n, err = r.Read(b)
	if err != nil && err != io.EOF || n != 1 || string(b[:n]) != "1" {
		t.Fatalf(`expected "st1" but got %q (%d, %v)`, b[:n], n, err)
	}
	n, err = r.Read(b)
	if err != io.EOF || n != 0 {
		t.Fatalf(`expected (0, io.EOF) but got %q (%d, %v)`, b[:n], n, err)
	}
}

type readers struct {
	S string
	i int
}

func (r *readers) Len() int {
	return (3*len(r.S) - 1) / 2
}

func (r *readers) Get(i int) io.Reader {
	rLen := r.Len()
	earlyEOF := i > rLen
	if i > rLen {
		i -= rLen
	}
	if i < len(r.S) {
		return &split1StringReader{s: r.S, split: i + 1, earlyEOF: earlyEOF}
	}
	i -= len(r.S) - 1
	return &splitNStringReader{s: r.S, split: i, earlyEOF: earlyEOF}
}

func (r *readers) Name(i int) string {
	if i < len(r.S) {
		return fmt.Sprintf("1_%d", i+1)
	}
	i -= len(r.S) - 1
	return fmt.Sprintf("N_%d", i)
}

type split1StringReader struct {
	s        string
	split    int
	pos      int
	earlyEOF bool
}

func (r *split1StringReader) Read(b []byte) (int, error) {
	if r.pos == len(r.s) {
		return 0, io.EOF
	}
	if r.pos < r.split {
		n := copy(b, r.s[r.pos:r.split])
		r.pos += n
		return n, nil
	}
	n := copy(b, r.s[r.pos:])
	r.pos += n
	if r.pos == len(r.s) { // only if earlyEOF
		return n, io.EOF
	}
	return n, nil
}

type splitNStringReader struct {
	s        string
	split    int
	pos      int
	earlyEOF bool
}

func (r *splitNStringReader) Read(b []byte) (int, error) {
	if r.pos == len(r.s) {
		return 0, io.EOF
	}
	end := r.pos + r.split
	if end > len(r.s) {
		end = len(r.s)
	}
	n := copy(b, r.s[r.pos:end])
	r.pos += n
	if r.pos == len(r.s) { // only if earlyEOF
		return n, io.EOF
	}
	return n, nil
}

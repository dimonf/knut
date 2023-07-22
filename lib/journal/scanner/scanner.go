// Copyright 2021 Silvio Böhler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scanner

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Scanner is a backtracking reader.
type Scanner struct {
	text string

	reader io.RuneReader

	// current contains the current rune
	current rune
	// Path is the file path.
	Path string
	// Location is the current position in the stream.
	Location Location
}

// New creates a new Scanner.
func New(text, path string) (*Scanner, error) {
	r := strings.NewReader(text)
	ch, _, err := r.ReadRune()
	if err != nil {
		if err != io.EOF {
			return nil, err
		}
		ch = EOF
	}
	return &Scanner{
		reader:  r,
		text:    text,
		current: ch,
		Path:    path,
		Location: Location{
			Line:    1,
			Column:  1,
			BytePos: 0,
			RunePos: 0,
		},
	}, nil
}

// ReadRune implements io.RuneReader.
func (s *Scanner) ReadRune() (r rune, size int, err error) {
	if err := s.Advance(); err != nil {
		return 0, 0, err
	}
	return s.Current(), utf8.RuneLen(s.Current()), nil
}

// Current returns the current rune.
func (s *Scanner) Current() rune {
	return s.current
}

// ParseError creates a new parser error with the current position.
func (s *Scanner) ParseError(err error) error {
	return fmt.Errorf("%s:%s: %v", s.Path, s.Location, err)
}

// Advance reads a rune.
func (s *Scanner) Advance() error {
	ch, _, err := s.reader.ReadRune()
	if err != nil {
		if err != io.EOF {
			return err
		}
		ch = EOF
	}
	s.Location.BytePos += utf8.RuneLen(s.current)
	s.Location.RunePos++
	if s.current == '\n' {
		s.Location.Line++
		s.Location.Column = 1
	} else {
		s.Location.Column++
	}
	s.current = ch
	return nil
}

// EOF is a rune representing the end of a file
const EOF = rune(0)

// ReadWhile reads runes into the builder while the predicate holds
func (s *Scanner) ReadWhile(pred func(r rune) bool) (string, error) {
	start := s.Location.BytePos
	for pred(s.Current()) && s.Current() != EOF {
		if err := s.Advance(); err != nil {
			return s.text[start:s.Location.BytePos], err
		}
	}
	return s.text[start:s.Location.BytePos], nil
}

// ConsumeWhile advances the parser while the predicate holds
func (s *Scanner) ConsumeWhile(pred func(r rune) bool) error {
	for pred(s.Current()) {
		if err := s.Advance(); err != nil {
			return err
		}
	}
	return nil
}

// ConsumeUntil advances the parser until the predicate holds
func (s *Scanner) ConsumeUntil(pred func(r rune) bool) error {
	for !pred(s.Current()) {
		if err := s.Advance(); err != nil {
			return err
		}
	}
	return nil
}

// ConsumeRune consumes the given rune
func (s *Scanner) ConsumeRune(r rune) error {
	if s.Current() != r {
		return fmt.Errorf("expected %c, got %c", r, s.Current())
	}
	return s.Advance()
}

// ParseString parses the given string
func (s *Scanner) ParseString(str string) error {
	start := s.Location.BytePos
	for _, ch := range str {
		if ch != s.Current() {
			return fmt.Errorf("expected %v, got %v", str, s.text[start:s.Location.BytePos])
		}
		if err := s.Advance(); err != nil {
			return err
		}
	}
	return nil
}

// ReadN reads a string with n runes
func (s *Scanner) ReadN(n int) (string, error) {
	start := s.Location.BytePos
	for i := 0; i < n; i++ {
		if err := s.Advance(); err != nil {
			return "", err
		}
	}
	return s.text[start:s.Location.BytePos], nil
}

// Location describes a location in the Scanner's stream.
type Location struct {
	BytePos, RunePos, Line, Column int
}

func (p Location) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

func FindLocation(text string, pos int) (Location, error) {
	res := Location{Line: 1, Column: 1}
	if pos > len(text) {
		return res, fmt.Errorf("invalid pos %d for string of length %d", pos, len(text))
	}
	r := strings.NewReader(text)
	for offset := 0; offset < pos; {
		ch, c, err := r.ReadRune()
		if err != nil {
			return res, err
		}
		if ch == '\n' {
			res.Line++
			res.Column = 1
		} else {
			res.Column++
		}
		offset += c
	}
	return res, nil
}

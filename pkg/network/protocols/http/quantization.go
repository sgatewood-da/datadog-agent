// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package http contains the userspace portion of USM's HTTP monitoring code
package http

import (
	"bytes"
)

// URLQuantizer is responsible for quantizing URLs
type URLQuantizer struct {
	tokenizer *Tokenizer
	buf       *bytes.Buffer
}

// NewURLQuantizer returns a new instance of a URLQuantizer
func NewURLQuantizer() *URLQuantizer {
	return &URLQuantizer{
		tokenizer: NewTokenizer(),
		buf:       bytes.NewBuffer(nil),
	}
}

// Quantize path (eg /segment1/segment2/segment3) by doing the following:
// * If a segment contains only letters, we keep it as it is;
// * If a segment contains one or more digits or special characters, we replace it by '*'
// * If a segments represents an API version (eg. v123) we keep it as it is
//
// Note that the quantization happens *in-place* and the supplied argument byte
// slice is modified, so the returned value will still point to the same
// underlying byte array.
func (q *URLQuantizer) Quantize(path []byte) []byte {
	q.tokenizer.Reset(path)
	q.buf.Reset()
	replacements := 0

	for q.tokenizer.Next() {
		q.buf.WriteByte('/')
		tokenType, tokenValue := q.tokenizer.Value()
		if tokenType == TokenWildcard {
			replacements++
			q.buf.WriteByte('*')
			continue
		}

		q.buf.Write(tokenValue)
	}

	if replacements == 0 {
		return path
	}

	// Copy quantized path into original byte slice
	n := copy(path[:], q.buf.Bytes())

	return path[:n]
}

// TokenType represents a type of token handled by the `Tokenizer`
type TokenType int

const (
	// TokenUnknown represents a token of type unknown
	TokenUnknown = TokenType(0)
	// TokenWildcard represents a token that contains digits or special chars
	TokenWildcard = TokenType(1)
	// TokenString represents a token that contains only letters
	TokenString = TokenType(2)
	// TokenAPIVersion represents an API version (eg. v123)
	TokenAPIVersion = TokenType(3)
)

// Tokenizer provides a stream of tokens given a certain URL string
type Tokenizer struct {
	pos  int
	path []byte
	buf  *bytes.Buffer

	ttype             TokenType
	countAllowedChars int // a-Z, "-", "_"
	countNumbers      int // 0-9
	countSpecialChars int // anything else
}

// NewTokenizer returns a new Tokenizer
func NewTokenizer() *Tokenizer {
	return &Tokenizer{buf: bytes.NewBuffer(nil)}
}

// Next attempts to parse the next token, and returns true if a token was read
func (t *Tokenizer) Next() bool {
	t.countNumbers = 0
	t.countAllowedChars = 0
	t.countSpecialChars = 0
	t.buf.Reset()

	if t.pos >= len(t.path)-1 {
		t.ttype = TokenUnknown
		return false
	}

	if t.pos < len(t.path) && t.path[t.pos] == '/' {
		t.pos++
	}

	for ; t.pos < len(t.path); t.pos++ {
		c := t.path[t.pos]

		if c == '/' {
			break
		} else if c >= '0' && c <= '9' {
			t.countNumbers++
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '_' {
			t.countAllowedChars++
		} else {
			t.countSpecialChars++
		}

		t.buf.WriteByte(c)
	}

	t.ttype = t.getType()
	return true
}

// Reset underlying path being consumed
func (t *Tokenizer) Reset(path []byte) {
	t.pos = 0
	t.path = path
	t.ttype = TokenUnknown
}

// Value returns the current token along with it's byte value
// Note that the byte value is only valid until the next call to `Reset()`
func (t *Tokenizer) Value() (TokenType, []byte) {
	return t.getType(), t.buf.Bytes()
}

func (t *Tokenizer) getType() TokenType {
	firstByte, _ := t.buf.ReadByte()
	_ = t.buf.UnreadByte()

	if t.countAllowedChars == 1 && t.countNumbers > 0 && firstByte == 'v' {
		return TokenAPIVersion
	}

	if t.countSpecialChars > 0 || t.countNumbers > 0 {
		return TokenWildcard
	}

	if t.countAllowedChars > 0 && t.countSpecialChars == 0 && t.countNumbers == 0 {
		return TokenString
	}

	return TokenUnknown
}

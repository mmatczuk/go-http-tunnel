// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package id

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/calmh/luhn"
)

// ID is the type representing a generated ID.
type ID [32]byte

// New generates a new ID from the given input bytes.
func New(data []byte) ID {
	var id ID

	hasher := sha256.New()
	hasher.Write(data)
	hasher.Sum(id[:0])

	return id
}

// String returns the canonical representation of the ID.
func (i ID) String() string {
	ss := base32.StdEncoding.EncodeToString(i[:])
	ss = strings.Trim(ss, "=")

	// Add a Luhn check 'digit' for the ID.
	ss, err := luhnify(ss)
	if err != nil {
		// Should never happen
		panic(err)
	}

	// Return the given ID as chunks.
	ss = chunkify(ss)

	return ss
}

// Compares the two given IDs.  Note that this function is NOT SAFE AGAINST
// TIMING ATTACKS.  If you are simply checking for equality, please use the
// Equals function, which is.
func (i ID) Compare(other ID) int {
	return bytes.Compare(i[:], other[:])
}

// Checks the two given IDs for equality.  This function uses a constant-time
// comparison algorithm to prevent timing attacks.
func (i ID) Equals(other ID) bool {
	return subtle.ConstantTimeCompare(i[:], other[:]) == 1
}

// Implements the `TextMarshaler` interface from the encoding package.
func (i *ID) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// Implements the `TextUnmarshaler` interface from the encoding package.
func (i *ID) UnmarshalText(bs []byte) (err error) {
	// Convert to the canonical encoding - uppercase, no '=', no chunks, and
	// with any potential typos fixed.
	id := string(bs)
	id = strings.Trim(id, "=")
	id = strings.ToUpper(id)
	id = untypeoify(id)
	id = unchunkify(id)

	if len(id) != 56 {
		return errors.New("device ID invalid: incorrect length")
	}

	// Remove & verify Luhn check digits
	id, err = unluhnify(id)
	if err != nil {
		return err
	}

	// Base32 decode
	dec, err := base32.StdEncoding.DecodeString(id + "====")
	if err != nil {
		return err
	}

	// Done!
	copy(i[:], dec)
	return nil
}

// Add Luhn check digits to a string, returning the new one.
func luhnify(s string) (string, error) {
	if len(s) != 52 {
		panic("unsupported string length")
	}

	// Split the string into chunks of length 13, and add a Luhn check digit to
	// each one.
	res := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		chunk := s[i*13 : (i+1)*13]

		l, err := luhn.Base32.Generate(chunk)
		if err != nil {
			return "", err
		}

		res = append(res, fmt.Sprintf("%s%c", chunk, l))
	}

	return res[0] + res[1] + res[2] + res[3], nil
}

// Remove Luhn check digits from the given string, validating that they are
// correct.
func unluhnify(s string) (string, error) {
	if len(s) != 56 {
		return "", fmt.Errorf("unsupported string length %d", len(s))
	}

	res := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		// 13 characters, plus the Luhn digit.
		chunk := s[i*14 : (i+1)*14]

		// Get the expected check digit.
		l, err := luhn.Base32.Generate(chunk[0:13])
		if err != nil {
			return "", err
		}

		// Validate the digits match.
		if fmt.Sprintf("%c", l) != chunk[13:] {
			return "", errors.New("check digit incorrect")
		}

		res = append(res, chunk[0:13])
	}

	return res[0] + res[1] + res[2] + res[3], nil
}

// Returns a string split into chunks of size 7.
func chunkify(s string) string {
	s = regexp.MustCompile("(.{7})").ReplaceAllString(s, "$1-")
	s = strings.Trim(s, "-")
	return s
}

// Un-chunks a string by removing all hyphens and spaces.
func unchunkify(s string) string {
	s = strings.Replace(s, "-", "", -1)
	s = strings.Replace(s, " ", "", -1)
	return s
}

// We use base32 encoding, which uses 26 characters, and then the numbers
// 234567.  This is useful since the alphabet doesn't contain the numbers 0, 1,
// or 8, which means we can replace them with their letter-lookalikes.
func untypeoify(s string) string {
	s = strings.Replace(s, "0", "O", -1)
	s = strings.Replace(s, "1", "I", -1)
	s = strings.Replace(s, "8", "B", -1)
	return s
}

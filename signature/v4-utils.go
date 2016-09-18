/*
 * Minio Cloud Storage, (C) 2015, 2016 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	. "git.letv.cn/yig/yig/error"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// http Header "x-amz-content-sha256" == "UNSIGNED-PAYLOAD" indicates that the
// client did not calculate sha256 of the payload.
const (
	UnsignedPayload = "UNSIGNED-PAYLOAD"
	REGION          = "cn-bj-1"
)

// isValidRegion - verify if incoming region value is valid with configured Region.
// TODO
func isValidRegion(reqRegion string) bool {
	if reqRegion == "" {
		return true
	}
	return reqRegion == REGION
}

// sumHMAC calculate hmac between two input byte array.
func sumHMAC(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

// Reserved string regexp.
var reservedNames = regexp.MustCompile("^[a-zA-Z0-9-_.~/]+$")

// getURLEncodedName encode the strings from UTF-8 byte representations to HTML hex escape sequences
//
// This is necessary since regular url.Parse() and url.Encode() functions do not support UTF-8
// non english characters cannot be parsed due to the nature in which url.Encode() is written
//
// This function on the other hand is a direct replacement for url.Encode() technique to support
// pretty much every UTF-8 character.
func getURLEncodedName(name string) string {
	// if object matches reserved string, no need to encode them
	if reservedNames.MatchString(name) {
		return name
	}
	var encodedName string
	for _, s := range name {
		if 'A' <= s && s <= 'Z' || 'a' <= s && s <= 'z' || '0' <= s && s <= '9' { // §2.3 Unreserved characters (mark)
			encodedName = encodedName + string(s)
			continue
		}
		switch s {
		case '-', '_', '.', '~', '/': // §2.3 Unreserved characters (mark)
			encodedName = encodedName + string(s)
			continue
		default:
			len := utf8.RuneLen(s)
			if len < 0 {
				return name
			}
			u := make([]byte, len)
			utf8.EncodeRune(u, s)
			for _, r := range u {
				hex := hex.EncodeToString([]byte{r})
				encodedName = encodedName + "%" + strings.ToUpper(hex)
			}
		}
	}
	return encodedName
}

// getCanonicalHeaders extract signed headers from Authorization header and form the required string:
//
// Lowercase(<HeaderName1>)+":"+Trim(<value>)+"\n"
// Lowercase(<HeaderName2>)+":"+Trim(<value>)+"\n"
// ...
// Lowercase(<HeaderNameN>)+":"+Trim(<value>)+"\n"
//
// Return ErrMissingRequiredSignedHeader if a header is missing in http header but exists in signedHeaders
func getCanonicalHeaders(signedHeaders []string, req *http.Request) (string, error) {
	canonicalHeaders := ""
	for _, header := range signedHeaders {
		values, ok := req.Header[http.CanonicalHeaderKey(header)]
		// Golang http server strips off 'Expect' header, if the
		// client sent this as part of signed headers we need to
		// handle otherwise we would see a signature mismatch.
		// `aws-cli` sets this as part of signed headers.
		//
		// According to
		// http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.20
		// Expect header is always of form:
		//
		//   Expect       =  "Expect" ":" 1#expectation
		//   expectation  =  "100-continue" | expectation-extension
		//
		// So it safe to assume that '100-continue' is what would
		// be sent, for the time being keep this work around.
		// Adding a *TODO* to remove this later when Golang server
		// doesn't filter out the 'Expect' header.
		if header == "expect" {
			values = []string{"100-continue"}
			ok = true
		}
		// Golang http server promotes 'Host' header to Request.Host field
		// and removed from the Header map.
		if header == "host" {
			values = []string{req.Host}
			ok = true
		}
		if !ok {
			return "", ErrMissingRequiredSignedHeader
		}
		canonicalHeaders += header + ":"
		for idx, v := range values {
			if idx > 0 {
				canonicalHeaders += ","
			}
			canonicalHeaders += v
		}
		canonicalHeaders += "\n"
	}
	return canonicalHeaders, nil
}

// SPDX-License-Identifier: BSD-3-Clause

//
// References:
//
// - https://datatracker.ietf.org/doc/html/rfc8446
// - https://datatracker.ietf.org/doc/html/rfc6066
// - https://tls13.xargs.org/#client-hello
//
// Adapted from: https://github.com/ooni/netem

package dissector

import (
	"fmt"

	"golang.org/x/crypto/cryptobyte"
)

// TLSDetail contains a dissected TLS ClientHello.
type TLSDetail struct {
	// Record is the TLS record header.
	Record *TLSRecordDetail `json:"record"`

	// Handshake is the handshake message detail.
	Handshake *TLSHandshakeDetail `json:"handshake,omitempty"`

	// SNI is the extracted server name (convenience field).
	SNI string `json:"sni,omitempty"`
}

// TLSRecordDetail contains TLS record header fields.
type TLSRecordDetail struct {
	// ContentType is the content type name and number.
	ContentType string `json:"content_type"`

	// Version is the protocol version string.
	Version string `json:"version"`

	// Length is the record payload length.
	Length uint16 `json:"length"`
}

// TLSHandshakeDetail contains TLS handshake message fields.
type TLSHandshakeDetail struct {
	// Type is the handshake type name and number.
	Type string `json:"type"`

	// Version is the ClientHello protocol version string.
	Version string `json:"version"`

	// RandomLen is the length of the random field (always 32).
	RandomLen int `json:"random_length"`

	// SessionIDLen is the length of the legacy session ID.
	SessionIDLen int `json:"session_id_length"`

	// CipherSuitesCount is the number of cipher suites offered.
	CipherSuitesCount int `json:"cipher_suites_count"`

	// CompressionMethodsCount is the number of compression methods.
	CompressionMethodsCount int `json:"compression_methods_count"`

	// Extensions is the list of extensions with name, type, and length.
	Extensions []TLSExtensionSummary `json:"extensions"`
}

// TLSExtensionSummary summarizes a single TLS extension.
type TLSExtensionSummary struct {
	// Name is the human-readable extension name (e.g., "server_name").
	Name string `json:"name"`

	// Type is the extension type number.
	Type uint16 `json:"type"`

	// Length is the extension data length in bytes.
	Length int `json:"length"`

	// Value is the parsed value (only for extensions we dissect).
	Value string `json:"value,omitempty"`
}

// dissectTLS attempts to parse a TCP payload as a TLS ClientHello.
// Returns nil if the payload is not a ClientHello.
func dissectTLS(payload []byte) *TLSDetail {
	if len(payload) <= 0 {
		return nil
	}

	cursor := cryptobyte.String(payload)

	// Parse the TLS record header.
	var contentType uint8
	if !cursor.ReadUint8(&contentType) {
		return nil
	}
	if contentType != 22 { // handshake
		return nil
	}

	var protoVersion uint16
	if !cursor.ReadUint16(&protoVersion) {
		return nil
	}

	var recordLen uint16
	if !cursor.ReadUint16(&recordLen) {
		return nil
	}

	var recordBody cryptobyte.String
	if !cursor.ReadBytes((*[]byte)(&recordBody), min(int(recordLen), len(cursor))) {
		return nil
	}

	detail := &TLSDetail{
		Record: &TLSRecordDetail{
			ContentType: tlsContentTypeName(contentType),
			Version:     tlsVersionName(protoVersion),
			Length:       recordLen,
		},
	}

	// Parse the handshake message header.
	var handshakeType uint8
	if !recordBody.ReadUint8(&handshakeType) {
		return detail
	}
	if handshakeType != 1 { // client_hello
		return detail
	}

	// Read the 3-byte handshake length but don't require all bytes
	// to be present — the TLS record may span multiple TCP segments.
	var hsLenBytes []byte
	if !recordBody.ReadBytes(&hsLenBytes, 3) {
		return detail
	}
	handshakeBody := recordBody

	// Parse the ClientHello fields.
	hs := &TLSHandshakeDetail{
		Type: fmt.Sprintf("ClientHello (%d)", handshakeType),
	}
	detail.Handshake = hs

	// Version.
	var chVersion uint16
	if !handshakeBody.ReadUint16(&chVersion) {
		return detail
	}
	hs.Version = tlsVersionName(chVersion)

	// Random (32 bytes).
	var random []byte
	if !handshakeBody.ReadBytes(&random, 32) {
		return detail
	}
	hs.RandomLen = len(random)

	// Legacy session ID.
	var sessionID cryptobyte.String
	if !handshakeBody.ReadUint8LengthPrefixed(&sessionID) {
		return detail
	}
	hs.SessionIDLen = len(sessionID)

	// Cipher suites (2 bytes each).
	var cipherSuites cryptobyte.String
	if !handshakeBody.ReadUint16LengthPrefixed(&cipherSuites) {
		return detail
	}
	hs.CipherSuitesCount = len(cipherSuites) / 2

	// Compression methods.
	var compressionMethods cryptobyte.String
	if !handshakeBody.ReadUint8LengthPrefixed(&compressionMethods) {
		return detail
	}
	hs.CompressionMethodsCount = len(compressionMethods)

	// Extensions — read the 2-byte length manually, then use whatever
	// bytes we have. The extensions often span beyond the first TCP
	// segment since they come at the end of the ClientHello.
	var extLen uint16
	if !handshakeBody.ReadUint16(&extLen) {
		return detail
	}
	extensions := cryptobyte.String(handshakeBody[:min(int(extLen), len(handshakeBody))])

	for !extensions.Empty() {
		var extType uint16
		if !extensions.ReadUint16(&extType) {
			break
		}

		var extData cryptobyte.String
		if !extensions.ReadUint16LengthPrefixed(&extData) {
			break
		}

		summary := TLSExtensionSummary{
			Name:   tlsExtensionName(extType),
			Type:   extType,
			Length: len(extData),
		}

		// Parse the SNI extension (type 0).
		if extType == 0 {
			if sni := parseSNI(extData); sni != "" {
				summary.Value = sni
				detail.SNI = sni
			}
		}

		hs.Extensions = append(hs.Extensions, summary)
	}

	return detail
}

// parseSNI extracts the server name from an SNI extension value.
func parseSNI(data cryptobyte.String) string {
	var serverNameList cryptobyte.String
	if !data.ReadUint16LengthPrefixed(&serverNameList) {
		return ""
	}

	for !serverNameList.Empty() {
		var nameType uint8
		if !serverNameList.ReadUint8(&nameType) {
			return ""
		}

		var hostName cryptobyte.String
		if !serverNameList.ReadUint16LengthPrefixed(&hostName) {
			return ""
		}

		if nameType == 0 { // host_name
			return string(hostName)
		}
	}

	return ""
}

// tlsInfoLine builds a Wireshark-style info string for a TLS ClientHello.
func tlsInfoLine(d *TLSDetail) string {
	if d.SNI != "" {
		return "TLS ClientHello SNI=" + d.SNI
	}
	if d.Handshake != nil {
		return "TLS ClientHello"
	}
	return "TLS"
}

// tlsContentTypeName maps a TLS content type to a display name.
func tlsContentTypeName(ct uint8) string {
	switch ct {
	case 20:
		return "ChangeCipherSpec (20)"
	case 21:
		return "Alert (21)"
	case 22:
		return "Handshake (22)"
	case 23:
		return "ApplicationData (23)"
	default:
		return fmt.Sprintf("Unknown (%d)", ct)
	}
}

// tlsVersionName maps a TLS protocol version to a display name.
func tlsVersionName(v uint16) string {
	switch v {
	case 0x0301:
		return "TLS 1.0"
	case 0x0302:
		return "TLS 1.1"
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", v)
	}
}

// tlsExtensionName maps a TLS extension type to a display name.
func tlsExtensionName(t uint16) string {
	switch t {
	case 0:
		return "server_name"
	case 1:
		return "max_fragment_length"
	case 5:
		return "status_request"
	case 10:
		return "supported_groups"
	case 11:
		return "ec_point_formats"
	case 13:
		return "signature_algorithms"
	case 14:
		return "use_srtp"
	case 15:
		return "heartbeat"
	case 16:
		return "application_layer_protocol_negotiation"
	case 18:
		return "signed_certificate_timestamp"
	case 21:
		return "padding"
	case 23:
		return "extended_master_secret"
	case 35:
		return "session_ticket"
	case 41:
		return "pre_shared_key"
	case 42:
		return "early_data"
	case 43:
		return "supported_versions"
	case 44:
		return "cookie"
	case 45:
		return "psk_key_exchange_modes"
	case 47:
		return "certificate_authorities"
	case 49:
		return "post_handshake_auth"
	case 50:
		return "signature_algorithms_cert"
	case 51:
		return "key_share"
	case 0xff01:
		return "renegotiation_info"
	default:
		return fmt.Sprintf("unknown_%d", t)
	}
}

package mail

import (
	"fmt"
	"strings"
)

type HeaderMode int

const (
	Rfc5322Header HeaderMode = iota
	MimeHeader
)

type Header struct {
	mode   HeaderMode
	Fields Fields

	err      error
	verified bool
}

func ReadHeader(rfc5322 string, m HeaderMode) (h *Header, err error) {
	h = &Header{mode: m}
	done := false

	i := 0
	end := len(rfc5322)

	for !done {
		if i >= end {
			done = true
		}

		// Skip past UTF8 byte order mark (BOM) if present
		if i+2 < end && rfc5322[i] == 0xEF && rfc5322[i+1] == 0xBB && rfc5322[i+2] == 0xBF {
			i += 3
		}

		j := i
		for j < end && rfc5322[j] >= 33 && rfc5322[j] <= 127 && rfc5322[j] != ':' {
			j++
		}

		if j == i+4 && m == Rfc5322Header && strings.ToLower(rfc5322[i:j+1]) == "from " {
			for i < end && rfc5322[i] != '\r' && rfc5322[i] != '\n' {
				i++
			}
			for rfc5322[i] == '\r' {
				i++
			}
			if rfc5322[i] == '\n' {
				i++
			}
		} else if j > i && rfc5322[j] == ':' {
			name := rfc5322[i:j]
			i = j
			i++
			for rfc5322[i] == ' ' || rfc5322[i] == '\t' {
				i++
			}
			j = i

			// Find the end of the value, including multiline values
			// NOTE: Deviates from https://github.com/aox/aox/blob/master/message/message.cpp#L224
			for j < end && (rfc5322[j] != '\n' || (j+1 < end && (rfc5322[j+1] == ' ' || rfc5322[j+1] == '\t'))) {
				j++
			}
			if j > 0 && rfc5322[j-1] == '\r' {
				j--
			}
			value := rfc5322[i:j]
			//233-237
			if simplify(value) != "" || strings.HasPrefix(strings.ToLower(name), "x-") {
				f := NewHeaderField(name, value)
				h.Add(f)
			}
			i = j
			if i+1 < end && rfc5322[i] == '\r' && rfc5322[i+1] == '\n' {
				i++
			}
			i++
		} else {
			done = true
		}
	}

	return h, nil
}

// Returns true if this Header fills all the conditions laid out in RFC 2821
// for validity, and false if not.
func (h *Header) Valid() bool {
	h.Verify()
	return h.err == nil
}

func (h *Header) Add(f Field) {
	h.Fields = append(h.Fields, f)
}

func (h *Header) field(fn string, n int) Field {
	for _, field := range h.Fields {
		if field.Name() == fn {
			if n > 0 {
				n--
			} else {
				return field
			}
		}
	}

	return nil
}

// Returns a pointer to the address field of type \a t at index \a n in this
// header, or a null pointer if no such field exists.
func (h *Header) addressField(fn string, n int) *AddressField {
	switch fn {
	case FromFieldName, ResentFromFieldName, SenderFieldName, ResentSenderFieldName,
		ReturnPathFieldName, ReplyToFieldName, ToFieldName, CcFieldName, BccFieldName,
		ResentToFieldName, ResentCcFieldName, ResentBccFieldName, MessageIdFieldName,
		ContentIdFieldName, ResentMessageIdFieldName, ReferencesFieldName:
		f, _ := h.field(fn, n).(*AddressField)
		return f
	}
	return nil
}

// Returns a pointer to the addresses in the \a t header field, which must be
// an address field such as From or Bcc. If not, or if the field is empty,
// addresses() returns a null pointer.
func (h *Header) addresses(fn string) []Address {
	as := []Address{}
	af := h.addressField(fn, 0)
	if af != nil {
		as = af.Addresses
	}
	if len(as) == 0 {
		as = nil
	}
	return as
}

// Returns a pointer to the Content-Type header field, or a null pointer if
// there isn't one.
func (h *Header) ContentType() *ContentType {
	f := h.field(ContentTypeFieldName, 0)
	if f == nil {
		return nil
	}

	return f.(*ContentType)
}

// Returns a pointer to the Content-Transfer-Encoding header field, or a null
// pointer if there isn't one.
func (h *Header) ContentTransferEncoding() *ContentTransferEncoding {
	f := h.field(ContentTransferEncodingFieldName, 0)
	if f == nil {
		return nil
	}

	return f.(*ContentTransferEncoding)
}

// Returns a pointer to the Content-Disposition header field, or a null pointer
// if there isn't one.
func (h *Header) ContentDisposition() *ContentDisposition {
	f := h.field(ContentDispositionFieldName, 0)
	if f == nil {
		return nil
	}

	return f.(*ContentDisposition)
}

// Returns the value of the Content-Description field, or an empty string if
// there isn't one. RFC 2047 encoding is not considered - should it be?
func (h *Header) ContentDescription() string {
	f := h.field(ContentDescriptionFieldName, 0)
	if f == nil {
		return ""
	}
	return simplify(f.rfc822(false))
}

// Returns the value of the Content-Location field, or an empty string if there
// isn't one. The URI is not validated in any way.
func (h *Header) ContentLocation() string {
	f := h.field(ContentLocationFieldName, 0)
	if f == nil {
		return ""
	}
	return f.rfc822(false)
}

// Returns a pointer to the Content-Language header field, or a null pointer if
// there isn't one.
func (h *Header) ContentLanguage() *ContentLanguage {
	f := h.field(ContentLanguageFieldName, 0)
	if f == nil {
		return nil
	}

	return f.(*ContentLanguage)
}

type HeaderFieldCondition struct {
	name     string
	min, max int
	m        HeaderMode
}

var conditions = []HeaderFieldCondition{
	HeaderFieldCondition{SenderFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{ReplyToFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{ToFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{CcFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{BccFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{MessageIdFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{ReferencesFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{SubjectFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{FromFieldName, 1, 1, Rfc5322Header},
	HeaderFieldCondition{DateFieldName, 1, 1, Rfc5322Header},
	HeaderFieldCondition{MimeVersionFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{MimeVersionFieldName, 0, 1, MimeHeader},
	HeaderFieldCondition{ContentTypeFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{ContentTypeFieldName, 0, 1, MimeHeader},
	HeaderFieldCondition{ContentTransferEncodingFieldName, 0, 1, Rfc5322Header},
	HeaderFieldCondition{ContentTransferEncodingFieldName, 0, 1, MimeHeader},
	HeaderFieldCondition{ReturnPathFieldName, 0, 1, Rfc5322Header},
}

// This private function verifies that the entire header is consistent and
// legal, and that each contained HeaderField is legal.
func (h *Header) Verify() {
	if h.verified {
		return
	}
	h.verified = true
	h.err = nil

	for _, f := range h.Fields {
		if !f.Valid() {
			h.err = fmt.Errorf("%s: %s", f.Name(), f.Error())
			return
		}
	}

	occurrences := make(map[string]int)
	for _, f := range h.Fields {
		occurrences[f.Name()]++
	}

	i := 0
	for h.err == nil && i < len(conditions) {
		if conditions[i].m == h.mode &&
			occurrences[conditions[i].name] < conditions[i].min ||
			occurrences[conditions[i].name] > conditions[i].max {
			if conditions[i].max < occurrences[conditions[i].name] {
				h.err = fmt.Errorf("%d %s fields seen. At most %d may be present.",
					occurrences[conditions[i].name], conditions[i].name, conditions[i].max)
			} else {
				h.err = fmt.Errorf("%d %s fields seen. At least %d must be present.",
					occurrences[conditions[i].name], conditions[i].name, conditions[i].min)
			}
		}
		i++
	}

	// strictly speaking, if From contains more than one address,
	// sender should contain one. we don't enforce that, because it
	// causes too much spam to be rejected that would otherwise go
	// through. we'll filter spam with something that's a little less
	// accidental, and which does not clutter up the logs with so many
	// misleading error messages.

	// we graciously ignore all the Resent-This-Or-That restrictions.
}

// Repairs problems that can be repaired without knowing the associated
// bodypart.
func (h *Header) Repair() {
	if h.Valid() {
		return
	}

	// We remove duplicates of any field that may occur only once.
	// (Duplication has been observed for Date/Subject/M-V/C-T-E/C-T/M-I.)

	occurrences := make(map[string]int)
	for _, f := range h.Fields {
		occurrences[f.Name()]++
	}

	i := 0
	for i < len(conditions) {
		if conditions[i].m == h.mode &&
			occurrences[conditions[i].name] > conditions[i].max {
			n := 0
			j := 0
			hf := h.field(conditions[i].name, 0)
			for j < len(h.Fields) {
				if h.Fields[j].Name() == conditions[i].name {
					n++
					if n > 1 && hf.rfc822(false) == h.Fields[j].rfc822(false) {
						h.Fields.RemoveAt(j)
					} else {
						j++
					}
				} else {
					j++
				}
			}
		}
		i++
	}

	// If there are several content-type fields, and they agree except
	// that one has options and the others not, remove the option-less
	// ones.

	if occurrences[ContentTypeFieldName] > 1 {
		ct := h.ContentType()
		other := ct
		var good *ContentType
		n := 0
		bad := false
		for other != nil && !bad {
			if other.Type != ct.Type ||
				other.Subtype != ct.Subtype {
				bad = true
			} else if len(other.Parameters) > 0 {
				if good != nil {
					bad = true
				}
				good = other
			}
			n++
			tmp := h.field(ContentTypeFieldName, n)
			if tmp != nil {
				other = tmp.(*ContentType)
			} else {
				other = nil
			}
		}
		if good != nil && !bad {
			i := 0
			for i < len(h.Fields) {
				if h.Fields[i].Name() == ContentTypeFieldName && h.Fields[i] != good {
					h.Fields.RemoveAt(i)
				} else {
					i++
				}
			}
		}
	}

	// We retain only the first valid Date field, Return-Path,
	// Message-Id, References and Content-Type fields. If there is one
	// or more valid such field, we delete all invalid fields,
	// otherwise we leave the fields as they are.

	// For most of these, we also delete subsequent valid fields. For
	// Content-Type we only delete invalid fields, since there isn't
	// any strong reason to believe that the one we would keep enables
	// correct interpretation of the body.

	// Several senders appear to send duplicate dates. qmail is
	// mentioned in the references chains of most examples we have.

	// We don't know who adds duplicate message-id, return-path and
	// content-type fields.

	// The only case we've seen of duplicate references involved
	// Thunderbird 1.5.0.4 and Scalix. Uncertain whose
	// bug. Thunderbird 1.5.0.5 looks correct.

	for _, name := range fieldNames {
		if occurrences[name] > 1 &&
			(name == DateFieldName ||
				name == ReturnPathFieldName ||
				name == MessageIdFieldName ||
				name == ContentTypeFieldName ||
				name == ReferencesFieldName) {
			var firstValid Field
			for _, f := range h.Fields {
				if f.Name() == name && f.Valid() {
					firstValid = f
					break
				}
			}
			if firstValid != nil {
				alsoValid := true
				if name == ContentTypeFieldName {
					alsoValid = false
				}
				i := 0
				for i < len(h.Fields) {
					if h.Fields[i].Name() == name && h.Fields[i] != firstValid &&
						(alsoValid || !h.Fields[i].Valid()) {
						h.Fields.RemoveAt(i)
					} else {
						i++
					}
				}
			}
		}
	}

	// Mime-Version is occasionally seen more than once, usually on
	// spam or mainsleaze.
	if h.field(MimeVersionFieldName, 1) != nil {
		h.Fields.Remove(h.field(MimeVersionFieldName, 1))
		fmv := h.field(MimeVersionFieldName, 0)
		fmv.Parse(fmt.Sprintf("1.0 (Note: original message contained %d Mime-Version fields)", occurrences[MimeVersionFieldName]))
	}

	// Content-Transfer-Encoding: should not occur on multiparts, and
	// when it does it usually has a syntax error. We don't care about
	// that error.
	if occurrences[ContentTransferEncodingFieldName] > 0 {
		ct := h.ContentType()
		if ct != nil && ct.Type == "multipart" || ct.Type == "message" {
			h.Fields.RemoveAllNamed(ContentTransferEncodingFieldName)
		}
	}

	// Sender sometimes is a straight copy of From, even if From
	// contains more than one address. If it's a copy, or even an
	// illegal subset, we drop it.

	senders := h.addresses(SenderFieldName)

	if occurrences[SenderFieldName] > 0 && len(senders) != 1 {
		from := make(map[string]bool)
		for _, a := range h.addresses(FromFieldName) {
			from[strings.ToLower(a.lpdomain())] = true
		}

		sender := []string{}
		for _, a := range h.addresses(FromFieldName) {
			sender = append(sender, strings.ToLower(a.lpdomain()))
		}

		i := 0
		difference := false
		for i < len(sender) && difference {
			if !from[sender[i]] {
				difference = true
			}
			i++
		}
		if !difference {
			h.Fields.RemoveAllNamed(SenderFieldName)
		}
	}
}

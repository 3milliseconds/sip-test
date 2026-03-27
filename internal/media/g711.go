package media

// G.711 mu-law and A-law encoding/decoding tables and functions.

const (
	MuLawBias = 0x84
	MuLawClip = 32635
)

// LinearToMuLaw converts a 16-bit signed PCM sample to 8-bit mu-law.
func LinearToMuLaw(sample int16) byte {
	sign := 0
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}
	if sample > MuLawClip {
		sample = MuLawClip
	}
	sample += MuLawBias

	exponent := 7
	for expMask := int16(0x4000); exponent > 0; exponent-- {
		if sample&expMask != 0 {
			break
		}
		expMask >>= 1
	}

	mantissa := int(sample>>uint(exponent+3)) & 0x0F
	muLaw := byte(sign | (exponent << 4) | mantissa)
	return ^muLaw
}

// MuLawToLinear converts an 8-bit mu-law sample to 16-bit signed PCM.
func MuLawToLinear(muLaw byte) int16 {
	muLaw = ^muLaw
	sign := int16(1)
	if muLaw&0x80 != 0 {
		sign = -1
		muLaw &= 0x7F
	}
	exponent := int((muLaw >> 4) & 0x07)
	mantissa := int(muLaw & 0x0F)
	sample := int16((mantissa<<3 + MuLawBias) << uint(exponent) - MuLawBias)
	return sign * sample
}

// LinearToALaw converts a 16-bit signed PCM sample to 8-bit A-law.
func LinearToALaw(sample int16) byte {
	sign := 0
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}

	if sample > 32767 {
		sample = 32767
	}

	exponent := 7
	for expMask := int16(0x4000); exponent > 0; exponent-- {
		if sample&expMask != 0 {
			break
		}
		expMask >>= 1
	}

	var mantissa int
	if exponent > 0 {
		mantissa = int(sample>>uint(exponent+3)) & 0x0F
	} else {
		mantissa = int(sample>>4) & 0x0F
	}

	aLaw := byte(sign | (exponent << 4) | mantissa)
	return aLaw ^ 0xD5
}

// ALawToLinear converts an 8-bit A-law sample to 16-bit signed PCM.
func ALawToLinear(aLaw byte) int16 {
	aLaw ^= 0xD5
	sign := int16(1)
	if aLaw&0x80 != 0 {
		sign = -1
		aLaw &= 0x7F
	}
	exponent := int((aLaw >> 4) & 0x07)
	mantissa := int(aLaw & 0x0F)

	var sample int16
	if exponent > 0 {
		sample = int16((mantissa<<3 | 0x84) << uint(exponent-1))
	} else {
		sample = int16(mantissa<<4 | 0x08)
	}
	return sign * sample
}

// EncodePCMToMuLaw converts a slice of 16-bit PCM samples to mu-law bytes.
func EncodePCMToMuLaw(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, s := range pcm {
		out[i] = LinearToMuLaw(s)
	}
	return out
}

// EncodePCMToALaw converts a slice of 16-bit PCM samples to A-law bytes.
func EncodePCMToALaw(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, s := range pcm {
		out[i] = LinearToALaw(s)
	}
	return out
}

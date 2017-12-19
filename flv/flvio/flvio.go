package flvio
import (
	"fmt"
	"github.com/nareix/bits/pio"
	"github.com/KouBianJing/flvparse/av"
	"io"
	"time"
	"encoding/hex"
)

func TsToTime(ts int32) time.Duration {
	return time.Millisecond * time.Duration(ts)
}

func TimeToTs(tm time.Duration) int32 {
	return int32(tm / time.Millisecond)
}


const MaxTagSubHeaderLength = 16

const (
	TAG_AUDIO      = 8
	TAG_VIDEO      = 9
	TAG_SCRIPTDATA = 18
)

const (
	SOUND_MP3                   = 2
	SOUND_NELLYMOSER_16KHZ_MONO = 4
	SOUND_NELLYMOSER_8KHZ_MONO  = 5
	SOUND_NELLYMOSER            = 6
	SOUND_ALAW                  = 7
	SOUND_MULAW                 = 8
	SOUND_AAC                   = 10
	SOUND_SPEEX                 = 11

	SOUND_5_5Khz = 0
	SOUND_11Khz  = 1
	SOUND_22Khz  = 2
	SOUND_44Khz  = 3

	SOUND_8BIT  = 0
	SOUND_16BIT = 1

	SOUND_MONO   = 0
	SOUND_STEREO = 1

	AAC_SEQHDR = 0
	AAC_RAW    = 1
)

const (
	AVC_SEQHDR = 0
	AVC_NALU   = 1
	AVC_EOS    = 2

	FRAME_KEY   = 1
	FRAME_INTER = 2

	VIDEO_H264 = 7
)


/* Video codecs */
const(
	NGX_RTMP_VIDEO_JPEG             = 1
	NGX_RTMP_VIDEO_SORENSON_H263    = 2
	NGX_RTMP_VIDEO_SCREEN           = 3
	NGX_RTMP_VIDEO_ON2_VP6          = 4
	NGX_RTMP_VIDEO_ON2_VP6_ALPHA    = 5
	NGX_RTMP_VIDEO_SCREEN2          = 6
	NGX_RTMP_VIDEO_H264             = 7
	NGX_RTMP_VIDEO_H265             = 12
)

func (self Tag)Vstring() string{
	switch self.CodecID {
		case 1:
			return "JPEG (currently unused)"
		case 2:
			return "Sorenson H.263"
		case 3:
			return "Screen video"
		case 4:
			return "On2 VP6"
		case 5:
			return "On2 VP6 with alpha channel"
	        case 6:
			return  "Screen video version 2"
		case 7:
			return "AVC"
	default:
		return "unknown"
	}
}
func (self Tag) AString() string {
	switch self.SoundFormat {
		case 0 :
			return "Linear PCM, platform endian"
		case 1 :
			return "ADPCM"
		case 2 :
			return "MP3"
		case 3:
			return "Linear PCM, little endian"
		case 4,5,6:
			return "Nellymoser"
		case 7,8:
			return "G.711 A-law logarithmic PCM"
		case 9:
			return "reserved"
		case 10:
			return "AAC"
		case 11:
			return "Speex"
		case 14:
			return "MP3 8-Khz"
		case 15:
			return "Device-specific sound"
	default:
		return "unknown"
	}
}

type Tag struct {
	Type uint8

	/*
		SoundFormat: UB[4]
		0 = Linear PCM, platform endian
		1 = ADPCM
		2 = MP3
		3 = Linear PCM, little endian
		4 = Nellymoser 16-kHz mono
		5 = Nellymoser 8-kHz mono
		6 = Nellymoser
		7 = G.711 A-law logarithmic PCM
		8 = G.711 mu-law logarithmic PCM
		9 = reserved
		10 = AAC
		11 = Speex
		14 = MP3 8-Khz
		15 = Device-specific sound
		Formats 7, 8, 14, and 15 are reserved for internal use
		AAC is supported in Flash Player 9,0,115,0 and higher.
		Speex is supported in Flash Player 10 and higher.
	*/
	SoundFormat uint8

	/*
		SoundRate: UB[2]
		Sampling rate
		0 = 5.5-kHz For AAC: always 3
		1 = 11-kHz
		2 = 22-kHz
		3 = 44-kHz
	*/
	SoundRate uint8

	/*
		SoundSize: UB[1]
		0 = snd8Bit
		1 = snd16Bit
		Size of each sample.
		This parameter only pertains to uncompressed formats.
		Compressed formats always decode to 16 bits internally
	*/
	SoundSize uint8

	/*
		SoundType: UB[1]
		0 = sndMono
		1 = sndStereo
		Mono or stereo sound For Nellymoser: always 0
		For AAC: always 1
	*/
	SoundType uint8

	/*
		0: AAC sequence header
		1: AAC raw
	*/
	AACPacketType uint8

	/*
		1: keyframe (for AVC, a seekable frame)
		2: inter frame (for AVC, a non- seekable frame)
		3: disposable inter frame (H.263 only)
		4: generated keyframe (reserved for server use only)
		5: video info/command frame
	*/
	FrameType uint8

	/*
		1: JPEG (currently unused)
		2: Sorenson H.263
		3: Screen video
		4: On2 VP6
		5: On2 VP6 with alpha channel
		6: Screen video version 2
		7: AVC
	*/
	CodecID uint8

	/*
		0: AVC sequence header
		1: AVC NALU
		2: AVC end of sequence (lower level NALU sequence ender is not required or supported)
	*/
	AVCPacketType uint8

	CompositionTime int32
	NoHead bool
	Data []byte
	DataPos int
}

func (self Tag) ChannelLayout() av.ChannelLayout {
	if self.SoundType == SOUND_MONO {
		return av.CH_MONO
	} else {
		return av.CH_STEREO
	}
}

func (self *Tag) audioParseHeader(b []byte) (n int, err error) {
	hex.Dump(b)
	if len(b) < n+1 {
		err = fmt.Errorf("%s","Flvio.Audio.Data.Parse.Invalid")
		return
	}

	flags := b[n]
	n++
	self.SoundFormat = flags >> 4
	self.SoundRate = (flags >> 2) & 0x3
	self.SoundSize = (flags >> 1) & 0x1
	self.SoundType = flags & 0x1

	switch self.SoundFormat {
	case SOUND_AAC:
		if len(b) < n+1 {
			err = fmt.Errorf("%s","Flvio.Audio.Data.Parse.Invalid")
			return
		}
		self.AACPacketType = b[n]
		n++
	}

	return
}

func (self *Tag) videoParseHeader(b []byte) (n int, err error) {
	if len(b) < n+1 {
		err = fmt.Errorf("%s","Flvio.Video.Data.Parse.Invalid")
		return
	}
	flags := b[n]
	self.FrameType = flags >> 4
	self.CodecID = flags & 0xf
	n++

	if self.FrameType == FRAME_INTER || self.FrameType == FRAME_KEY {
		if len(b) < n+4 {
			err = fmt.Errorf("%s","Flvio.Video.Data.Parse.Invalid")
			return
		}
		self.AVCPacketType = b[n]
		n++

		self.CompositionTime = pio.I24BE(b[n:])
		n += 3
	}

	return
}

func (self Tag) videoFillHeader(b []byte) (n int) {
	flags := self.FrameType<<4 | self.CodecID
	b[n] = flags
	n++
	b[n] = self.AVCPacketType
	n++
	pio.PutI24BE(b[n:], self.CompositionTime)
	n += 3
	return
}

func (self *Tag) ParseHeader(b []byte) (n int, err error) {
	switch self.Type {
	case TAG_AUDIO:
		return self.audioParseHeader(b)

	case TAG_VIDEO:
		return self.videoParseHeader(b)
	}

	return
}

const (
	// TypeFlagsReserved UB[5]
	// TypeFlagsAudio    UB[1] Audio tags are present
	// TypeFlagsReserved UB[1] Must be 0
	// TypeFlagsVideo    UB[1] Video tags are present
	FILE_HAS_AUDIO = 0x4
	FILE_HAS_VIDEO = 0x1
)

const TagHeaderLength = 11
const TagTrailerLength = 4

func ParseTagHeader(b []byte) (tag Tag, ts int32, datalen int, err error) {
	//fmt.Println(hex.Dump(b))
	//tag type one byte
	/*
	00 type 0
	00 00 00 data len 1:4
	00 00 00 timeStamp 4:7
	00 time extended 7
	00 00 00 00 streamId 7:11
	 */
	tagtype := b[0]
	//tag data len 4 bytes
	datalen = int(pio.U24BE(b[1:4]))
	switch tagtype {
	case TAG_AUDIO, TAG_VIDEO, TAG_SCRIPTDATA:
		tag = Tag{Type: tagtype}

	default:
		if datalen != 0 {
			//fmt.Println("============44",datalen,tagtype)
			err = fmt.Errorf("Flvio.Read.Tag.TagType=%d.Invalid", tagtype)
		}
		return
	}



	var tslo uint32
	var tshi uint8
	tslo = pio.U24BE(b[4:7])
	tshi = b[7]
	ts = int32(tslo | uint32(tshi)<<24)

	return
}

func ReadTag(r io.Reader, b []byte) (tag Tag, ts int32, err error) {
	if _, err = io.ReadFull(r, b[:TagHeaderLength]); err != nil {
		return
	}
	//tag header
	//fmt.Println("flv tag header:",hex.Dump(b[:TagHeaderLength]))
	var datalen int
	if tag, ts, datalen, err = ParseTagHeader(b); err != nil {
		return
	}

	if datalen == 0{
		if _, err = io.ReadFull(r, b[:4]); err != nil {
			return
		}
		//fmt.Println("tag_data_leng:",datalen,"previous_tag_leng:",(pio.U32BE(b[0:4])))
		return
	}

	data := make([]byte, datalen)
	if _, err = io.ReadFull(r, data); err != nil {
		return
	}

	var n int
	if n, err = (&tag).ParseHeader(data); err != nil {
		return
	}
	tag.Data = data
	tag.DataPos = n

	if _, err = io.ReadFull(r, b[:4]); err != nil {
		return
	}
	//fmt.Println("tag_data_leng:",datalen,"previous_tag_leng:",(pio.U32BE(b[0:4])))
	return
}

const FileHeaderLength = 9

func ParseFileHeader(b []byte) (flags uint8, skip int, err error) {
	flv := pio.U24BE(b[0:3])
	if flv != 0x464c56 { // 'FLV'
		err = fmt.Errorf("%s","Flvio.File.Header.FLV.Invalid")
		return
	}

	//have audio vdeio or all have
	flags = b[4]

	skip = int(pio.U32BE(b[5:9])) - 9 + 4
	if skip < 0 {
		err = fmt.Errorf("%s","Flvio.File.Header.DataSize.Invalid")
		return
	}

	return
}




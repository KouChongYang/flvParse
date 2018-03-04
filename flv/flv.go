package flv

import (
	"bufio"
	"github.com/nareix/bits/pio"
	"io"
	"fmt"
	"encoding/json"
	"github.com/KouChongYang/flvParse/av"
	"github.com/KouChongYang/flvParse/flv/flvio"
	"github.com/KouChongYang/flvParse/h264Parse"
	"github.com/KouChongYang/flvParse/aacParse"
	//"time"
	//"encoding/hex"
	//"encoding/hex"
	//"encoding/hex"
	"github.com/KouChongYang/flvParse/amf"
	//"encoding/hex"
	"encoding/hex"
)

var MaxProbePacketCount = 20

type Prober struct {
	HasAudio, HasVideo             bool
	GotAudio, GotVideo             bool
	VideoStreamIdx, AudioStreamIdx int
	PushedCount                    int
	Streams                        []av.CodecData
	CachedPkts                     []av.Packet
}

func (self *Prober) Empty() bool {
	return len(self.CachedPkts) == 0
}

func (self *Prober) PopPacket() av.Packet {
	pkt := self.CachedPkts[0]
	self.CachedPkts = self.CachedPkts[1:]
	return pkt
}
type Demuxer struct {
	prober *Prober
	bufr   *bufio.Reader
	b      []byte
	stage  int
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		bufr:   bufio.NewReaderSize(r, pio.RecommendBufioSize),
		prober: &Prober{},
		b:      make([]byte, 256),
	}
}

func (self *Prober) Probed() (ok bool) {
	if self.HasAudio || self.HasVideo {
		if self.HasAudio == self.GotAudio && self.HasVideo == self.GotVideo {
			return true
		}
	} else {
		if self.PushedCount == MaxProbePacketCount {
			return true
		}
	}
	return
}

type NALUs struct {
	NaluType string //
	NaluTypeNum int
	NaluLen int
	NaluData string
}

type flvHeadMsg struct {
	FlvFlag string
	TagHeaderLeng int
	HeadData []byte

}

type TagMsg struct {
	TagType int
	TagHeaderLeng int
	TagDataLeng int
	TagNum int64
	CodecType string
	CompositionTime int32 // packet presentation time minus decode time for H264 B-Frame
	Time            int32 // packet decode time
	Packet interface{}
}

type VideoPacket struct {
	IsKeyFrame      bool // video packet is key frame
	//GopIsKeyFrame   bool // just for no video
	//DataPos         int
	PacketLen       int
	Nalus           []NALUs
	//Data []byte // packet data
}

func  handleCommandMsgAMF0(b []byte) (datamsgvals []interface{}) {
	fmt.Println("=====================fdsa========")
	//var datamsgvals []interface{}
	n:=0
	var err error
	//var metadata amf.AMFMap
	var size int
	var  obj interface{}
	if obj, size, err = amf.ParseAMF0Val(b[n:]); err != nil {
		fmt.Println(err)
		return
	}
	n+=size
	fmt.Println(obj,len(b))
	for n < len(b) {
		var  obj interface{}
		if obj, size, err = amf.ParseAMF0Val(b[n:]); err != nil {
			//fmt.Println(err)
			//return
		}else{
			datamsgvals = append(datamsgvals, obj)
		}
		n += size
	}
	//fmt.Println("=====================fdsa========")
	//fmt.Println(datamsgvals)
	return
}

func(self *Demuxer) FlvParse()(err error){
	if _, err = io.ReadFull(self.bufr, self.b[:flvio.FileHeaderLength]); err != nil {
		return
	}

	var flags uint8
	var skip int
	if flags, skip, err = flvio.ParseFileHeader(self.b); err != nil {
		return
	}
	if _, err = self.bufr.Discard(skip); err != nil {
		return
	}
	//fmt.Println("flv header: ",hex.Dump(self.b[:flvio.TagHeaderLength + skip]))
	var flvhead flvHeadMsg
	tagflag:=""
	tagflag0:=""
	tagflag1:=""
	if flags&flvio.FILE_HAS_AUDIO != 0 {
		tagflag0 = " haveAudio "
		//fmt.Printf("%s","have audio")
		self.prober.HasAudio = true
	}
	if flags&flvio.FILE_HAS_VIDEO != 0 {
		//fmt.Printf("%s"," have video")
		tagflag1 = " haveVideo "
		self.prober.HasVideo = true
	}

	tagflag = tagflag0 + tagflag1
	flvhead.FlvFlag = tagflag
	flvhead.TagHeaderLeng = flvio.FileHeaderLength + skip
	flvhead.HeadData = self.b[:flvio.TagHeaderLength + skip]
	flvheads,_:=json.MarshalIndent(flvhead,"","")
	fmt.Println(string(flvheads))
	fmt.Println("===================================flv data===============================")
	tagnum:=int64(0)

	var VideoPacket VideoPacket

	for {
		var tag flvio.Tag
		var tagmsg TagMsg
		var timestamp int32

		if tag, timestamp, err = flvio.ReadTag(self.bufr, self.b); err != nil {
			return
		}

		tagmsg.TagHeaderLeng = 9
		tagmsg.TagDataLeng = len(tag.Data)
		tagmsg.TagNum = tagnum
		tagmsg.Time = timestamp

		tagmsg.CompositionTime = tag.CompositionTime
		tagmsg.TagType = int(tag.Type)
		if  tag.Type == flvio.TAG_SCRIPTDATA{
			tagmsg.CodecType = "onMeta"
			//tagmsg.Packet = hex.Dump(tag.Data)
			tagmsg.Packet =handleCommandMsgAMF0(tag.Data[0:])
		}

		if len(tag.Data) == 0{
			goto Print
		}

		if tag.Type == flvio.TAG_VIDEO {
			tagmsg.CodecType =  tag.Vstring()
			VideoPacket.PacketLen = len(tag.Data[tag.DataPos:])
			VideoPacket.IsKeyFrame = tag.FrameType == flvio.FRAME_KEY

			if tag.CodecID == flvio.VIDEO_H264 {
				if !(tag.FrameType == flvio.FRAME_INTER || tag.FrameType == flvio.FRAME_KEY) {
					fmt.Println("parse frame err fomat is err")
					return
				}
				var stream h264parser.CodecData
				switch tag.AVCPacketType {
				case flvio.AVC_SEQHDR:
					tagmsg.CodecType =  "AVC_SEQHDR"
					//fmt.Println("find avc seqhdr")
					if stream, err = h264parser.NewCodecDataFromAVCDecoderConfRecord(tag.Data[tag.DataPos:]); err != nil {
						return
					}
					tagmsg.Packet = stream
					goto Print
				case flvio.AVC_NALU:
					b := tag.Data[tag.DataPos:]
					nalus, _ := h264parser.SplitNALUs(b)

					for _, nalu := range nalus {
						var nls NALUs
						if len(nalu) > 0 {
							naltype := nalu[0] & 0x1f
							if len(nalu) < 32 {
								nls.NaluData = hex.EncodeToString(nalu)
							}else{
								nls.NaluData = hex.EncodeToString(nalu[0:32])
							}
							//case 1, 2, 5, 19:
							nls.NaluTypeNum = int(naltype)
							nls.NaluLen = len(nalu)
							switch naltype {
							case 0:
								nls.NaluType = "unknown"
							case 1:
								nls.NaluType ,err = h264parser.ParseSliceHeaderFromNALU(nalu)
								if err != nil{
									nls.NaluType = err.Error()
								}
							case 2:
								nls.NaluType ,err = h264parser.ParseSliceHeaderFromNALU(nalu)
								if err != nil{
									nls.NaluType = err.Error()
								}
							case 3:
								nls.NaluType = "Coded slice data partition B"
							case 4:
								nls.NaluType = "Coded slice data partition C"
							case 5:
								nls.NaluType ,err = h264parser.ParseSliceHeaderFromNALU(nalu)
								if err != nil{
									nls.NaluType = err.Error()
								}
							case 6:
								nls.NaluType ="SEI"
							case 7:
								nls.NaluType ="SPS"
							case 8:
								nls.NaluType ="PPS"
							case 9:
								nls.NaluType ="AUD"
							case 10:
								nls.NaluType ="End of sequence"
							case 11:
								nls.NaluType ="End of stream"
							case 12:
								nls.NaluType ="Filler data"
							case 13:
								nls.NaluType ="Sequence parameter set extension"
							case 14:
								nls.NaluType ="Prefix NAL unit"
							case 15:
								nls.NaluType ="Subset sequence parameter set"
							case 16:
								nls.NaluType ="Depth parameter set"
							case 19:
								nls.NaluType ,err = h264parser.ParseSliceHeaderFromNALU(nalu)
								if err != nil{
									nls.NaluType = err.Error()
								}
							default:
								nls.NaluType ="unknown"
							}

							VideoPacket.Nalus = append(VideoPacket.Nalus,nls)
						}
					}
					tagmsg.Packet = &VideoPacket
				}
				//
			}
		}
		if tag.Type == flvio.TAG_AUDIO {
			tagmsg.CodecType = tag.AString()
			switch tag.SoundFormat {
			case flvio.SOUND_AAC:
				switch tag.AACPacketType {
				case flvio.AAC_SEQHDR:
					tagmsg.CodecType =  "AAC_SEQHDR"
					//fmt.Println("find acc seqhdr")
					var stream aacparser.CodecData
					if stream, err = aacparser.NewCodecDataFromMPEG4AudioConfigBytes(tag.Data[tag.DataPos:]); err != nil {
						return
					}
					tagmsg.Packet = stream
					//fmt.Println(stream)
				}
			}
			if len(tag.Data) < 32 {
				tagmsg.Packet = hex.EncodeToString(tag.Data)
			}else{
				tagmsg.Packet = hex.EncodeToString(tag.Data[0:32])
			}
		}
		Print:
			tagmsgs,_:=json.MarshalIndent(tagmsg,"","")
			fmt.Println(string(tagmsgs))

			fmt.Printf("============**********************=========%d=======********************============\n",tagnum)
		tagnum++
	}
}














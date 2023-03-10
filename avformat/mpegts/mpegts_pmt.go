package mpegts

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/Monibuca/engine/util"
)

// ios13818-1-CN.pdf 46(60)-153(167)/page
//
// PMT
//

var DefaultPMTPacket = []byte{
	// TS Header
	0x47, 0x41, 0x00, 0x10,

	// Pointer Field
	0x00,

	// PSI
	0x02, 0xb0, 0x17, 0x00, 0x01, 0xc1, 0x00, 0x00,

	// PMT
	0xe1, 0x01,
	0xf0, 0x00,

	// H264
	0x1b, 0xe1, 0x01, 0xf0, 0x00,

	// AAC
	0x0f, 0xe1, 0x02, 0xf0, 0x00,

	//0x00, 0x00, 0x00, 0x00, 0x00,

	// CRC for not audio
	//0x00, 0x00, 0x00, 0x00,

	// CRC for AAC
	0x9e, 0x28, 0xc6, 0xdd,

	// CRC for MP3
	// 0x4e, 0x59, 0x3d, 0x1e,

	// Stuffing 157 bytes
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

// TS Header :
// SyncByte = 0x47
// TransportErrorIndicator = 0(B:0), PayloadUnitStartIndicator = 1(B:0), TransportPriority = 0(B:0),
// Pid = 4097(0x1001),
// TransportScramblingControl = 0(B:00), AdaptionFieldControl = 1(B:01), ContinuityCounter = 0(B:0000),

// PSI :
// TableID = 0x02,
// SectionSyntaxIndicator = 1(B:1), Zero = 0(B:0), Reserved1 = 3(B:11),
// SectionLength = 23(0x17)
// ProgramNumber = 0x0001
// Reserved2 = 3(B:11), VersionNumber = (B:00000), CurrentNextIndicator = 1(B:0),
// SectionNumber = 0x00
// LastSectionNumber = 0x00

// PMT:
// Reserved3 = 15(B:1110), PcrPID = 256(0x100)
// Reserved4 = 16(B:1111), ProgramInfoLength = 0(0x000)
// H264:
// StreamType = 0x1b,
// Reserved5 = 15(B:1110), ElementaryPID = 256(0x100)
// Reserved6 = 16(B:1111), EsInfoLength = 0(0x000)
// AAC:
// StreamType = 0x0f,
// Reserved5 = 15(B:1110), ElementaryPID = 257(0x101)
// Reserved6 = 16(B:1111), EsInfoLength = 0(0x000)

type MpegTsPmtStream struct {
	StreamType    byte   // 8 bits ???????????? PID????????????????????????????????????,??? PID?????? elementary_PID?????????
	Reserved5     byte   // 3 bits ?????????
	ElementaryPID uint16 // 13 bits ????????????????????????????????????????????? PID
	Reserved6     byte   // 4 bits ?????????
	EsInfoLength  uint16 // 12 bits ??????????????????????????????'00',?????? 10?????????????????? ES_info_length?????????????????????????????????????????????

	// N Loop Descriptors
	Descriptor []MpegTsDescriptor // ??????????????????,??????
}

// Program Map Table (???????????????)
type MpegTsPMT struct {
	// PSI
	TableID                byte   // 8 bits 0x00->PAT,0x02->PMT
	SectionSyntaxIndicator byte   // 1 bit  ??????????????????,?????????1
	Zero                   byte   // 1 bit  0
	Reserved1              byte   // 2 bits ?????????
	SectionLength          uint16 // 12 bits ??????????????????????????????'00',?????? 10 ?????????????????????????????????,?????? section_length ????????????,????????? CRC.?????????????????????????????? 1021(0x3FD)
	ProgramNumber          uint16 // 16 bits ?????? program_map_PID ??????????????????
	Reserved2              byte   // 2 bits  ?????????
	VersionNumber          byte   // 5 bits  ??????0-31,??????PAT????????????
	CurrentNextIndicator   byte   // 1 bit  ?????????PAT??????????????????????????????PAT??????
	SectionNumber          byte   // 8 bits  ???????????????.PAT????????????????????????.????????????00,?????????????????????1,???????????????256?????????
	LastSectionNumber      byte   // 8 bits  ???????????????????????????

	Reserved3             byte               // 3 bits  ????????? 0x07
	PcrPID                uint16             // 13 bits ??????TS??????PID???.???TS?????????PCR???,???PCR?????????????????????????????????????????????.?????????????????????????????????????????????PCR??????.?????????????????????0x1FFF
	Reserved4             byte               // 4 bits  ????????? 0x0F
	ProgramInfoLength     uint16             // 12 bits ?????????bit???00.???????????????????????????????????????????????????byte???
	ProgramInfoDescriptor []MpegTsDescriptor // N Loop Descriptors ?????? ??????????????????

	// N Loop
	Stream []MpegTsPmtStream // PMT???????????????????????????????????????

	Crc32 uint32 // 32 bits ???????????????????????????????????????????????????,????????? B ???????????????????????????????????????????????? CRC ???
}

func ReadPMT(r io.Reader) (pmt MpegTsPMT, err error) {
	lr, psi, err := ReadPSI(r, PSI_TYPE_PMT)
	if err != nil {
		return
	}

	pmt = psi.Pmt

	// reserved3(3) + pcrPID(13)
	pcrPID, err := util.ReadByteToUint16(lr, true)
	if err != nil {
		return
	}

	pmt.PcrPID = pcrPID & 0x1fff

	// reserved4(4) + programInfoLength(12)
	// programInfoLength(12) == 0x00(?????????0) + programInfoLength(10)
	programInfoLength, err := util.ReadByteToUint16(lr, true)
	if err != nil {
		return
	}

	pmt.ProgramInfoLength = programInfoLength & 0x3ff

	// ??????length>0??????,??????programInfoLength????????????length?????????
	if pmt.ProgramInfoLength > 0 {
		lr := &io.LimitedReader{R: lr, N: int64(pmt.ProgramInfoLength)}
		pmt.ProgramInfoDescriptor, err = ReadPMTDescriptor(lr)
		if err != nil {
			return
		}
	}

	// N Loop
	// ??????N??????,???????????????????????????
	for lr.N > 0 {
		var streams MpegTsPmtStream
		// streamType(8)
		streams.StreamType, err = util.ReadByteToUint8(lr)
		if err != nil {
			return
		}

		// reserved5(3) + elementaryPID(13)
		streams.ElementaryPID, err = util.ReadByteToUint16(lr, true)
		if err != nil {
			return
		}

		streams.ElementaryPID = streams.ElementaryPID & 0x1fff

		// reserved6(4) + esInfoLength(12)
		// esInfoLength(12) == 0x00(?????????0) + esInfoLength(10)
		streams.EsInfoLength, err = util.ReadByteToUint16(lr, true)
		if err != nil {
			return
		}

		streams.EsInfoLength = streams.EsInfoLength & 0x3ff

		// ??????length>0??????,??????esInfoLength????????????length?????????
		if streams.EsInfoLength > 0 {
			lr := &io.LimitedReader{R: lr, N: int64(streams.EsInfoLength)}
			streams.Descriptor, err = ReadPMTDescriptor(lr)
			if err != nil {
				return
			}
		}

		// ???????????????????????????(????????????????????????????????????),???????????????
		pmt.Stream = append(pmt.Stream, streams)
	}
	if cr, ok := r.(*util.Crc32Reader); ok {
		err = cr.ReadCrc32UIntAndCheck()
		if err != nil {
			return
		}
	}
	return
}

func ReadPMTDescriptor(lr *io.LimitedReader) (Desc []MpegTsDescriptor, err error) {
	var desc MpegTsDescriptor
	for lr.N > 0 {
		// tag (8)
		desc.Tag, err = util.ReadByteToUint8(lr)
		if err != nil {
			return
		}

		// length (8)
		desc.Length, err = util.ReadByteToUint8(lr)
		if err != nil {
			return
		}

		desc.Data = make([]byte, desc.Length)
		_, err = lr.Read(desc.Data)
		if err != nil {
			return
		}

		Desc = append(Desc, desc)
	}

	return
}

func WritePMTDescriptor(w io.Writer, descs []MpegTsDescriptor) (err error) {
	for _, desc := range descs {
		// tag(8)
		if err = util.WriteUint8ToByte(w, desc.Tag); err != nil {
			return
		}

		// length (8)
		if err = util.WriteUint8ToByte(w, uint8(len(desc.Data))); err != nil {
			return
		}

		// data
		if _, err = w.Write(desc.Data); err != nil {
			return
		}
	}

	return
}

func WritePMTBody(w io.Writer, pmt MpegTsPMT) (err error) {
	// reserved3(3) + pcrPID(13)
	if err = util.WriteUint16ToByte(w, pmt.PcrPID|7<<13, true); err != nil {
		return
	}

	// programInfoDescriptor ??????????????????,?????????????????????
	bw := &bytes.Buffer{}
	if err = WritePMTDescriptor(bw, pmt.ProgramInfoDescriptor); err != nil {
		return
	}

	pmt.ProgramInfoLength = uint16(bw.Len())

	// reserved4(4) + programInfoLength(12)
	// programInfoLength(12) == 0x00(?????????0) + programInfoLength(10)
	if err = util.WriteUint16ToByte(w, pmt.ProgramInfoLength|0xf000, true); err != nil {
		return
	}

	// programInfoDescriptor
	if _, err = w.Write(bw.Bytes()); err != nil {
		return
	}

	// ?????????????????????????????????(??????????????????)
	for _, esinfo := range pmt.Stream {
		// streamType(8)
		if err = util.WriteUint8ToByte(w, esinfo.StreamType); err != nil {
			return
		}

		// reserved5(3) + elementaryPID(13)
		if err = util.WriteUint16ToByte(w, esinfo.ElementaryPID|7<<13, true); err != nil {
			return
		}

		// descriptor ES???????????????,?????????????????????
		bw := &bytes.Buffer{}
		if err = WritePMTDescriptor(bw, esinfo.Descriptor); err != nil {
			return
		}

		esinfo.EsInfoLength = uint16(bw.Len())

		// reserved6(4) + esInfoLength(12)
		// esInfoLength(12) == 0x00(?????????0) + esInfoLength(10)
		if err = util.WriteUint16ToByte(w, esinfo.EsInfoLength|0xf000, true); err != nil {
			return
		}

		// descriptor
		if _, err = w.Write(bw.Bytes()); err != nil {
			return
		}
	}

	return
}

func WritePMT(w io.Writer, pmt MpegTsPMT) (err error) {
	bw := &bytes.Buffer{}

	if err = WritePMTBody(bw, pmt); err != nil {
		return
	}

	if pmt.SectionLength == 0 {
		pmt.SectionLength = 2 + 3 + 4 + uint16(len(bw.Bytes()))
	}

	psi := MpegTsPSI{}

	psi.Pmt = pmt

	if err = WritePSI(w, PSI_TYPE_PMT, psi, bw.Bytes()); err != nil {
		return
	}

	return
}

func WritePMTPacket(w io.Writer, tsHeader []byte, pmt MpegTsPMT) (err error) {
	if pmt.TableID != TABLE_TSPMS {
		err = errors.New("PMT table ID error")
		return
	}

	// ????????????????????????(PMT),???????????????buffer??????.
	// 	buffer ???????????????????????????PMT???(PointerField+PSI+PMT+CRC)
	bw := &bytes.Buffer{}
	if err = WritePMT(bw, pmt); err != nil {
		return
	}

	// TODO:??????Pmt.Stream???????????????????????????,??????188?
	stuffingBytes := util.GetFillBytes(0xff, TS_PACKET_SIZE-4-bw.Len())

	var PMTPacket []byte
	PMTPacket = append(PMTPacket, tsHeader...)
	PMTPacket = append(PMTPacket, bw.Bytes()...)
	PMTPacket = append(PMTPacket, stuffingBytes...)

	fmt.Println("-------------------------")
	fmt.Println("Write PMT :", PMTPacket)
	fmt.Println("-------------------------")

	// ???PMT??????
	if _, err = w.Write(PMTPacket); err != nil {
		return
	}

	return
}

func WriteDefaultPMTPacket(w io.Writer) (err error) {
	_, err = w.Write(DefaultPMTPacket)
	if err != nil {
		return
	}

	return
}

package mp4tag

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	//"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	//"strconv"
	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)

var atomsMap = map[string]mp4.BoxType{
	"Album":          {'\251', 'a', 'l', 'b'},
	"AlbumArtist":    {'a', 'A', 'R', 'T'},
	"Artist":         {'\251', 'A', 'R', 'T'},
	"Comment":        {'\251', 'c', 'm', 't'},
	"Composer":       {'\251', 'w', 'r', 't'},
	"Copyright":      {'c', 'p', 'r', 't'},
	"Cover":          {'c', 'o', 'v', 'r'},
	"Disk":           {'d', 'i', 's', 'k'},
	"Genre":          {'\251', 'g', 'e', 'n'},
	"Label":          {'\251', 'l', 'a', 'b'},
	"Title":          {'\251', 'n', 'a', 'm'},
	"Track":          {'t', 'r', 'k', 'n'},
	"Year":           {'\251', 'd', 'a', 'y'},
	"UnsyncedLyrics": {'\251', 'l', 'y', 'r'},
	"ContentRating":  {'r', 't', 'n', 'g'},
	"compilation":    {'c', 'p', 'i', 'l'},
	"albumSort":      {'s', 'o', 'a', 'l'},
	"artistSort":     {'s', 'o', 'a', 'a'},
}

func copy(w *mp4.Writer, h *mp4.ReadHandle) error {
	_, err := w.StartBox(&h.BoxInfo)
	if err != nil {
		return err
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
	if err != nil {
		return err
	}
	_, err = h.Expand()
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

// See which atoms don't already exist and will need creating.
func populateAtoms(f *os.File, tags *Tags) (map[string]bool, error) {
	ilst, err := mp4.ExtractBox(
		f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst()})
	if err != nil {
		return nil, err
	}
	atoms := map[string]bool{}
	fields := reflect.VisibleFields(reflect.TypeOf(*tags))
	for _, field := range fields {
		fieldName := field.Name
		if fieldName == "Custom" || fieldName == "TrackTotal" || fieldName == "DiskTotal" {
			continue
		}

		if len(ilst) == 0 {
			atoms[fieldName] = true
		}

		if fieldName == "TrackNumber" {
			fieldName = "Track"
		} else if fieldName == "DiskNumber" {
			fieldName = "Disk"
		}
		boxType, ok := atomsMap[fieldName]
		if !ok {
			continue
		}
		boxes, err := mp4.ExtractBox(
			f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst(), boxType})
		if err != nil {
			return nil, err
		}
		atoms[fieldName] = len(boxes) == 0
	}
	if len(ilst) == 0 {
		return atoms, errors.New("no ilst atom")
	}
	return atoms, nil
}

func marshalData(w *mp4.Writer, ctx mp4.Context, val interface{}) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeData()})
	if err != nil {
		return err
	}
	var boxData mp4.Data
	switch v := val.(type) {
	case string:
		boxData.DataType = mp4.DataTypeStringUTF8
		boxData.Data = []byte(v)
	case []byte:
		boxData.DataType = mp4.DataTypeBinary
		boxData.Data = v
	}
	_, err = mp4.Marshal(w, &boxData, ctx)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

func writeMeta(w *mp4.Writer, tag mp4.BoxType, ctx mp4.Context, val interface{}) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: tag})
	if err != nil {
		return err
	}
	err = marshalData(w, ctx, val)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

func writeCovers(w *mp4.Writer, ctx mp4.Context, coversData [][]byte) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: atomsMap["Cover"]})
	if err != nil {
		return err
	}
	for _, coverData := range coversData {
		err = marshalData(w, ctx, coverData)
		if err != nil {
			return err
		}
	}
	_, err = w.EndBox()
	return err
}

func writeCustomMeta(w *mp4.Writer, ctx mp4.Context, field string, val interface{}) error {
	_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxType{'-', '-', '-', '-'}, Context: ctx})
	if err != nil {
		return err
	}
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxType{'m', 'e', 'a', 'n'}, Context: ctx})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'\x00', '\x00', '\x00', '\x00'})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "com.apple.iTunes")
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	if err != nil {
		return err
	}
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxType{'n', 'a', 'm', 'e'}, Context: ctx})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'\x00', '\x00', '\x00', '\x00'})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, field)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	if err != nil {
		return err
	}
	err = marshalData(w, ctx, val)
	if err != nil {
		return err
	}
	_, err = w.EndBox()
	return err
}

// func writeCover(h *mp4.ReadHandle, w *mp4.Writer, ctx mp4.Context, coverData []byte) error {
// 	_, err := w.StartBox(&h.BoxInfo)
// 	if err != nil {
// 		return err
// 	}
// 	box, _, err := h.ReadPayload()
// 	if err != nil {
// 		return err
// 	}
// 	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
// 	if err != nil {
// 		return err
// 	}
// 	err = writeMeta(w, h.BoxInfo.Type, ctx, coverData)
// 	if err != nil {
// 		return err
// 	}
// 	_, err = w.EndBox()
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// Make new atoms and write to.
func createAndWrite(h *mp4.ReadHandle, w *mp4.Writer, ctx mp4.Context, tags *Tags, atoms map[string]bool) error {
	_, err := w.StartBox(&h.BoxInfo)
	if err != nil {
		return err
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
	if err != nil {
		return err
	}
	if len(tags.CoversData) > 0 {
		err := writeCovers(w, ctx, tags.CoversData)
		if err != nil {
			return err
		}
	}
	if tags.TrackNumber > 0 {
		trkn := make([]byte, 8)
		binary.BigEndian.PutUint32(trkn, uint32(tags.TrackNumber))
		if tags.TrackTotal > 0 {
			binary.BigEndian.PutUint16(trkn[4:], uint16(tags.TrackTotal))
		}
		err = writeMeta(w, atomsMap["Track"], ctx, trkn)
		if err != nil {
			return err
		}
	}
	if tags.ContentRating > 0 {
		err = writeMeta(w, atomsMap["ContentRating"], ctx, []byte{byte(tags.ContentRating)})
		if err != nil {
			return err
		}
	}
	if tags.DiskNumber > 0 {
		disk := make([]byte, 8)
		binary.BigEndian.PutUint32(disk, uint32(tags.DiskNumber))
		if tags.DiskTotal > 0 {
			binary.BigEndian.PutUint16(disk[4:], uint16(tags.DiskTotal))
		}
		err = writeMeta(w, atomsMap["Disk"], ctx, disk)
		if err != nil {
			return err
		}
	}
	for tagName, needCreate := range atoms {
		if tagName == "Cover" || tagName == "Track" || tagName == "Disk" || tagName == "ContentRating" {
			continue
		}
		var val string

		switch tagName {
		case "Year":
			val = tags.yearStr
		default:
			val = reflect.ValueOf(*tags).FieldByName(tagName).String()
		}
		if !needCreate || val == "" {
			continue
		}
		boxType := atomsMap[tagName]
		err = writeMeta(w, boxType, ctx, val)
		if err != nil {
			return err
		}
	}
	for field, value := range tags.Custom {
		if value == "" {
			continue
		}
		err = writeCustomMeta(w, ctx, field, strings.ToUpper(value))
		if err != nil {
			return err
		}
	}

	_, err = h.Expand()
	if err != nil {
		// If you write no atoms, you should not expand the box.
		// Removed return statement to finish writing the file.
		fmt.Println("could not expand box:", err)
	}
	_, err = w.EndBox()
	return err
}

func writeExisting(h *mp4.ReadHandle, w *mp4.Writer, tags *Tags, currentKey string, ctx mp4.Context) (bool, error) {
	if currentKey == "Cover" && len(tags.CoversData) == 0 {
		return true, nil
	}
	if currentKey == "Cover" && len(tags.CoversData) > 0 {
		// err := writeCover(h, w, ctx, _tags.Cover)
		// if err != nil {
		// 	return false, nil
		// }
		//err := writeMeta(w, atomsMap["Cover"], ctx, _tags.Cover)
		_, err := w.StartBox(&h.BoxInfo)
		if err != nil {
			return false, err
		}
		box, _, err := h.ReadPayload()
		if err != nil {
			return false, err
		}
		data := box.(*mp4.Data)
		data.DataType = mp4.DataTypeBinary
		data.Data = tags.CoversData[0]
		_, err = mp4.Marshal(w, data, h.BoxInfo.Context)
		if err != nil {
			return false, err
		}
		_, err = w.EndBox()
		if err != nil {
			return false, err
		}
		// err := writeMeta(w, h.BoxInfo.Type, ctx, _tags.Cover)
		// if err != nil {
		// 	return false, err
		// }
	} else if currentKey == "Disk" {
		if tags.DiskNumber < 1 {
			return true, nil
		}
		// disk := make([]byte, 8)
		// binary.BigEndian.PutUint32(disk, uint32(_tags.DiskNumber))
		// if _tags.DiskTotal > 0 {
		// 	binary.BigEndian.PutUint16(disk[4:], uint16(_tags.DiskTotal))
		// }
		// err := writeMeta(w, h.BoxInfo.Type, ctx, disk)
		// if err != nil {
		// 	return false, err
		// }
	} else if currentKey == "Track" {
		if tags.TrackNumber < 1 {
			return true, nil
		}
		// trkn := make([]byte, 8)
		// binary.BigEndian.PutUint32(trkn, uint32(_tags.TrackNumber))
		// if _tags.TrackTotal > 0 {
		// 	binary.BigEndian.PutUint16(trkn[4:], uint16(_tags.TrackTotal))
		// }
		// // err := writeMeta(w, h.BoxInfo.Type, ctx, trkn)
	} else if currentKey == "ContentRating" {
		if !(tags.ContentRating >= 0 && tags.ContentRating <= 2) {
			return true, nil
		}
	} else {
		var toWrite string

		switch currentKey {
		case "Year":
			toWrite = tags.yearStr
		default:
			toWrite = reflect.ValueOf(*tags).FieldByName(currentKey).String()
		}

		if toWrite == "" {
			return true, nil
		}
		//fmt.Println(toWrite)
		// Not working here.
		// err := writeMeta(w, h.BoxInfo.Type, ctx, toWrite)
		// if err != nil {
		// 	return false, err
		// }
		_, err := w.StartBox(&h.BoxInfo)
		if err != nil {
			return false, err
		}
		box, _, err := h.ReadPayload()
		if err != nil {
			return false, err
		}
		data := box.(*mp4.Data)
		data.DataType = mp4.DataTypeStringUTF8
		data.Data = []byte(toWrite)
		_, err = mp4.Marshal(w, data, h.BoxInfo.Context)
		if err != nil {
			return false, err
		}
		_, err = w.EndBox()
		return false, err

	}
	return false, nil
}

func containsAtom(boxType mp4.BoxType, boxes []mp4.BoxType) mp4.BoxType {
	for _, _boxType := range boxes {
		if boxType == _boxType {
			return boxType
		}
	}
	return mp4.BoxType{}
}

func containsTag(delete []string, currentTag string) bool {
	for _, tag := range delete {
		if strings.EqualFold(tag, currentTag) {
			return true
		}
	}
	return false
}

func getTag(boxType mp4.BoxType) string {
	for k, v := range atomsMap {
		if v == boxType {
			return k
		}
	}
	return ""
}

func getAtomsList() []mp4.BoxType {
	var atomsList []mp4.BoxType
	for _, atom := range atomsMap {
		atomsList = append(atomsList, atom)
	}
	return atomsList
}

func copyTrack(srcPath, destPath string) error {
	srcFile, err := os.OpenFile(srcPath, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = io.Copy(outFile, srcFile)
	return err
}

func (mp4File *MP4File) actualWrite(tags *Tags) error {
	var currentKey string
	ctx := mp4.Context{UnderIlstMeta: true}
	tempPath, err := os.MkdirTemp(os.TempDir(), "go-mp4tag")
	if err != nil {
		return errors.New(
			"failed to make temp directory\n" + err.Error())
	}
	defer os.RemoveAll(tempPath)
	tempPath = filepath.Join(tempPath, "tmp.m4a")
	atomsList := getAtomsList()
	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	r := bufseekio.NewReadSeeker(mp4File.f, 128*1024, 4)
	var createIlst bool
	atoms, err := populateAtoms(mp4File.f, tags)
	if err != nil {
		if err.Error() != "no ilst atom" {
			tempFile.Close()
			return err
		} else {
			createIlst = true
		}
	}
	w := mp4.NewWriter(tempFile)
	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta():
			if !(h.BoxInfo.Type == mp4.BoxTypeMoov() && createIlst) {
				err := copy(w, h)
				return nil, err
			}
			err = createIlstAndWrite(w, h, tags, atoms)
			if err != nil {
				return nil, err
			}
		case mp4.BoxTypeIlst():
			// In case the ilst atom had to be created, we won't encounter the ilst while looping anymore
			// Hence, we don't check if ilst already exists here.
			err := createAndWrite(h, w, ctx, tags, atoms)
			return nil, err
		case containsAtom(h.BoxInfo.Type, atomsList):
			if h.BoxInfo.Type == atomsMap["Cover"] && len(tags.CoversData) > 0 {
				return nil, nil
			}
			currentKey = getTag(h.BoxInfo.Type)
			if containsTag(tags.Delete, currentKey) {
				return nil, nil
			}
			err = copy(w, h)
			return nil, err
		case mp4.BoxTypeData():
			if currentKey == "" {
				return nil, w.CopyBox(r, &h.BoxInfo)
			}
			needCreate := atoms[currentKey]
			if !needCreate {
				valEmpty, err := writeExisting(h, w, tags, currentKey, ctx)
				currentKey = ""
				if err != nil {
					return nil, err
				} else if valEmpty {
					return nil, w.CopyBox(r, &h.BoxInfo)
				}
			}
			return nil, nil
		default:
			return nil, w.CopyBox(r, &h.BoxInfo)
		}
		return nil, nil
	})
	tempFile.Close()
	if err != nil {
		return err
	}
	err = copyTrack(tempPath, mp4File.trackPath)
	return err
}

func createIlstAndWrite(w *mp4.Writer, h *mp4.ReadHandle, tags *Tags, atoms map[string]bool) error {
	//fmt.Println("Creating ilst")
	_, err := w.StartBox(&h.BoxInfo)
	if err != nil {
		return err
	}
	// read payload
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}

	// expand children
	_, err = h.Expand()
	if err != nil {
		return err
	}

	// marshal existing payload (so we append ilst after trak/mvhd)
	_, err = mp4.Marshal(w, box, h.BoxInfo.Context)
	if err != nil {
		return err
	}

	// Create tree moov > udta > meta > hdlr, ilst > tags
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeUdta(), Context: h.BoxInfo.Context})
	if err != nil {
		return err
	}

	ctx := mp4.Context{UnderUdta: true}

	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMeta(), Context: ctx})
	if err != nil {
		return err
	}

	// Padding
	_, err = w.Write(bytes.Repeat([]byte{0x00}, 4))
	if err != nil {
		return err
	}

	// start handler box
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeHdlr(), Context: ctx})

	mdirBuffer := bytes.Buffer{}
	mdirBuffer.Write(bytes.Repeat([]byte{0x00}, 8))
	mdirBuffer.WriteString("mdirappl")
	mdirBuffer.Write(bytes.Repeat([]byte{0x00}, 10))
	_, err = w.Write(mdirBuffer.Bytes())
	if err != nil {
		return err
	}

	//Close handler box
	_, err = w.EndBox()
	if err != nil {
		return err
	}

	// Start tag atom
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeIlst()})
	if err != nil {
		return err
	}

	ctx = mp4.Context{UnderIlstMeta: true}

	if len(tags.CoversData) > 0 {
		err := writeCovers(w, ctx, tags.CoversData)
		if err != nil {
			return err
		}
	}
	if tags.TrackNumber > 0 {
		trkn := make([]byte, 8)
		binary.BigEndian.PutUint32(trkn, uint32(tags.TrackNumber))
		if tags.TrackTotal > 0 {
			binary.BigEndian.PutUint16(trkn[4:], uint16(tags.TrackTotal))
		}
		err = writeMeta(w, atomsMap["Track"], ctx, trkn)
		if err != nil {
			return err
		}
	}
	if tags.ContentRating > 0 {
		err = writeMeta(w, atomsMap["ContentRating"], ctx, []byte{byte(tags.ContentRating)})
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
	}
	if tags.DiskNumber > 0 {
		disk := make([]byte, 8)
		binary.BigEndian.PutUint32(disk, uint32(tags.DiskNumber))
		if tags.DiskTotal > 0 {
			binary.BigEndian.PutUint16(disk[4:], uint16(tags.DiskTotal))
		}
		err = writeMeta(w, atomsMap["Disk"], ctx, disk)
		if err != nil {
			return err
		}
	}
	for tagName, needCreate := range atoms {
		if tagName == "Cover" || tagName == "Track" || tagName == "Disk" || tagName == "ContentRating" {
			continue
		}
		var val string

		switch tagName {
		case "Year":
			val = tags.yearStr
		default:
			val = reflect.ValueOf(*tags).FieldByName(tagName).String()
		}
		if !needCreate || val == "" {
			continue
		}
		boxType := atomsMap[tagName]
		err = writeMeta(w, boxType, ctx, val)
		if err != nil {
			return err
		}
	}
	for field, value := range tags.Custom {
		if value == "" {
			continue
		}
		err = writeCustomMeta(w, ctx, field, strings.ToUpper(value))
		if err != nil {
			return err
		}
	}

	// rewrite ilst size
	_, err = w.EndBox()
	if err != nil {
		return err
	}

	//rewrite meta box size
	_, err = w.EndBox()
	if err != nil {
		return err
	}

	// rewrite udta box size
	_, err = w.EndBox()
	if err != nil {
		return err
	}

	// rewrite moov box size
	_, err = w.EndBox()
	if err != nil {
		return err
	}

	// create free box
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeFree()})

	// Since header takes 8 bytes, make buffer 2040 giving the atom a total length of 2048 bytes
	freeBuffer := bytes.Buffer{}
	freeBuffer.Write(bytes.Repeat([]byte{0x00}, 2040))
	_, err = w.Write(freeBuffer.Bytes())
	if err != nil {
		return err
	}

	// Rewrite free box size
	_, err = w.EndBox()
	if err != nil {
		return err
	}

	return nil
}

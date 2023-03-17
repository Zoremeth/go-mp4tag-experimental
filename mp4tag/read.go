package mp4tag

import (
	"encoding/binary"
	"errors"
	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
	"reflect"
	"strconv"
)

var boxesList = []mp4.BoxType{
	{'\251', 'a', 'l', 'b'},
	{'t', 'r', 'k', 'n'},
	{'\251', 'A', 'R', 'T'},
	{'a', 'A', 'R', 'T'},
	{'c', 'p', 'r', 't'},
	{'\251', 'n', 'a', 'm'},
	{'\251', 'd', 'a', 'y'},
	{'\251', 'g', 'e', 'n'},
	{'\251', 'w', 'r', 't'},
	{'c', 'o', 'v', 'r'},
	{'\251', 'l', 'y', 'r'},
	{'r', 't', 'n', 'g'},
	{'\251', 'l', 'a', 'b'},
}

var boxesMap = map[mp4.BoxType]string{
	{'\251', 'a', 'l', 'b'}: "Album",
	{'t', 'r', 'k', 'n'}:    "TrackNumber",
	{'\251', 'A', 'R', 'T'}: "Artist",
	{'a', 'A', 'R', 'T'}:    "AlbumArtist",
	{'c', 'p', 'r', 't'}:    "Copyright",
	{'\251', 'n', 'a', 'm'}: "Title",
	{'\251', 'd', 'a', 'y'}: "Year",
	{'\251', 'g', 'e', 'n'}: "Genre",
	{'\251', 'w', 'r', 't'}: "Composer",
	{'c', 'o', 'v', 'r'}:    "Cover",
	{'\251', 'l', 'y', 'r'}: "UnsyncedLyrics",
	{'\251', 'l', 'a', 'b'}: "Label",
	{'r', 't', 'n', 'g'}:    "ContentRating",
}

func readValue(h *mp4.ReadHandle, customField bool) (string, error) {
	startIdx := 1
	if customField {
		startIdx += 4
	}
	box, _, err := h.ReadPayload()
	if err != nil {
		return "", err
	}
	value, ok := box.StringifyField("Data", "", 0, h.BoxInfo.Context)
	if !ok {
		return "", errors.New("Failed to stringify value.")
	}
	return value[startIdx : len(value)-1], nil
}

func readTrack(h *mp4.ReadHandle) (int, int, error) {
	box, _, err := h.ReadPayload()
	if err != nil {
		return -1, -1, err
	}
	data := box.(*mp4.Data).Data
	trackNumber := binary.BigEndian.Uint32(data[:4])
	trackTotal := binary.BigEndian.Uint16(data[4:])
	return int(trackNumber), int(trackTotal), nil
}

func setCover(parsedTags *Tags, h *mp4.ReadHandle) error {
	box, _, err := h.ReadPayload()
	if err != nil {
		return err
	}
	data := box.(*mp4.Data).Data
	parsedTags.CoversData = append(parsedTags.CoversData, data)
	return nil
}

func readInt(h *mp4.ReadHandle) (int, error) {
	valStr, err := readValue(h, false)
	if err != nil {
		return -1, nil
	}
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return -1, err
	}
	return valInt, nil
}

func setValue(parsedTags *Tags, h *mp4.ReadHandle, currentKey string, isCustom bool) error {
	currentValue, err := readValue(h, false)
	if err != nil {
		return err
	}
	if isCustom {
		parsedTags.Custom[currentKey] = currentValue
	} else {
		reflect.ValueOf(parsedTags).Elem().FieldByName(currentKey).Set(reflect.ValueOf(currentValue))
	}
	return nil
}

func setTrack(parsedTags *Tags, h *mp4.ReadHandle) error {
	trackNum, trackTotal, err := readTrack(h)
	if err != nil {
		return err
	}
	parsedTags.TrackNumber = trackNum
	parsedTags.TrackTotal = trackTotal
	return nil
}

func setContentRating(parsedTags *Tags, h *mp4.ReadHandle) error {
	advisory, err := readInt(h)
	if err != nil {
		return err
	}

	if !(advisory > 0 && advisory <= 2) {
		return errors.New("invalid value for content rating:" + strconv.Itoa(advisory))
	}

	// Content Rating
	// 0 = Unrated
	// 1 = Explicit
	// 2 = Clean

	parsedTags.ContentRating = advisory
	return nil
}

func setYear(parsedTags *Tags, h *mp4.ReadHandle) error {
	year, err := readInt(h)
	if err != nil {
		return err
	}

	parsedTags.Year = year
	return nil
}

func contains(boxType mp4.BoxType) mp4.BoxType {
	for _, _boxType := range boxesList {
		if boxType == _boxType {
			return boxType
		}
	}
	return mp4.BoxType{}
}

func (mp4File *MP4File) actualRead() (*Tags, error) {
	var (
		err        error
		currentKey string
		isCustom   bool
	)
	parsedTags := &Tags{Custom: map[string]string{}}
	r := bufseekio.NewReadSeeker(mp4File.f, 128*1024, 4)
	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		boxType := h.BoxInfo.Type
		switch boxType {
		case mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.BoxTypeMeta(), mp4.BoxTypeIlst():
			_, err := h.Expand()
			return nil, err
		case contains(boxType):
			currentKey, _ = boxesMap[boxType]
			_, err := h.Expand()
			if err != nil {
				return nil, err
			}
			return nil, nil
		case mp4.StrToBoxType("----"):
			_, err := h.Expand()
			return nil, err
		case mp4.StrToBoxType("name"):
			currentKey, err = readValue(h, true)
			if err != nil {
				return nil, err
			}
			isCustom = true
			_, err = h.Expand()
			return nil, err
		case mp4.BoxTypeData():
			switch currentKey {
			case "TrackNumber":
				err = setTrack(parsedTags, h)
				if err != nil {
					return nil, err
				}
			case "ContentRating":
				err = setContentRating(parsedTags, h)
				if err != nil {
					return nil, err
				}
			case "Year":
				err = setYear(parsedTags, h)
				if err != nil {
					return nil, err
				}
			case "Cover":
				err = setCover(parsedTags, h)
				if err != nil {
					return nil, err
				}
			default:
				err := setValue(parsedTags, h, currentKey, isCustom)
				if err != nil {
					return nil, err
				}
				isCustom = false
				if err != nil {
					return nil, err
				}
			}
			return nil, nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	return parsedTags, nil
}

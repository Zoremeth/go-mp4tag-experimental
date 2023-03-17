package mp4tag

import (
	"os"
	"strconv"
	"strings"
)

func (mp4File *MP4File) Close() error {
	//fmt.Println("close")
	return mp4File.f.Close()
}

func (mp4File *MP4File) Read() (*Tags, error) {
	return mp4File.actualRead()
}

func (mp4File *MP4File) Write(tags *Tags) error {
	if tags.ContentRatingStr != "" {
		switch strings.ToLower(tags.ContentRatingStr) {
		case "explicit":
			tags.ContentRating = 1
		case "clean":
			tags.ContentRating = 2
		}
	}
	if tags.Year > 0 {
		tags.yearStr = strconv.Itoa(tags.Year)
	}
	return mp4File.actualWrite(tags)
}

func Open(trackPath string) (*MP4File, error) {
	//var currentKey string
	// tempPath, err := os.MkdirTemp(os.TempDir(), "go-mp4tag")
	// if err != nil {
	// 	return errors.New(
	// 		"Failed to make temp directory.\n" + err.Error())
	// }
	// defer os.RemoveAll(tempPath)
	// tempPath = filepath.Join(tempPath, "tmp.m4a")
	outFile, err := os.OpenFile(trackPath, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}
	// tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	// if err != nil {
	// 	outFile.Close()
	// 	return err
	// }
	mp4File := &MP4File{
		f:         outFile,
		trackPath: trackPath,
	}
	return mp4File, nil
}
